package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type JoinIntent string

const (
	JoinIntentAuto      JoinIntent = "auto"
	JoinIntentPlayer    JoinIntent = "player"
	JoinIntentSpectator JoinIntent = "spectator"
)

type RoomAction string

const (
	RoomActionAuto   RoomAction = "auto"
	RoomActionCreate RoomAction = "create"
	RoomActionJoin   RoomAction = "join"
)

type SeatType string

const (
	SeatHost        SeatType = "host"
	SeatRedPlayer   SeatType = "red_player"
	SeatBlackPlayer SeatType = "black_player"
	SeatSpectator   SeatType = "spectator"
)

type RevealState string

const (
	RevealStateHidden  RevealState = "hidden"
	RevealStatePartial RevealState = "partial_reveal"
	RevealStateFull    RevealState = "full_reveal"
)

type ArenaRoomStatus string

const (
	RoomStatusWaiting  ArenaRoomStatus = "waiting"
	RoomStatusPlaying  ArenaRoomStatus = "playing"
	RoomStatusPaused   ArenaRoomStatus = "paused"
	RoomStatusFinished ArenaRoomStatus = "finished"
)

type Participant struct {
	ID          string    `json:"id"`
	Token       string    `json:"-"`
	PublicAlias string    `json:"public_alias"`
	RealType    string    `json:"real_type,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	BaseURL     string    `json:"base_url,omitempty"`
	APIKey      string    `json:"-"`
	Seat        SeatType  `json:"seat"`
	Connection  string    `json:"connection"`
	JoinedAt    time.Time `json:"joined_at"`
}

type Seat struct {
	Type          SeatType `json:"type"`
	ParticipantID string   `json:"participant_id,omitempty"`
	PublicAlias   string   `json:"public_alias,omitempty"`
	RealType      string   `json:"real_type,omitempty"`
	DisplayName   string   `json:"display_name,omitempty"`
}

type ArenaRoom struct {
	Code              string                          `json:"code"`
	OwnerToken        string                          `json:"-"`
	HostParticipantID string                          `json:"host_participant_id"`
	Status            ArenaRoomStatus                 `json:"status"`
	StepIntervalMS    int                             `json:"step_interval_ms"`
	RevealState       RevealState                     `json:"reveal_state"`
	RevealRed         bool                            `json:"reveal_red"`
	RevealBlack       bool                            `json:"reveal_black"`
	DefaultView       string                          `json:"default_view"`
	CreatedAt         time.Time                       `json:"created_at"`
	UpdatedAt         time.Time                       `json:"updated_at"`
	Participants      []*Participant                  `json:"participants"`
	Seats             map[SeatType]*Seat              `json:"seats"`
	PicoclawRuntime   map[string]PicoclawRuntimeState `json:"picoclaw_runtime,omitempty"`
	ActiveMatch       *Match                          `json:"active_match,omitempty"`
	NextActionAt      time.Time                       `json:"next_action_at,omitempty"`
}

type EnterRequest struct {
	RoomCode    string     `json:"room_code"`
	ClientToken string     `json:"client_token"`
	DisplayName string     `json:"display_name,omitempty"`
	JoinIntent  JoinIntent `json:"join_intent"`
	RoomAction  RoomAction `json:"room_action,omitempty"`
}

type ArenaEnterView struct {
	IsHost      bool              `json:"is_host"`
	Room        PublicRoom        `json:"room"`
	Participant PublicParticipant `json:"participant"`
}

type PublicParticipant struct {
	ID          string   `json:"id"`
	PublicAlias string   `json:"public_alias"`
	Seat        SeatType `json:"seat"`
}

type PublicRoom struct {
	Code              string            `json:"code"`
	HostParticipantID string            `json:"host_participant_id,omitempty"`
	Status            ArenaRoomStatus   `json:"status"`
	StepIntervalMS    int               `json:"step_interval_ms"`
	RevealState       RevealState       `json:"reveal_state"`
	DefaultView       string            `json:"default_view"`
	Seats             map[SeatType]Seat `json:"seats"`
	SpectatorCount    int               `json:"spectator_count"`
}

type HostParticipantView struct {
	ID          string   `json:"id"`
	PublicAlias string   `json:"public_alias"`
	RealType    string   `json:"real_type"`
	DisplayName string   `json:"display_name"`
	Seat        SeatType `json:"seat"`
	Connection  string   `json:"connection"`
	BaseURL     string   `json:"base_url,omitempty"`
}

type HostRoomView struct {
	IsHost       bool                            `json:"is_host"`
	Room         PublicRoom                      `json:"room"`
	Participants []HostParticipantView           `json:"participants"`
	Runtime      map[string]PicoclawRuntimeState `json:"runtime,omitempty"`
}

type AgentBinding struct {
	RealType    string `json:"real_type"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	PublicAlias string `json:"public_alias,omitempty"`
	Connection  string `json:"connection,omitempty"`
}

