package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type failOnDemandSnapshotStore struct {
	base *MemorySnapshotStore
	fail bool
}

func newFailOnDemandSnapshotStore() *failOnDemandSnapshotStore {
	return &failOnDemandSnapshotStore{
		base: NewMemorySnapshotStore(),
	}
}

func (s *failOnDemandSnapshotStore) Load() (*ArenaSnapshot, error) {
	return s.base.Load()
}

func (s *failOnDemandSnapshotStore) Save(snapshot *ArenaSnapshot) error {
	if s.fail {
		return errors.New("save failed")
	}
	return s.base.Save(snapshot)
}

func forceRoomReadyForAdvance(t *testing.T, arena *Arena, roomCode string) {
	t.Helper()
	arena.mu.Lock()
	defer arena.mu.Unlock()
	room := arena.rooms[normalizeRoomCode(roomCode)]
	if room == nil {
		t.Fatalf("room %q not found", roomCode)
	}
	room.NextActionAt = time.Time{}
}

func TestArenaEnterCreatesRoomAndAssignsHost(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	view, err := arena.Enter(EnterRequest{
		RoomCode:    "duququ",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentAuto,
	})
	if err != nil {
		t.Fatalf("Enter() error = %v", err)
	}
	if !view.IsHost {
		t.Fatalf("expected first entrant to become host")
	}
	if view.Room.Code != "duququ" {
		t.Fatalf("expected room code duququ, got %q", view.Room.Code)
	}
	if view.Room.HostParticipantID == "" {
		t.Fatalf("expected host participant id to be assigned")
	}
}

func TestArenaSeatsFirstTwoPlayersAndDowngradesOthersToSpectator(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	first, err := arena.Enter(EnterRequest{
		RoomCode:    "ring",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("first enter error = %v", err)
	}
	second, err := arena.Enter(EnterRequest{
		RoomCode:    "ring",
		ClientToken: "guest-1",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("second enter error = %v", err)
	}
	third, err := arena.Enter(EnterRequest{
		RoomCode:    "ring",
		ClientToken: "guest-2",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("third enter error = %v", err)
	}

	if first.Participant.Seat != SeatRedPlayer {
		t.Fatalf("expected first player to take red seat, got %q", first.Participant.Seat)
	}
	if second.Participant.Seat != SeatBlackPlayer {
		t.Fatalf("expected second player to take black seat, got %q", second.Participant.Seat)
	}
	if third.Participant.Seat != SeatSpectator {
		t.Fatalf("expected third player to become spectator, got %q", third.Participant.Seat)
	}
}

func TestArenaPublicViewHidesIdentityUntilReveal(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "mask",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}

	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "mask",
		ClientToken: "guest",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}

	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatRedPlayer, AgentBinding{
		RealType: AgentTypePicoclaw,
		Name:     "本地 Pico",
	}); err != nil {
		t.Fatalf("bind red error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType: AgentTypeHuman,
		Name:     "真人选手",
	}); err != nil {
		t.Fatalf("bind black error = %v", err)
	}

	publicView, err := arena.PublicRoom(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicRoom() error = %v", err)
	}
	if publicView.Seats[SeatRedPlayer].RealType != "" || publicView.Seats[SeatBlackPlayer].RealType != "" {
		t.Fatalf("expected public view to hide real types before reveal")
	}

	if err := arena.UpdateReveal(hostView.Room.Code, hostView.Participant.ID, RevealStateFull); err != nil {
		t.Fatalf("UpdateReveal() error = %v", err)
	}
	publicView, err = arena.PublicRoom(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicRoom() after reveal error = %v", err)
	}
	if publicView.Seats[SeatRedPlayer].RealType != AgentTypePicoclaw {
		t.Fatalf("expected red type to be revealed, got %q", publicView.Seats[SeatRedPlayer].RealType)
	}
	if publicView.Seats[SeatBlackPlayer].RealType != AgentTypeHuman {
		t.Fatalf("expected black type to be revealed, got %q", publicView.Seats[SeatBlackPlayer].RealType)
	}

	if guestView.Participant.PublicAlias == "" {
		t.Fatalf("expected guest to have generated alias")
	}
}

func TestBuildMovePromptIncludesArenaRules(t *testing.T) {
	player := PlayerConfig{
		Type:    AgentTypePicoclaw,
		Name:    "台灯",
		BaseURL: "http://localhost:8081",
	}
	state := NewGame()
	legal := []string{"a6-a5", "c6-c5"}
	arenaState := PromptArenaState{
		RoomCode:       "mask-ring",
		StepIntervalMS: 2500,
		OpponentAlias:  "黑雨伞",
	}

	prompt := buildMovePrompt("match-1", player, state, legal, arenaState)
	for _, want := range []string{
		"比赛场地：mask-ring",
		"步间隔：2500ms",
		"对手公开身份：黑雨伞",
		"对手真实身份未知",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestArenaStartMatchUsesConfiguredStepInterval(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "interval-room",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "interval-room",
		ClientToken: "guest",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 4200,
		DefaultView:    "commentary",
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.StepIntervalMS != 4200 {
		t.Fatalf("expected step interval 4200, got %d", matchView.StepIntervalMS)
	}
	if matchView.RoomStatus != RoomStatusPlaying {
		t.Fatalf("expected room status playing, got %q", matchView.RoomStatus)
	}
	if matchView.Seats[SeatRedPlayer].ParticipantID != hostView.Participant.ID {
		t.Fatalf("expected red seat participant to remain host")
	}
	if matchView.Seats[SeatBlackPlayer].ParticipantID != guestView.Participant.ID {
		t.Fatalf("expected black seat participant to remain guest")
	}
}

func TestArenaHumanMoveRequiresCurrentSeatOwner(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "move-room",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "move-room",
		ClientToken: "guest",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, guestView.Participant.ID, "a6-a5"); err == nil {
		t.Fatalf("expected black player to be rejected on red turn")
	}
	matchView, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5")
	if err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}
	if matchView.LastMove != "a6-a5" {
		t.Fatalf("expected last move a6-a5, got %q", matchView.LastMove)
	}
	if matchView.Turn != SideBlack {
		t.Fatalf("expected turn to switch to black, got %q", matchView.Turn)
	}
}

