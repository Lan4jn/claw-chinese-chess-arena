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
	ParticipantID              string                `json:"participant_id"`
	PreferredMode              PicoclawPreferredMode `json:"preferred_mode"`
	ActiveMode                 PicoclawActiveMode    `json:"active_mode"`
	SessionState               PicoclawSessionState  `json:"session_state"`
	SessionID                  string                `json:"session_id,omitempty"`
	LastHeartbeatAt            time.Time             `json:"last_heartbeat_at,omitempty"`
	LeaseExpiresAt             time.Time             `json:"lease_expires_at,omitempty"`
	RecoveryDeadlineAt         time.Time             `json:"recovery_deadline_at,omitempty"`
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
	}
}

func (s PicoclawRuntimeState) SessionHealthy(now time.Time) bool {
	return s.SessionState == PicoclawSessionStateActive && !s.LeaseExpiresAt.IsZero() && now.Before(s.LeaseExpiresAt)
}

func resolvePicoclawActiveMode(state PicoclawRuntimeState, now time.Time) PicoclawActiveMode {
	switch state.PreferredMode {
	case PicoclawModePreferMessage:
		return PicoclawActiveModeMessage
	case PicoclawModePreferSession:
		if state.SessionState == PicoclawSessionStateOpening || state.SessionHealthy(now) {
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
