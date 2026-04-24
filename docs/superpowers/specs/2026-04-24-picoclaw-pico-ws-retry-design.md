# Picoclaw Pico WS + Message Retry Design

## Context

The current arena supports managed `picoclaw` seats through two paths:

- arena-local long-lived `session` turn polling
- remote `POST {base_url}/message`

This is enough for a hybrid runtime, but the user now wants two additional guarantees:

1. `message` mode should tolerate transient HTTP failures through bounded retries.
2. the arena should be able to use the real PicoClaw native WebSocket channel exposed by the target PicoClaw fork at `/pico/ws`.

The user explicitly plans to use `https://github.com/Lan4jn/picoclaw`, so this design must match that fork rather than generic upstream assumptions.

## Goal

Extend managed `picoclaw` runtime with:

- a new independent `pico_ws` move-delivery mode
- bounded retry logic for `message` mode
- participant-level runtime state and fallback behavior that can choose among:
  - `pico_ws`
  - arena-local `session`
  - remote `/message`
- no dependence on launcher-only PicoClaw proxy semantics

## Non-Goals

This design does not include:

- replacing the existing arena-local `session` flow
- removing `/message`
- media transfer through Pico WS
- message deletion support
- launcher dashboard token orchestration
- browser-to-arena WebSocket changes

## External Compatibility Assumption

The target PicoClaw fork keeps the native gateway WebSocket endpoint at:

- `ws://host:port/pico/ws`

Supported authentication remains compatible with PicoClaw's direct gateway channel:

- `Authorization: Bearer <token>`
- `Sec-WebSocket-Protocol: token.<token>`
- `?token=<token>` only if explicitly allowed by PicoClaw configuration

The arena should treat launcher-proxied `/pico/ws` as out of scope for now. The supported integration target is the direct gateway endpoint.

## High-Level Model

Each managed `picoclaw` participant may expose up to three usable turn-delivery capabilities:

1. `pico_ws`
   Arena-managed outbound WebSocket client connected to PicoClaw's native `/pico/ws`.
2. `session`
   Arena-local long-lived session endpoints already implemented in the current project.
3. `message`
   Direct request-response `POST {base_url}/message`.

Invitation remains `/message`-driven. Runtime move delivery is independent from invitation semantics.

## Terminology

- `preferred_mode`
  Operator-selected steady-state preference.
- `active_mode`
  First mode attempted for the next managed turn.
- `fallback`
  Same-turn recovery attempt through an alternate mode after the primary mode fails.
- `message retry`
  Repeated attempts within `message` mode before that mode is considered failed.

## Runtime State Changes

Extend `PicoclawPreferredMode` with:

- `prefer_pico_ws`

Extend `PicoclawActiveMode` with:

- `pico_ws`

Extend `PicoclawRuntimeState` with Pico WS diagnostics:

- `WSState`
  - enum: `idle`, `connecting`, `active`, `stale`, `recovering`, `closed`
- `WSURL`
- `WSSessionID`
- `WSAuthMode`
  - enum: `bearer`, `subprotocol`
- `WSConnectedAt`
- `WSLastRecvAt`
- `WSLastSendAt`
- `WSLastError`
- `ConsecutiveWSFailures`

These fields remain participant-scoped, not room-scoped and not service-scoped.

## Base URL Semantics

The current `Base URL` field for managed `picoclaw` remains the user-entered root service address, for example:

- `http://192.168.1.20:18790`

Resolution rules:

- `/message` requests continue to normalize this root into `POST {base_url}/message`
- `pico_ws` resolves the same root into `{ws_scheme}://{host}/pico/ws`
- the host UI and README should explicitly document that `Base URL` means the PicoClaw service root, not a launcher UI page and not a pre-expanded endpoint path

## Pico WS Session Identity

The arena should keep one long-lived Pico WS connection per managed participant while that participant is configured and the room is alive.

Recommended `session_id` query value:

- `xiangqi-{room_code}-{participant_id}`

This value is stable for the participant in the room and should not change every turn.

## Pico WS Authentication

The first implementation should support two outbound authentication strategies:

1. preferred: `Authorization: Bearer <token>`
2. fallback: `Sec-WebSocket-Protocol: token.<token>`