func TestArenaHumanMoveRequiresCurrentSeatOwnerToken(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "move-token-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "move-token-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("guest enter error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, "guest-token", "a6-a5"); err == nil {
		t.Fatalf("expected guest token to be rejected on red turn")
	}
	matchView, err := arena.SubmitMove(hostView.Room.Code, "host-token", "a6-a5")
	if err != nil {
		t.Fatalf("SubmitMove() with host token error = %v", err)
	}
	if matchView.LastMove != "a6-a5" {
		t.Fatalf("expected last move a6-a5, got %q", matchView.LastMove)
	}
	if matchView.Turn != SideBlack {
		t.Fatalf("expected turn to switch to black, got %q", matchView.Turn)
	}
}

func TestArenaAdvanceOnceRequestsAgentMoveOnAgentTurn(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		return "a3-a4", "MOVE: a3-a4", nil
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "agent-room",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "agent-room",
		ClientToken: "guest",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType: AgentTypePicoclaw,
		Name:     "远程 Pico",
		BaseURL:  "http://127.0.0.1:9001",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected agent move a3-a4, got %q", matchView.LastMove)
	}
	if matchView.Turn != SideRed {
		t.Fatalf("expected turn to return to red, got %q", matchView.Turn)
	}
	if matchView.Seats[SeatBlackPlayer].ParticipantID == "" {
		t.Fatalf("expected black seat to stay occupied after agent move")
	}
	if matchView.Seats[SeatBlackPlayer].ParticipantID == guestView.Participant.ID {
		t.Fatalf("expected black seat occupant to be managed participant after binding")
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	foundGuest := false
	for _, participant := range hostRoom.Participants {
		if participant.ID != guestView.Participant.ID {
			continue
		}
		foundGuest = true
		if participant.Seat != SeatSpectator {
			t.Fatalf("expected original guest to remain as spectator, got seat %q", participant.Seat)
		}
		break
	}
	if !foundGuest {
		t.Fatalf("expected original guest participant to remain in room")
	}
}

func TestAssignSeatBindingDoesNotOverwriteExistingHumanParticipant(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "managed-room",
		ClientToken: "host",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("host enter error = %v", err)
	}
	guestView, err := arena.Enter(EnterRequest{
		RoomCode:    "managed-room",
		ClientToken: "guest",
		DisplayName: "真人黑方",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("guest enter error = %v", err)
	}

	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "托管 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:9001",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}

	var guestSeen bool
	var managedSeen bool
	for _, participant := range hostRoom.Participants {
		switch participant.ID {
		case guestView.Participant.ID:
			guestSeen = true
			if participant.RealType != AgentTypeHuman {
				t.Fatalf("expected guest real type to stay human, got %q", participant.RealType)
			}
			if participant.Seat != SeatSpectator {
				t.Fatalf("expected guest to be moved to spectator, got %q", participant.Seat)
			}
			if participant.DisplayName != "真人黑方" {
				t.Fatalf("expected guest display name to be preserved, got %q", participant.DisplayName)
			}
		case hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID:
			managedSeen = true
			if participant.RealType != AgentTypePicoclaw {
				t.Fatalf("expected managed participant to be picoclaw, got %q", participant.RealType)
			}
			if participant.Seat != SeatBlackPlayer {
				t.Fatalf("expected managed participant to occupy black seat, got %q", participant.Seat)
			}
		}
	}
	if !guestSeen {
		t.Fatalf("expected original guest participant to remain in room")
	}
	if !managedSeen {
		t.Fatalf("expected a new managed participant to occupy black seat")
	}
}

func TestArenaAdvanceOnceUsesPicoclawMessageTransportForManagedSeat(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	var received picoMessageRequest
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		move, reply, req, err := askPicoclawForMoveWithRequest(context.Background(), defaultHTTPClient(), matchID, player, state, legal, arenaState)
		received = req
		return move, reply, err
	}

	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("Decode() message request error = %v", err)
		}
		writeJSON(w, http.StatusOK, picoMessageResponse{
			Reply: "MOVE: a3-a4",
		})
	}))
	defer messageServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "http-session-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "http-session-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
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
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, "host-token", "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if received.SessionID == "" {
		t.Fatalf("expected message request to include session_id")
	}
	if received.SenderID != "picoclaw-xiangqi-arena" {
		t.Fatalf("expected sender_id picoclaw-xiangqi-arena, got %q", received.SenderID)
	}
	if !strings.Contains(received.Message, "只能从下面合法走法中选择一个") {
		t.Fatalf("expected prompt to include legal move guidance, got %q", received.Message)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected picoclaw move a3-a4, got %q", matchView.LastMove)
	}
}