type RoomSettingsRequest struct {
	StepIntervalMS int    `json:"step_interval_ms"`
	DefaultView    string `json:"default_view"`
}

type SeatAssignRequest struct {
	Seat          SeatType     `json:"seat"`
	ParticipantID string       `json:"participant_id,omitempty"`
	Binding       AgentBinding `json:"binding"`
}

type AgentRegisterRequest struct {
	ClientToken string       `json:"client_token"`
	DisplayName string       `json:"display_name,omitempty"`
	JoinIntent  JoinIntent   `json:"join_intent"`
	Binding     AgentBinding `json:"binding"`
}

type PublicMatchView struct {
	RoomCode       string            `json:"room_code"`
	RoomStatus     ArenaRoomStatus   `json:"room_status"`
	StepIntervalMS int               `json:"step_interval_ms"`
	Turn           Side              `json:"turn"`
	LastMove       string            `json:"last_move,omitempty"`
	BoardRows      []string          `json:"board_rows"`
	BoardText      string            `json:"board_text"`
	Status         string            `json:"status"`
	Reason         string            `json:"reason,omitempty"`
	Winner         Side              `json:"winner,omitempty"`
	MoveCount      int               `json:"move_count"`
	NextActionAt   time.Time         `json:"next_action_at,omitempty"`
	Seats          map[SeatType]Seat `json:"seats"`
	Logs           []MatchLogView    `json:"logs"`
	LegalMoves     []string          `json:"legal_moves"`
	Phase          string            `json:"phase"`
}

