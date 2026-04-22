package main

import (
	"fmt"
	"time"
)

type TransportMode string

const (
	TransportModeHTTPSession TransportMode = "http_session"
	TransportModeWebSocket   TransportMode = "websocket"
)

type MatchTransportState string

const (
	MatchTransportStatePending  MatchTransportState = "pending"
	MatchTransportStateActive   MatchTransportState = "active"
	MatchTransportStateDegraded MatchTransportState = "degraded"
	MatchTransportStateFailed   MatchTransportState = "failed"
)

type ServiceTransportConfig struct {
	DefaultMode   TransportMode `json:"default_mode"`
	ConfigVersion int           `json:"config_version"`
	UpdatedAt     time.Time     `json:"updated_at,omitempty"`
}

type AgentConnectionState string

const (
	AgentConnectionStateConnected    AgentConnectionState = "connected"
	AgentConnectionStateDisconnected AgentConnectionState = "disconnected"
	AgentConnectionStateRecovering   AgentConnectionState = "recovering"
)

type AgentSessionState struct {
	SessionID       string               `json:"session_id,omitempty"`
	ResumeToken     string               `json:"resume_token,omitempty"`
	ConnectionState AgentConnectionState `json:"connection_state,omitempty"`
	LastHeartbeatAt time.Time            `json:"last_heartbeat_at,omitempty"`
	LeaseExpiresAt  time.Time            `json:"lease_expires_at,omitempty"`
	WSConnID        string               `json:"ws_conn_id,omitempty"`
}

type AgentTurnRequest struct {
	ProtocolVersion int      `json:"protocol_version"`
	MatchID         string   `json:"match_id"`
	RoomCode        string   `json:"room_code"`
	Seat            string   `json:"seat"`
	Side            string   `json:"side"`
	TransportMode   string   `json:"transport_mode,omitempty"`
	TurnID          string   `json:"turn_id"`
	MoveCount       int      `json:"move_count"`
	StepIntervalMS  int      `json:"step_interval_ms"`
	OpponentAlias   string   `json:"opponent_alias"`
	BoardRows       []string `json:"board_rows"`
	BoardText       string   `json:"board_text"`
	LegalMoves      []string `json:"legal_moves"`
	Prompt          string   `json:"prompt"`
	SessionID       string   `json:"session_id,omitempty"`
	ResumeToken     string   `json:"resume_token,omitempty"`
}

type AgentTurnResponse struct {
	TurnID       string `json:"turn_id"`
	Move         string `json:"move"`
	Reply        string `json:"reply"`
	AgentState   string `json:"agent_state"`
	RetryAfterMS int    `json:"retry_after_ms,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}

type HTTPSessionOpenResponse struct {
	SessionID       string `json:"session_id"`
	ResumeToken     string `json:"resume_token,omitempty"`
	LeaseTTLMS      int    `json:"lease_ttl_ms,omitempty"`
	ConnectionState string `json:"connection_state,omitempty"`
}

type TransportConfigView struct {
	DefaultMode   string    `json:"default_mode"`
	ConfigVersion int       `json:"config_version"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

func defaultServiceTransportConfig() ServiceTransportConfig {
	return ServiceTransportConfig{DefaultMode: TransportModeHTTPSession}
}

func buildTransportConfigView(config ServiceTransportConfig) TransportConfigView {
	return TransportConfigView{
		DefaultMode:   string(config.DefaultMode),
		ConfigVersion: config.ConfigVersion,
		UpdatedAt:     config.UpdatedAt,
	}
}

func buildTurnID(match *Match) string {
	return fmt.Sprintf("%s-%s-%d", match.ID, match.State.Side, match.State.MoveCount)
}

func seatTypeForSide(side Side) SeatType {
	if side == SideRed {
		return SeatRedPlayer
	}
	return SeatBlackPlayer
}

func buildAgentTurnRequest(match *Match, room *ArenaRoom, mode TransportMode, player PlayerConfig, arenaState PromptArenaState, session *AgentSessionState) AgentTurnRequest {
	req := AgentTurnRequest{
		ProtocolVersion: 1,
		MatchID:         match.ID,
		RoomCode:        room.Code,
		Seat:            string(seatTypeForSide(match.State.Side)),
		Side:            string(match.State.Side),
		TransportMode:   string(mode),
		TurnID:          buildTurnID(match),
		MoveCount:       match.State.MoveCount,
		StepIntervalMS:  room.StepIntervalMS,
		OpponentAlias:   arenaState.OpponentAlias,
		BoardRows:       BoardRows(match.State.Board),
		BoardText:       BoardText(match.State.Board),
		LegalMoves:      match.LegalMoves(),
		Prompt:          buildMovePrompt(match.ID, player, match.State, match.LegalMoves(), arenaState),
	}
	if session != nil {
		req.SessionID = session.SessionID
		req.ResumeToken = session.ResumeToken
	}
	return req
}
