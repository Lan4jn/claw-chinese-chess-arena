package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type picoWSMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type picoclawWSClient struct {
	mu          sync.Mutex
	conn        *websocket.Conn
	endpoint    string
	token       string
	sessionID   string
	authMode    string
	connectedAt time.Time
}

func buildPicoclawWSSessionID(roomCode, participantID string) string {
	return "xiangqi-" + normalizeRoomCode(roomCode) + "-" + strings.TrimSpace(participantID)
}

func normalizePicoWSURL(raw string, sessionID string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported pico ws scheme: %s", parsed.Scheme)
	}
	switch strings.TrimRight(parsed.Path, "/") {
	case "":
		parsed.Path = "/pico/ws"
	case "/pico/ws":
	default:
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/pico/ws"
	}
	query := parsed.Query()
	if strings.TrimSpace(sessionID) != "" {
		query.Set("session_id", strings.TrimSpace(sessionID))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (a *Arena) closeWSClients() {
	a.mu.Lock()
	clients := make([]*picoclawWSClient, 0, len(a.wsClients))
	for _, client := range a.wsClients {
		clients = append(clients, client)
	}
	a.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func (a *Arena) getOrCreateWSClient(participantID string) *picoclawWSClient {
	a.mu.Lock()
	defer a.mu.Unlock()
	client := a.wsClients[participantID]
	if client == nil {
		client = &picoclawWSClient{}
		a.wsClients[participantID] = client
	}
	return client
}

func (a *Arena) requestManagedWSMove(matchID string, participantID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
	token := strings.TrimSpace(player.APIKey)
	if token == "" {
		return "", "", fmt.Errorf("picoclaw pico_ws requires api_key as websocket token")
	}
	sessionID := buildPicoclawWSSessionID(arenaState.RoomCode, participantID)
	endpoint, err := normalizePicoWSURL(player.BaseURL, sessionID)
	if err != nil {
		return "", "", err
	}

	client := a.getOrCreateWSClient(participantID)
	move, reply, authMode, connectedAt, recvAt, err := client.requestMove(
		context.Background(),
		endpoint,
		token,
		sessionID,
		buildMovePrompt(matchID, player, state, legal, arenaState),
		legal,
	)

	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	room, ok := a.rooms[normalizeRoomCode(arenaState.RoomCode)]
	if !ok {
		return move, reply, err
	}
	runtime, runtimeErr := managedPicoclawRuntimeLocked(room, participantID)
	if runtimeErr != nil {
		return move, reply, err
	}
	runtime.WSURL = endpoint
	runtime.WSSessionID = sessionID
	runtime.WSLastSendAt = now
	if authMode != "" {
		runtime.WSAuthMode = authMode
	}
	if !connectedAt.IsZero() {
		runtime.WSConnectedAt = connectedAt
	}
	if !recvAt.IsZero() {
		runtime.WSLastRecvAt = recvAt
	}
	if err != nil {
		runtime.WSLastError = err.Error()
		if runtime.WSHealthy(now) {
			runtime.WSState = PicoclawWSStateRecovering
		} else {
			runtime.WSState = PicoclawWSStateStale
		}
	} else {
		runtime.WSLastError = ""
		runtime.WSState = PicoclawWSStateActive
	}
	room.PicoclawRuntime[participantID] = runtime
	return move, reply, err
}

func (c *picoclawWSClient) requestMove(ctx context.Context, endpoint, token, sessionID, prompt string, legal []string) (string, string, string, time.Time, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnected(ctx, endpoint, token, sessionID); err != nil {
		return "", "", c.authMode, c.connectedAt, time.Time{}, err
	}

	message := picoWSMessage{
		Type:      "message.send",
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
		Payload: map[string]any{
			"content": prompt,
		},
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
		c.closeLocked()
		return "", "", c.authMode, c.connectedAt, time.Time{}, err
	}
	if err := c.conn.WriteJSON(message); err != nil {
		c.closeLocked()
		return "", "", c.authMode, c.connectedAt, time.Time{}, err
	}

	readDeadline := time.Now().Add(90 * time.Second)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(readDeadline) {
		readDeadline = deadline
	}
	for {
		if err := c.conn.SetReadDeadline(readDeadline); err != nil {
			c.closeLocked()
			return "", "", c.authMode, c.connectedAt, time.Time{}, err
		}
		var msg picoWSMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			c.closeLocked()
			return "", "", c.authMode, c.connectedAt, time.Time{}, err
		}
		recvAt := time.Now()
		switch msg.Type {
		case "message.create", "message.update":
			content, _ := msg.Payload["content"].(string)
			if move := extractMove(content, legal); move != "" {
				return move, content, c.authMode, c.connectedAt, recvAt, nil
			}
		case "error":
			payload, _ := json.Marshal(msg.Payload)
			return "", string(payload), c.authMode, c.connectedAt, recvAt, fmt.Errorf("pico_ws error: %s", string(payload))
		}
	}
}

func (c *picoclawWSClient) ensureConnected(ctx context.Context, endpoint, token, sessionID string) error {
	if c.conn != nil && c.endpoint == endpoint && c.token == token && c.sessionID == sessionID {
		return nil
	}
	c.closeLocked()

	type dialAttempt struct {
		authMode    string
		header      http.Header
		subprotocol []string
	}
	attempts := []dialAttempt{
		{
			authMode: "bearer",
			header: http.Header{
				"Authorization": []string{"Bearer " + token},
			},
		},
		{
			authMode:    "subprotocol",
			header:      http.Header{},
			subprotocol: []string{"token." + token},
		},
	}

	var lastErr error
	for _, attempt := range attempts {
		dialer := websocket.Dialer{
			HandshakeTimeout: 15 * time.Second,
			Subprotocols:     attempt.subprotocol,
		}
		conn, _, err := dialer.DialContext(ctx, endpoint, attempt.header)
		if err != nil {
			lastErr = err
			continue
		}
		c.conn = conn
		c.endpoint = endpoint
		c.token = token
		c.sessionID = sessionID
		c.authMode = attempt.authMode
		c.connectedAt = time.Now()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("pico_ws connect failed")
	}
	return lastErr
}

func (c *picoclawWSClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

func (c *picoclawWSClient) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
}