func TestAskPicoclawForMoveWithRetryRetriesTransientFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "temporary unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, picoMessageResponse{Reply: "MOVE: a3-a4"})
	}))
	defer server.Close()

	move, reply, req, meta, err := askPicoclawForMoveWithRetry(
		context.Background(),
		defaultHTTPClient(),
		"retry-match",
		PlayerConfig{Name: "黑雨伞", BaseURL: server.URL},
		NewGame(),
		[]string{"a3-a4"},
		PromptArenaState{RoomCode: "retry-room", StepIntervalMS: 1, OpponentAlias: "红灯笼"},
	)
	if err != nil {
		t.Fatalf("askPicoclawForMoveWithRetry() error = %v", err)
	}
	if move != "a3-a4" {
		t.Fatalf("expected move a3-a4, got %q", move)
	}
	if !strings.Contains(reply, "MOVE: a3-a4") {
		t.Fatalf("expected reply to contain move, got %q", reply)
	}
	if req.SessionID == "" {
		t.Fatalf("expected request session_id to be set")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if meta.Attempts != 3 {
		t.Fatalf("expected retry metadata attempts=3, got %d", meta.Attempts)
	}
}

func TestAskPicoclawForMoveWithRetryStopsOnInvalidJSON(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>not-json</html>"))
	}))
	defer server.Close()

	_, reply, _, meta, err := askPicoclawForMoveWithRetry(
		context.Background(),
		defaultHTTPClient(),
		"retry-invalid-json-match",
		PlayerConfig{Name: "黑雨伞", BaseURL: server.URL},
		NewGame(),
		[]string{"a3-a4"},
		PromptArenaState{RoomCode: "retry-invalid-json-room", StepIntervalMS: 1, OpponentAlias: "红灯笼"},
	)
	if err == nil {
		t.Fatalf("expected invalid json error")
	}
	if attempts != 1 {
		t.Fatalf("expected invalid json to stop retries after 1 attempt, got %d", attempts)
	}
	if meta.Attempts != 1 {
		t.Fatalf("expected retry metadata attempts=1, got %d", meta.Attempts)
	}
	if !strings.Contains(reply, "<html>") {
		t.Fatalf("expected raw reply body to be returned, got %q", reply)
	}
}

func TestAskPicoclawForMoveWithRetryReturnsRepetitionErrorForRejectedMove(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, picoMessageResponse{Reply: "MOVE: e2-e1"})
	}))
	defer server.Close()

	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedCheck := stateAfterMove(t, base, "e2-e1")
	afterBlackOut := stateAfterMove(t, afterRedCheck, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "e1-e2")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")

	state := base
	state.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true, RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	_, reply, _, meta, err := askPicoclawForMoveWithRetry(
		context.Background(),
		defaultHTTPClient(),
		"repeat-match",
		PlayerConfig{Name: "黑雨伞", BaseURL: server.URL},
		state,
		state.LegalMoveStrings(),
		PromptArenaState{RoomCode: "repeat-room", StepIntervalMS: 1, OpponentAlias: "红灯笼"},
	)
	if err == nil || err.Error() != "move causes forbidden long-check repetition" {
		t.Fatalf("expected long-check repetition error, got %v", err)
	}
	if reply != "MOVE: e2-e1" {
		t.Fatalf("expected raw reply to be preserved, got %q", reply)
	}
	if meta.Attempts != 1 {
		t.Fatalf("expected non-retryable repetition error, got attempts=%d", meta.Attempts)
	}
}

func TestArenaAdvanceOnceFallsBackFromSessionToMessage(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	callOrder := make([]string, 0, 2)
	sessionCalls := 0
	messageCalls := 0
	arena.requestSessionMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		sessionCalls++
		callOrder = append(callOrder, "session")
		return "", "session unavailable", errors.New("session unavailable")
	}
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		messageCalls++
		callOrder = append(callOrder, "message")
		return "a3-a4", "MOVE: a3-a4", nil
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "session-fallback-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "session-fallback-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
		BaseURL:     "http://127.0.0.1:9001",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if sessionCalls != 1 {
		t.Fatalf("expected one session attempt, got %d", sessionCalls)
	}
	if messageCalls != 1 {
		t.Fatalf("expected one message attempt, got %d", messageCalls)
	}
	if len(callOrder) != 2 || callOrder[0] != "session" || callOrder[1] != "message" {
		t.Fatalf("expected attempt order [session message], got %v", callOrder)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected fallback move a3-a4, got %q", matchView.LastMove)
	}

	publicRoom, err := arena.PublicRoom(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicRoom() error = %v", err)
	}
	if publicRoom.Status != RoomStatusPlaying {
		t.Fatalf("expected room to keep playing after fallback success, got %q", publicRoom.Status)
	}

	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after AdvanceOnce error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	if runtime.ActiveMode != PicoclawActiveModeMessage {
		t.Fatalf("expected runtime active_mode message after fallback, got %q", runtime.ActiveMode)
	}
	if runtime.LastModeSwitchAt.IsZero() {
		t.Fatalf("expected runtime last_mode_switch_at to be updated on fallback")
	}
	if strings.TrimSpace(runtime.LastSwitchReason) == "" {
		t.Fatalf("expected runtime last_switch_reason to be set on fallback")
	}
}

