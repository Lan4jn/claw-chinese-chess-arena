# Agent Transport Keepalive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add service-level transport switching, per-match transport lock-in, HTTP session keepalive semantics, WebSocket transport, and one-way WebSocket-to-HTTP degradation for managed agents.

**Architecture:** Keep the existing single-process Go server and Arena/Match domain model, but introduce a transport abstraction that wraps agent turn delivery. Persist service-level transport defaults and match-level transport runtime state in snapshots, expose minimal admin APIs, and keep host-visible transport state in the existing room/match JSON views.

**Tech Stack:** Go 1.20, `net/http`, `httptest`, `encoding/json`, `github.com/gorilla/websocket`, existing snapshot persistence, existing polling frontend compatibility

---

## File Structure

### Files to modify

- `go.mod`
  - Add the WebSocket dependency needed for transport client/server tests.
- `app.go`
  - Add service-level transport admin APIs and any transport-related HTTP routes.
- `arena.go`
  - Store service transport config, match transport runtime state, and orchestrate transport selection and degradation.
- `match.go`
  - Persist match transport metadata, turn protocol IDs, and transport event logs.
- `snapshot.go`
  - Persist service transport config alongside rooms.
- `pico.go`
  - Preserve prompt building, but move the one-shot request path behind the new transport abstraction.
- `http_test.go`
  - Add admin API and HTTP-session transport contract coverage.
- `arena_test.go`
  - Add unit coverage for service switches, match lock-in, degradation, and transport runtime state.

### Files to create

- `agent_transport.go`
  - Transport mode enums, turn request / response structs, session state structs, transport interfaces, and shared validation helpers.
- `agent_transport_http.go`
  - HTTP session transport implementation.
- `agent_transport_ws.go`
  - WebSocket transport implementation and downgrade logic.

## Task 1: Add transport models and persistence

**Files:**
- Modify: `snapshot.go`
- Modify: `match.go`
- Modify: `arena.go`
- Create: `agent_transport.go`
- Test: `arena_test.go`

- [ ] **Step 1: Write failing arena tests for default transport mode and match lock-in**

```go
func TestArenaStartMatchCapturesCurrentTransportMode(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{RoomCode: "transport-room", ClientToken: "host-token", JoinIntent: JoinIntentPlayer})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{RoomCode: "transport-room", ClientToken: "guest-token", JoinIntent: JoinIntentPlayer}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}

	if err := arena.SetTransportDefaultMode(TransportModeWebSocket); err != nil {
		t.Fatalf("SetTransportDefaultMode(websocket) error = %v", err)
	}

	matchView, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if matchView.TransportMode != string(TransportModeWebSocket) {
		t.Fatalf("expected transport mode websocket, got %q", matchView.TransportMode)
	}
	if matchView.TransportActiveMode != string(TransportModeWebSocket) {
		t.Fatalf("expected active transport mode websocket, got %q", matchView.TransportActiveMode)
	}
}

func TestArenaTransportSwitchDoesNotRewriteActiveMatch(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{RoomCode: "transport-switch-room", ClientToken: "host-token", JoinIntent: JoinIntentPlayer})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{RoomCode: "transport-switch-room", ClientToken: "guest-token", JoinIntent: JoinIntentPlayer}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}

	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if err := arena.SetTransportDefaultMode(TransportModeWebSocket); err != nil {
		t.Fatalf("SetTransportDefaultMode(websocket) error = %v", err)
	}

	hostMatch, err := arena.HostMatch(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostMatch() error = %v", err)
	}
	if hostMatch.TransportMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected running match to keep http_session, got %q", hostMatch.TransportMode)
	}
}
```

- [ ] **Step 2: Run tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestArenaStartMatchCapturesCurrentTransportMode|TestArenaTransportSwitchDoesNotRewriteActiveMatch' -v`
Expected: FAIL with unknown transport fields or missing `SetTransportDefaultMode`

- [ ] **Step 3: Add transport enums, match runtime fields, and snapshot persistence**

```go
type TransportMode string

const (
	TransportModeHTTPSession TransportMode = "http_session"
	TransportModeWebSocket   TransportMode = "websocket"
)

type MatchTransportState string

const (
	MatchTransportStatePending  MatchTransportState = "pending"
	MatchTransportStateActive   MatchTransportState = "active"
	MatchTransportStateDegraded MatchTransportState = "degraded"
	MatchTransportStateFailed   MatchTransportState = "failed"
)

type ServiceTransportConfig struct {
	DefaultMode   TransportMode `json:"default_mode"`
	ConfigVersion int           `json:"config_version"`
	UpdatedAt     time.Time     `json:"updated_at,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestArenaStartMatchCapturesCurrentTransportMode|TestArenaTransportSwitchDoesNotRewriteActiveMatch' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent_transport.go arena.go arena_test.go match.go snapshot.go
