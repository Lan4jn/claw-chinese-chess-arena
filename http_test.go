package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnterArenaCreatesRoomAndReturnsPublicAlias(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	body, err := json.Marshal(map[string]string{
		"room_code":    "demo-room",
		"client_token": "host-token",
		"join_intent":  string(JoinIntentAuto),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var decoded struct {
		IsHost      bool `json:"is_host"`
		Participant struct {
			PublicAlias string `json:"public_alias"`
			Seat        string `json:"seat"`
		} `json:"participant"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&decoded); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !decoded.IsHost {
		t.Fatalf("expected first entrant to be host")
	}
	if decoded.Participant.PublicAlias == "" {
		t.Fatalf("expected public alias to be generated")
	}
	if decoded.Participant.Seat != string(SeatRedPlayer) {
		t.Fatalf("expected host to occupy red player seat, got %q", decoded.Participant.Seat)
	}
}

func TestEnterArenaJoinModeDoesNotCreateMissingRoom(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/arena/enter",
		bytes.NewReader([]byte(`{"room_code":"join-only-room","client_token":"guest-token","room_action":"join","join_intent":"spectator"}`)),
	)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST /api/arena/enter room_action=join expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "room not found") {
		t.Fatalf("expected room not found error, got body=%s", rr.Body.String())
	}
}

func TestEnterArenaCreateModeRejectsExistingRoom(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	firstReq := httptest.NewRequest(
		http.MethodPost,
		"/api/arena/enter",
		bytes.NewReader([]byte(`{"room_code":"create-only-room","client_token":"host-token","room_action":"create","join_intent":"spectator"}`)),
	)
	firstReq.Header.Set("Content-Type", "application/json")
	firstRR := httptest.NewRecorder()
	app.routes().ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first create expected 200, got %d body=%s", firstRR.Code, firstRR.Body.String())
	}

	secondReq := httptest.NewRequest(
		http.MethodPost,
		"/api/arena/enter",
		bytes.NewReader([]byte(`{"room_code":"create-only-room","client_token":"other-token","room_action":"create","join_intent":"spectator"}`)),
	)
	secondReq.Header.Set("Content-Type", "application/json")
	secondRR := httptest.NewRecorder()
	app.routes().ServeHTTP(secondRR, secondReq)

	if secondRR.Code != http.StatusConflict {
		t.Fatalf("duplicate create expected 409, got %d body=%s", secondRR.Code, secondRR.Body.String())
	}
	if !strings.Contains(secondRR.Body.String(), "room already exists") {
		t.Fatalf("expected room already exists error, got body=%s", secondRR.Body.String())
	}
}

func TestArenaHTTPEnterThenFetchPublicRoom(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterBody := []byte(`{"room_code":"Flow-Room","client_token":"flow-host-token","display_name":"Flow Host","join_intent":"spectator"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader(enterBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/enter expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var enterDecoded struct {
		Participant struct {
			ID          string `json:"id"`
			PublicAlias string `json:"public_alias"`
			Seat        string `json:"seat"`
		} `json:"participant"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&enterDecoded); err != nil {
		t.Fatalf("Decode() enter response error = %v", err)
	}
	if enterDecoded.Participant.ID == "" {
		t.Fatalf("expected participant id in enter response")
	}
	if enterDecoded.Participant.PublicAlias == "" {
		t.Fatalf("expected participant public alias in enter response")
	}
	if enterDecoded.Participant.Seat != string(SeatSpectator) {
		t.Fatalf("expected spectator seat for join_intent=spectator, got %q", enterDecoded.Participant.Seat)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/flow-room", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code} expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var roomDecoded struct {
		Code              string `json:"code"`
		HostParticipantID string `json:"host_participant_id"`
		Status            string `json:"status"`
		StepIntervalMS    int    `json:"step_interval_ms"`
		RevealState       string `json:"reveal_state"`
		DefaultView       string `json:"default_view"`
		SpectatorCount    int    `json:"spectator_count"`
		Seats             map[string]struct {
			Type          string `json:"type"`
			ParticipantID string `json:"participant_id"`
			PublicAlias   string `json:"public_alias"`
			RealType      string `json:"real_type"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&roomDecoded); err != nil {
		t.Fatalf("Decode() public room response error = %v", err)
	}
	if roomDecoded.Code != "flow-room" {
		t.Fatalf("expected normalized room code flow-room, got %q", roomDecoded.Code)
	}
	if roomDecoded.Status != string(RoomStatusWaiting) {
		t.Fatalf("expected waiting room status, got %q", roomDecoded.Status)
	}
	if roomDecoded.StepIntervalMS <= 0 {
		t.Fatalf("expected positive step_interval_ms, got %d", roomDecoded.StepIntervalMS)
	}
	if roomDecoded.RevealState != string(RevealStateHidden) {
		t.Fatalf("expected hidden reveal_state, got %q", roomDecoded.RevealState)
	}
	if roomDecoded.DefaultView == "" {
		t.Fatalf("expected default_view to be set")
	}
	if roomDecoded.HostParticipantID != "" {
		t.Fatalf("expected host_participant_id hidden in public room, got %q", roomDecoded.HostParticipantID)
	}
	if roomDecoded.SpectatorCount != 1 {
		t.Fatalf("expected spectator_count=1 after spectator entry, got %d", roomDecoded.SpectatorCount)
	}

	hostSeat, ok := roomDecoded.Seats[string(SeatHost)]
	if !ok {
		t.Fatalf("expected host seat in public room")
	}
	if hostSeat.ParticipantID != enterDecoded.Participant.ID {
		t.Fatalf("expected host seat participant_id to match entrant id")
	}
	if hostSeat.PublicAlias != enterDecoded.Participant.PublicAlias {
		t.Fatalf("expected host seat alias to match entrant alias")
	}
	if hostSeat.RealType != "" {
		t.Fatalf("expected host seat real_type hidden in public room, got %q", hostSeat.RealType)
	}
}

func TestArenaHTTPStartMatchAndFetchPublicState(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterBody, err := json.Marshal(map[string]string{
		"room_code":    "http-room",
		"client_token": "host-token",
		"join_intent":  string(JoinIntentPlayer),
	})
	if err != nil {
		t.Fatalf("Marshal() enter body error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader(enterBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"http-room","client_token":"guest-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/http-room/match/start", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("start match expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/http-room/match", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("public match expected 200, got %d", rr.Code)
	}

	var decoded struct {
		RoomStatus string `json:"room_status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&decoded); err != nil {
		t.Fatalf("Decode() public match error = %v", err)
	}
	if decoded.RoomStatus != string(RoomStatusPlaying) {
		t.Fatalf("expected room_status=playing, got %q", decoded.RoomStatus)
	}
}

func TestArenaHTTPRoutingCompatibilityEndpoints(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterBody := []byte(`{"room_code":"compat-room","client_token":"compat-host","join_intent":"player"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader(enterBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/enter expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"compat-room","client_token":"compat-guest","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/enter (guest) expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/compat-room/match/start", bytes.NewReader([]byte(`{"host_token":"compat-host"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/match/start expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/compat-room/match", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code}/match expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/compat-room/host?token=compat-host", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code}/host?token=... expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestArenaHTTPHostMatchReturns404BeforeStart(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-match-room","client_token":"host-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/enter expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/host-match-room/host/match?token=host-token", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/arena/{code}/host/match before start expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestArenaHTTPRoutingMethodContracts(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/api/arena/method-room/match/start", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/arena/{code}/match/start expected 405, got %d", rr.Code)
	}
	if got := rr.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("GET /api/arena/{code}/match/start Allow expected %q, got %q", http.MethodPost, got)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/method-room/match", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/arena/{code}/match expected 405, got %d", rr.Code)
	}
	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("POST /api/arena/{code}/match Allow expected %q, got %q", "GET, HEAD", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/static/style.css", bytes.NewReader([]byte("ignored")))
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /static/style.css expected 405, got %d", rr.Code)
	}
	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("POST /static/style.css Allow expected %q, got %q", "GET, HEAD", got)
	}
}

func TestArenaHTTPHeadSupportOnGetEndpoints(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodHead, "/api/health", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HEAD /api/health expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"head-room","client_token":"head-host","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setup enter host expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"head-room","client_token":"head-guest","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setup enter guest expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/head-room/match/start", bytes.NewReader([]byte(`{"host_token":"head-host"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("setup start match expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodHead, "/api/arena/head-room/match", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HEAD /api/arena/{code}/match expected 200, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodHead, "/static/style.css", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HEAD /static/style.css expected 200, got %d", rr.Code)
	}
}

func TestArenaHTTPHostSettingsFlow(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-settings-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	settingsReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-settings-room/settings", bytes.NewReader([]byte(`{"host_token":"host-token","step_interval_ms":1200,"default_view":"commentary"}`)))
	settingsReq.Header.Set("Content-Type", "application/json")
	settingsRR := httptest.NewRecorder()
	app.routes().ServeHTTP(settingsRR, settingsReq)
	if settingsRR.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/settings expected 200, got %d body=%s", settingsRR.Code, settingsRR.Body.String())
	}

	var hostView struct {
		Room struct {
			StepIntervalMS int    `json:"step_interval_ms"`
			DefaultView    string `json:"default_view"`
		} `json:"room"`
	}
	if err := json.NewDecoder(settingsRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() settings response error = %v", err)
	}
	if hostView.Room.StepIntervalMS != 1200 {
		t.Fatalf("expected host room step_interval_ms=1200, got %d", hostView.Room.StepIntervalMS)
	}
	if hostView.Room.DefaultView != "commentary" {
		t.Fatalf("expected host room default_view=commentary, got %q", hostView.Room.DefaultView)
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-settings-room", nil)
	publicRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicRR, publicReq)
	if publicRR.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code} expected 200, got %d body=%s", publicRR.Code, publicRR.Body.String())
	}

	var publicView struct {
		StepIntervalMS int    `json:"step_interval_ms"`
		DefaultView    string `json:"default_view"`
	}
	if err := json.NewDecoder(publicRR.Body).Decode(&publicView); err != nil {
		t.Fatalf("Decode() public room response error = %v", err)
	}
	if publicView.StepIntervalMS != 1200 {
		t.Fatalf("expected public room step_interval_ms=1200, got %d", publicView.StepIntervalMS)
	}
	if publicView.DefaultView != "commentary" {
		t.Fatalf("expected public room default_view=commentary, got %q", publicView.DefaultView)
	}
}

func TestArenaHTTPHostRevealFlow(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-reveal-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-reveal-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	revealReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-reveal-room/reveal", bytes.NewReader([]byte(`{"host_token":"host-token","scope":"red"}`)))
	revealReq.Header.Set("Content-Type", "application/json")
	revealRR := httptest.NewRecorder()
	app.routes().ServeHTTP(revealRR, revealReq)
	if revealRR.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/reveal scope=red expected 200, got %d body=%s", revealRR.Code, revealRR.Body.String())
	}

	var hostView struct {
		Room struct {
			RevealState string `json:"reveal_state"`
		} `json:"room"`
	}
	if err := json.NewDecoder(revealRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() reveal response error = %v", err)
	}
	if hostView.Room.RevealState != string(RevealStatePartial) {
		t.Fatalf("expected host reveal_state=%q after scope=red, got %q", string(RevealStatePartial), hostView.Room.RevealState)
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-reveal-room", nil)
	publicRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicRR, publicReq)
	if publicRR.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code} expected 200, got %d body=%s", publicRR.Code, publicRR.Body.String())
	}

	var publicView struct {
		RevealState string `json:"reveal_state"`
	}
	if err := json.NewDecoder(publicRR.Body).Decode(&publicView); err != nil {
		t.Fatalf("Decode() public room response error = %v", err)
	}
	if publicView.RevealState != string(RevealStatePartial) {
		t.Fatalf("expected public reveal_state=%q after scope=red, got %q", string(RevealStatePartial), publicView.RevealState)
	}

	revealAllReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-reveal-room/reveal", bytes.NewReader([]byte(`{"host_token":"host-token","scope":"all"}`)))
	revealAllReq.Header.Set("Content-Type", "application/json")
	revealAllRR := httptest.NewRecorder()
	app.routes().ServeHTTP(revealAllRR, revealAllReq)
	if revealAllRR.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/reveal scope=all expected 200, got %d body=%s", revealAllRR.Code, revealAllRR.Body.String())
	}

	var hostViewAll struct {
		Room struct {
			RevealState string `json:"reveal_state"`
		} `json:"room"`
	}
	if err := json.NewDecoder(revealAllRR.Body).Decode(&hostViewAll); err != nil {
		t.Fatalf("Decode() reveal all response error = %v", err)
	}
	if hostViewAll.Room.RevealState != string(RevealStateFull) {
		t.Fatalf("expected host reveal_state=%q after scope=all, got %q", string(RevealStateFull), hostViewAll.Room.RevealState)
	}
}

func TestArenaHTTPHostSettingsRejectsNonHostMutation(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-settings-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	publicBeforeReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-settings-auth-room", nil)
	publicBeforeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicBeforeRR, publicBeforeReq)
	if publicBeforeRR.Code != http.StatusOK {
		t.Fatalf("GET public room before mutation expected 200, got %d body=%s", publicBeforeRR.Code, publicBeforeRR.Body.String())
	}
	var before struct {
		StepIntervalMS int    `json:"step_interval_ms"`
		DefaultView    string `json:"default_view"`
	}
	if err := json.NewDecoder(publicBeforeRR.Body).Decode(&before); err != nil {
		t.Fatalf("Decode() public before response error = %v", err)
	}

	settingsReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-settings-auth-room/settings", bytes.NewReader([]byte(`{"host_token":"wrong-token","step_interval_ms":1200,"default_view":"commentary"}`)))
	settingsReq.Header.Set("Content-Type", "application/json")
	settingsRR := httptest.NewRecorder()
	app.routes().ServeHTTP(settingsRR, settingsReq)
	if settingsRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/settings with wrong token expected 400, got %d body=%s", settingsRR.Code, settingsRR.Body.String())
	}

	publicAfterReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-settings-auth-room", nil)
	publicAfterRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicAfterRR, publicAfterReq)
	if publicAfterRR.Code != http.StatusOK {
		t.Fatalf("GET public room after mutation expected 200, got %d body=%s", publicAfterRR.Code, publicAfterRR.Body.String())
	}
	var after struct {
		StepIntervalMS int    `json:"step_interval_ms"`
		DefaultView    string `json:"default_view"`
	}
	if err := json.NewDecoder(publicAfterRR.Body).Decode(&after); err != nil {
		t.Fatalf("Decode() public after response error = %v", err)
	}

	if after.StepIntervalMS != before.StepIntervalMS {
		t.Fatalf("expected step_interval_ms unchanged on failed settings mutation, before=%d after=%d", before.StepIntervalMS, after.StepIntervalMS)
	}
	if after.DefaultView != before.DefaultView {
		t.Fatalf("expected default_view unchanged on failed settings mutation, before=%q after=%q", before.DefaultView, after.DefaultView)
	}
}

func TestArenaHTTPHostRevealRejectsNonHostMutation(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-reveal-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-reveal-auth-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	publicBeforeReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-reveal-auth-room", nil)
	publicBeforeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicBeforeRR, publicBeforeReq)
	if publicBeforeRR.Code != http.StatusOK {
		t.Fatalf("GET public room before reveal mutation expected 200, got %d body=%s", publicBeforeRR.Code, publicBeforeRR.Body.String())
	}
	var before struct {
		RevealState string `json:"reveal_state"`
	}
	if err := json.NewDecoder(publicBeforeRR.Body).Decode(&before); err != nil {
		t.Fatalf("Decode() public before response error = %v", err)
	}

	revealReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-reveal-auth-room/reveal", bytes.NewReader([]byte(`{"host_token":"wrong-token","scope":"all"}`)))
	revealReq.Header.Set("Content-Type", "application/json")
	revealRR := httptest.NewRecorder()
	app.routes().ServeHTTP(revealRR, revealReq)
	if revealRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/reveal with wrong token expected 400, got %d body=%s", revealRR.Code, revealRR.Body.String())
	}

	publicAfterReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-reveal-auth-room", nil)
	publicAfterRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicAfterRR, publicAfterReq)
	if publicAfterRR.Code != http.StatusOK {
		t.Fatalf("GET public room after reveal mutation expected 200, got %d body=%s", publicAfterRR.Code, publicAfterRR.Body.String())
	}
	var after struct {
		RevealState string `json:"reveal_state"`
	}
	if err := json.NewDecoder(publicAfterRR.Body).Decode(&after); err != nil {
		t.Fatalf("Decode() public after response error = %v", err)
	}

	if after.RevealState != before.RevealState {
		t.Fatalf("expected reveal_state unchanged on failed reveal mutation, before=%q after=%q", before.RevealState, after.RevealState)
	}
}