func TestArenaPausesOnlyAfterSessionAndMessageBothFail(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	callOrder := make([]string, 0, 2)
	sessionCalls := 0
	messageCalls := 0
	arena.requestSessionMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		sessionCalls++
		callOrder = append(callOrder, "session")
		return "", "session failed", errors.New("session failed")
	}
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		messageCalls++
		callOrder = append(callOrder, "message")
		return "", "message failed", errors.New("message failed")
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "double-failure-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "double-failure-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
		BaseURL:     "http://127.0.0.1:9001",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if sessionCalls != 1 {
		t.Fatalf("expected one session attempt, got %d", sessionCalls)
	}
	if messageCalls != 1 {
		t.Fatalf("expected one message fallback attempt, got %d", messageCalls)
	}
	if len(callOrder) != 2 || callOrder[0] != "session" || callOrder[1] != "message" {
		t.Fatalf("expected attempt order [session message], got %v", callOrder)
	}

	publicRoom, err := arena.PublicRoom(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicRoom() error = %v", err)
	}
	if publicRoom.Status != RoomStatusPaused {
		t.Fatalf("expected room paused after both attempts fail, got %q", publicRoom.Status)
	}
}

func TestArenaAdvanceOnceFallsBackFromMessageToSession(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	callOrder := make([]string, 0, 2)
	sessionCalls := 0
	messageCalls := 0
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		messageCalls++
		callOrder = append(callOrder, "message")
		return "", "message unavailable", errors.New("message unavailable")
	}
	arena.requestSessionMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		sessionCalls++
		callOrder = append(callOrder, "session")
		return "a3-a4", "MOVE: a3-a4", nil
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "message-fallback-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "message-fallback-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
		BaseURL:     "http://127.0.0.1:9001",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferMessage); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if messageCalls != 1 {
		t.Fatalf("expected one message attempt, got %d", messageCalls)
	}
	if sessionCalls != 1 {
		t.Fatalf("expected one session attempt, got %d", sessionCalls)
	}
	if len(callOrder) != 2 || callOrder[0] != "message" || callOrder[1] != "session" {
		t.Fatalf("expected attempt order [message session], got %v", callOrder)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected fallback move a3-a4, got %q", matchView.LastMove)
	}

	publicRoom, err := arena.PublicRoom(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicRoom() error = %v", err)
	}
	if publicRoom.Status != RoomStatusPlaying {
		t.Fatalf("expected room to keep playing after fallback success, got %q", publicRoom.Status)
	}

	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after AdvanceOnce error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	if runtime.ActiveMode != PicoclawActiveModeSession {
		t.Fatalf("expected runtime active_mode session after fallback, got %q", runtime.ActiveMode)
	}
	if runtime.LastModeSwitchAt.IsZero() {
		t.Fatalf("expected runtime last_mode_switch_at to be updated on fallback")
	}
	if strings.TrimSpace(runtime.LastSwitchReason) == "" {
		t.Fatalf("expected runtime last_switch_reason to be set on fallback")
	}
}

func TestArenaAdvanceOnceFallsBackFromPicoWSToMessage(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	callOrder := make([]string, 0, 2)
	wsCalls := 0
	messageCalls := 0
	arena.requestWSMove = func(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		wsCalls++
		callOrder = append(callOrder, "pico_ws")
		return "", "ws failed", errors.New("ws failed")
	}
	arena.requestMove = func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
		messageCalls++
		callOrder = append(callOrder, "message")
		return "a3-a4", "MOVE: a3-a4", nil
	}

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "pico-ws-fallback-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "pico-ws-fallback-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
		BaseURL:     "http://127.0.0.1:18790",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferPicoWS); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after mode update error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	runtime.WSState = PicoclawWSStateActive
	runtime.ActiveMode = PicoclawActiveModePicoWS
	room := arena.rooms[normalizeRoomCode(hostView.Room.Code)]
	room.PicoclawRuntime[participantID] = runtime

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if wsCalls != 1 {
		t.Fatalf("expected one pico_ws attempt, got %d", wsCalls)
	}
	if messageCalls != 1 {
		t.Fatalf("expected one message fallback attempt, got %d", messageCalls)
	}
	if len(callOrder) != 2 || callOrder[0] != "pico_ws" || callOrder[1] != "message" {
		t.Fatalf("expected attempt order [pico_ws message], got %v", callOrder)
	}
}

