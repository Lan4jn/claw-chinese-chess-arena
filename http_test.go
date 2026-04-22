package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		}
	}
}

func TestStaticAppWiresJoinAndPublicPollingFlow(t *testing.T) {
	app := NewApp(NewMemorySnapshotStore())

	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /static/app.js expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	for _, snippet := range []string{
		"function handleJoin()",
		"function refreshAll()",
		"function startPolling()",
		"/api/arena/enter",
		"/api/arena/\" + encodeURIComponent(state.roomCode)",
		"/match",
	} {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected /static/app.js to include %q for room entry/public polling flow", snippet)
		}
	}
}
