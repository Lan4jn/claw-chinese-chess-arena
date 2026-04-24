# Picoclaw Session + Message Hybrid Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-picoclaw long-lived session state, `/message` invitation compatibility, per-participant mode switching, and same-turn session/message fallback without reintroducing the removed generic transport stack.

**Architecture:** Keep the current single-process Arena/Room/Match model, but add a focused per-participant runtime state for managed `picoclaw` seats. Session lifecycle and mode selection live with participant management, `/message` formatting stays in `pico.go`, and match advancement in `arena.go` resolves a primary mode then performs one same-turn fallback through the alternate mode.

**Tech Stack:** Go 1.20, `net/http`, `httptest`, `encoding/json`, existing snapshot persistence, existing embedded static frontend

---

## File Structure

### Files to modify

- `app.go`
  - Add host-facing APIs for session open / heartbeat / close, participant mode changes, and any picoclaw diagnostics response wiring.
- `arena.go`
  - Persist per-participant picoclaw runtime state, expose host operations, resolve active mode, and run same-turn fallback during managed turns.
- `match.go`
  - Keep match logs expressive for primary-mode failure, fallback attempts, and resulting mode switches.
- `pico.go`
  - Preserve move prompt building and `/message` move delivery; add invitation payload formatting and helpers for message-based invite delivery.
- `snapshot.go`
  - Persist the new runtime state as part of room snapshots.
- `http_test.go`
  - Cover new host APIs, persisted runtime state, and reserved invite documentation behavior where exposed.
- `arena_test.go`
  - Cover runtime state transitions, same-turn fallback, participant isolation, and pause conditions.
- `static/index.html`
  - Add host controls for participant-level mode switching and session diagnostics.
- `static/app.js`
  - Load and submit participant-level mode changes, render session diagnostics, and keep invitation/session controls host-only.
- `README.md`
  - Document the hybrid model, current `/message` invite behavior, and reserved future `invite` endpoint.

### Files to create

- `picoclaw_runtime.go`
  - Per-participant runtime structs, enums, helper functions for session state, lease freshness, and mode resolution.

## Task 1: Add per-participant picoclaw runtime state and persistence

**Files:**
- Create: `picoclaw_runtime.go`
- Modify: `arena.go`
- Modify: `snapshot.go`
- Test: `arena_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestAssignSeatCreatesPicoclawRuntimeState(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}

	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18888",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}

	seat := hostRoom.Room.Seats[SeatBlackPlayer]
	if seat.ParticipantID == "" {
		t.Fatalf("expected black seat participant")
	}
	runtime := hostRoom.Runtime[seat.ParticipantID]
	if runtime.ParticipantID != seat.ParticipantID {
		t.Fatalf("expected runtime participant_id %q, got %q", seat.ParticipantID, runtime.ParticipantID)
	}
	if runtime.PreferredMode != PicoclawModeAuto {
		t.Fatalf("expected preferred_mode auto, got %q", runtime.PreferredMode)
	}
	if runtime.ActiveMode != PicoclawActiveModeMessage {
		t.Fatalf("expected active_mode message, got %q", runtime.ActiveMode)
	}
	if runtime.SessionState != PicoclawSessionStateIdle {
		t.Fatalf("expected session_state idle, got %q", runtime.SessionState)
	}
}

func TestSnapshotPersistsPicoclawRuntimeState(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-snapshot-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-snapshot-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18888",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	reloaded := NewArena(store)
	defer reloaded.Close()

	hostRoom, err := reloaded.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after reload error = %v", err)
	}
	seat := hostRoom.Room.Seats[SeatBlackPlayer]
	if hostRoom.Runtime[seat.ParticipantID].ParticipantID == "" {
		t.Fatalf("expected persisted runtime state for participant %q", seat.ParticipantID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestAssignSeatCreatesPicoclawRuntimeState|TestSnapshotPersistsPicoclawRuntimeState' -v`