func TestArenaAdvanceOnceUsesPicoWSTransportForManagedSeat(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	var seenAuth string
	var seenSessionID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pico/ws" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		seenAuth = r.Header.Get("Authorization")
		seenSessionID = r.URL.Query().Get("session_id")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade() error = %v", err)
		}
		defer conn.Close()

		var incoming picoWSMessage
		if err := conn.ReadJSON(&incoming); err != nil {
			t.Fatalf("ReadJSON() error = %v", err)
		}
		if incoming.Type != "message.send" {
			t.Fatalf("expected message.send, got %q", incoming.Type)
		}

		if err := conn.WriteJSON(picoWSMessage{
			Type: "message.create",
			Payload: map[string]any{
				"content": "MOVE: a3-a4",
			},
		}); err != nil {
			t.Fatalf("WriteJSON() error = %v", err)
		}
	}))
	defer server.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "pico-ws-success-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "pico-ws-success-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     server.URL,
			APIKey:      "ws-secret",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, hostView.Participant.ID, "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferPicoWS); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if seenAuth != "Bearer ws-secret" {
		t.Fatalf("expected bearer auth, got %q", seenAuth)
	}
	if !strings.Contains(seenSessionID, participantID) {
		t.Fatalf("expected session_id to contain participant id, got %q", seenSessionID)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected pico_ws move a3-a4, got %q", matchView.LastMove)
	}

	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() second error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	if runtime.ActiveMode != PicoclawActiveModePicoWS {
		t.Fatalf("expected active_mode pico_ws, got %q", runtime.ActiveMode)
	}
	if runtime.WSState != PicoclawWSStateActive {
		t.Fatalf("expected ws_state active, got %q", runtime.WSState)
	}
}

func TestResolvePicoclawActiveModePrefersPicoWSWhenHealthy(t *testing.T) {
	now := time.Now()
	state := PicoclawRuntimeState{
		PreferredMode: PicoclawModeAuto,
		WSState:       PicoclawWSStateActive,
		WSLastRecvAt:  now,
	}

	got := resolvePicoclawActiveMode(state, now)
	if got != PicoclawActiveModePicoWS {
		t.Fatalf("expected active mode pico_ws, got %q", got)
	}
}

func TestInvitePicoclawUsesMessageEndpoint(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()
	arena.SetPublicBaseURL("https://arena.example.test")

	var messageReq picoMessageRequest
	messageCalls := 0
	inviteCalls := 0
	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/message":
			messageCalls++
			if err := json.NewDecoder(r.Body).Decode(&messageReq); err != nil {
				t.Fatalf("Decode() message request error = %v", err)
			}
			writeJSON(w, http.StatusOK, picoMessageResponse{
				Reply: "邀请已发送",
			})
		case "/invite":
			inviteCalls++
			writeJSON(w, http.StatusOK, picoMessageResponse{
				Reply: "unused",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer messageServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-message-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-message-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
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

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed picoclaw participant on black seat")
	}

	reply, err := arena.InvitePicoclaw(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("InvitePicoclaw() error = %v", err)
	}
	if reply != "邀请已发送" {
		t.Fatalf("expected invite reply 邀请已发送, got %q", reply)
	}
	if messageCalls != 1 {
		t.Fatalf("expected exactly one /message call, got %d", messageCalls)
	}
	if inviteCalls != 0 {
		t.Fatalf("expected no /invite call, got %d", inviteCalls)
	}
	if strings.TrimSpace(messageReq.Message) == "" {
		t.Fatalf("expected invite payload message to be non-empty")
	}
	if !strings.Contains(messageReq.Message, "邀请") {
		t.Fatalf("expected invite prompt to indicate invitation semantics, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "arena_base_url：https://arena.example.test") {
		t.Fatalf("expected invite prompt to include arena_base_url, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "/api/arena/invite-message-room/picoclaw/"+participantID+"/session/heartbeat") {
		t.Fatalf("expected invite prompt to include heartbeat url, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "/api/arena/invite-message-room/picoclaw/"+participantID+"/turn") {
		t.Fatalf("expected invite prompt to include turn url, got %q", messageReq.Message)
	}
	if !strings.Contains(messageReq.Message, "session_token：") {
		t.Fatalf("expected invite prompt to include session_token, got %q", messageReq.Message)
	}

	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after invite error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	if runtime.LastInviteAt.IsZero() {
		t.Fatalf("expected runtime.last_invite_at to be set")
	}
	if runtime.LastInviteStatus != "ok" {
		t.Fatalf("expected runtime.last_invite_status ok, got %q", runtime.LastInviteStatus)
	}

	arena.mu.Lock()
	internalRuntime := arena.rooms[normalizeRoomCode(hostView.Room.Code)].PicoclawRuntime[participantID]
	arena.mu.Unlock()
	if strings.TrimSpace(internalRuntime.SessionID) == "" {
		t.Fatalf("expected invite to ensure session_id exists")
	}
	if strings.TrimSpace(internalRuntime.SessionAuthToken) == "" {
		t.Fatalf("expected invite to ensure session_token exists")
	}
}

func TestInvitePicoclawUsesInjectedRequestHook(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-hook-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-hook-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:19999",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed picoclaw participant on black seat")
	}

	var hookCalled bool
	arena.requestInvite = func(invite PicoclawInviteRequest) (string, error) {
		hookCalled = true
		if invite.RoomCode != hostView.Room.Code {
			t.Fatalf("expected room code %q, got %q", hostView.Room.Code, invite.RoomCode)
		}
		if invite.ParticipantID != participantID {
			t.Fatalf("expected participant id %q, got %#v", participantID, invite)
		}
		return "hook-invite-ok", nil
	}

	reply, err := arena.InvitePicoclaw(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("InvitePicoclaw() error = %v", err)
	}
	if !hookCalled {
		t.Fatalf("expected requestInvite hook to be called")
	}
	if reply != "hook-invite-ok" {
		t.Fatalf("expected hook reply hook-invite-ok, got %q", reply)
	}
}

func TestInvitePicoclawRecordsFailureStatusFromMessageError(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	messageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		http.Error(w, "upstream bad gateway", http.StatusBadGateway)
	}))
	defer messageServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-error-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-error-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
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

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed picoclaw participant on black seat")
	}

	if _, err := arena.InvitePicoclaw(hostView.Room.Code, hostView.Participant.ID, participantID); err == nil {
		t.Fatalf("expected InvitePicoclaw() error")
	}

	hostRoom, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after invite error = %v", err)
	}
	runtime := hostRoom.Runtime[participantID]
	if runtime.LastInviteAt.IsZero() {
		t.Fatalf("expected runtime.last_invite_at to be set on failure")
	}
	if !strings.Contains(runtime.LastInviteStatus, "HTTP 502") {
		t.Fatalf("expected runtime.last_invite_status to contain HTTP 502, got %q", runtime.LastInviteStatus)
	}
}

