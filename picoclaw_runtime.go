package main

import "time"

type PicoclawPreferredMode string
type PicoclawActiveMode string
type PicoclawSessionState string
type PicoclawWSState string
type PicoclawSessionTurnStatus string

const (
	PicoclawModeAuto          PicoclawPreferredMode = "auto"
	PicoclawModePreferPicoWS  PicoclawPreferredMode = "prefer_pico_ws"
	PicoclawModePreferSession PicoclawPreferredMode = "prefer_session"
	PicoclawModePreferMessage PicoclawPreferredMode = "prefer_message"
)

const (
	PicoclawActiveModePicoWS  PicoclawActiveMode = "pico_ws"
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

const (
	PicoclawWSStateIdle       PicoclawWSState = "idle"
	PicoclawWSStateConnecting PicoclawWSState = "connecting"
	PicoclawWSStateActive     PicoclawWSState = "active"
	PicoclawWSStateStale      PicoclawWSState = "stale"
	PicoclawWSStateRecovering PicoclawWSState = "recovering"
	PicoclawWSStateClosed     PicoclawWSState = "closed"
)

const (
	PicoclawSessionTurnStatusIdle     PicoclawSessionTurnStatus = "idle"
	PicoclawSessionTurnStatusTurn     PicoclawSessionTurnStatus = "turn"
	PicoclawSessionTurnStatusAccepted PicoclawSessionTurnStatus = "accepted"
)

type PicoclawRuntimeState struct {
	ParticipantID              string                `json:"participant_id"`
	PreferredMode              PicoclawPreferredMode `json:"preferred_mode"`
	ActiveMode                 PicoclawActiveMode    `json:"active_mode"`
	SessionState               PicoclawSessionState  `json:"session_state"`
	WSState                    PicoclawWSState       `json:"ws_state,omitempty"`
	SessionID                  string                `json:"session_id,omitempty"`
	SessionAuthToken           string                `json:"session_token,omitempty"`
	WSURL                      string                `json:"ws_url,omitempty"`
	WSSessionID                string                `json:"ws_session_id,omitempty"`
	WSAuthMode                 string                `json:"ws_auth_mode,omitempty"`
	SessionOpenedAt            time.Time             `json:"session_opened_at,omitempty"`
	LastHeartbeatAt            time.Time             `json:"last_heartbeat_at,omitempty"`
	LeaseExpiresAt             time.Time             `json:"lease_expires_at,omitempty"`
	WSConnectedAt              time.Time             `json:"ws_connected_at,omitempty"`
	WSLastRecvAt               time.Time             `json:"ws_last_recv_at,omitempty"`
	WSLastSendAt               time.Time             `json:"ws_last_send_at,omitempty"`
	WSLastError                string                `json:"ws_last_error,omitempty"`
	RecoveryDeadlineAt         time.Time             `json:"recovery_deadline_at,omitempty"`
	ConsecutiveWSFailures      int                   `json:"consecutive_ws_failures,omitempty"`
	ConsecutiveSessionFailures int                   `json:"consecutive_session_failures,omitempty"`
	ConsecutiveMessageFailures int                   `json:"consecutive_message_failures,omitempty"`
	LastModeSwitchAt           time.Time             `json:"last_mode_switch_at,omitempty"`
	LastSwitchReason           string                `json:"last_switch_reason,omitempty"`
	LastInviteAt               time.Time             `json:"last_invite_at,omitempty"`
	LastInviteStatus           string                `json:"last_invite_status,omitempty"`
}

func newPicoclawRuntimeState(participantID string) PicoclawRuntimeState {
	return PicoclawRuntimeState{
		ParticipantID: participantID,
		PreferredMode: PicoclawModeAuto,
		ActiveMode:    PicoclawActiveModeMessage,
		SessionState:  PicoclawSessionStateIdle,
		WSState:       PicoclawWSStateIdle,
	}
}

func (s PicoclawRuntimeState) SessionHealthy(now time.Time) bool {
	return s.SessionState == PicoclawSessionStateActive && !s.LeaseExpiresAt.IsZero() && now.Before(s.LeaseExpiresAt)
}

func (s PicoclawRuntimeState) WSHealthy(now time.Time) bool {
	if s.WSState != PicoclawWSStateActive {
		return false
	}
	if !s.WSLastRecvAt.IsZero() {
		return now.Sub(s.WSLastRecvAt) <= 2*time.Minute
	}
	if !s.WSConnectedAt.IsZero() {
		return now.Sub(s.WSConnectedAt) <= 2*time.Minute
	}
	return true
}

func resolvePicoclawAttemptModes(state PicoclawRuntimeState, now time.Time) []PicoclawActiveMode {
	out := make([]PicoclawActiveMode, 0, 3)
	seen := map[PicoclawActiveMode]bool{}
	add := func(mode PicoclawActiveMode) {
		if mode == "" || seen[mode] {
			return
		}
		seen[mode] = true
		out = append(out, mode)
	}

	sessionReady := state.SessionState == PicoclawSessionStateOpening || state.SessionHealthy(now)
	wsReady := state.WSHealthy(now)

	switch state.PreferredMode {
	case PicoclawModePreferPicoWS:
		add(PicoclawActiveModePicoWS)
		if sessionReady {
			add(PicoclawActiveModeSession)
		}
		add(PicoclawActiveModeMessage)
	case PicoclawModePreferSession:
		if sessionReady {
			add(PicoclawActiveModeSession)
		}
		if wsReady {
			add(PicoclawActiveModePicoWS)
		}
		add(PicoclawActiveModeMessage)
	case PicoclawModePreferMessage:
		add(PicoclawActiveModeMessage)
		if wsReady {
			add(PicoclawActiveModePicoWS)
		}
		if sessionReady {
			add(PicoclawActiveModeSession)
		}
	default:
		if wsReady {
			add(PicoclawActiveModePicoWS)
		}
		if sessionReady {
			add(PicoclawActiveModeSession)
		}
		add(PicoclawActiveModeMessage)
	}
	return out
}

func resolvePicoclawActiveMode(state PicoclawRuntimeState, now time.Time) PicoclawActiveMode {
	modes := resolvePicoclawAttemptModes(state, now)
	if len(modes) == 0 {
		return PicoclawActiveModeMessage
	}
	return modes[0]
}

type PicoclawSessionTurn struct {
	TurnID         string   `json:"turn_id"`
	MatchID        string   `json:"match_id"`
	RoomCode       string   `json:"room_code"`
	Seat           string   `json:"seat"`
	Side           string   `json:"side"`
	MoveCount      int      `json:"move_count"`
	StepIntervalMS int      `json:"step_interval_ms"`
	OpponentAlias  string   `json:"opponent_alias"`
	BoardRows      []string `json:"board_rows"`
	BoardText      string   `json:"board_text"`
	LegalMoves     []string `json:"legal_moves"`
	Prompt         string   `json:"prompt"`
	SessionID      string   `json:"session_id,omitempty"`
}

type PicoclawSessionTurnResponse struct {
	Status       PicoclawSessionTurnStatus `json:"status"`
	RetryAfterMS int                       `json:"retry_after_ms,omitempty"`
	Turn         *PicoclawSessionTurn      `json:"turn,omitempty"`
}
