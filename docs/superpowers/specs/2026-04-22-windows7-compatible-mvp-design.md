# Windows 7 Compatible MVP Design

## Context

This project already has a clear backend architecture for room management, match orchestration, agent integration, and snapshot persistence. The main gaps are:

- the current service targets `go 1.26`, which is incompatible with Windows 7 / Windows Server 2008 R2
- the HTTP router in `app.go` uses Go 1.22+ `net/http` route patterns and `Request.PathValue`
- the frontend shell exists, but `static/app.js` is missing, so the primary user flow is incomplete
- tests currently assume static assets are present, but the app shell is not wired end-to-end

The user clarified that deployment should remain a single server application and must be compatible with Windows 7. The same Windows 7 machine will run the backend service and may serve or proxy frontend assets. Client access will come from browsers on other machines, though modern Chromium is also available on Windows 7 if needed. Because of that, the project should stay as a single version rather than split into legacy and modern branches.

## Goal

Deliver a single-version MVP that:

- compiles and runs on Windows 7 / Windows Server 2008 R2 with Go 1.20
- preserves the current single-process Go server architecture
- serves a working browser experience from the same application
- closes the core host, spectator, and human-move match flow
- passes the project test suite in a Go 1.20-compatible codebase

## Non-Goals

This MVP does not try to complete every unchecked item in `doc/arena/tasks.md`. In particular, it does not target:

- a separate modern-only branch or feature split
- a full game-mode UI beyond the existing placeholder
- a large expansion of property-based or end-to-end test coverage
- extra deployment components such as a database, reverse proxy dependency, or separate frontend build system

## Architecture Decision

Keep the existing architecture:

- one Go process
- one HTTP server
- one set of JSON APIs
- one static frontend served by the backend

The compatibility work should be concentrated in the server entry and HTTP routing layer, not in the Arena, Match, Engine, Snapshot, or Agent domain logic. The business model does not need to fork for Windows 7. The frontend remains a static HTML/CSS/JS app and should keep consuming the same JSON endpoints.

## MVP Scope

The MVP is complete when the following flows work:

1. A user can enter or create a room with a match code.
2. The first entrant becomes host.
3. Users can see room status, seat occupancy, reveal state, spectator count, and step interval.
4. The host can open the settings drawer and:
   - update step interval and default view
   - bind red and black seats
   - clear seats
   - toggle reveal scope
   - start, pause, resume, and reset the match
5. Spectators and players can see the public match view, board, logs, and current phase.
6. When it is a human player's turn, the seat holder can submit a move from the browser UI.
7. Static assets are present and served correctly.
8. The codebase builds and tests under a Go 1.20-compatible setup.

## Compatibility Requirements

### Go Version

The codebase must target Go 1.20 to align with Windows 7 / Windows Server 2008 R2 support. That means:

- `go.mod` must not require a newer version than Go 1.20
- code must avoid depending on standard-library APIs introduced after Go 1.20 in production paths

### HTTP Routing

The current route registration in `app.go` uses Go 1.22-style method-aware patterns and `r.PathValue(...)`. Those must be replaced with a Go 1.20-compatible routing approach. The replacement should:

- preserve the current URL contract
- preserve request and response JSON shapes as much as possible
- keep handler code readable rather than scattering route parsing logic everywhere

The preferred design is a small compatibility router layer in `app.go` that:

- dispatches by HTTP method
- parses known path forms such as `/api/arena/{code}` and `/api/arena/{code}/match`
- passes extracted route parameters into shared helper functions

This keeps the rest of the application logic stable while removing the Go 1.22 dependency.

## Frontend Design

The current frontend shell in `static/index.html` and `static/style.css` already defines the desired layout. The implementation task is to add `static/app.js` and wire the shell to the existing API contract.

### Client State

The browser should keep a small local state:

- `client_token`
- `room_code`
- `display_name`
- `join_intent`
- selected view mode
- current selection for human move input
- whether the current participant is host

This state should be persisted with local storage where useful, especially for `client_token`, `room_code`, and view preference.

### Polling Model

The frontend should use simple polling rather than websockets. It should periodically fetch:

- public room state
- public match state when a match exists
- host room state when the current participant is host
- host match state when the current participant is host and a match exists

Polling should degrade gracefully when a room or match does not exist yet.

### Interaction Model

The board interaction should stay simple:

- render the board from `board_rows`
- allow selecting a source square and then a target square
- translate the interaction into the backend move format
- submit moves only when the current viewer is the human participant whose turn it is

If a move is rejected, the UI should surface the backend error and keep the page usable.

### Host Controls

The host drawer should call the existing endpoints for:

- saving settings
- assigning seats
- removing seats
- changing reveal scope
- starting, pausing, resuming, and resetting matches

After each host action, the frontend should refresh host and public views rather than relying on optimistic state.

## Backend Changes

Only minimal backend changes should be made for MVP support.

### Required

- lower `go.mod` to a Go 1.20-compatible version
- replace Go 1.22-only route registration and path extraction
- add any small response or handler adjustments strictly required to support the frontend flow
- keep the current API semantics intact where possible

### Avoid

- broad refactors in Arena or Match logic
- introducing a new router dependency unless compatibility work becomes significantly more complex than expected
- changing domain models just to satisfy UI convenience

## Testing Strategy

The implementation should follow TDD for each production change.

### Backend

Add or update tests for:

- Go 1.20-compatible HTTP routing behavior
- room entry and host ownership
- match lifecycle endpoints used by the frontend
- static asset serving once `static/app.js` exists

### Frontend

Do not introduce a heavy frontend test framework for this MVP. Instead:

- keep the frontend logic modular enough to inspect and maintain
- rely on backend tests plus manual smoke testing for the browser flow

### Smoke Verification

Before completion, verify:

- the server starts locally
- the join flow works
- host controls work
- a human move can be submitted successfully
- static assets load from the server

## Risks and Mitigations

### Risk: Router rewrite changes API behavior

Mitigation:

- preserve existing endpoint paths exactly
- add targeted HTTP tests before and after the routing change

### Risk: Frontend logic becomes too large and brittle

Mitigation:

- keep `static/app.js` organized around API helpers, state helpers, rendering helpers, and event wiring
- avoid over-engineering with build tooling

### Risk: Windows 7 compatibility assumptions drift later

Mitigation:

- keep the module and production code within Go 1.20 constraints
- avoid reintroducing Go 1.21+ and Go 1.22+ APIs during MVP work

## Deliverables

The MVP implementation should produce:

- a Go 1.20-compatible codebase
- a working `static/app.js`
- a functional single-server browser flow
- passing tests for the supported codebase
- no split legacy/modern branches

## Success Criteria

This work is successful when:

- the project remains a single-version application
- the server architecture is unchanged at a high level
- the backend is compatible with Windows 7-era Go support constraints
- the browser UI is usable for host and spectator workflows
- the main development blockers identified in the existing task document are cleared