func TestInvitePicoclawReturnsCombinedErrorWhenInviteAndSaveBothFail(t *testing.T) {
	store := newFailOnDemandSnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-combined-error-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "invite-combined-error-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:19999",
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed picoclaw participant on black seat")
	}

	arena.requestInvite = func(invite PicoclawInviteRequest) (string, error) {
		return "", errors.New("invite failed")
	}
	if _, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID); err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	store.fail = true

	if _, err := arena.InvitePicoclaw(hostView.Room.Code, hostView.Participant.ID, participantID); err == nil {
		t.Fatalf("expected InvitePicoclaw() to fail")
	} else {
		if !strings.Contains(err.Error(), "invite failed") {
			t.Fatalf("expected combined error to include invite failure, got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("expected combined error to include save failure, got %q", err.Error())
		}
	}
}

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

	hostRoomBefore, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() before reload error = %v", err)
	}
	seatBefore := hostRoomBefore.Room.Seats[SeatBlackPlayer]
	if seatBefore.ParticipantID == "" {
		t.Fatalf("expected black seat participant before reload")
	}

	wantLease := time.Now().Add(24 * time.Hour).UTC().Round(time.Second)
	want := PicoclawRuntimeState{
		ParticipantID:  seatBefore.ParticipantID,
		PreferredMode:  PicoclawModePreferSession,
		ActiveMode:     PicoclawActiveModeSession,
		SessionState:   PicoclawSessionStateActive,
		SessionID:      "sess-roundtrip-1",
		LeaseExpiresAt: wantLease,
	}

	snapshot, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}
	var roomFound bool
	for _, room := range snapshot.Rooms {
		if room.Code != hostView.Room.Code {
			continue
		}
		roomFound = true
		if room.PicoclawRuntime == nil {
			room.PicoclawRuntime = make(map[string]PicoclawRuntimeState)
		}
		room.PicoclawRuntime[seatBefore.ParticipantID] = want
	}
	if !roomFound {
		t.Fatalf("expected room %q in snapshot", hostView.Room.Code)
	}
	if err := store.Save(snapshot); err != nil {
		t.Fatalf("store.Save() error = %v", err)
	}

	reloaded := NewArena(store)
	defer reloaded.Close()

	hostRoom, err := reloaded.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after reload error = %v", err)
	}
	seat := hostRoom.Room.Seats[SeatBlackPlayer]
	got, ok := hostRoom.Runtime[seat.ParticipantID]
	if !ok {
		t.Fatalf("expected persisted runtime state for participant %q", seat.ParticipantID)
	}
	if got.ParticipantID != want.ParticipantID {
		t.Fatalf("expected participant_id %q, got %q", want.ParticipantID, got.ParticipantID)
	}
	if got.PreferredMode != want.PreferredMode {
		t.Fatalf("expected preferred_mode %q, got %q", want.PreferredMode, got.PreferredMode)
	}
	if got.ActiveMode != want.ActiveMode {
		t.Fatalf("expected active_mode %q, got %q", want.ActiveMode, got.ActiveMode)
	}
	if got.SessionState != want.SessionState {
		t.Fatalf("expected session_state %q, got %q", want.SessionState, got.SessionState)
	}
	if got.SessionID != want.SessionID {
		t.Fatalf("expected session_id %q, got %q", want.SessionID, got.SessionID)
	}
	if !got.LeaseExpiresAt.Equal(want.LeaseExpiresAt) {
		t.Fatalf("expected lease_expires_at %s, got %s", want.LeaseExpiresAt.Format(time.RFC3339), got.LeaseExpiresAt.Format(time.RFC3339))
	}
}

