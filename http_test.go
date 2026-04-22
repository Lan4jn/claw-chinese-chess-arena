package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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

func TestAdminTransportModeSwitchAffectsFutureMatchesOnly(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/transport", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/transport expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var initial struct {
		DefaultMode   string `json:"default_mode"`
		ConfigVersion int    `json:"config_version"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&initial); err != nil {
		t.Fatalf("Decode() initial transport config error = %v", err)
	}
	if initial.DefaultMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected default transport mode http_session, got %q", initial.DefaultMode)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"transport-api-room","client_token":"host-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter host expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var hostEnter struct {
		Participant struct {
			ID string `json:"id"`
		} `json:"participant"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&hostEnter); err != nil {
		t.Fatalf("Decode() host enter error = %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/enter", bytes.NewReader([]byte(`{"room_code":"transport-api-room","client_token":"guest-token","join_intent":"player"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enter guest expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/arena/transport-api-room/match/start", bytes.NewReader([]byte(`{"host_token":"host-token"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("start match expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var firstMatch struct {
		TransportMode string `json:"transport_mode"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&firstMatch); err != nil {
		t.Fatalf("Decode() first match error = %v", err)
	}
	if firstMatch.TransportMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected first match transport_mode http_session, got %q", firstMatch.TransportMode)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/transport", bytes.NewReader([]byte(`{"default_mode":"websocket"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/admin/transport expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var updated struct {
		DefaultMode   string `json:"default_mode"`
		ConfigVersion int    `json:"config_version"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatalf("Decode() updated transport config error = %v", err)
	}
	if updated.DefaultMode != string(TransportModeWebSocket) {
		t.Fatalf("expected updated transport mode websocket, got %q", updated.DefaultMode)
	}
	if updated.ConfigVersion <= initial.ConfigVersion {
		t.Fatalf("expected config version to increase, before=%d after=%d", initial.ConfigVersion, updated.ConfigVersion)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/arena/transport-api-room/host/match?token=host-token", nil)
	rr = httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/arena/{code}/host/match expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var hostMatch struct {
		TransportMode       string `json:"transport_mode"`
		TransportActiveMode string `json:"transport_active_mode"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&hostMatch); err != nil {
		t.Fatalf("Decode() host match error = %v", err)
	}
	if hostMatch.TransportMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected existing match to keep http_session, got %q", hostMatch.TransportMode)
	}
	if hostMatch.TransportActiveMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected existing active mode to stay http_session, got %q", hostMatch.TransportActiveMode)
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

	assignReq := httptest.NewRequest(http.MethodPost, "/api/arena/host-seat-auth-room/seats/assign", bytes.NewReader([]byte(`{"host_token":"wrong-token","seat":"red_player","binding":{"real_type":"pico","name":"bad","base_url":"http://127.0.0.1:9000","api_key":"x","public_alias":"bad"}}`)))
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
	} {
		if !strings.Contains(body, target) {
			t.Fatalf("expected static shell to include %q for room entry/public rendering flow", target)
		}
	}
}