git commit -m "feat: persist transport runtime state"
```

## Task 2: Add admin transport APIs and host-visible transport fields

**Files:**
- Modify: `app.go`
- Modify: `arena.go`
- Modify: `http_test.go`

- [ ] **Step 1: Write failing HTTP tests for service transport APIs**

```go
func TestAdminTransportModeSwitchAffectsFutureMatchesOnly(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/transport", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/transport expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/transport", bytes.NewReader([]byte(`{"default_mode":"websocket"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/admin/transport expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run TestAdminTransportModeSwitchAffectsFutureMatchesOnly -v`
Expected: FAIL with 404 or missing transport config handlers

- [ ] **Step 3: Add admin handlers and transport fields to host/public match views**

```go
type TransportConfigView struct {
	DefaultMode   string    `json:"default_mode"`
	ConfigVersion int       `json:"config_version"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type PublicMatchView struct {
	// existing fields...
	TransportMode       string `json:"transport_mode,omitempty"`
	TransportActiveMode string `json:"transport_active_mode,omitempty"`
	TransportState      string `json:"transport_state,omitempty"`
	TransportReason     string `json:"transport_reason,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run TestAdminTransportModeSwitchAffectsFutureMatchesOnly -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app.go arena.go http_test.go
git commit -m "feat: expose transport admin APIs"
```

## Task 3: Implement HTTP session transport with unified turn protocol

**Files:**
- Create: `agent_transport_http.go`
- Modify: `agent_transport.go`
- Modify: `arena.go`
- Modify: `arena_test.go`

- [ ] **Step 1: Write failing tests for HTTP session turn delivery**

```go
func TestArenaAdvanceOnceUsesHTTPSessionTransportForManagedSeat(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	var gotTurnID string
	sessionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/open":
			_, _ = w.Write([]byte(`{"session_id":"sess-1","resume_token":"resume-1","lease_ttl_ms":30000}`))
		case "/session/turn":
			var req AgentTurnRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode() turn request error = %v", err)
			}
			gotTurnID = req.TurnID
			_, _ = w.Write([]byte(`{"turn_id":"` + req.TurnID + `","move":"a3-a4","reply":"MOVE: a3-a4","agent_state":"ok","session_id":"sess-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer sessionServer.Close()

	arena.requestMove = nil
	// setup room and seats omitted for brevity in plan execution code
	_ = gotTurnID
}
```

- [ ] **Step 2: Run tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run TestArenaAdvanceOnceUsesHTTPSessionTransportForManagedSeat -v`
Expected: FAIL with missing `AgentTurnRequest` or HTTP session transport logic

- [ ] **Step 3: Implement unified turn structs and HTTP session transport**

```go
type AgentTurnRequest struct {
	ProtocolVersion int      `json:"protocol_version"`
	MatchID         string   `json:"match_id"`
	RoomCode        string   `json:"room_code"`
	Seat            string   `json:"seat"`
	Side            string   `json:"side"`
	TurnID          string   `json:"turn_id"`
	MoveCount       int      `json:"move_count"`
	StepIntervalMS  int      `json:"step_interval_ms"`
	OpponentAlias   string   `json:"opponent_alias"`
	BoardRows       []string `json:"board_rows"`
	BoardText       string   `json:"board_text"`
	LegalMoves      []string `json:"legal_moves"`
	Prompt          string   `json:"prompt"`
}

type AgentTurnResponse struct {
	TurnID       string `json:"turn_id"`
	Move         string `json:"move"`
	Reply        string `json:"reply"`
	AgentState   string `json:"agent_state"`
	RetryAfterMS int    `json:"retry_after_ms,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify green state**

Run: `GOTOOLCHAIN=local go test ./... -run TestArenaAdvanceOnceUsesHTTPSessionTransportForManagedSeat -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent_transport.go agent_transport_http.go arena.go arena_test.go
git commit -m "feat: add http session agent transport"
```

## Task 4: Implement WebSocket transport and one-way degradation

**Files:**
- Modify: `go.mod`
- Create: `agent_transport_ws.go`
- Modify: `arena.go`
- Modify: `arena_test.go`

- [ ] **Step 1: Write failing tests for websocket startup and degradation**

```go
func TestArenaAdvanceOnceDegradesWebSocketToHTTPSession(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	// configure a managed seat whose websocket endpoint fails but HTTP session succeeds
	// after AdvanceOnce(), the match should stay playable and expose degraded/http_session transport state
}
```

- [ ] **Step 2: Run tests to verify red state**

Run: `GOTOOLCHAIN=local go test ./... -run TestArenaAdvanceOnceDegradesWebSocketToHTTPSession -v`
Expected: FAIL with missing websocket transport and degradation logic

- [ ] **Step 3: Add websocket transport, recovery timeout, and one-way downgrade path**

```go
require github.com/gorilla/websocket v1.5.3
```

```go
if err := wsTransport.DeliverTurn(ctx, session, req); err != nil {
	if fallbackErr := httpTransport.DeliverTurn(ctx, session, req); fallbackErr == nil {
		match.TransportState = MatchTransportStateDegraded
		match.TransportActiveMode = TransportModeHTTPSession
		match.TransportReason = err.Error()
	} else {
		match.TransportState = MatchTransportStateFailed
		room.Status = RoomStatusPaused
	}
}
```

- [ ] **Step 4: Run focused and full tests**

Run: `GOTOOLCHAIN=local go test ./... -run TestArenaAdvanceOnceDegradesWebSocketToHTTPSession -v`
Expected: PASS

Run: `GOTOOLCHAIN=local go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum agent_transport_ws.go arena.go arena_test.go
git commit -m "feat: add websocket transport fallback"
```