func TestHostCanChangePicoclawPreferredMode(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-mode-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-mode-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatRedPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "红方 Pico",
			PublicAlias: "红雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18881",
		},
	}); err != nil {
		t.Fatalf("AssignSeat(red) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePicoclaw,
			Name:        "黑方 Pico",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     "http://127.0.0.1:18882",
		},
	}); err != nil {
		t.Fatalf("AssignSeat(black) error = %v", err)
	}

	roomView, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	redID := roomView.Room.Seats[SeatRedPlayer].ParticipantID
	blackID := roomView.Room.Seats[SeatBlackPlayer].ParticipantID
	if redID == "" || blackID == "" {
		t.Fatalf("expected both managed participants to exist")
	}

	runtime, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, blackID, PicoclawModePreferSession)
	if err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	if runtime.PreferredMode != PicoclawModePreferSession {
		t.Fatalf("expected preferred_mode prefer_session, got %q", runtime.PreferredMode)
	}

	roomView, err = arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() after mode change error = %v", err)
	}
	if roomView.Runtime[blackID].PreferredMode != PicoclawModePreferSession {
		t.Fatalf("expected black runtime preferred_mode prefer_session, got %q", roomView.Runtime[blackID].PreferredMode)
	}
	if roomView.Runtime[redID].PreferredMode != PicoclawModeAuto {
		t.Fatalf("expected red runtime preferred_mode auto, got %q", roomView.Runtime[redID].PreferredMode)
	}
}

func TestPicoclawSessionCloseMarksSessionClosed(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-close-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-close-room",
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

	roomView, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := roomView.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if opened.SessionState != PicoclawSessionStateOpening {
		t.Fatalf("expected opening session_state, got %q", opened.SessionState)
	}
	if strings.TrimSpace(opened.SessionID) == "" {
		t.Fatalf("expected open to create session_id")
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}

	closed, err := arena.ClosePicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("ClosePicoclawSession() error = %v", err)
	}
	if closed.SessionState != PicoclawSessionStateClosed {
		t.Fatalf("expected session_state closed, got %q", closed.SessionState)
	}
	if !closed.LeaseExpiresAt.IsZero() {
		t.Fatalf("expected lease_expires_at to be cleared, got %s", closed.LeaseExpiresAt.Format(time.RFC3339Nano))
	}
	if !closed.RecoveryDeadlineAt.IsZero() {
		t.Fatalf("expected recovery_deadline_at to be cleared, got %s", closed.RecoveryDeadlineAt.Format(time.RFC3339Nano))
	}
}

func TestPicoclawSessionHeartbeatRejectsSessionMismatch(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-heartbeat-auth-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-heartbeat-auth-room",
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

	roomView, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := roomView.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if strings.TrimSpace(opened.SessionID) == "" {
		t.Fatalf("expected open to create session_id")
	}

	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, "wrong-session-id", opened.SessionAuthToken, 45*time.Second); err == nil {
		t.Fatalf("expected HeartbeatPicoclawSession() to reject session mismatch")
	}
}

func TestPicoclawSessionHeartbeatRequiresSessionToken(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-heartbeat-token-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "runtime-heartbeat-token-room",
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

	roomView, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := roomView.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}

	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if strings.TrimSpace(opened.SessionAuthToken) == "" {
		t.Fatalf("expected open to create session_auth_token")
	}

	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, "wrong-token", 45*time.Second); err == nil {
		t.Fatalf("expected HeartbeatPicoclawSession() to reject token mismatch")
	}

	heartbeat, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second)
	if err != nil {
		t.Fatalf("HeartbeatPicoclawSession() with token error = %v", err)
	}
	if heartbeat.SessionState != PicoclawSessionStateActive {
		t.Fatalf("expected session_state active, got %q", heartbeat.SessionState)
	}
}

