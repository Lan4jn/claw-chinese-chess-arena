package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	AgentTypeHuman  = "human"
	AgentTypePico   = "pico"
	AgentTypeClaw   = "claw"
	AgentTypeCustom = "custom_agent"
)

type PlayerConfig struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

type Match struct {
	ID          string                 `json:"id"`
	RoomCode    string                 `json:"room_code"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Players     map[Side]PlayerConfig  `json:"players"`
	Aliases     map[Side]string        `json:"aliases"`
	Participants map[Side]string       `json:"participants"`
	State       GameState              `json:"state"`
	IntervalMS  int                    `json:"interval_ms"`
	Logs        []MatchLog             `json:"logs"`
}

type MatchLog struct {
	Time    time.Time `json:"time"`
	Side    Side      `json:"side,omitempty"`
	Message string    `json:"message"`
	Reply   string    `json:"reply,omitempty"`
	Error   string    `json:"error,omitempty"`
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
		ID:          id,
		RoomCode:    roomCode,
		CreatedAt:   now,
		UpdatedAt:   now,
		Players:     make(map[Side]PlayerConfig, len(players)),
		Aliases:     make(map[Side]string, len(aliases)),
		Participants: make(map[Side]string, len(participants)),
		State:       NewGame(),
		IntervalMS:  intervalMS,
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
	if err := m.State.Apply(move); err != nil {
		now := time.Now()
		m.UpdatedAt = now
		m.appendLog(MatchLog{Time: now, Side: side, Message: "手动走子失败", Error: err.Error()})
		return err
	}
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Message: "手动走子：" + m.State.LastMove})
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
	if err := m.State.Apply(move); err != nil {
		now := time.Now()
		m.UpdatedAt = now
		m.appendLog(MatchLog{Time: now, Side: side, Message: "选手返回非法走法：" + move, Reply: reply, Error: err.Error()})
		return err
	}
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Message: "选手走子：" + m.State.LastMove, Reply: reply})
	return nil
}

func (m *Match) AppendAgentError(side Side, reply string, err error) {
	if err == nil {
		return
	}
	now := time.Now()
	m.UpdatedAt = now
	m.appendLog(MatchLog{Time: now, Side: side, Message: "请求选手走子失败", Reply: reply, Error: err.Error()})
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
