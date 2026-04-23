# Picoclaw Session + Message Hybrid Design

## Context

The current arena server can drive a `picoclaw` seat only by calling the remote `/message` endpoint when it is that seat's turn. This is enough to request a move, but it does not provide a long-lived connection model, an online/offline signal, or a recovery path that survives temporary transport instability.

The user wants a more realistic runtime model:

- keep a long-lived arena-side session for each connected `picoclaw`
- preserve the current `/message` path
- allow automatic and manual switching between the two turn-delivery modes
- keep switching state isolated per `picoclaw`, not per room or per service
- keep invitation semantics independent from whichever move-delivery mode is active
- continue to use `/message` for invitation today
- reserve an `invite` endpoint in the protocol documentation for future implementation

The design must fit the current single-process Go arena server and must not reintroduce the removed `http_session` / `websocket` transport stack.

## Goal

Add a per-picoclaw hybrid transport model with:

- a long-lived arena-managed session lifecycle
- continued `/message` compatibility
- per-picoclaw preferred mode, active mode, and health state
- per-turn fallback between session mode and `/message`
- manual mode switching by the host
- automatic mode switching by the server
- `/message`-based invitation that works regardless of the active move mode
- protocol documentation for a future `invite` endpoint without depending on it now

## Non-Goals

This design does not include:

- a browser-facing persistent connection model
- service-wide transport switching
- room-wide transport switching
- a true remote push channel such as WebSocket
- implementing the future `invite` endpoint now
- supporting non-human, non-picoclaw agents

## Scope Boundary

This feature applies only to managed `picoclaw` participants. Human seats continue to use the existing browser move flow and have no session keepalive state.

The unit of isolation is one managed participant. If red and black are both `picoclaw`, they each own separate session state, fallback counters, lease timers, and mode selection.

## High-Level Model

Each managed `picoclaw` has two concurrent capabilities:

1. A long-lived arena-side session
   This establishes whether the remote participant is considered online, recently alive, and eligible for session-mode move delivery.

2. A direct `/message` path
   This remains available for invitation, fallback turn delivery, and compatibility when session-mode move delivery is not healthy.

The server does not treat these as mutually exclusive transports. Session mode and `/message` mode can coexist. One is preferred for the next move, while the other remains available as a same-turn recovery path.

## Terminology

- `preferred_mode`
  The operator-selected or server-selected steady-state preference for a specific `picoclaw`.
- `active_mode`
  The mode that should be attempted first for the next move.
- `session_mode`
  Move delivery that relies on an arena-managed long-lived session state.
- `message_mode`
  Direct request-response turn delivery using `POST {base_url}/message`.
- `recovery_fallback`
  A same-turn attempt using the alternate mode after the primary attempt fails.

## Architecture

### Picoclaw Runtime State

Add per-participant runtime state stored in the room snapshot for managed `picoclaw` seats.

Recommended fields:

- `participant_id`
- `preferred_mode`
  - enum: `auto`, `prefer_session`, `prefer_message`
- `active_mode`
  - enum: `session`, `message`
- `session_id`
- `session_state`
  - enum: `idle`, `opening`, `active`, `stale`, `recovering`, `closed`
- `session_opened_at`
- `last_heartbeat_at`
- `lease_expires_at`
- `recovery_deadline_at`
- `consecutive_session_failures`
- `consecutive_message_failures`
- `last_mode_switch_at`
- `last_switch_reason`
- `last_invite_at`
- `last_invite_status`

These fields are attached to the managed participant identity, not the match. They survive across multiple matches in the same room unless the participant is removed from the seat.

### Match Interaction

Matches remain lightweight. A match reads the current state of the managed participant occupying the acting seat and asks a mode resolver which path to try first.

The match does not own the long-lived session. It only consumes it.

This separation allows:

- one session to span multiple matches
- seat replacement without corrupting older match data
- independent runtime state for red and black managed participants

## Session Lifecycle

### Open

When a managed `picoclaw` is first invited or bound into a seat, the arena may create session state locally and attempt to open the remote relationship.

Arena-side API:

- `POST /api/arena/{code}/picoclaw/{participant_id}/session/open`