func TestArenaAdvanceOnceUsesSessionTurnChannel(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "session-turn-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "session-turn-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.BindSeatAgent(hostView.Room.Code, hostView.Participant.ID, SeatBlackPlayer, AgentBinding{
		RealType:    AgentTypePicoclaw,
		Name:        "托管黑方",
		PublicAlias: "黑雨伞",
		Connection:  "managed",
	}); err != nil {
		t.Fatalf("BindSeatAgent() error = %v", err)
	}
	if err := arena.UpdateSettings(hostView.Room.Code, hostView.Participant.ID, RoomSettingsRequest{
		StepIntervalMS: 1,
	}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	hostRoom, err := arena.HostRoom(hostView.Room.Code, hostView.Participant.ID)
	if err != nil {
		t.Fatalf("HostRoom() error = %v", err)
	}
	participantID := hostRoom.Room.Seats[SeatBlackPlayer].ParticipantID
	if participantID == "" {
		t.Fatalf("expected managed participant on black seat")
	}
	if _, err := arena.SetPicoclawMode(hostView.Room.Code, hostView.Participant.ID, participantID, PicoclawModePreferSession); err != nil {
		t.Fatalf("SetPicoclawMode() error = %v", err)
	}
	opened, err := arena.OpenPicoclawSession(hostView.Room.Code, hostView.Participant.ID, participantID)
	if err != nil {
		t.Fatalf("OpenPicoclawSession() error = %v", err)
	}
	if strings.TrimSpace(opened.SessionAuthToken) == "" {
		t.Fatalf("expected session_auth_token to be issued")
	}
	if _, err := arena.HeartbeatPicoclawSession(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 45*time.Second); err != nil {
		t.Fatalf("HeartbeatPicoclawSession() error = %v", err)
	}
	if _, err := arena.StartMatch(hostView.Room.Code, hostView.Participant.ID); err != nil {
		t.Fatalf("StartMatch() error = %v", err)
	}
	if _, err := arena.SubmitMove(hostView.Room.Code, "host-token", "a6-a5"); err != nil {
		t.Fatalf("SubmitMove() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		poll, err := arena.PollPicoclawTurn(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, 5*time.Second)
		if err != nil {
			done <- err
			return
		}
		if poll.Status != PicoclawSessionTurnStatusTurn {
			done <- fmt.Errorf("expected turn status, got %q", poll.Status)
			return
		}
		if poll.Turn == nil {
			done <- errors.New("expected turn payload")
			return
		}
		if poll.Turn.SessionID != opened.SessionID {
			done <- fmt.Errorf("expected session_id %q, got %q", opened.SessionID, poll.Turn.SessionID)
			return
		}
		if _, err := arena.SubmitPicoclawTurn(hostView.Room.Code, participantID, opened.SessionID, opened.SessionAuthToken, poll.Turn.TurnID, "a3-a4", "MOVE: a3-a4"); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	forceRoomReadyForAdvance(t, arena, hostView.Room.Code)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("session turn flow error = %v", err)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected session turn move a3-a4, got %q", matchView.LastMove)
	}
}

func TestMatchApplyHumanMovePreservesRepetitionErrorText(t *testing.T) {
	match, err := NewMatch("repeat-room", 3000, map[Side]PlayerConfig{
		SideRed:   {Type: AgentTypeHuman, Name: "Red"},
		SideBlack: {Type: AgentTypeHuman, Name: "Black"},
	}, map[Side]string{
		SideRed:   "Red",
		SideBlack: "Black",
	}, map[Side]string{
		SideRed:   "red-id",
		SideBlack: "black-id",
	})
	if err != nil {
		t.Fatalf("NewMatch() error = %v", err)
	}

	base := GameState{
		Board: boardFromRows([]string{
			"....k....",
			".........",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedCheck := stateAfterMove(t, base, "e2-e1")
	afterBlackOut := stateAfterMove(t, afterRedCheck, "e0-f0")
	afterRedBack := stateAfterMove(t, afterBlackOut, "e1-e2")
	afterBlackBack := stateAfterMove(t, afterRedBack, "f0-e0")

	match.State = base
	match.State.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey()},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e2-e1", PositionKey: afterRedCheck.PositionKey(), GivesCheck: true, RepeatedPosition: true},
		{Side: SideBlack, Move: "e0-f0", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e1-e2", PositionKey: afterRedBack.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "f0-e0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	err = match.ApplyHumanMove(SideRed, "e2-e1")
	if err == nil || err.Error() != "move causes forbidden long-check repetition" {
		t.Fatalf("expected long-check repetition error, got %v", err)
	}
	last := match.Logs[len(match.Logs)-1]
	if last.Error != "move causes forbidden long-check repetition" {
		t.Fatalf("expected repetition error to reach logs, got %#v", last)
	}
}

func TestMatchApplyAgentMovePreservesRepetitionErrorText(t *testing.T) {
	match, err := NewMatch("repeat-agent-room", 3000, map[Side]PlayerConfig{
		SideRed:   {Type: AgentTypePicoclaw, Name: "Red Pico"},
		SideBlack: {Type: AgentTypePicoclaw, Name: "Black Pico"},
	}, map[Side]string{
		SideRed:   "Red Pico",
		SideBlack: "Black Pico",
	}, map[Side]string{
		SideRed:   "red-id",
		SideBlack: "black-id",
	})
	if err != nil {
		t.Fatalf("NewMatch() error = %v", err)
	}

	base := GameState{
		Board: boardFromRows([]string{
			"...k.....",
			".........",
			".....n...",
			"....R....",
			".........",
			".........",
			".........",
			".........",
			".........",
			"....K....",
		}),
		Side:   SideRed,
		Status: "playing",
	}
	afterRedChase := stateAfterMove(t, base, "e3-e2")
	afterBlackOut := stateAfterMove(t, afterRedChase, "d0-d1")
	afterRedReset := stateAfterMove(t, afterBlackOut, "e2-e3")
	afterBlackBack := stateAfterMove(t, afterRedReset, "d1-d0")

	match.State = base
	match.State.RuleTraces = []RuleTrace{
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey()},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey()},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey()},
		{Side: SideRed, Move: "e3-e2", PositionKey: afterRedChase.PositionKey(), ChaseTargets: []string{"n@f2"}, RepeatedPosition: true},
		{Side: SideBlack, Move: "d0-d1", PositionKey: afterBlackOut.PositionKey(), RepeatedPosition: true},
		{Side: SideRed, Move: "e2-e3", PositionKey: afterRedReset.PositionKey(), RepeatedPosition: true},
		{Side: SideBlack, Move: "d1-d0", PositionKey: afterBlackBack.PositionKey(), RepeatedPosition: true},
	}

	err = match.ApplyAgentMove(SideRed, "e3-e2", "MOVE: e3-e2")
	if err == nil || err.Error() != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected long-chase repetition error, got %v", err)
	}
	last := match.Logs[len(match.Logs)-1]
	if last.Error != "move causes forbidden long-chase repetition" {
		t.Fatalf("expected repetition error to reach logs, got %#v", last)
	}
}
