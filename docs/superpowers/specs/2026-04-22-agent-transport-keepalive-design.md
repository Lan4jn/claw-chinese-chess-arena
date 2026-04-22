# Agent Transport Keepalive Hybrid Design

## Context

The current project drives agent turns through short-lived HTTP requests. When a managed seat reaches its turn, the server builds a prompt from the current match state and sends one `POST` request to the configured agent endpoint. This keeps the implementation simple, but it has two limits:

- there is no true session keepalive between the arena server and the agent
- a transient transport failure pauses the match immediately instead of attempting a transport-level recovery path

The user wants a stronger session model with keepalive support, while still preserving the Windows 7-compatible single-process Go server architecture. The user also wants two transport styles to coexist:

- HTTP session mode with lease / heartbeat / resume semantics
- WebSocket mode with long-lived connections

The user clarified that transport selection should be controlled at the whole-service level, not per seat, but that transport mode changes must not disrupt matches already in progress. Existing matches should remain on the mode they started with, while future matches should use the new service default. If a match starts in WebSocket mode and the agent cannot establish or keep the connection, the system may automatically degrade that match to HTTP session mode.

## Goal

Add a transport abstraction that supports:

- a service-level default transport mode
- live switching of the service default at runtime
- match-level transport mode lock-in at match start
- HTTP session keepalive semantics
- WebSocket long-connection semantics
- automatic one-way degradation from WebSocket to HTTP session within a running match
- consistent turn request / response semantics across both transports

## Non-Goals

This design does not include:

- browser-related compatibility work
- per-seat transport mode selection
- automatic re-upgrade from HTTP session back to WebSocket during a running match
- distributed coordination across multiple arena server instances
- a full operator dashboard beyond minimal read and switch APIs

## Architecture

The design introduces two new conceptual layers:

### Agent Turn Protocol

This layer defines what the arena server sends to an agent and what an agent sends back. It is transport-agnostic. Both HTTP session mode and WebSocket mode use the same payload semantics so that transport differences do not leak into match logic.

### Agent Transport

This layer defines how the protocol payload is delivered. There are two concrete implementations:

- `HTTPSessionTransport`
- `WebSocketTransport`

The arena runtime chooses one active transport implementation per match. Match logic only asks the selected transport to deliver a turn request and receive a turn response.

## Service-Level Configuration

The server keeps a global transport configuration in process memory and persisted snapshot state.

Recommended fields:

- `transport_default_mode`
  - enum: `http_session` or `websocket`
- `transport_config_version`
  - incremented on every service-level mode switch
- `transport_updated_at`
  - timestamp of the latest switch

This configuration is live and writable during server runtime. Changing it affects only future matches. It does not mutate active matches.

## Match-Level Runtime State

Each match stores its own transport runtime state at the moment the match starts.

Recommended fields:

- `match_transport_mode`
  - the configured mode captured at match creation
- `match_transport_active_mode`
  - the mode currently being used for this match
  - normally equals `match_transport_mode`
  - becomes `http_session` after a WebSocket degradation
- `match_transport_state`
  - enum: `pending`, `active`, `degraded`, `failed`
- `match_transport_reason`
  - latest transition reason, especially useful for degradation and failure
- `match_transport_since`
  - timestamp for when the current active mode took effect
- `transport_config_version_at_start`
  - the service config version captured when the match started

This separation makes the desired behavior explicit:

- service switches affect new matches only
- active matches remain stable
- degradation changes only the affected match

## Session State Per Managed Seat

To support keepalive and recovery, each managed seat should have session runtime data scoped to the match.

Recommended fields:

- `agent_session_id`
- `agent_resume_token`
- `agent_connection_state`
  - enum: `connected`, `disconnected`, `recovering`
- `agent_last_heartbeat_at`
- `agent_session_lease_expires_at`
- `agent_ws_conn_id`
  - only populated when WebSocket mode is active

These fields should be tracked separately for red and black managed seats. Human seats do not use transport session state.

## Unified Turn Protocol

The arena server should send a single structured turn payload regardless of transport.

Recommended `AgentTurnRequest` fields:

- `protocol_version`
- `match_id`
- `room_code`
- `seat`
- `side`
- `transport_mode`
- `turn_id`
- `move_count`
- `step_interval_ms`
- `opponent_alias`
- `board_rows`
- `board_text`
- `legal_moves`
- `prompt`

Key design choices:

- `turn_id` is mandatory to prevent duplicate or stale move submissions
- `legal_moves` is authoritative for move validation
- `board_rows` serves machine parsing, while `board_text` and `prompt` serve model-oriented agents
- `transport_mode` is included for debugging and agent-side telemetry, not as an authority input

Recommended `AgentTurnResponse` fields:

- `turn_id`
- `move`
- `reply`
- `agent_state`
  - enum such as `ok`, `retry_later`, `error`
- `retry_after_ms`
- `session_id`

Server validation rules:

- response `turn_id` must match the current unresolved turn
- `move` must be present in the current `legal_moves`
- stale responses are ignored
- duplicate responses for the same turn are accepted only once

## HTTP Session Mode

HTTP session mode is not just the current one-shot POST behavior. It becomes a lease-based session model.

Recommended behavior:

- the server creates or refreshes a logical session per managed seat
- the session has a lease expiration time
- the agent can send heartbeat or resume requests over HTTP
- the agent can continue using the same session while the lease is valid
- the server delivers turn requests using the unified turn protocol