type MatchLogView struct {
	Time    time.Time `json:"time"`
	Side    Side      `json:"side,omitempty"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
	Reply   string    `json:"reply,omitempty"`
}

type HostMatchView struct {
	PublicMatchView
	RawLogs []MatchLogView `json:"raw_logs"`
}

type Arena struct {
	mu          sync.Mutex
	store       SnapshotStore
	rooms       map[string]*ArenaRoom
	requestMove func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error)
	ticker      *time.Ticker
	done        chan struct{}
}

func NewArena(store SnapshotStore) *Arena {
	a := &Arena{
		store: store,
		rooms: make(map[string]*ArenaRoom),
		requestMove: func(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			return askPicoForMove(ctx, defaultHTTPClient(), matchID, player, state, legal, arenaState)
		},
		ticker: time.NewTicker(400 * time.Millisecond),
		done:   make(chan struct{}),
	}
	if store != nil {
		if snapshot, err := store.Load(); err == nil && snapshot != nil {
			for _, room := range snapshot.Rooms {
				a.rooms[room.Code] = room
			}
		}
	}
	go a.run()
	return a
}

func (a *Arena) Close() {
	close(a.done)
	a.ticker.Stop()
}

func (a *Arena) Enter(req EnterRequest) (ArenaEnterView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	roomCode := normalizeRoomCode(req.RoomCode)
	if roomCode == "" {
		return ArenaEnterView{}, fmt.Errorf("room_code is required")
	}
	if strings.TrimSpace(req.ClientToken) == "" {
		return ArenaEnterView{}, fmt.Errorf("client_token is required")
	}
	if req.JoinIntent == "" {
		req.JoinIntent = JoinIntentAuto
	}
	if req.RoomAction == "" {
		req.RoomAction = RoomActionAuto
	}

	room, ok := a.rooms[roomCode]
	if !ok {
		if req.RoomAction == RoomActionJoin {
			return ArenaEnterView{}, fmt.Errorf("room not found")
		}
		now := time.Now()
		room = &ArenaRoom{
			Code:           roomCode,
			OwnerToken:     req.ClientToken,
			Status:         RoomStatusWaiting,
			StepIntervalMS: 3000,
			RevealState:    RevealStateHidden,
			DefaultView:    "board",
			CreatedAt:      now,
			UpdatedAt:      now,
			Seats: map[SeatType]*Seat{
				SeatHost:        {Type: SeatHost},
				SeatRedPlayer:   {Type: SeatRedPlayer},
				SeatBlackPlayer: {Type: SeatBlackPlayer},
			},
		}
		a.rooms[roomCode] = room
	} else if req.RoomAction == RoomActionCreate {
		return ArenaEnterView{}, fmt.Errorf("room already exists")
	}

	participant := findParticipantByToken(room, req.ClientToken)
	if participant == nil {
		id, err := randomID()
		if err != nil {
			return ArenaEnterView{}, err
		}
		participant = &Participant{
			ID:          id,
			Token:       req.ClientToken,
			PublicAlias: generateAlias(room),
			DisplayName: strings.TrimSpace(req.DisplayName),
			Seat:        SeatSpectator,
			Connection:  "ui",
			JoinedAt:    time.Now(),
		}
		if participant.DisplayName == "" {
			participant.DisplayName = participant.PublicAlias
		}
		room.Participants = append(room.Participants, participant)
	}

	isHost := room.OwnerToken == req.ClientToken
	if isHost {
		room.HostParticipantID = participant.ID
	}
	assignSeatOnEnter(room, participant, req.JoinIntent)
	syncSeats(room)
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return ArenaEnterView{}, err
	}

	return ArenaEnterView{
		IsHost:      isHost,
		Room:        buildPublicRoom(room, isHost),
		Participant: buildPublicParticipant(participant),
	}, nil
}

func (a *Arena) PublicRoom(code string) (PublicRoom, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return PublicRoom{}, fmt.Errorf("room not found")
	}
	return buildPublicRoom(room, false), nil
}

func (a *Arena) HostRoom(code string, requester string) (HostRoomView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, requester)
	if err != nil {
		return HostRoomView{}, err
	}
	view := HostRoomView{
		IsHost:  true,
		Room:    buildPublicRoom(room, true),
		Runtime: clonePicoclawRuntime(room.PicoclawRuntime),
	}
	for _, participant := range room.Participants {
		view.Participants = append(view.Participants, HostParticipantView{
			ID:          participant.ID,
			PublicAlias: participant.PublicAlias,
			RealType:    normalizeAgentType(participant.RealType),
			DisplayName: participant.DisplayName,
			Seat:        participant.Seat,
			Connection:  participant.Connection,
			BaseURL:     participant.BaseURL,
		})
	}
	return view, nil
}

func (a *Arena) OpenPicoclawSession(code, hostParticipantID, participantID string) (PicoclawRuntimeState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	state, err := managedPicoclawRuntimeLocked(room, participantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}

	sessionID, err := randomID()
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	now := time.Now()
	state.SessionID = sessionID
	state.SessionState = PicoclawSessionStateOpening
	state.LastHeartbeatAt = time.Time{}
	state.LeaseExpiresAt = time.Time{}
	state.RecoveryDeadlineAt = time.Time{}
	state.ActiveMode = resolvePicoclawActiveMode(state, now)
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = now
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
	state, err := managedPicoclawRuntimeLocked(room, participantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	if strings.TrimSpace(sessionID) == "" {
		return PicoclawRuntimeState{}, fmt.Errorf("session_id is required")
	}
	if state.SessionID != sessionID {
		return PicoclawRuntimeState{}, fmt.Errorf("session mismatch")
	}
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	now := time.Now()
	state.LastHeartbeatAt = now
	state.LeaseExpiresAt = now.Add(ttl)
	state.SessionState = PicoclawSessionStateActive
	state.ActiveMode = resolvePicoclawActiveMode(state, now)
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = now
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
	state, err := managedPicoclawRuntimeLocked(room, participantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}

	now := time.Now()
	state.SessionState = PicoclawSessionStateClosed
	state.SessionID = ""
	state.LastHeartbeatAt = time.Time{}
	state.LeaseExpiresAt = time.Time{}
	state.RecoveryDeadlineAt = time.Time{}
	state.ActiveMode = resolvePicoclawActiveMode(state, now)
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

	switch mode {
	case PicoclawModeAuto, PicoclawModePreferSession, PicoclawModePreferMessage:
	default:
		return PicoclawRuntimeState{}, fmt.Errorf("unsupported preferred_mode")
	}

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}
	state, err := managedPicoclawRuntimeLocked(room, participantID)
	if err != nil {
		return PicoclawRuntimeState{}, err
	}

	now := time.Now()
	state.PreferredMode = mode
	state.ActiveMode = resolvePicoclawActiveMode(state, now)
	room.PicoclawRuntime[participantID] = state
	room.UpdatedAt = now
	if err := a.saveLocked(); err != nil {
		return PicoclawRuntimeState{}, err
	}
	return state, nil
}

func (a *Arena) UpdateReveal(code string, hostParticipantID string, state RevealState) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	room.RevealState = state
	switch state {
	case RevealStateFull:
		room.RevealRed = true
		room.RevealBlack = true
	case RevealStateHidden:
		room.RevealRed = false
		room.RevealBlack = false
	}
	syncSeats(room)
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func (a *Arena) SetReveal(code string, hostParticipantID string, scope string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	switch strings.TrimSpace(strings.ToLower(scope)) {
	case "red":
		room.RevealRed = true
	case "black":
		room.RevealBlack = true
	case "all":
		room.RevealRed = true
		room.RevealBlack = true
	default:
		room.RevealRed = false
		room.RevealBlack = false
	}
	room.RevealState = currentRevealState(room)
	syncSeats(room)
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func (a *Arena) BindSeatAgent(code string, hostParticipantID string, seatType SeatType, binding AgentBinding) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	return a.bindSeatLocked(room, seatType, binding)
}

func (a *Arena) AssignSeat(code string, hostParticipantID string, req SeatAssignRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	if req.Seat != SeatRedPlayer && req.Seat != SeatBlackPlayer {
		return fmt.Errorf("seat must be red_player or black_player")
	}
	if req.ParticipantID != "" {
		participant := findParticipantByID(room, req.ParticipantID)
		if participant == nil {
			return fmt.Errorf("participant not found")
		}
		clearSeatOccupant(room, req.Seat)
		participant.Seat = req.Seat
		syncSeats(room)
		room.UpdatedAt = time.Now()
		return a.saveLocked()
	}
	return a.bindSeatLocked(room, req.Seat, req.Binding)
}

func (a *Arena) RemoveSeat(code string, hostParticipantID string, seatType SeatType) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	clearSeatOccupant(room, seatType)
	syncSeats(room)
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func (a *Arena) PauseMatch(code string, hostParticipantID string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PublicMatchView{}, err
	}
	if room.ActiveMatch == nil {
		return PublicMatchView{}, fmt.Errorf("match not started")
	}
	room.Status = RoomStatusPaused
	room.NextActionAt = time.Time{}
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PublicMatchView{}, err
	}
	return buildPublicMatchView(room), nil
}

func (a *Arena) ResumeMatch(code string, hostParticipantID string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PublicMatchView{}, err
	}
	if room.ActiveMatch == nil {
		return PublicMatchView{}, fmt.Errorf("match not started")
	}
	if room.ActiveMatch.State.Status == "finished" {
		room.Status = RoomStatusFinished
		return buildPublicMatchView(room), nil
	}
	room.Status = RoomStatusPlaying
	scheduleNextAction(room)
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PublicMatchView{}, err
	}
	return buildPublicMatchView(room), nil
}

func (a *Arena) ResetMatch(code string, hostParticipantID string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PublicMatchView{}, err
	}
	room.ActiveMatch = nil
	room.Status = RoomStatusWaiting
	room.NextActionAt = time.Time{}
	room.RevealRed = false
	room.RevealBlack = false
	room.RevealState = RevealStateHidden
	syncSeats(room)
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PublicMatchView{}, err
	}
	return PublicMatchView{
		RoomCode:       room.Code,
		RoomStatus:     room.Status,
		StepIntervalMS: room.StepIntervalMS,
		Seats:          cloneSeats(room),
		Phase:          "waiting_match",
	}, nil
}

func (a *Arena) RegisterAgent(code string, req AgentRegisterRequest) (ArenaEnterView, error) {
	_, err := a.Enter(EnterRequest{
		RoomCode:    code,
		ClientToken: req.ClientToken,
		DisplayName: req.DisplayName,
		JoinIntent:  req.JoinIntent,
	})
	if err != nil {
		return ArenaEnterView{}, err
	}
	if err := a.updateParticipantBinding(code, req.ClientToken, req.Binding); err != nil {
		return ArenaEnterView{}, err
	}
	return a.Enter(EnterRequest{
		RoomCode:    code,
		ClientToken: req.ClientToken,
		DisplayName: req.DisplayName,
		JoinIntent:  req.JoinIntent,
	})
}

func (a *Arena) UpdateSettings(code string, hostParticipantID string, req RoomSettingsRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return err
	}
	if req.StepIntervalMS > 0 {
		room.StepIntervalMS = req.StepIntervalMS
	}
	if view := strings.TrimSpace(req.DefaultView); view != "" {
		room.DefaultView = view
	}
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func (a *Arena) StartMatch(code string, hostParticipantID string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, hostParticipantID)
	if err != nil {
		return PublicMatchView{}, err
	}
	red := findParticipantByID(room, room.Seats[SeatRedPlayer].ParticipantID)
	black := findParticipantByID(room, room.Seats[SeatBlackPlayer].ParticipantID)
	if red == nil || black == nil {
		return PublicMatchView{}, fmt.Errorf("both player seats must be occupied")
	}
	players := map[Side]PlayerConfig{
		SideRed: {
			Type:    normalizeAgentType(red.RealType),
			Name:    red.DisplayName,
			BaseURL: red.BaseURL,
			APIKey:  red.APIKey,
		},
		SideBlack: {
			Type:    normalizeAgentType(black.RealType),
			Name:    black.DisplayName,
			BaseURL: black.BaseURL,
			APIKey:  black.APIKey,
		},
	}
	aliases := map[Side]string{
		SideRed:   red.PublicAlias,
		SideBlack: black.PublicAlias,
	}
	participants := map[Side]string{
		SideRed:   red.ID,
		SideBlack: black.ID,
	}
	match, err := NewMatch(room.Code, room.StepIntervalMS, players, aliases, participants)
	if err != nil {
		return PublicMatchView{}, err
	}
	room.ActiveMatch = match
	room.Status = RoomStatusPlaying
	scheduleNextAction(room)
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PublicMatchView{}, err
	}
	return buildPublicMatchView(room), nil
}

func (a *Arena) PublicMatch(code string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return PublicMatchView{}, fmt.Errorf("room not found")
	}
	if room.ActiveMatch == nil {
		return PublicMatchView{}, fmt.Errorf("match not started")
	}
	return buildPublicMatchView(room), nil
}

func (a *Arena) HostMatch(code string, requester string) (HostMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, err := a.hostRoomLocked(code, requester)
	if err != nil {
		return HostMatchView{}, err
	}
	if room.ActiveMatch == nil {
		return HostMatchView{}, fmt.Errorf("match not started")
	}
	return buildHostMatchView(room), nil
}

func (a *Arena) SubmitMove(code string, requester string, move string) (PublicMatchView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return PublicMatchView{}, fmt.Errorf("room not found")
	}
	if room.ActiveMatch == nil {
		return PublicMatchView{}, fmt.Errorf("match not started")
	}
	match := room.ActiveMatch
	side := match.State.Side
	expectedParticipantID := match.Participants[side]
	requesterParticipant := findParticipantByID(room, requester)
	if requesterParticipant == nil {
		requesterParticipant = findParticipantByToken(room, requester)
	}
	if requesterParticipant == nil || requesterParticipant.ID != expectedParticipantID {
		return PublicMatchView{}, fmt.Errorf("current turn belongs to another participant")
	}
	player := match.CurrentPlayer()
	if normalizeAgentType(player.Type) != AgentTypeHuman {
		return PublicMatchView{}, fmt.Errorf("current participant is not human-controlled")
	}
	if err := match.ApplyHumanMove(side, move); err != nil {
		return PublicMatchView{}, err
	}
	if match.State.Status == "playing" {
		room.Status = RoomStatusPlaying
		scheduleNextAction(room)
	}
	if match.State.Status == "finished" {
		room.Status = RoomStatusFinished
		room.NextActionAt = time.Time{}
	}
	room.UpdatedAt = time.Now()
	if err := a.saveLocked(); err != nil {
		return PublicMatchView{}, err
	}
	return buildPublicMatchView(room), nil
}

func (a *Arena) AdvanceOnce() error {
	var roomCodes []string
	a.mu.Lock()
	for code, room := range a.rooms {
		if room.Status != RoomStatusPlaying || room.ActiveMatch == nil {
			continue
		}
		match := room.ActiveMatch
		if match.State.Status != "playing" {
			continue
		}
		current := currentParticipant(room, match)
		if current == nil || normalizeAgentType(current.RealType) == AgentTypeHuman {
			continue
		}
		if !room.NextActionAt.IsZero() && time.Now().Before(room.NextActionAt) {
			continue
		}
		roomCodes = append(roomCodes, code)
	}
	a.mu.Unlock()

	for _, code := range roomCodes {
		if err := a.advanceRoom(code); err != nil {
			return err
		}
	}
	return nil
}

func (a *Arena) saveLocked() error {
	if a.store == nil {
		return nil
	}
	snapshot := &ArenaSnapshot{
		Rooms: make([]*ArenaRoom, 0, len(a.rooms)),
	}
	for _, room := range a.rooms {
		snapshot.Rooms = append(snapshot.Rooms, room)
	}
	return a.store.Save(snapshot)
}

func (a *Arena) run() {
	for {
		select {
		case <-a.ticker.C:
			_ = a.AdvanceOnce()
		case <-a.done:
			return
		}
	}
}

func (a *Arena) hostRoomLocked(code string, hostParticipantID string) (*ArenaRoom, error) {
	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return nil, fmt.Errorf("room not found")
	}
	if room.HostParticipantID == "" {
		return nil, fmt.Errorf("host permission required")
	}
	if room.HostParticipantID != hostParticipantID && room.OwnerToken != hostParticipantID {
		return nil, fmt.Errorf("host permission required")
	}
	return room, nil
}

func assignSeatOnEnter(room *ArenaRoom, participant *Participant, joinIntent JoinIntent) {
	if participant.Seat == SeatRedPlayer || participant.Seat == SeatBlackPlayer {
		return
	}
	if joinIntent == JoinIntentSpectator {
		participant.Seat = SeatSpectator
		return
	}
	if room.Seats[SeatRedPlayer].ParticipantID == "" {
		participant.Seat = SeatRedPlayer
		return
	}
	if room.Seats[SeatBlackPlayer].ParticipantID == "" {
		participant.Seat = SeatBlackPlayer
		return
	}
	participant.Seat = SeatSpectator
}

func syncSeats(room *ArenaRoom) {
	for seatType, seat := range room.Seats {
		if seatType == SeatHost {
			host := findParticipantByID(room, room.HostParticipantID)
			if host != nil {
				seat.ParticipantID = host.ID
				seat.PublicAlias = host.PublicAlias
				seat.DisplayName = host.DisplayName
				seat.RealType = AgentTypeHuman
			}
			continue
		}
		seat.ParticipantID = ""
		seat.PublicAlias = ""
		seat.DisplayName = ""
		seat.RealType = ""
	}
	for _, participant := range room.Participants {
		if participant.Seat != SeatRedPlayer && participant.Seat != SeatBlackPlayer {
			continue
		}
		seat := room.Seats[participant.Seat]
		seat.ParticipantID = participant.ID
		seat.PublicAlias = participant.PublicAlias
		seat.DisplayName = participant.DisplayName
		if seat.Type == SeatRedPlayer && room.RevealRed {
			seat.RealType = participant.RealType
		}
		if seat.Type == SeatBlackPlayer && room.RevealBlack {
			seat.RealType = participant.RealType
		}
	}
	reconcilePicoclawRuntime(room)
}

func buildPublicRoom(room *ArenaRoom, includeHost bool) PublicRoom {
	out := PublicRoom{
		Code:           room.Code,
		Status:         room.Status,
		StepIntervalMS: room.StepIntervalMS,
		RevealState:    currentRevealState(room),
		DefaultView:    room.DefaultView,
		Seats:          make(map[SeatType]Seat, len(room.Seats)),
	}
	if includeHost {
		out.HostParticipantID = room.HostParticipantID
	}
	for seatType, seat := range room.Seats {
		cp := *seat
		if !includeHost && seatType == SeatHost {
			cp.RealType = ""
		}
		out.Seats[seatType] = cp
	}
	for _, participant := range room.Participants {
		if participant.Seat == SeatSpectator {
			out.SpectatorCount++
		}
	}
	return out
}

func buildPublicMatchView(room *ArenaRoom) PublicMatchView {
	match := room.ActiveMatch
	seats := make(map[SeatType]Seat, len(room.Seats))
	for seatType, seat := range room.Seats {
		seats[seatType] = *seat
	}
	return PublicMatchView{
		RoomCode:       room.Code,
		RoomStatus:     room.Status,
		StepIntervalMS: room.StepIntervalMS,
		Turn:           match.State.Side,
		LastMove:       match.State.LastMove,
		BoardRows:      BoardRows(match.State.Board),
		BoardText:      BoardText(match.State.Board),
		Status:         match.State.Status,
		Reason:         match.State.Reason,
		Winner:         match.State.Winner,
		MoveCount:      match.State.MoveCount,
		NextActionAt:   room.NextActionAt,
		Seats:          seats,
		Logs:           buildLogViews(match.Logs, false),
		LegalMoves:     match.LegalMoves(),
		Phase:          matchPhase(room),
	}
}

func buildHostMatchView(room *ArenaRoom) HostMatchView {
	public := buildPublicMatchView(room)
	return HostMatchView{
		PublicMatchView: public,
		RawLogs:         buildLogViews(room.ActiveMatch.Logs, true),
	}
}

func (a *Arena) advanceRoom(code string) error {
	a.mu.Lock()
	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok || room.ActiveMatch == nil {
		a.mu.Unlock()
		return nil
	}
	match := room.ActiveMatch
	if room.Status != RoomStatusPlaying || match.State.Status != "playing" {
		a.mu.Unlock()
		return nil
	}
	current := currentParticipant(room, match)
	if current == nil {
		a.mu.Unlock()
		return fmt.Errorf("current participant not found")
	}
	player := match.CurrentPlayer()
	if normalizeAgentType(current.RealType) == AgentTypeHuman || normalizeAgentType(player.Type) == AgentTypeHuman {
		a.mu.Unlock()
		return nil
	}
	arenaState := PromptArenaState{
		RoomCode:       room.Code,
		StepIntervalMS: room.StepIntervalMS,
		OpponentAlias:  match.OpponentAlias(),
	}
	state := match.State
	legal := match.LegalMoves()
	side := state.Side
	matchID := match.ID
	a.mu.Unlock()

	move, reply, err := a.requestMove(matchID, player, state, legal, arenaState)

	a.mu.Lock()
	defer a.mu.Unlock()
	room, ok = a.rooms[normalizeRoomCode(code)]
	if !ok || room.ActiveMatch == nil {
		return nil
	}
	match = room.ActiveMatch
	if err != nil {
		match.AppendAgentError(side, reply, err)
		room.Status = RoomStatusPaused
		room.NextActionAt = time.Time{}
		room.UpdatedAt = time.Now()
		return a.saveLocked()
	}
	if err := match.ApplyAgentMove(side, move, reply); err != nil {
		room.Status = RoomStatusPaused
		room.NextActionAt = time.Time{}
		room.UpdatedAt = time.Now()
		return a.saveLocked()
	}
	if match.State.Status == "finished" {
		room.Status = RoomStatusFinished
		room.NextActionAt = time.Time{}
	} else {
		scheduleNextAction(room)
	}
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func buildPublicParticipant(participant *Participant) PublicParticipant {
	return PublicParticipant{
		ID:          participant.ID,
		PublicAlias: participant.PublicAlias,
		Seat:        participant.Seat,
	}
}

func findParticipantByToken(room *ArenaRoom, token string) *Participant {
	for _, participant := range room.Participants {
		if participant.Token == token {
			return participant
		}
	}
	return nil
}

func findParticipantByID(room *ArenaRoom, id string) *Participant {
	for _, participant := range room.Participants {
		if participant.ID == id {
			return participant
		}
	}
	return nil
}

func managedPicoclawRuntimeLocked(room *ArenaRoom, participantID string) (PicoclawRuntimeState, error) {
	participant := findParticipantByID(room, participantID)
	if participant == nil {
		return PicoclawRuntimeState{}, fmt.Errorf("participant not found")
	}
	if participant.Connection != "managed" ||
		normalizeAgentType(participant.RealType) != AgentTypePicoclaw ||
		(participant.Seat != SeatRedPlayer && participant.Seat != SeatBlackPlayer) {
		return PicoclawRuntimeState{}, fmt.Errorf("participant is not managed picoclaw")
	}
	ensurePicoclawRuntime(room, participant)
	state, ok := room.PicoclawRuntime[participantID]
	if !ok {
		return PicoclawRuntimeState{}, fmt.Errorf("participant runtime not found")
	}
	return state, nil
}

func normalizeRoomCode(code string) string {
	return strings.TrimSpace(strings.ToLower(code))
}

var defaultAliases = []string{
	"玻璃杯",
	"黑雨伞",
	"折叠椅",
	"白瓷盘",
	"卷尺",
	"保温杯",
	"台灯",
	"帆布包",
	"闹钟",
	"订书机",
}

func generateAlias(room *ArenaRoom) string {
	used := make(map[string]struct{}, len(room.Participants))
	for _, participant := range room.Participants {
		used[participant.PublicAlias] = struct{}{}
	}
	for _, alias := range defaultAliases {
		if _, ok := used[alias]; !ok {
			return alias
		}
	}
	return fmt.Sprintf("物件%s", fmt.Sprint(len(room.Participants)+1))
}

func normalizeAgentType(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "", AgentTypeHuman:
		return AgentTypeHuman
	case "pico", AgentTypePicoclaw:
		return AgentTypePicoclaw
	default:
		return kind
	}
}

func currentParticipant(room *ArenaRoom, match *Match) *Participant {
	if match == nil {
		return nil
	}
	return findParticipantByID(room, match.CurrentParticipantID())
}

func (a *Arena) bindSeatLocked(room *ArenaRoom, seatType SeatType, binding AgentBinding) error {
	seat, ok := room.Seats[seatType]
	if !ok || seatType == SeatHost {
		return fmt.Errorf("seat not found")
	}
	participant := findParticipantByID(room, seat.ParticipantID)
	if participant != nil && participant.Connection != "managed" {
		participant.Seat = SeatSpectator
		participant = nil
	}
	if participant == nil {
		id, err := randomID()
		if err != nil {
			return err
		}
		alias := generateAlias(room)
		participant = &Participant{
			ID:          id,
			PublicAlias: alias,
			DisplayName: alias,
			Seat:        seatType,
			Connection:  "managed",
			JoinedAt:    time.Now(),
		}
		room.Participants = append(room.Participants, participant)
	}
	participant.RealType = normalizeAgentType(binding.RealType)
	participant.DisplayName = strings.TrimSpace(binding.Name)
	participant.BaseURL = strings.TrimSpace(binding.BaseURL)
	participant.APIKey = strings.TrimSpace(binding.APIKey)
	if alias := strings.TrimSpace(binding.PublicAlias); alias != "" {
		participant.PublicAlias = alias
	}
	if connection := strings.TrimSpace(binding.Connection); connection != "" {
		participant.Connection = connection
	}
	if participant.DisplayName == "" {
		participant.DisplayName = participant.PublicAlias
	}
	participant.Seat = seatType
	syncSeats(room)
	room.UpdatedAt = time.Now()
	return a.saveLocked()
}

func clearSeatOccupant(room *ArenaRoom, seatType SeatType) {
	if seatType != SeatRedPlayer && seatType != SeatBlackPlayer {
		return
	}
	for _, participant := range room.Participants {
		if participant.Seat == seatType {
			participant.Seat = SeatSpectator
		}
	}
}

func cloneSeats(room *ArenaRoom) map[SeatType]Seat {
	out := make(map[SeatType]Seat, len(room.Seats))
	for seatType, seat := range room.Seats {
		out[seatType] = *seat
	}
	return out
}

func clonePicoclawRuntime(runtime map[string]PicoclawRuntimeState) map[string]PicoclawRuntimeState {
	if len(runtime) == 0 {
		return nil
	}
	out := make(map[string]PicoclawRuntimeState, len(runtime))
	for participantID, state := range runtime {
		out[participantID] = state
	}
	return out
}

func (a *Arena) updateParticipantBinding(code string, requester string, binding AgentBinding) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	room, ok := a.rooms[normalizeRoomCode(code)]
	if !ok {
		return fmt.Errorf("room not found")
	}
	participant := findParticipantByToken(room, requester)
	if participant == nil {
		return fmt.Errorf("participant not found")
	}
	participant.RealType = normalizeAgentType(binding.RealType)
	participant.DisplayName = strings.TrimSpace(binding.Name)
	participant.BaseURL = strings.TrimSpace(binding.BaseURL)
	participant.APIKey = strings.TrimSpace(binding.APIKey)
	if alias := strings.TrimSpace(binding.PublicAlias); alias != "" {
		participant.PublicAlias = alias
	}
	if connection := strings.TrimSpace(binding.Connection); connection != "" {
		participant.Connection = connection
	}
	room.UpdatedAt = time.Now()
	syncSeats(room)
	return a.saveLocked()
}

func ensurePicoclawRuntime(room *ArenaRoom, participant *Participant) {
	if room.PicoclawRuntime == nil {
		room.PicoclawRuntime = make(map[string]PicoclawRuntimeState)
	}
	if participant == nil || participant.ID == "" {
		return
	}
	if participant.Connection != "managed" ||
		normalizeAgentType(participant.RealType) != AgentTypePicoclaw ||
		(participant.Seat != SeatRedPlayer && participant.Seat != SeatBlackPlayer) {
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

func reconcilePicoclawRuntime(room *ArenaRoom) {
	if room == nil {
		return
	}
	keep := make(map[string]struct{})
	for _, participant := range room.Participants {
		if participant.Connection != "managed" ||
			normalizeAgentType(participant.RealType) != AgentTypePicoclaw ||
			(participant.Seat != SeatRedPlayer && participant.Seat != SeatBlackPlayer) {
			continue
		}
		ensurePicoclawRuntime(room, participant)
		keep[participant.ID] = struct{}{}
	}
	for participantID := range room.PicoclawRuntime {
		if _, ok := keep[participantID]; !ok {
			delete(room.PicoclawRuntime, participantID)
		}
	}
}

func currentRevealState(room *ArenaRoom) RevealState {
	switch {
	case room.RevealRed && room.RevealBlack:
		return RevealStateFull
	case room.RevealRed || room.RevealBlack:
		return RevealStatePartial
	default:
		return RevealStateHidden
	}
}

func buildLogViews(logs []MatchLog, includeReply bool) []MatchLogView {
	out := make([]MatchLogView, 0, len(logs))
	for _, log := range logs {
		item := MatchLogView{
			Time:    log.Time,
			Side:    log.Side,
			Message: log.Message,
			Error:   log.Error,
		}
		if includeReply {
			item.Reply = log.Reply
		}
		out = append(out, item)
	}
	return out
}

func scheduleNextAction(room *ArenaRoom) {
	match := room.ActiveMatch
	if match == nil || match.State.Status != "playing" {
		room.NextActionAt = time.Time{}
		return
	}
	current := currentParticipant(room, match)
	if current == nil || normalizeAgentType(current.RealType) == AgentTypeHuman {
		room.NextActionAt = time.Time{}
		return
	}
	delay := time.Duration(room.StepIntervalMS) * time.Millisecond
	if delay < 0 {
		delay = 0
	}
	room.NextActionAt = time.Now().Add(delay)
}

func matchPhase(room *ArenaRoom) string {
	if room == nil || room.ActiveMatch == nil {
		return "waiting_match"
	}
	match := room.ActiveMatch
	if room.Status == RoomStatusPaused {
		return "paused"
	}
	if room.Status == RoomStatusFinished || match.State.Status == "finished" {
		return "finished"
	}
	current := currentParticipant(room, match)
	if current == nil {
		return "waiting_match"
	}
	if normalizeAgentType(current.RealType) == AgentTypeHuman {
		return "waiting_human"
	}
	return "waiting_agent"
}