This endpoint is for local operator flow and internal orchestration. It creates or refreshes local session state and records the participant as session-capable.

### Heartbeat

The remote `picoclaw` periodically renews its lease.

Arena-side API:

- `POST /api/arena/{code}/picoclaw/{participant_id}/session/heartbeat`

Expected effect:

- update `last_heartbeat_at`
- extend `lease_expires_at`
- clear stale / recovering state if appropriate

The recommended initial lease is 45 seconds with a heartbeat cadence around 10 to 15 seconds.

### Close

The session may be closed by:

- explicit participant removal
- host seat replacement
- room reset that removes the managed participant
- operator action

Arena-side API:

- `POST /api/arena/{code}/picoclaw/{participant_id}/session/close`

Closing marks the session closed but does not erase historical diagnostics immediately.

## Invitation Model

Invitation remains `/message`-driven in the current implementation.

When the system invites a `picoclaw` to participate, it sends a structured invitation message to:

- `POST {base_url}/message`

The invitation message should include:

- room code
- target seat
- public alias
- expected arena base URL
- local `participant_id`
- whether session keepalive is enabled
- the arena-side session endpoints it may call

The current design intentionally keeps invitation independent from move mode. Even if a participant is in `prefer_session`, `prefer_message`, or `auto`, invitation still uses `/message`.

### Reserved Future Endpoint

Reserve the following remote endpoint in the documentation only:

- `POST {base_url}/invite`

Current status:

- not required
- not called by the arena
- documented as a future compatibility target

This gives future developers a clean semantic separation path without blocking current integration.

## Move Delivery Modes

### Session Mode

Session mode is the preferred path when all of the following are true:

- the participant preference resolves to session-first
- the participant has an `active` session
- `lease_expires_at` is still in the future

Session mode should use an arena-side turn endpoint tied to the long-lived participant session.

Arena-side API shape:

- `POST /api/arena/{code}/picoclaw/{participant_id}/turn`

The remote `picoclaw` may either:

- poll this endpoint pattern as part of its long-lived session workflow, or
- use the session relationship to acknowledge and reply to outstanding turns through agreed payloads

The important design point is that session mode is modeled as a long-lived relationship with per-turn requests, not as a brand-new transport family.

### Message Mode

Message mode continues to use:

- `POST {base_url}/message`

The payload remains prompt-oriented and backward compatible with the current implementation.

Message mode is valid for:

- direct steady-state operation when the operator prefers it
- same-turn recovery fallback
- invitation

## Mode Resolution

Each managed participant supports:

- `auto`
- `prefer_session`
- `prefer_message`

Resolution rules:

- `auto`
  - use `session` first when session health is good
  - otherwise use `message`
- `prefer_session`
  - use `session` first whenever the lease is valid
  - if session is stale or unavailable, use `message`
- `prefer_message`
  - use `message` first
  - keep session heartbeat alive in the background if session support exists

The server writes the resolved first choice into `active_mode` before attempting the move.

## Automatic Fallback and Switching

The approved strategy is same-turn dual-path recovery.

### Same-Turn Recovery

For each managed turn:

1. Resolve the primary mode from participant state.
2. Attempt the move through that mode.
3. If it succeeds, apply the move and update health counters.
4. If it fails, immediately try the alternate mode once in the same turn.
5. If fallback succeeds, apply the move, record the primary failure, and keep the match running.
6. If both attempts fail, pause the match and record both errors.

### Automatic Switching Rules

The server may update `active_mode` and, in some cases, `preferred_mode` based on observed health.

Recommended initial rules:

- if `session` fails but same-turn `/message` fallback succeeds
  - set `active_mode = message`
  - keep `preferred_mode` unchanged when it is `auto`
  - keep `preferred_mode = prefer_session` if the operator explicitly chose it
  - mark session state as `recovering` or `stale`
- if `/message` fails but same-turn session fallback succeeds
  - set `active_mode = session`
- if one mode succeeds repeatedly while the other remains unhealthy
  - the server may continue to prefer the healthy mode for future turns
- the server must not silently overwrite a host-selected `prefer_session` or `prefer_message` setting
  - it may only adjust `active_mode`
  - the host preference remains the declared preference