The arena should not depend on query-token authentication.

`APIKey` in the existing seat binding can be reused as the direct Pico WS token for this MVP. This keeps the seat form unchanged.

## Pico WS Message Contract

The arena only depends on the minimal text-chat subset of the Pico protocol.

### Outbound turn request

When it is the participant's turn, the arena sends:

- type: `message.send`
- payload:
  - `content`: the same prompt string already used for `/message`

The current move prompt remains authoritative.

### Inbound turn response

The arena listens for:

- `message.create`
- `message.update`
- `error`
- `pong`

Move extraction rule:

- read `payload.content`
- extract the first legal move that matches the existing `MOVE: a0-a1` parsing rules
- once a legal move is found, complete the turn

Ignored traffic:

- `typing.start`
- `typing.stop`
- non-move text that does not contain a legal move

## Pico WS Lifecycle

### Open

When a managed participant is configured for Pico WS use, the arena may lazily connect on first need or proactively connect after invitation. The MVP should choose lazy connect on first turn or when `preferred_mode` becomes `prefer_pico_ws`.

### Keepalive

The PicoClaw server already handles WebSocket ping/pong. The arena only needs to:

- keep the socket open
- track last receive time
- fail the mode if the socket closes or write/read operations fail

### Reconnect

If the connection is lost:

- mark `WSState=recovering`
- attempt a fast reconnect once for the same turn
- if reconnect and move request still fail, treat `pico_ws` as failed for that turn and move to fallback resolution

Background reconnects may continue between turns, but they should not block room progress.

## Message Retry Rules

`message` mode should retry up to 3 attempts total per turn.

### Retryable failures

Retry only when the failure is likely transport-transient:

- request timeout
- TCP connect/reset failure
- DNS temporary failure
- HTTP `502`
- HTTP `503`
- HTTP `504`

### Non-retryable failures

Do not retry when the remote PicoClaw answered successfully but the content is semantically unusable:

- response body is invalid JSON
- response `error` is non-empty
- reply contains no legal move
- reply contains an illegal move
- explicit HTTP `4xx`

### Logging

Each failed retry attempt should be added to match diagnostics with:

- mode
- retry attempt index
- short failure summary

The final mode error should remain visible in the existing log stream.

## Mode Resolution

Participant-level `preferred_mode` becomes:

- `auto`
- `prefer_pico_ws`
- `prefer_session`
- `prefer_message`

Resolution rules:

- `auto`
  - use `pico_ws` first when WS is healthy
  - otherwise use `session` when session is healthy
  - otherwise use `message`
- `prefer_pico_ws`
  - use `pico_ws` first
  - fallback order: `session`, then `message`
- `prefer_session`
  - use `session` first when healthy
  - fallback order: `pico_ws`, then `message`
- `prefer_message`
  - use `message` first
  - fallback order: `pico_ws`, then `session`

The fallback list should skip modes that are clearly unavailable.

## Same-Turn Fallback

The approved behavior remains same-turn recovery.

For a managed turn:

1. resolve ordered candidate modes
2. attempt the first mode
3. if that mode is `message`, consume up to 3 retry attempts before declaring the mode failed
4. if the mode still fails, try the next candidate mode once
5. continue until a move is produced or all candidates fail
6. if all candidates fail, pause the room

## Host UI Changes

The host runtime panel should expose the new mode values:

- `auto`
- `prefer_pico_ws`
- `prefer_session`
- `prefer_message`

Runtime diagnostics should show:

- current `active_mode`
- `ws_state`
- `ws_connected_at`
- `ws_last_recv_at`
- `ws_last_error`
- existing session diagnostics

## Testing

Add automated coverage for:

1. message retry success on third attempt
2. message retry stops on non-retryable response failure
3. `prefer_pico_ws` resolves to WS when healthy
4. WS primary failure falls back to session or message in the configured order
5. successful Pico WS turn flow from `message.send` to parsed legal move
6. host mode API accepts and persists `prefer_pico_ws`

## Documentation

Update README to clarify:

- `Base URL` is the PicoClaw service root, typically `http://host:18790`
- `/message` is derived automatically
- Pico WS uses the same root and derives `/pico/ws`
- launcher-proxied `/pico/ws` is not the supported arena integration target in this MVP