For compatibility and operational simplicity, the preferred HTTP pattern is:

- agent-initiated polling or pull
- plus heartbeat / resume endpoints

This is preferred over server-push long polling because it is friendlier to:

- Windows 7-era deployment environments
- NAT and firewall boundaries
- older network stacks
- single-process server resource limits

If the session lease expires and cannot be renewed, the match pauses with a transport error.

## WebSocket Mode

In WebSocket mode, a managed agent establishes a long-lived connection and binds itself to its match session.

Recommended behavior:

- the agent connects and authenticates with session binding data
- heartbeat flows through the WebSocket channel
- turn requests and turn responses use the unified protocol over the same connection
- if the connection drops, the match enters a recovery window instead of failing immediately
- the agent may reconnect using `agent_resume_token`

The server must track:

- active connection identity
- last heartbeat time
- current recovery deadline
- the currently outstanding `turn_id`

If the connection recovers within the allowed window, the match continues without transport change.

## Automatic Degradation

Automatic degradation is allowed only in one direction:

- from `websocket` to `http_session`

It is never allowed in the reverse direction during a running match.

Recommended degradation triggers:

- initial WebSocket connection timeout
- repeated heartbeat timeout
- recovery window expiration
- protocol upgrade rejection
- explicit transport incompatibility reported by the agent

Recommended degradation behavior:

- set `match_transport_state` to `degraded`
- set `match_transport_active_mode` to `http_session`
- record a human-readable `match_transport_reason`
- append a match log event for host visibility
- continue the match if HTTP session mode is available

If both WebSocket mode and HTTP session mode are unavailable, the match transitions to paused state with a transport failure reason.

## Match Lifecycle Semantics

At `StartMatch` time:

1. read the current service default transport mode
2. copy it into the new match runtime state
3. initialize transport state as `pending`
4. establish or initialize the required agent session state
5. transition to `active` when the transport becomes ready

During runtime:

- the match must always read its own transport runtime state
- it must never re-read the service default as an authority for the active match

When the service default is switched at runtime:

- update service-level transport config only
- do not mutate active match transport state
- do not reopen or rebind active sessions

## Admin Interface

The MVP should add a minimal service-level transport management API.

Recommended endpoints:

- `GET /api/admin/transport`
  - returns current default mode, config version, and last updated timestamp
- `POST /api/admin/transport`
  - updates the service default mode to `http_session` or `websocket`
  - affects only future matches

This API should be separate from room host controls. It is a service-level operator function, not a room-level host capability.

## Host Visibility

Room or host views should expose read-only transport state for each active match.

Recommended fields to surface:

- configured transport mode at match start
- current active transport mode
- transport runtime state
- whether degradation has occurred
- latest transport reason

This allows hosts and operators to understand:

- whether a match is still on WebSocket
- whether it degraded to HTTP session
- why a match paused due to transport problems

## Error Handling

The system should separate match logic errors from transport errors.

Transport errors include:

- no active WebSocket connection
- heartbeat timeout
- expired HTTP lease
- resume token mismatch
- turn response timeout

Recommended outcomes:

- recoverable WebSocket problems trigger degradation when possible
- unrecoverable transport loss pauses the match
- every transport state change is logged with a clear reason

## Persistence

Transport configuration and match transport runtime state should be persisted in the existing snapshot model so that server restarts do not silently lose transport semantics.

Recommended persistence scope:

- service-level transport config
- active match transport mode and state
- per-seat session metadata needed for safe resume or cleanup

Ephemeral connection handles such as live WebSocket objects should not be persisted directly. Only their serializable runtime metadata should be stored.

## Testing Strategy

The MVP should add tests at four levels.

### Unit Tests

- service-level transport mode switching
- match start captures current service default
- active matches ignore later service-level changes
- degradation updates only the affected match

### Protocol Tests

- `AgentTurnRequest` and `AgentTurnResponse` validation
- `turn_id` matching
- duplicate response suppression
- stale response rejection

### Integration Tests

- HTTP session agent lifecycle with heartbeat and lease
- WebSocket agent lifecycle with connection, heartbeat, and resume
- WebSocket startup failure followed by automatic degradation to HTTP session

### Smoke Tests

- switch service default mode during runtime
- start one match before the switch and one after the switch
- verify old and new matches keep their respective transport modes
- verify degradation and pause behavior is visible to the host

## Windows 7 Compatibility Considerations

This design deliberately avoids making WebSocket transport the only viable mode. HTTP session mode remains the compatibility baseline for older or less stable environments.

This reduces deployment risk because:

- service operation does not depend exclusively on long-lived downstream sockets
- transport degradation has a compatible fallback
- the single-process Go server architecture remains unchanged

## Deliverables

The implementation based on this design should produce:

- a service-level transport mode switch
- per-match transport mode lock-in
- HTTP session keepalive semantics
- WebSocket long-connection semantics
- one-way automatic degradation from WebSocket to HTTP session
- unified turn request / response structures
- minimal admin APIs and host-visible transport state

## Success Criteria

This design is successful when:

- the service default transport mode can be changed at runtime
- matches started before the switch remain on their original configured mode
- matches started after the switch use the new default
- WebSocket failures can degrade a match to HTTP session without changing global service configuration
- transport failures are visible, explainable, and isolated to the affected match
- the implementation remains compatible with the existing single-server Windows 7-oriented deployment strategy
