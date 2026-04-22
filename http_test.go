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
	}
}