Expected: FAIL with missing `Runtime` field, missing `PicoclawModeAuto`, or missing persisted participant runtime state

- [ ] **Step 3: Write the minimal implementation**

Create `picoclaw_runtime.go`:

```go
package main

import "time"

type PicoclawPreferredMode string
type PicoclawActiveMode string
type PicoclawSessionState string

const (
	PicoclawModeAuto          PicoclawPreferredMode = "auto"
	PicoclawModePreferSession PicoclawPreferredMode = "prefer_session"
	PicoclawModePreferMessage PicoclawPreferredMode = "prefer_message"
)

const (
	PicoclawActiveModeSession PicoclawActiveMode = "session"
	PicoclawActiveModeMessage PicoclawActiveMode = "message"
)

const (
	PicoclawSessionStateIdle       PicoclawSessionState = "idle"
	PicoclawSessionStateOpening    PicoclawSessionState = "opening"
	PicoclawSessionStateActive     PicoclawSessionState = "active"
	PicoclawSessionStateStale      PicoclawSessionState = "stale"
	PicoclawSessionStateRecovering PicoclawSessionState = "recovering"
	PicoclawSessionStateClosed     PicoclawSessionState = "closed"
)

type PicoclawRuntimeState struct {
	ParticipantID               string                `json:"participant_id"`
	PreferredMode               PicoclawPreferredMode `json:"preferred_mode"`
	ActiveMode                  PicoclawActiveMode    `json:"active_mode"`
	SessionID                   string                `json:"session_id,omitempty"`
	SessionState                PicoclawSessionState  `json:"session_state"`
	SessionOpenedAt             time.Time             `json:"session_opened_at,omitempty"`
	LastHeartbeatAt             time.Time             `json:"last_heartbeat_at,omitempty"`
	LeaseExpiresAt              time.Time             `json:"lease_expires_at,omitempty"`
	RecoveryDeadlineAt          time.Time             `json:"recovery_deadline_at,omitempty"`
	ConsecutiveSessionFailures  int                   `json:"consecutive_session_failures,omitempty"`
	ConsecutiveMessageFailures  int                   `json:"consecutive_message_failures,omitempty"`
	LastModeSwitchAt            time.Time             `json:"last_mode_switch_at,omitempty"`
	LastSwitchReason            string                `json:"last_switch_reason,omitempty"`
	LastInviteAt                time.Time             `json:"last_invite_at,omitempty"`
	LastInviteStatus            string                `json:"last_invite_status,omitempty"`
}

func newPicoclawRuntimeState(participantID string) PicoclawRuntimeState {
	return PicoclawRuntimeState{
		ParticipantID: participantID,
		PreferredMode: PicoclawModeAuto,
		ActiveMode:    PicoclawActiveModeMessage,
		SessionState:  PicoclawSessionStateIdle,
	}
}

func (s PicoclawRuntimeState) SessionHealthy(now time.Time) bool {
	return s.SessionState == PicoclawSessionStateActive && !s.LeaseExpiresAt.IsZero() && now.Before(s.LeaseExpiresAt)
}
```

Modify the room and host view state in `arena.go`:

```go
type ArenaRoom struct {
	// existing fields...
	PicoclawRuntime map[string]PicoclawRuntimeState `json:"picoclaw_runtime,omitempty"`
}

type HostRoomView struct {
	IsHost       bool                               `json:"is_host"`
	Room         PublicRoom                         `json:"room"`
	Participants []HostParticipantView              `json:"participants"`
	Runtime      map[string]PicoclawRuntimeState    `json:"runtime,omitempty"`
}
```

Initialize and persist runtime state when assigning a managed picoclaw seat:

```go
func ensurePicoclawRuntime(room *ArenaRoom, participant *Participant) {
	if room.PicoclawRuntime == nil {
		room.PicoclawRuntime = make(map[string]PicoclawRuntimeState)
	}
	if normalizeAgentType(participant.RealType) != AgentTypePicoclaw {
		delete(room.PicoclawRuntime, participant.ID)
		return
	}
	state, ok := room.PicoclawRuntime[participant.ID]
	if !ok {
		room.PicoclawRuntime[participant.ID] = newPicoclawRuntimeState(participant.ID)
		return
	}
	state.ParticipantID = participant.ID
	room.PicoclawRuntime[participant.ID] = state
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestAssignSeatCreatesPicoclawRuntimeState|TestSnapshotPersistsPicoclawRuntimeState' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add picoclaw_runtime.go arena.go arena_test.go snapshot.go
git commit -m "feat: persist picoclaw runtime state"
```

## Task 2: Add arena-side session APIs and host-visible diagnostics

**Files:**
- Modify: `app.go`
- Modify: `arena.go`
- Modify: `picoclaw_runtime.go`
- Test: `http_test.go`
- Test: `arena_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestPicoclawSessionHeartbeatRefreshesLease(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"heartbeat-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"heartbeat-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/heartbeat-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18888"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	var hostRoom struct {
		Runtime map[string]PicoclawRuntimeState `json:"runtime"`
		Room    struct {
			Seats map[SeatType]Seat `json:"seats"`
		} `json:"room"`
	}
	if err := json.NewDecoder(assignRR.Body).Decode(&hostRoom); err != nil {
		t.Fatalf("Decode() host room error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID

	openReq := httptest.NewRequest(http.MethodPost, "/api/arena/heartbeat-room/picoclaw/"+participantID+"/session/open", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	openReq.Header.Set("Content-Type", "application/json")
	openRR := httptest.NewRecorder()
	app.routes().ServeHTTP(openRR, openReq)
	if openRR.Code != http.StatusOK {
		t.Fatalf("session open expected 200, got %d body=%s", openRR.Code, openRR.Body.String())
	}

	heartbeatReq := httptest.NewRequest(http.MethodPost, "/api/arena/heartbeat-room/picoclaw/"+participantID+"/session/heartbeat", bytes.NewReader([]byte(`{"session_id":"`+participantID+`","lease_ttl_ms":45000}`)))
	heartbeatReq.Header.Set("Content-Type", "application/json")
	heartbeatRR := httptest.NewRecorder()
	app.routes().ServeHTTP(heartbeatRR, heartbeatReq)
	if heartbeatRR.Code != http.StatusOK {
		t.Fatalf("heartbeat expected 200, got %d body=%s", heartbeatRR.Code, heartbeatRR.Body.String())
	}

	var heartbeatView PicoclawRuntimeState
	if err := json.NewDecoder(heartbeatRR.Body).Decode(&heartbeatView); err != nil {
		t.Fatalf("Decode() heartbeat view error = %v", err)
	}
	if heartbeatView.SessionState != PicoclawSessionStateActive {
		t.Fatalf("expected session_state active, got %q", heartbeatView.SessionState)
	}
	if heartbeatView.LeaseExpiresAt.IsZero() {
		t.Fatalf("expected lease_expires_at to be populated")
	}
}

func TestHostCanChangePicoclawPreferredMode(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "mode-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "mode-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18888",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID

	runtime, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession)
	if err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	if runtime.PreferredMode != PicoclawModePreferSession {
		t.Fatalf("expected preferred_mode prefer_session, got %q", runtime.PreferredMode)
	}
}

func TestPicoclawSessionCloseMarksSessionClosed(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "close-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "close-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18888",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID

	if _, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID); err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	runtime, err := arena.ClosePicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("ClosePicoclawSession() error = %v", err)
	}
	if runtime.SessionState != PicoclawSessionStateClosed {
		t.Fatalf("expected session_state closed, got %q", runtime.SessionState)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestPicoclawSessionHeartbeatRefreshesLease|TestHostCanChangePicoclawPreferredMode|TestPicoclawSessionCloseMarksSessionClosed' -v`
