package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	AgentTypeHuman    = "human"
	AgentTypePicoclaw = "picoclaw"
)

type PlayerConfig struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

type Match struct {
	ID           string                `json:"id"`
	RoomCode     string                `json:"room_code"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Players      map[Side]PlayerConfig `json:"players"`
	Aliases      map[Side]string       `json:"aliases"`
	Participants map[Side]string       `json:"participants"`
	State        GameState             `json:"state"`
	IntervalMS   int                   `json:"interval_ms"`
	Logs         []MatchLog            `json:"logs"`
}

type MatchLog struct {
	Time              time.Time `json:"time"`
	Side              Side      `json:"side,omitempty"`
	Type              string    `json:"type,omitempty"`
	Message           string    `json:"message"`
	Reply             string    `json:"reply,omitempty"`
	Error             string    `json:"error,omitempty"`
	Move              string    `json:"move,omitempty"`
	Piece             string    `json:"piece,omitempty"`
	Notation          string    `json:"notation,omitempty"`
	Plain             string    `json:"plain,omitempty"`
	Capture           string    `json:"capture,omitempty"`
	GivesCheck        bool      `json:"gives_check,omitempty"`
	CorrectionAttempt int       `json:"correction_attempt,omitempty"`
	CorrectionLimit   int       `json:"correction_limit,omitempty"`
	Mode              string    `json:"mode,omitempty"`
}

func NewMatch(roomCode string, intervalMS int, players map[Side]PlayerConfig, aliases map[Side]string, participants map[Side]string) (*Match, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	if intervalMS <= 0 {
		intervalMS = 3000
	}
	now := time.Now()
	match := &Match{
		ID:           id,
		RoomCode:     roomCode,
		CreatedAt:    now,
		UpdatedAt:    now,
		Players:      make(map[Side]PlayerConfig, len(players)),
		Aliases:      make(map[Side]string, len(aliases)),
		Participants: make(map[Side]string, len(participants)),
		State:        NewGame(),
		IntervalMS:   intervalMS,
	}
	for side, player := range players {
		cp := player
		normalizePlayer(&cp, defaultPlayerName(side), AgentTypeHuman)
		match.Players[side] = cp
	}
	for side, alias := range aliases {
		match.Aliases[side] = strings.TrimSpace(alias)
	}
	for side, participantID := range participants {
		match.Participants[side] = participantID
	}
	match.appendLog(MatchLog{Time: now, Message: "比赛已创建"})
	return match, nil
}

func (m *Match) ApplyHumanMove(side Side, move string) error {
	if m.State.Status != "playing" {
		return fmt.Errorf("match is not playing")
	}
	if m.State.Side != side {
		return fmt.Errorf("it is not %s's turn", side)
	}
	move = strings.TrimSpace(move)
	beforeBoard := m.State.Board
	if err := m.State.Apply(move); err != nil {
		now := time.Now()
		m.UpdatedAt = now
		m.appendLog(MatchLog{Time: now, Side: side, Type: "human_move_failed", Message: "手动走子失败", Error: err.Error(), Move: move})
		return err
	}
	now := time.Now()
	m.UpdatedAt = now
	commentary := buildMoveCommentary(beforeBoard, side, m.State.LastMove)
	if len(m.State.History) > 0 {
		last := m.State.History[len(m.State.History)-1]
		commentary.Piece = last.Piece
		commentary.Capture = last.Capture
	}
	m.appendLog(MatchLog{
		Time:       now,
		Side:       side,
		Type:       "human_move",
		Message:    "手动走子：" + m.State.LastMove,
		Move:       commentary.Move,
		Piece:      commentary.Piece,
		Notation:   commentary.Notation,
		Plain:      commentary.Plain,
		Capture:    commentary.Capture,
		GivesCheck: m.State.inCheck(m.State.Side),
	})
	return nil
}

func (m *Match) ApplyAgentMove(side Side, move string, reply string) error {
	if m.State.Status != "playing" {
		return fmt.Errorf("match is not playing")
	}
	if m.State.Side != side {
		return fmt.Errorf("it is not %s's turn", side)
	}
	move = strings.TrimSpace(move)
	beforeBoard := m.State.Board
	if err := m.State.Apply(move); err != nil {
		now := time.Now()
		m.UpdatedAt = now
		commentary := buildMoveCommentary(beforeBoard, side, move)
		m.appendLog(MatchLog{
			Time:     now,
			Side:     side,
			Type:     "agent_move_invalid",
			Message:  "选手返回非法走法：" + move,
			Reply:    reply,
			Error:    err.Error(),
			Move:     commentary.Move,
			Piece:    commentary.Piece,
			Notation: commentary.Notation,
			Plain:    commentary.Plain,
		})
		return err
	}
	now := time.Now()
	m.UpdatedAt = now
	commentary := buildMoveCommentary(beforeBoard, side, m.State.LastMove)
	if len(m.State.History) > 0 {
		last := m.State.History[len(m.State.History)-1]
		commentary.Piece = last.Piece
		commentary.Capture = last.Capture
	}
	m.appendLog(MatchLog{
		Time:       now,
		Side:       side,
		Type:       "agent_move",
		Message:    "选手走子：" + m.State.LastMove,
		Reply:      reply,
		Move:       commentary.Move,
		Piece:      commentary.Piece,
		Notation:   commentary.Notation,
		Plain:      commentary.Plain,
		Capture:    commentary.Capture,
		GivesCheck: m.State.inCheck(m.State.Side),
	})
	return nil
}

func (m *Match) AppendAgentError(side Side, reply string, err error) {
	if err == nil {
		return
	}
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Type: "agent_request_failed", Message: "请求选手走子失败", Reply: reply, Error: err.Error()})
}

func (m *Match) AppendAgentModeError(side Side, mode PicoclawActiveMode, reply string, err error) {
	if err == nil {
		return
	}
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{
		Time:    now,
		Side:    side,
		Type:    "agent_request_failed",
		Message: fmt.Sprintf("请求选手走子失败（%s 模式）", mode),
		Reply:   reply,
		Error:   err.Error(),
		Mode:    string(mode),
	})
}

func (m *Match) AppendAgentModeFallback(side Side, from PicoclawActiveMode, to PicoclawActiveMode, reason string) {
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{
		Time:    now,
		Side:    side,
		Type:    "agent_mode_fallback",
		Message: fmt.Sprintf("走子模式切换：%s -> %s", from, to),
		Reply:   strings.TrimSpace(reason),
	})
}

func (m *Match) AppendAgentMoveRejected(side Side, mode PicoclawActiveMode, board Board, move string, reply string, err error, attempt int, limit int) {
	now := time.Now()
	m.UpdatedAt = now
	message := fmt.Sprintf("选手走子被驳回：%s（第 %d/%d 次）", strings.TrimSpace(move), attempt, limit)
	commentary := buildMoveCommentary(board, side, move)
	m.appendLog(MatchLog{
		Time:              now,
		Side:              side,
		Type:              "agent_move_rejected",
		Message:           message,
		Reply:             reply,
		Error:             err.Error(),
		Move:              commentary.Move,
		Piece:             commentary.Piece,
		Notation:          commentary.Notation,
		Plain:             commentary.Plain,
		CorrectionAttempt: attempt,
		CorrectionLimit:   limit,
		Mode:              string(mode),
	})
}

func (m *Match) AppendAgentRetryRequested(side Side, mode PicoclawActiveMode, nextAttempt int, limit int) {
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{
		Time:              now,
		Side:              side,
		Type:              "agent_retry_requested",
		Message:           fmt.Sprintf("系统要求选手重新走子（%s 模式，第 %d/%d 次）", mode, nextAttempt, limit),
		CorrectionAttempt: nextAttempt,
		CorrectionLimit:   limit,
		Mode:              string(mode),
	})
}

func (m *Match) AppendAgentRetryExhausted(side Side, mode PicoclawActiveMode, limit int) {
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{
		Time:            now,
		Side:            side,
		Type:            "agent_retry_exhausted",
		Message:         fmt.Sprintf("选手连续%d次提交违规着法（%s 模式）", limit, mode),
		CorrectionLimit: limit,
		Mode:            string(mode),
	})
}

func (m *Match) CurrentPlayer() PlayerConfig {
	return m.Players[m.State.Side]
}

func (m *Match) CurrentParticipantID() string {
	return m.Participants[m.State.Side]
}

func (m *Match) CurrentAlias() string {
	return m.Aliases[m.State.Side]
}

func (m *Match) OpponentAlias() string {
	return m.Aliases[opposite(m.State.Side)]
}

func (m *Match) LegalMoves() []string {
	if m.State.Status != "playing" {
		return nil
	}
	return m.State.LegalMoveStrings()
}

func (m *Match) appendLog(log MatchLog) {
	m.Logs = append(m.Logs, log)
	if len(m.Logs) > 120 {
		m.Logs = m.Logs[len(m.Logs)-120:]
	}
}

func normalizePlayer(p *PlayerConfig, fallbackName, fallbackType string) {
	p.Type = strings.TrimSpace(strings.ToLower(p.Type))
	if p.Type == "" {
		p.Type = fallbackType
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = fallbackName
	}
	p.BaseURL = strings.TrimSpace(p.BaseURL)
	p.APIKey = strings.TrimSpace(p.APIKey)
}

func defaultPlayerName(side Side) string {
	if side == SideRed {
		return "红方选手"
	}
	return "黑方选手"
}

func randomID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
