package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Match struct {
	ID        string                 `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Players   map[Side]PlayerConfig  `json:"players"`
	State     GameState              `json:"state"`
	Auto      bool                   `json:"auto"`
	Interval  int                    `json:"interval_ms"`
	Logs      []MatchLog             `json:"logs"`
}

type MatchLog struct {
	Time    time.Time `json:"time"`
	Side    Side      `json:"side,omitempty"`
	Message string    `json:"message"`
	Reply   string    `json:"reply,omitempty"`
	Error   string    `json:"error,omitempty"`
}

type CreateMatchRequest struct {
	Red        PlayerConfig `json:"red"`
	Black      PlayerConfig `json:"black"`
	Auto       bool         `json:"auto"`
	IntervalMS int          `json:"interval_ms"`
}

type ManualMoveRequest struct {
	Move string `json:"move"`
}

type AutoRequest struct {
	Enabled    bool `json:"enabled"`
	IntervalMS int  `json:"interval_ms"`
}

type Manager struct {
	mu      sync.Mutex
	matches map[string]*Match
	client  *http.Client
}

func NewManager() *Manager {
	return &Manager{
		matches: make(map[string]*Match),
		client:  defaultHTTPClient(),
	}
}

func (m *Manager) Create(req CreateMatchRequest) (*Match, error) {
	normalizePlayer(&req.Red, "本地 Pico", "pico")
	normalizePlayer(&req.Black, "130 Pico", "pico")
	if req.IntervalMS <= 0 {
		req.IntervalMS = 3000
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	match := &Match{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Players: map[Side]PlayerConfig{
			SideRed:   req.Red,
			SideBlack: req.Black,
		},
		State:    NewGame(),
		Auto:     req.Auto,
		Interval: req.IntervalMS,
	}
	match.Logs = append(match.Logs, MatchLog{Time: now, Message: "对局已创建"})
	m.mu.Lock()
	m.matches[id] = match
	m.mu.Unlock()
	if req.Auto {
		go m.autoplay(id)
	}
	return match, nil
}

func (m *Manager) List() []*Match {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Match, 0, len(m.matches))
	for _, match := range m.matches {
		out = append(out, cloneMatch(match))
	}
	return out
}

func (m *Manager) Get(id string) (*Match, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	match, ok := m.matches[id]
	if !ok {
		return nil, false
	}
	return cloneMatch(match), true
}

func (m *Manager) ManualMove(id string, move string) (*Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	match, ok := m.matches[id]
	if !ok {
		return nil, fmt.Errorf("match not found")
	}
	if err := match.State.Apply(strings.TrimSpace(move)); err != nil {
		match.appendLog(MatchLog{Time: time.Now(), Side: match.State.Side, Message: "手动走子失败", Error: err.Error()})
		return cloneMatch(match), err
	}
	match.UpdatedAt = time.Now()
	match.appendLog(MatchLog{Time: time.Now(), Message: "手动走子：" + match.State.LastMove})
	return cloneMatch(match), nil
}

func (m *Manager) Step(ctx context.Context, id string) (*Match, error) {
	m.mu.Lock()
	match, ok := m.matches[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("match not found")
	}
	if match.State.Status != "playing" {
		m.mu.Unlock()
		return cloneMatch(match), nil
	}
	side := match.State.Side
	player := match.Players[side]
	if strings.EqualFold(player.Type, "human") {
		m.mu.Unlock()
		return cloneMatch(match), fmt.Errorf("%s is human, use manual move", side)
	}
	state := match.State
	legal := state.LegalMoveStrings()
	m.mu.Unlock()

	move, reply, err := askPicoForMove(ctx, m.client, id, player, state, legal)

	m.mu.Lock()
	defer m.mu.Unlock()
	match = m.matches[id]
	now := time.Now()
	if err != nil {
		match.appendLog(MatchLog{Time: now, Side: side, Message: "请求 Pico 走子失败", Reply: reply, Error: err.Error()})
		match.UpdatedAt = now
		return cloneMatch(match), err
	}
	if err := match.State.Apply(move); err != nil {
		match.appendLog(MatchLog{Time: now, Side: side, Message: "Pico 返回非法走法："+move, Reply: reply, Error: err.Error()})
		match.UpdatedAt = now
		return cloneMatch(match), err
	}
	match.appendLog(MatchLog{Time: now, Side: side, Message: "Pico 走子：" + move, Reply: reply})
	match.UpdatedAt = now
	return cloneMatch(match), nil
}

func (m *Manager) SetAuto(id string, req AutoRequest) (*Match, error) {
	m.mu.Lock()
	match, ok := m.matches[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("match not found")
	}
	if req.IntervalMS <= 0 {
		req.IntervalMS = match.Interval
	}
	wasAuto := match.Auto
	match.Auto = req.Enabled
	match.Interval = req.IntervalMS
	match.UpdatedAt = time.Now()
	out := cloneMatch(match)
	m.mu.Unlock()
	if req.Enabled && !wasAuto {
		go m.autoplay(id)
	}
	return out, nil
}

func (m *Manager) autoplay(id string) {
	for {
		m.mu.Lock()
		match, ok := m.matches[id]
		if !ok || !match.Auto || match.State.Status != "playing" {
			m.mu.Unlock()
			return
		}
		interval := time.Duration(match.Interval) * time.Millisecond
		if interval <= 0 {
			interval = 3 * time.Second
		}
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, _ = m.Step(ctx, id)
		cancel()
		time.Sleep(interval)
	}
}

func (m *Match) appendLog(log MatchLog) {
	m.Logs = append(m.Logs, log)
	if len(m.Logs) > 80 {
		m.Logs = m.Logs[len(m.Logs)-80:]
	}
}

func cloneMatch(match *Match) *Match {
	cp := *match
	cp.Players = make(map[Side]PlayerConfig, len(match.Players))
	for k, v := range match.Players {
		if v.APIKey != "" {
			v.APIKey = "已设置"
		}
		cp.Players[k] = v
	}
	cp.State.History = append([]MoveRecord(nil), match.State.History...)
	cp.Logs = append([]MatchLog(nil), match.Logs...)
	return &cp
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

func randomID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