func TestArenaHTTPHostMatchStartRejectsNonHostMutation(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-match-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-match-auth-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	publicBeforeReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-match-auth-room", nil)
	publicBeforeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicBeforeRR, publicBeforeReq)
	if publicBeforeRR.Code != http.StatusOK {
		t.Fatalf("GET public room before match mutation expected 200, got %d body=%s", publicBeforeRR.Code, publicBeforeRR.Body.String())
	}
	var before struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(publicBeforeRR.Body).Decode(&before); err != nil {
		t.Fatalf("Decode() public before response error = %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-match-auth-room/match/start", bytes.NewReader([]byte(`{"host_token":"wrong-token"}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRR := httptest.NewRecorder()
	app.routes().ServeHTTP(startRR, startReq)
	if startRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/match/start with wrong token expected 400, got %d body=%s", startRR.Code, startRR.Body.String())
	}

	publicAfterReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-match-auth-room", nil)
	publicAfterRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicAfterRR, publicAfterReq)
	if publicAfterRR.Code != http.StatusOK {
		t.Fatalf("GET public room after match mutation expected 200, got %d body=%s", publicAfterRR.Code, publicAfterRR.Body.String())
	}
	var after struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(publicAfterRR.Body).Decode(&after); err != nil {
		t.Fatalf("Decode() public after response error = %v", err)
	}
	if after.Status != before.Status {
		t.Fatalf("expected room status unchanged on failed match start mutation, before=%q after=%q", before.Status, after.Status)
	}
	if after.Status != string(RoomStatusWaiting) {
		t.Fatalf("expected room status to remain waiting, got %q", after.Status)
	}

	publicMatchReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-match-auth-room/match", nil)
	publicMatchRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicMatchRR, publicMatchReq)
	if publicMatchRR.Code != http.StatusNotFound {
		t.Fatalf("GET /api/arena/{code}/match expected 404 after failed start, got %d body=%s", publicMatchRR.Code, publicMatchRR.Body.String())
	}
}

func TestArenaHTTPHostSeatAssignRejectsNonHostMutation(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-seat-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-seat-auth-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	publicBeforeReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-seat-auth-room", nil)
	publicBeforeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicBeforeRR, publicBeforeReq)
	if publicBeforeRR.Code != http.StatusOK {
		t.Fatalf("GET public room before seat mutation expected 200, got %d body=%s", publicBeforeRR.Code, publicBeforeRR.Body.String())
	}
	var before struct {
		Seats map[string]struct {
			ParticipantID string `json:"participant_id"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(publicBeforeRR.Body).Decode(&before); err != nil {
		t.Fatalf("Decode() public before response error = %v", err)
	}
	beforeRedID := before.Seats[string(SeatRedPlayer)].ParticipantID

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-seat-auth-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"wrong-token","seat":"red_player","binding":{"real_type":"picoclaw","name":"bad","base_url":"http://127.0.0.1:9000","api_key":"x","public_alias":"bad"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/seats/assign with wrong token expected 400, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	publicAfterReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-seat-auth-room", nil)
	publicAfterRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicAfterRR, publicAfterReq)
	if publicAfterRR.Code != http.StatusOK {
		t.Fatalf("GET public room after seat mutation expected 200, got %d body=%s", publicAfterRR.Code, publicAfterRR.Body.String())
	}
	var after struct {
		Seats map[string]struct {
			ParticipantID string `json:"participant_id"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(publicAfterRR.Body).Decode(&after); err != nil {
		t.Fatalf("Decode() public after response error = %v", err)
	}
	afterRedID := after.Seats[string(SeatRedPlayer)].ParticipantID
	if afterRedID != beforeRedID {
		t.Fatalf("expected red seat participant unchanged on failed seat assign mutation, before=%q after=%q", beforeRedID, afterRedID)
	}
}

func TestArenaHTTPHostSeatRemoveRejectsNonHostMutation(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-seat-remove-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"host-seat-remove-auth-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	publicBeforeReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-seat-remove-auth-room", nil)
	publicBeforeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicBeforeRR, publicBeforeReq)
	if publicBeforeRR.Code != http.StatusOK {
		t.Fatalf("GET public room before seat remove mutation expected 200, got %d body=%s", publicBeforeRR.Code, publicBeforeRR.Body.String())
	}
	var before struct {
		Seats map[string]struct {
			ParticipantID string `json:"participant_id"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(publicBeforeRR.Body).Decode(&before); err != nil {
		t.Fatalf("Decode() public before response error = %v", err)
	}
	beforeRedID := before.Seats[string(SeatRedPlayer)].ParticipantID
	if beforeRedID == "" {
		t.Fatalf("expected red seat to be occupied before failed remove mutation")
	}

	removeReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-seat-remove-auth-room/seats/remove", bytes.NewReader([]byte(`{"host_token":"wrong-token","seat":"red_player"}`)))
	removeReq.Header.Set("Content-Type", "application/json")
	removeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(removeRR, removeReq)
	if removeRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/seats/remove with wrong token expected 400, got %d body=%s", removeRR.Code, removeRR.Body.String())
	}

	publicAfterReq := httptest.NewRequest(http.MethodGet, "/api/arena/host-seat-remove-auth-room", nil)
	publicAfterRR := httptest.NewRecorder()
	app.routes().ServeHTTP(publicAfterRR, publicAfterReq)
	if publicAfterRR.Code != http.StatusOK {
		t.Fatalf("GET public room after seat remove mutation expected 200, got %d body=%s", publicAfterRR.Code, publicAfterRR.Body.String())
	}
	var after struct {
		Seats map[string]struct {
			ParticipantID string `json:"participant_id"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(publicAfterRR.Body).Decode(&after); err != nil {
		t.Fatalf("Decode() public after response error = %v", err)
	}
	afterRedID := after.Seats[string(SeatRedPlayer)].ParticipantID
	if afterRedID != beforeRedID {
		t.Fatalf("expected red seat participant unchanged on failed seat remove mutation, before=%q after=%q", beforeRedID, afterRedID)
	}
}

func TestArenaHTTPHumanMoveSubmissionFlow(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"http-move-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("host enter expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"http-move-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("guest enter expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	startReq := httptest.NewRequest(http.MethodPost, "/api/arena/http-move-room/match/start", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRR := httptest.NewRecorder()
	app.routes().ServeHTTP(startRR, startReq)
	if startRR.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/match/start expected 200, got %d body=%s", startRR.Code, startRR.Body.String())
	}

	moveReq := httptest.NewRequest(http.MethodPost, "/api/arena/http-move-room/move", bytes.NewReader([]byte(`{"client_token":"guest-token","move":"a6-a5"}`)))
	moveReq.Header.Set("Content-Type", "application/json")
	moveRR := httptest.NewRecorder()
	app.routes().ServeHTTP(moveRR, moveReq)
	if moveRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/arena/{code}/move with wrong player expected 400, got %d body=%s", moveRR.Code, moveRR.Body.String())
	}

	moveReq = httptest.NewRequest(http.MethodPost, "/api/arena/http-move-room/move", bytes.NewReader([]byte(`{"client_token":"host-token","move":"a6-a5"}`)))
	moveReq.Header.Set("Content-Type", "application/json")
	moveRR = httptest.NewRecorder()
	app.routes().ServeHTTP(moveRR, moveReq)
	if moveRR.Code != http.StatusOK {
		t.Fatalf("POST /api/arena/{code}/move expected 200, got %d body=%s", moveRR.Code, moveRR.Body.String())
	}

	var moveResp struct {
		LastMove string `json:"last_move"`
		Turn     string `json:"turn"`
	}
	if err := json.NewDecoder(moveRR.Body).Decode(&moveResp); err != nil {
		t.Fatalf("Decode() move response error = %v", err)
	}
	if moveResp.LastMove != "a6-a5" {
		t.Fatalf("expected move response last_move a6-a5, got %q", moveResp.LastMove)
	}
	if moveResp.Turn != string(SideBlack) {
		t.Fatalf("expected move response turn black, got %q", moveResp.Turn)
	}
}

func TestStaticAssetsAreServedFromAnyWorkingDirectory(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tempDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore Chdir(%q) error = %v", originalWD, err)
		}
	})

	for _, path := range []string{"/", "/static/style.css", "/static/app.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		app.routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s expected 200 from cwd=%q, got %d body=%s", path, tempDir, rr.Code, rr.Body.String())
		}
	}
}

func TestStaticAssetsAreServed(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Pico Xiangqi Arena") {
		t.Fatalf("expected index page to contain app title")
	}

	for _, path := range []string{"/static/style.css", "/static/app.js"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rr = httptest.NewRecorder()
		app.routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s expected 200, got %d", path, rr.Code)
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("GET %s returned empty body", path)
		}
		if path == "/static/app.js" {
			body := rr.Body.String()
			if !strings.Contains(body, "DOMContentLoaded") {
				t.Fatalf("expected %s to register a DOM ready bootstrapping hook", path)
			}
			if !strings.Contains(body, "/api/arena/enter") {
				t.Fatalf("expected %s to include arena enter API wiring", path)
			}
			if !strings.Contains(body, "syncSeatAPIKeyCacheWithHostRoom") {
				t.Fatalf("expected %s to include host seat api key cache sync guard", path)
			}
			if !strings.Contains(body, "delete state.hostSeatAPIKeyCache[seat]") {
				t.Fatalf("expected %s to include host seat api key cache invalidation", path)
			}
			if !strings.Contains(body, "/move") {
				t.Fatalf("expected %s to include human move submit endpoint wiring", path)
			}
			if !strings.Contains(body, "state.selectedFrom") {
				t.Fatalf("expected %s to include board source selection state", path)
			}
			if !strings.Contains(body, "/picoclaw/") {
				t.Fatalf("expected %s to include picoclaw participant mode route wiring", path)
			}
			if !strings.Contains(body, "runtime-mode-save-btn") {
				t.Fatalf("expected %s to include host runtime mode controls", path)
			}
			if !strings.Contains(body, "帅") || !strings.Contains(body, "卒") {
				t.Fatalf("expected %s to include xiangqi piece label mapping", path)
			}
		}
	}
}

func TestStaticAppWiresJoinAndPublicPollingFlow(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	for _, target := range []string{
		`id="create-room-btn"`,
		`id="join-room-btn"`,
		`id="room-code-badge"`,
		`id="room-status-badge"`,
		`id="interval-badge"`,
		`id="reveal-badge"`,
		`id="seat-red-card"`,
		`id="seat-black-card"`,
		`id="board-grid"`,
		`id="event-list"`,
		`id="participant-list"`,
		`id="picoclaw-runtime-list"`,
	} {
		if !strings.Contains(body, target) {
			t.Fatalf("expected static shell to include %q for room entry/public rendering flow", target)
		}
	}
}

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

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/heartbeat-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostView struct {
		Room struct {
			Seats map[string]struct {
				ParticipantID string `json:"participant_id"`
			} `json:"seats"`
		} `json:"room"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() host view error = %v", err)
	}
	participantID := hostView.Room.Seats[string(SeatBlackPlayer)].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	openReq := httptest.NewRequest(http.MethodPost, "/api/arena/heartbeat-room/picoclaw/"+participantID+"/session/open", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	openReq.Header.Set("Content-Type", "application/json")
	openRR := httptest.NewRecorder()
	app.routes().ServeHTTP(openRR, openReq)
	if openRR.Code != http.StatusOK {
		t.Fatalf("open expected 200, got %d body=%s", openRR.Code, openRR.Body.String())
	}

	var openView PicoclawRuntimeState
	if err := json.NewDecoder(openRR.Body).Decode(&openView); err != nil {
		t.Fatalf("Decode() open view error = %v", err)
	}
	if openView.SessionState != PicoclawSessionStateOpening {
		t.Fatalf("expected session_state opening after open, got %q", openView.SessionState)
	}
	if strings.TrimSpace(openView.SessionID) == "" {
		t.Fatalf("expected open to create session_id")
	}
	if strings.TrimSpace(openView.SessionAuthToken) == "" {
		t.Fatalf("expected open to create session_auth_token")
	}

	requestedTTL := 45 * time.Second
	tolerance := 3 * time.Second
	beforeHeartbeat := time.Now()
	heartbeatReq := httptest.NewRequest(http.MethodPost, "/api/arena/heartbeat-room/picoclaw/"+participantID+"/session/heartbeat", bytes.NewReader([]byte(`{"session_id":"`+openView.SessionID+`","session_token":"`+openView.SessionAuthToken+`","lease_ttl_ms":45000}`)))
	heartbeatReq.Header.Set("Content-Type", "application/json")
	heartbeatRR := httptest.NewRecorder()
	app.routes().ServeHTTP(heartbeatRR, heartbeatReq)
	afterHeartbeat := time.Now()
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
	if heartbeatView.LastHeartbeatAt.IsZero() {
		t.Fatalf("expected last_heartbeat_at to be set")
	}
	if heartbeatView.LeaseExpiresAt.IsZero() {
		t.Fatalf("expected lease_expires_at to be set")
	}
	if !heartbeatView.LeaseExpiresAt.After(heartbeatView.LastHeartbeatAt) {
		t.Fatalf("expected lease_expires_at (%s) after last_heartbeat_at (%s)", heartbeatView.LeaseExpiresAt.Format(time.RFC3339Nano), heartbeatView.LastHeartbeatAt.Format(time.RFC3339Nano))
	}
	earliestExpected := beforeHeartbeat.Add(requestedTTL - tolerance)
	latestExpected := afterHeartbeat.Add(requestedTTL + tolerance)
	if heartbeatView.LeaseExpiresAt.Before(earliestExpected) || heartbeatView.LeaseExpiresAt.After(latestExpected) {
		t.Fatalf(
			"expected lease_expires_at near requested ttl: got=%s expected_range=[%s,%s]",
			heartbeatView.LeaseExpiresAt.Format(time.RFC3339Nano),
			earliestExpected.Format(time.RFC3339Nano),
			latestExpected.Format(time.RFC3339Nano),
		)
	}
}

func TestPicoclawModeRouteRejectsNonHostWith403(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-auth-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-auth-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-auth-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18888"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/mode-auth-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostView struct {
		Room struct {
			Seats map[string]struct {
				ParticipantID string `json:"participant_id"`
			} `json:"seats"`
		} `json:"room"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() host view error = %v", err)
	}
	participantID := hostView.Room.Seats[string(SeatBlackPlayer)].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	modeReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-auth-room/picoclaw/"+participantID+"/mode", bytes.NewReader([]byte(`{"host_token":"guest-token","preferred_mode":"prefer_session"}`)))
	modeReq.Header.Set("Content-Type", "application/json")
	modeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(modeRR, modeReq)
	if modeRR.Code != http.StatusForbidden {
		t.Fatalf("mode route expected 403 for non-host token, got %d body=%s", modeRR.Code, modeRR.Body.String())
	}
}

func TestPicoclawTurnRouteDeliversAndAcceptsSessionMoves(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	hostView, err := app.arena.Enter(EnterRequest{
		RoomCode:    "turn-route-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := app.arena.Enter(EnterRequest{
		RoomCode:    "turn-route-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := app.arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := app.arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	hostRoom, err := app.arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := app.arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	opened, err := app.arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if _, err := app.arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}
	if _, err := app.arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := app.arena.SubmitMove(hostView.Room.Code, "host-token", "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		pollBody := []byte(`{"session_id":"` + opened.SessionID + `","session_token":"` + opened.SessionAuthToken + `","wait_ms":5000}`)
		pollReq := httptest.NewRequest(http.MethodPost, "/api/arena/turn-route-room/picoclaw/"+participantID+"/turn", bytes.NewReader(pollBody))
		pollReq.Header.Set("Content-Type", "application/json")
		pollRR := httptest.NewRecorder()
		app.routes().ServeHTTP(pollRR, pollReq)
		if pollRR.Code != http.StatusOK {
			done <- errString("poll expected 200, got " + pollRR.Body.String())
			return
		}
		var pollView PicoclawSessionTurnResponse
		if err := json.NewDecoder(pollRR.Body).Decode(&pollView); err != nil {
			done <- err
			return
		}
		if pollView.Status != PicoclawSessionTurnStatusTurn || pollView.Turn == nil {
			done <- errString("expected turn payload from /turn")
			return
		}

		submitBody := []byte(`{"session_id":"` + opened.SessionID + `","session_token":"` + opened.SessionAuthToken + `","turn_id":"` + pollView.Turn.TurnID + `","move":"a3-a4","reply":"MOVE: a3-a4"}`)
		submitReq := httptest.NewRequest(http.MethodPost, "/api/arena/turn-route-room/picoclaw/"+participantID+"/turn", bytes.NewReader(submitBody))
		submitReq.Header.Set("Content-Type", "application/json")
		submitRR := httptest.NewRecorder()
		app.routes().ServeHTTP(submitRR, submitReq)
		if submitRR.Code != http.StatusOK {
			done <- errString("submit expected 200, got " + submitRR.Body.String())
			return
		}
		done <- nil
	}()

	forceRoomReadyForAdvance(t, app.arena, hostView.Room.Code)
	if err := app.arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("turn route flow error = %v", err)
	}

	matchView, err := app.arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected turn route move a3-a4, got %q", matchView.LastMove)
	}
}

func TestPicoclawInviteRouteUsesRequestBaseURL(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	var messageReq picoMessageRequest
	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&messageReq); err != nil {
			t.Fatalf("Decode() invite request error = %v", err)
		}
		writeJSON(w, http.StatusOK, picoMessageResponse{Reply: "invite-ok"})
	}))
	defer messageServer.Close()

	hostView, err := app.arena.Enter(EnterRequest{
		RoomCode:    "invite-route-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := app.arena.Enter(EnterRequest{
		RoomCode:    "invite-route-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := app.arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     messageServer.URL,
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := app.arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	body := []byte(`{"host_token":"host-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/arena/invite-route-room/picoclaw/"+participantID+"/invite", bytes.NewReader(body))
	req.Host = "arena.example.test:18889"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("invite route expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(messageReq.Message, "arena_base_url：https://arena.example.test:18889") {
		t.Fatalf("expected invite prompt to include forwarded base url, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "heartbeat_url：https://arena.example.test:18889/api/arena/invite-route-room/picoclaw/"+participantID+"/session/heartbeat") {
		t.Fatalf("expected invite prompt to include heartbeat_url, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "turn_url：https://arena.example.test:18889/api/arena/invite-route-room/picoclaw/"+participantID+"/turn") {
		t.Fatalf("expected invite prompt to include turn_url, got %q", messageReq.Message)
	}
}

func errString(msg string) error {
	return &httpRouteTestError{msg: msg}
}

type httpRouteTestError struct {
	msg string
}

func (e *httpRouteTestError) Error() string {
	return e.msg
}

func TestPicoclawModeRouteUpdatesManagedRuntimeState(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-success-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-success-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-success-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18888"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/mode-success-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostView struct {
		Room struct {
			Seats map[string]struct {
				ParticipantID string `json:"participant_id"`
			} `json:"seats"`
		} `json:"room"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() host view error = %v", err)
	}
	participantID := hostView.Room.Seats[string(SeatBlackPlayer)].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	modeReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-success-room/picoclaw/"+participantID+"/mode", bytes.NewReader([]byte(`{"host_token":"host-token","preferred_mode":"prefer_session"}`)))
	modeReq.Header.Set("Content-Type", "application/json")
	modeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(modeRR, modeReq)
	if modeRR.Code != http.StatusOK {
		t.Fatalf("mode route expected 200, got %d body=%s", modeRR.Code, modeRR.Body.String())
	}

	var modeView PicoclawRuntimeState
	if err := json.NewDecoder(modeRR.Body).Decode(&modeView); err != nil {
		t.Fatalf("Decode() mode view error = %v", err)
	}
	if modeView.ParticipantID != participantID {
		t.Fatalf("expected participant_id %q, got %q", participantID, modeView.ParticipantID)
	}
	if modeView.PreferredMode != PicoclawModePreferSession {
		t.Fatalf("expected preferred_mode prefer_session, got %q", modeView.PreferredMode)
	}

	verifyHostReq := httptest.NewRequest(http.MethodGet, "/api/arena/mode-success-room/host?token=host-token", nil)
	verifyHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(verifyHostRR, verifyHostReq)
	if verifyHostRR.Code != http.StatusOK {
		t.Fatalf("host room verify expected 200, got %d body=%s", verifyHostRR.Code, verifyHostRR.Body.String())
	}

	var verifyHostView struct {
		Runtime map[string]PicoclawRuntimeState `json:"runtime"`
	}
	if err := json.NewDecoder(verifyHostRR.Body).Decode(&verifyHostView); err != nil {
		t.Fatalf("Decode() verify host view error = %v", err)
	}
	persisted, ok := verifyHostView.Runtime[participantID]
	if !ok {
		t.Fatalf("expected persisted runtime for participant %q", participantID)
	}
	if persisted.PreferredMode != PicoclawModePreferSession {
		t.Fatalf("expected persisted preferred_mode prefer_session, got %q", persisted.PreferredMode)
	}
}

func TestPicoclawModeRouteAcceptsPreferPicoWS(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-pico-ws-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"mode-pico-ws-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-pico-ws-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18790"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/mode-pico-ws-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostView struct {
		Room struct {
			Seats map[string]struct {
				ParticipantID string `json:"participant_id"`
			} `json:"seats"`
		} `json:"room"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() host view error = %v", err)
	}
	participantID := hostView.Room.Seats[string(SeatBlackPlayer)].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	modeReq := httptest.NewRequest(http.MethodPost, "/api/arena/mode-pico-ws-room/picoclaw/"+participantID+"/mode", bytes.NewReader([]byte(`{"host_token":"host-token","preferred_mode":"prefer_pico_ws"}`)))
	modeReq.Header.Set("Content-Type", "application/json")
	modeRR := httptest.NewRecorder()
	app.routes().ServeHTTP(modeRR, modeReq)
	if modeRR.Code != http.StatusOK {
		t.Fatalf("mode route expected 200, got %d body=%s", modeRR.Code, modeRR.Body.String())
	}

	var modeView PicoclawRuntimeState
	if err := json.NewDecoder(modeRR.Body).Decode(&modeView); err != nil {
		t.Fatalf("Decode() mode view error = %v", err)
	}
	if modeView.PreferredMode != PicoclawModePreferPicoWS {
		t.Fatalf("expected preferred_mode prefer_pico_ws, got %q", modeView.PreferredMode)
	}
}

func TestHostRoomIncludesPicoclawRuntimeDiagnostics(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	enterHostReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"runtime-room","client_token":"host-token","join_intent":"player"}`)))
	enterHostReq.Header.Set("Content-Type", "application/json")
	enterHostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterHostRR, enterHostReq)
	if enterHostRR.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", enterHostRR.Code, enterHostRR.Body.String())
	}

	enterGuestReq := httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"runtime-room","client_token":"guest-token","join_intent":"player"}`)))
	enterGuestReq.Header.Set("Content-Type", "application/json")
	enterGuestRR := httptest.NewRecorder()
	app.routes().ServeHTTP(enterGuestRR, enterGuestReq)
	if enterGuestRR.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", enterGuestRR.Code, enterGuestRR.Body.String())
	}

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/runtime-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"host-token","seat":"black_player","binding":{"real_type":"picoclaw","name":"black pico","public_alias":"黑雨伞","connection":"managed","base_url":"http://127.0.0.1:18888"}}`)))
	assignReq.Header.Set("Content-Type", "application/json")
	assignRR := httptest.NewRecorder()
	app.routes().ServeHTTP(assignRR, assignReq)
	if assignRR.Code != http.StatusOK {
		t.Fatalf("assign expected 200, got %d body=%s", assignRR.Code, assignRR.Body.String())
	}

	hostReq := httptest.NewRequest(http.MethodGet, "/api/arena/runtime-room/host?token=host-token", nil)
	hostRR := httptest.NewRecorder()
	app.routes().ServeHTTP(hostRR, hostReq)
	if hostRR.Code != http.StatusOK {
		t.Fatalf("host room expected 200, got %d body=%s", hostRR.Code, hostRR.Body.String())
	}

	var hostView struct {
		Room struct {
			Seats map[string]struct {
				ParticipantID string `json:"participant_id"`
			} `json:"seats"`
		} `json:"room"`
		Runtime map[string]PicoclawRuntimeState `json:"runtime"`
	}
	if err := json.NewDecoder(hostRR.Body).Decode(&hostView); err != nil {
		t.Fatalf("Decode() host view error = %v", err)
	}

	participantID := hostView.Room.Seats[string(SeatBlackPlayer)].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	runtime, ok := hostView.Runtime[participantID]
	if !ok {
		t.Fatalf("expected runtime diagnostics for managed participant %q", participantID)
	}
	if runtime.ParticipantID != participantID {
		t.Fatalf("expected runtime participant_id=%q, got %q", participantID, runtime.ParticipantID)
	}
	if runtime.PreferredMode != PicoclawModeAuto {
		t.Fatalf("expected preferred_mode auto, got %q", runtime.PreferredMode)
	}
	if runtime.ActiveMode != PicoclawActiveModeMessage {
		t.Fatalf("expected active_mode message before session heartbeat, got %q", runtime.ActiveMode)
	}
	if runtime.SessionState != PicoclawSessionStateIdle {
		t.Fatalf("expected session_state idle for new managed runtime, got %q", runtime.SessionState)
	}
}
