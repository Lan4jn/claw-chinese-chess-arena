package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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
		RealType: AgentTypePico,
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
	if publicView.Seats[SeatRedPlayer].RealType != AgentTypePico {
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
		Type:    AgentTypePico,
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
		RealType: AgentTypePico,
		Name:     "远程 Pico",
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

	time.Sleep(3 * time.Millisecond)
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
			RealType:    AgentTypePico,
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
			if participant.RealType != AgentTypePico {
				t.Fatalf("expected managed participant to be pico, got %q", participant.RealType)
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

func TestArenaStartMatchCapturesCurrentTransportMode(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "transport-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "transport-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
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
	if matchView.TransportState != string(MatchTransportStatePending) && matchView.TransportState != string(MatchTransportStateActive) {
		t.Fatalf("expected transport state pending or active, got %q", matchView.TransportState)
	}
}

func TestArenaTransportSwitchDoesNotRewriteActiveMatch(t *testing.T) {
	arena := NewArena(NewMemorySnapshotStore())
	defer arena.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "transport-switch-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "transport-switch-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
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
	if hostMatch.TransportActiveMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected active mode http_session, got %q", hostMatch.TransportActiveMode)
	}
}

func TestArenaAdvanceOnceUsesHTTPSessionTransportForManagedSeat(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	var opened bool
	var receivedTurn AgentTurnRequest
	sessionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/open":
			opened = true
			writeJSON(w, http.StatusOK, map[string]any{
				"session_id":       "sess-1",
				"resume_token":     "resume-1",
				"lease_ttl_ms":     30000,
				"connection_state": "connected",
			})
		case "/session/turn":
			if err := json.NewDecoder(r.Body).Decode(&receivedTurn); err != nil {
				t.Fatalf("Decode() turn request error = %v", err)
			}
			writeJSON(w, http.StatusOK, AgentTurnResponse{
				TurnID:     receivedTurn.TurnID,
				Move:       "a3-a4",
				Reply:      "MOVE: a3-a4",
				AgentState: "ok",
				SessionID:  "sess-1",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer sessionServer.Close()

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
			RealType:    AgentTypePico,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     sessionServer.URL,
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

	time.Sleep(3 * time.Millisecond)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	if !opened {
		t.Fatalf("expected HTTP session to be opened")
	}
	if receivedTurn.TurnID == "" {
		t.Fatalf("expected turn request to include turn_id")
	}
	if receivedTurn.MatchID == "" {
		t.Fatalf("expected turn request to include match_id")
	}
	if len(receivedTurn.LegalMoves) == 0 {
		t.Fatalf("expected turn request to include legal_moves")
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected http session move a3-a4, got %q", matchView.LastMove)
	}
	if matchView.TransportActiveMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected active transport mode http_session, got %q", matchView.TransportActiveMode)
	}
}

func TestArenaAdvanceOnceUsesWebSocketTransportWhenAvailable(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	upgrader := websocket.Upgrader{}
	var receivedTurn AgentTurnRequest
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade() error = %v", err)
		}
		defer conn.Close()

		if err := conn.ReadJSON(&receivedTurn); err != nil {
			t.Fatalf("ReadJSON() error = %v", err)
		}
		if err := conn.WriteJSON(AgentTurnResponse{
			TurnID:     receivedTurn.TurnID,
			Move:       "a3-a4",
			Reply:      "MOVE: a3-a4",
			AgentState: "ok",
		}); err != nil {
			t.Fatalf("WriteJSON() error = %v", err)
		}
	}))
	defer wsServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "ws-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "ws-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePico,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     wsServer.URL,
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}
	if err := arena.SetTransportDefaultMode(TransportModeWebSocket); err != nil {
		t.Fatalf("SetTransportDefaultMode(websocket) error = %v", err)
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

	time.Sleep(3 * time.Millisecond)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if receivedTurn.TurnID == "" {
		t.Fatalf("expected websocket turn request to include turn_id")
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected websocket move a3-a4, got %q", matchView.LastMove)
	}
	if matchView.TransportMode != string(TransportModeWebSocket) {
		t.Fatalf("expected transport mode websocket, got %q", matchView.TransportMode)
	}
	if matchView.TransportActiveMode != string(TransportModeWebSocket) {
		t.Fatalf("expected active transport mode websocket, got %q", matchView.TransportActiveMode)
	}
}

func TestArenaAdvanceOnceDegradesWebSocketToHTTPSession(t *testing.T) {
	store := NewMemorySnapshotStore()
	arena := NewArena(store)
	defer arena.Close()

	httpFallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/open":
			writeJSON(w, http.StatusOK, map[string]any{
				"session_id":       "sess-2",
				"resume_token":     "resume-2",
				"lease_ttl_ms":     30000,
				"connection_state": "connected",
			})
		case "/session/turn":
			var req AgentTurnRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("Decode() turn request error = %v", err)
			}
			writeJSON(w, http.StatusOK, AgentTurnResponse{
				TurnID:     req.TurnID,
				Move:       "a3-a4",
				Reply:      "MOVE: a3-a4",
				AgentState: "ok",
				SessionID:  "sess-2",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer httpFallbackServer.Close()

	hostView, err := arena.Enter(EnterRequest{
		RoomCode:    "ws-fallback-room",
		ClientToken: "host-token",
		JoinIntent:  JoinIntentPlayer,
	})
	if err != nil {
		t.Fatalf("Enter(host) error = %v", err)
	}
	if _, err := arena.Enter(EnterRequest{
		RoomCode:    "ws-fallback-room",
		ClientToken: "guest-token",
		JoinIntent:  JoinIntentPlayer,
	}); err != nil {
		t.Fatalf("Enter(guest) error = %v", err)
	}
	if err := arena.AssignSeat(hostView.Room.Code, hostView.Participant.ID, SeatAssignRequest{
		Seat: SeatBlackPlayer,
		Binding: AgentBinding{
			RealType:    AgentTypePico,
			Name:        "托管黑方",
			PublicAlias: "黑雨伞",
			Connection:  "managed",
			BaseURL:     httpFallbackServer.URL,
		},
	}); err != nil {
		t.Fatalf("AssignSeat() error = %v", err)
	}
	if err := arena.SetTransportDefaultMode(TransportModeWebSocket); err != nil {
		t.Fatalf("SetTransportDefaultMode(websocket) error = %v", err)
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

	time.Sleep(3 * time.Millisecond)
	if err := arena.AdvanceOnce(); err != nil {
		t.Fatalf("AdvanceOnce() error = %v", err)
	}

	matchView, err := arena.PublicMatch(hostView.Room.Code)
	if err != nil {
		t.Fatalf("PublicMatch() error = %v", err)
	}
	if matchView.LastMove != "a3-a4" {
		t.Fatalf("expected degraded fallback move a3-a4, got %q", matchView.LastMove)
	}
	if matchView.TransportMode != string(TransportModeWebSocket) {
		t.Fatalf("expected configured transport mode websocket, got %q", matchView.TransportMode)
	}
	if matchView.TransportActiveMode != string(TransportModeHTTPSession) {
		t.Fatalf("expected active mode to degrade to http_session, got %q", matchView.TransportActiveMode)
	}
	if matchView.TransportState != string(MatchTransportStateDegraded) && matchView.TransportState != string(MatchTransportStateActive) {
		t.Fatalf("expected degraded or active transport state after fallback, got %q", matchView.TransportState)
	}
}