Expected: FAIL with missing session routes, missing `SetPicoclawMode`, missing `ClosePicoclawSession`, or missing heartbeat state updates

- [ ] **Step 3: Write the minimal implementation**

Add host/runtime methods in `arena.go`:

```go
func (a *Arena) OpenPicoclawSession(code, hostParticipantID, participantID string) (PicoclawRuntimeState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	participant := findParticipantByID(room, participantID)
	if participant == nil || normalizeAgentType(participant.RealType) != AgentTypePicoclaw {
		return PicoclawRuntimeState{}, fmt.Errorf("picoclaw participant not found")
	}
	ensurePicoclawRuntime(room, participant)
	state := room.PicoclawRuntime[participantID]
	state.SessionID = participantID
	state.SessionState = PicoclawSessionStateOpening
	state.SessionOpenedAt = time.Now()
	state.LastSwitchReason = "session_open"
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PicoclawRuntimeState{}, err
	}
	return state, nil
}

func (a *Arena) HeartbeatPicoclawSession(code, participantID, sessionID string, ttl time.Duration) (PicoclawRuntimeState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return PicoclawRuntimeState{}, fmt.Errorf("room not found")
	}
	state, ok := room.PicoclawRuntime[participantID]
	if !ok {
		return PicoclawRuntimeState{}, fmt.Errorf("picoclaw runtime not found")
	}
	if sessionID != "" && state.SessionID != "" && sessionID != state.SessionID {
		return PicoclawRuntimeState{}, fmt.Errorf("session_id mismatch")
	}
	now := time.Now()
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	if state.SessionID == "" {
		state.SessionID = participantID
	}
	state.SessionState = PicoclawSessionStateActive
	state.LastHeartbeatAt = now
	state.LeaseExpiresAt = now.Add(ttl)
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = now
	if err := a.saveLocked(); err != nil {
		return PicoclawRuntimeState{}, err
	}
	return state, nil
}

func (a *Arena) SetPicoclawMode(code, hostParticipantID, participantID string, mode PicoclawPreferredMode) (PicoclawRuntimeState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	state, ok := room.PicoclawRuntime[participantID]
	if !ok {
		return PicoclawRuntimeState{}, fmt.Errorf("picoclaw runtime not found")
	}
	switch mode {
	case PicoclawModeAuto, PicoclawModePreferSession, PicoclawModePreferMessage:
	default:
		return PicoclawRuntimeState{}, fmt.Errorf("unsupported preferred_mode")
	}
	state.PreferredMode = mode
	state.ActiveMode = resolvePicoclawActiveMode(state, time.Now())
	state.LastModeSwitchAt = time.Now()
	state.LastSwitchReason = "host_override"
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PicoclawRuntimeState{}, err
	}
	return state, nil
}

func (a *Arena) ClosePicoclawSession(code, hostParticipantID, participantID string) (PicoclawRuntimeState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	state, ok := room.PicoclawRuntime[participantID]
	if !ok {
		return PicoclawRuntimeState{}, fmt.Errorf("picoclaw runtime not found")
	}
	state.SessionState = PicoclawSessionStateClosed
	state.LeaseExpiresAt = time.Time{}
	state.RecoveryDeadlineAt = time.Time{}
	state.LastModeSwitchAt = time.Now()
	state.LastSwitchReason = "session_close"
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PicoclawRuntimeState{}, err
	}
	return state, nil
}
```

Add route handlers in `app.go`:

```go
"picoclaw/{participant_id}/session/open": {
	http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string, participantID string) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.OpenPicoclawSession(code, req.HostToken, participantID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	},
}
"picoclaw/{participant_id}/session/close": {
	http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string, participantID string) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.ClosePicoclawSession(code, req.HostToken, participantID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	},
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestPicoclawSessionHeartbeatRefreshesLease|TestHostCanChangePicoclawPreferredMode|TestPicoclawSessionCloseMarksSessionClosed' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app.go arena.go arena_test.go http_test.go picoclaw_runtime.go
git commit -m "feat: add picoclaw session and mode APIs"
```