### Recovery Window

The user did not want a pure heartbeat-based delayed pause disconnected from actual game flow. Therefore:

- heartbeat loss alone should not immediately pause the match
- heartbeat loss should mark the session degraded
- the actual pause happens only when the current turn cannot be completed through either mode

This matches realistic operation better than pausing solely because a heartbeat was late.

## Manual Switching

Add a host-controlled endpoint:

- `POST /api/arena/{code}/picoclaw/{participant_id}/mode`

Request values:

- `auto`
- `prefer_session`
- `prefer_message`

Effects:

- updates `preferred_mode`
- recalculates `active_mode`
- records `last_switch_reason = host_override`

If a match is already in progress:

- the new preference applies to the next unresolved turn for that participant
- the current turn, if already being processed, keeps its in-flight behavior

## Error Handling

### Session Health Errors

Examples:

- heartbeat timeout
- lease expired
- malformed session turn reply
- participant session not open

Handling:

- mark session stale
- allow message fallback on the next turn
- log host-visible diagnostics

### Message Errors

Examples:

- HTTP failure
- non-JSON body
- JSON without a legal move
- explicit remote error

Handling:

- increment message failure counters
- try session fallback once if available
- pause the match if both paths fail

### Match Pauses

A managed turn pauses only when:

- primary mode fails
- alternate mode fallback also fails

The match log should show:

- primary mode attempted
- primary failure summary
- fallback mode attempted
- fallback failure summary, if any
- resulting mode switch, if any

## Data Flow

### Invite Flow

1. Host binds or invites a `picoclaw`.
2. Arena sends `/message` invitation.
3. Arena creates local session runtime state.
4. Remote `picoclaw` begins heartbeat calls if it supports session keepalive.

### Turn Flow

1. Match scheduler reaches a managed participant turn.
2. Arena resolves that participant's primary mode.
3. Arena attempts the primary mode.
4. On failure, arena attempts the alternate mode once.
5. Arena applies the move if either attempt succeeds.
6. Arena updates runtime state and logs.

### Reconnect Flow

1. Heartbeat stops arriving.
2. Arena marks the session stale.
3. Future turns use message-first fallback behavior until heartbeats resume.
4. When heartbeat resumes, arena restores session health.
5. Future turns may return to session-first if preference permits.

## Persistence

Participant runtime state must be included in arena snapshots so that a process restart does not erase:

- preferred mode
- last healthy mode
- recent lease timestamps
- recent failure counts

On restore:

- expired leases should be treated as stale
- active matches should not automatically resume a stale in-memory assumption

## API Summary

Arena-local APIs:

- `POST /api/arena/{code}/picoclaw/{participant_id}/session/open`
- `POST /api/arena/{code}/picoclaw/{participant_id}/session/heartbeat`
- `POST /api/arena/{code}/picoclaw/{participant_id}/session/close`
- `POST /api/arena/{code}/picoclaw/{participant_id}/mode`
- `POST /api/arena/{code}/picoclaw/{participant_id}/turn`

Remote picoclaw APIs in current scope:

- `POST {base_url}/message`

Remote picoclaw API reserved for future documentation only:

- `POST {base_url}/invite`

## Compatibility

Backward compatibility rules:

- existing `base_url` configuration continues to work
- existing `/message`-only picoclaw deployments continue to work
- if a picoclaw never calls session heartbeat APIs, it can still participate through message mode
- the new session workflow is additive, not a breaking requirement

## Testing Strategy

Add tests for:

- per-participant mode isolation when both red and black are managed
- invitation via `/message`
- session heartbeat refreshing lease state
- same-turn fallback from `session` to `message`
- same-turn fallback from `message` to `session`
- host manual mode switching
- persistence and restore of participant runtime state
- pausing only after both modes fail for the same turn
- documenting but not invoking the reserved `/invite` endpoint

## Open Implementation Notes

Implementation should keep the current codebase boundaries clean:

- participant runtime state belongs near room / participant management
- turn resolution belongs in arena match advancement
- protocol helpers for `/message` and future invite formatting belong in the picoclaw integration file
- host APIs should expose enough state for diagnostics without flooding the public room view