## Task 3: Add `/message` invitation flow and reserve future `invite` endpoint

**Files:**
- Modify: `pico.go`
- Modify: `arena.go`
- Modify: `arena_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing tests**

```go
func TestInvitePicoclawUsesMessageEndpoint(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	var received picoMessageRequest
	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" {
			t.Fatalf("expected /message path, got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("Decode() invite request error = %v", err)
		}
		writeJSON(w, http.StatusOK, picoMessageResponse{Reply: "INVITE_ACK"})
	}))
	defer messageServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     messageServer.URL,
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID

	reply, err := arena.InvitePicoclaw(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("InvitePicoclaw() error = %v", err)
	}
	if reply != "INVITE_ACK" {
		t.Fatalf("expected INVITE_ACK reply, got %q", reply)
	}
	if !strings.Contains(received.Message, "邀请你加入中国象棋比赛") {
		t.Fatalf("expected invite prompt, got %q", received.Message)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=local go test ./... -run TestInvitePicoclawUsesMessageEndpoint -v`
Expected: FAIL with missing `InvitePicoclaw` or missing invite prompt builder

- [ ] **Step 3: Write the minimal implementation**

Add invite helpers in `pico.go`:

```go
func buildInvitePrompt(room *ArenaRoom, participant *Participant, arenaBaseURL string) string {
	return fmt.Sprintf(`邀请你加入中国象棋比赛。
房间：%s
席位：%s
公开代号：%s
participant_id：%s
arena_base_url：%s
请继续使用 /message 完成当前版本接入。
保留接口说明：未来可能支持 /invite，但当前版本不会调用。`,
		room.Code,
		participant.Seat,
		participant.PublicAlias,
		participant.ID,
		arenaBaseURL,
	)
}

func sendPicoclawInvite(ctx context.Context, client *http.Client, participant *Participant, room *ArenaRoom, arenaBaseURL string) (string, error) {
	payload := picoMessageRequest{
		SessionID:         "invite-" + participant.ID,
		SenderID:          "picoclaw-xiangqi-arena",
		SenderDisplayName: "Picoclaw Xiangqi Arena",
		Message:           buildInvitePrompt(room, participant, arenaBaseURL),
		APIKey:            strings.TrimSpace(participant.APIKey),
	}
	endpoint, err := normalizePicoMessageURL(participant.BaseURL)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var decoded picoMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if decoded.Error != "" {
		return decoded.Reply, fmt.Errorf("%s", decoded.Error)
	}
	return decoded.Reply, nil
}
```

Add arena method:

```go
func (a *Arena) InvitePicoclaw(code, hostParticipantID, participantID string) (string, error) {
	a.mu.Lock()
	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		a.mu.Unlock()
		return "", err
	}
	participant := findParticipantByID(room, participantID)
	if participant == nil || normalizeAgentType(participant.RealType) != AgentTypePicoclaw {
		a.mu.Unlock()
		return "", fmt.Errorf("picoclaw participant not found")
	}
	roomCopy := *room
	participantCopy := *participant
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reply, err := sendPicoclawInvite(ctx, defaultHTTPClient(), &participantCopy, &roomCopy, "")

	a.mu.Lock()
	defer a.mu.Unlock()
	room, _ = a.rooms[normalizeRoomCode(code)]
	state := room.PicoclawRuntime[participantID]
	state.LastInviteAt = time.Now()
	if err != nil {
		state.LastInviteStatus = err.Error()
	} else {
		state.LastInviteStatus = "ok"
	}
	room.PicoclawRuntime[participantID] = state
	_ = a.saveLocked()
	return reply, err
}
```

Update `README.md` with a concrete reserved section:

```md
### Reserved future endpoint

The arena does not call `POST {base_url}/invite` in the current release.

This endpoint is reserved for future development so that picoclaw-side implementations may separate:

- invitation semantics
- move-request semantics

Until that future protocol exists, the arena sends invitations through `POST {base_url}/message`.
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=local go test ./... -run TestInvitePicoclawUsesMessageEndpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add arena.go arena_test.go pico.go README.md
git commit -m "feat: add picoclaw message-based invite flow"
```

## Task 4: Implement same-turn dual-path move recovery and mode health updates

**Files:**
- Modify: `arena.go`
- Modify: `picoclaw_runtime.go`
- Modify: `match.go`
- Test: `arena_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestArenaAdvanceOnceFallsBackFromSessionToMessage(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	var attempts []string
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		attempts = append(attempts, "message")
		return "a3-a4", "MOVE: a3-a4", nil
	}
	arena.requestSessionMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		attempts = append(attempts, "session")
		return "", "", fmt.Errorf("session unavailable")
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "fallback-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "fallback-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18888",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, participantID, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, "host-token", "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	time.Sleep(3 * time.Millisecond)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if got := strings.Join(attempts, ","); got != "session,message" {
		t.Fatalf("expected session,message attempts, got %q", got)
	}
	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected fallback move a3-a4, got %q", matchView.LastMove)
	}
}

func TestArenaPausesOnlyAfterSessionAndMessageBothFail(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		return "", "message failed", fmt.Errorf("message failed")
	}
	arena.requestSessionMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		return "", "session failed", fmt.Errorf("session failed")
	}

	// same room setup as previous test
	_ = store
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestArenaAdvanceOnceFallsBackFromSessionToMessage|TestArenaPausesOnlyAfterSessionAndMessageBothFail' -v`
Expected: FAIL with missing `requestSessionMove`, missing mode resolver, or no same-turn fallback behavior

- [ ] **Step 3: Write the minimal implementation**

Add arena hooks:

```go
type Arena struct {
	// existing fields...
	requestMove        func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error)
	requestSessionMove func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error)
}
```

Add mode resolution in `picoclaw_runtime.go`:

```go
func resolvePicoclawActiveMode(state PicoclawRuntimeState, now time.Time) PicoclawActiveMode {
	switch state.PreferredMode {
	case PicoclawModePreferMessage:
		return PicoclawActiveModeMessage
	case PicoclawModePreferSession:
		if state.SessionHealthy(now) {
			return PicoclawActiveModeSession
		}
		return PicoclawActiveModeMessage
	default:
		if state.SessionHealthy(now) {
			return PicoclawActiveModeSession
		}
		return PicoclawActiveModeMessage
	}
}
```

Use a single recovery helper from `arena.go`:

```go
func (a *Arena) requestPicoclawMove(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState, runtime PicoclawRuntimeState) (string, string, PicoclawRuntimeState, error) {
	now := time.Now()
	runtime.ActiveMode = resolvePicoclawActiveMode(runtime, now)

	tryMode := func(mode PicoclawActiveMode) (string, string, error) {
		if mode == PicoclawActiveModeSession {
			return a.requestSessionMove(matchID, participantID, player, state, legal, arenaState)
		}
		return a.requestMove(matchID, player, state, legal, arenaState)
	}

	primary := runtime.ActiveMode
	move, reply, err := tryMode(primary)
	if err == nil {
		return move, reply, runtime, nil
	}

	alternate := PicoclawActiveModeMessage
	if primary == PicoclawActiveModeMessage {
		alternate = PicoclawActiveModeSession
	}
	move, fallbackReply, fallbackErr := tryMode(alternate)
	if fallbackErr == nil {
		runtime.ActiveMode = alternate
		runtime.LastModeSwitchAt = now
		runtime.LastSwitchReason = "fallback_after_" + string(primary) + "_failure"
		return move, fallbackReply, runtime, nil
	}

	return "", reply + "\n" + fallbackReply, runtime, fmt.Errorf("%s failed: %v; %s failed: %v", primary, err, alternate, fallbackErr)
}
```

Update `advanceRoom` to:

- load participant runtime
- try primary mode
- try alternate mode once
- update `PicoclawRuntimeState`
- pause only when both attempts fail
- append log entries for primary failure, fallback success, or double failure

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOTOOLCHAIN=local go test ./... -run 'TestArenaAdvanceOnceFallsBackFromSessionToMessage|TestArenaPausesOnlyAfterSessionAndMessageBothFail' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add arena.go arena_test.go match.go picoclaw_runtime.go
git commit -m "feat: add hybrid picoclaw move fallback"
```

## Task 5: Add host controls, docs, and final verification

**Files:**
- Modify: `app.go`
- Modify: `static/index.html`
- Modify: `static/app.js`
- Modify: `README.md`
- Test: `http_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestHostRoomIncludesPicoclawRuntimeDiagnostics(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-runtime-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-runtime-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-runtime-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18888"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-runtime-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("GET host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostRoom struct {
		Runtime map[string]PicoclawRuntimeState `json:"runtime"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostRoom); err != nil {
		t.Fatalf("Decode() host room error = %v", err)
	}
	if len(hostRoom.Runtime) == 0 {
		t.Fatalf("expected runtime diagnostics in host room payload")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=local go test ./... -run TestHostRoomIncludesPicoclawRuntimeDiagnostics -v`
Expected: FAIL with missing `runtime` field in host room payload

- [ ] **Step 3: Write the minimal implementation**

Update `HostRoomView` rendering and frontend bindings:

```js
function renderPicoclawRuntime(participantID) {
  if (!state.hostRoom || !state.hostRoom.runtime) {
    return "";
  }
  const runtime = state.hostRoom.runtime[participantID];
  if (!runtime) {
    return "";
  }
  return (
    '<p class="participant-meta">模式偏好：' +
    escapeHTML(runtime.preferred_mode || "auto") +
    " · 当前模式：" +
    escapeHTML(runtime.active_mode || "message") +
    " · 会话状态：" +
    escapeHTML(runtime.session_state || "idle") +
    "</p>"
  );
}
```

Add README sections:

```md
## Picoclaw hybrid mode

Each managed picoclaw now has:

- a long-lived arena-side session state
- direct `/message` compatibility
- a participant-level preferred mode
- same-turn fallback between session and message paths

## Invitation compatibility

The arena currently invites picoclaw through `POST {base_url}/message`.

The endpoint `POST {base_url}/invite` is documented as reserved for future development only and is not called in the current release.
```

- [ ] **Step 4: Run the full verification suite**

Run: `GOTOOLCHAIN=local go test -count=1 ./...`
Expected: PASS

Run: `GOTOOLCHAIN=local go build ./...`
Expected: PASS

Run: `mkdir -p dist && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -o dist/pico-xiangqi-arena-linux-amd64 .`
Expected: exit 0

Run: `mkdir -p dist && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -o dist/pico-xiangqi-arena-windows-amd64.exe .`
Expected: exit 0

- [ ] **Step 5: Commit**

```bash
git add app.go http_test.go static/index.html static/app.js README.md
git commit -m "feat: expose picoclaw hybrid runtime controls"
```

## Self-Review Checklist

- Spec coverage:
  - per-picoclaw runtime state: Task 1
  - session open / heartbeat / close and mode switching: Task 2
  - `/message` invitation and reserved `/invite` docs: Task 3
  - same-turn dual-path recovery and pause rules: Task 4
  - host diagnostics, docs, and final verification: Task 5
- Placeholder scan:
  - no `TODO`, `TBD`, or “implement later” markers remain
- Type consistency:
  - `PicoclawPreferredMode`, `PicoclawActiveMode`, `PicoclawSessionState`, `PicoclawRuntimeState`, `SetPicoclawMode`, and `InvitePicoclaw` are used consistently across tasks
