package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func normalizeWebSocketURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "", "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported websocket scheme")
	}
	if parsed.Host == "" && parsed.Path != "" {
		parsed.Host = parsed.Path
		parsed.Path = ""
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("base_url must include host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/ws"
	} else if !strings.HasSuffix(parsed.Path, "/ws") {
		parsed.Path += "/ws"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func wsConnKey(matchID string, side Side) string {
	return matchID + ":" + string(side)
}

func (a *Arena) websocketConnFor(key, endpoint string) (*websocket.Conn, error) {
	a.wsMu.Lock()
	if conn := a.wsConns[key]; conn != nil {
		a.wsMu.Unlock()
		return conn, nil
	}
	a.wsMu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	conn, _, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return nil, err
	}

	a.wsMu.Lock()
	a.wsConns[key] = conn
	a.wsMu.Unlock()
	return conn, nil
}

func (a *Arena) closeWebSocketConn(key string) {
	a.wsMu.Lock()
	conn := a.wsConns[key]
	delete(a.wsConns, key)
	a.wsMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (a *Arena) closeAllWebSocketConns() {
	a.wsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(a.wsConns))
	for key, conn := range a.wsConns {
		delete(a.wsConns, key)
		conns = append(conns, conn)
	}
	a.wsMu.Unlock()
	for _, conn := range conns {
		if conn != nil {
			_ = conn.Close()
		}
	}
}

func (a *Arena) deliverWebSocketTurn(ctx context.Context, match *Match, room *ArenaRoom, player PlayerConfig, arenaState PromptArenaState) (AgentTurnResponse, error) {
	if strings.TrimSpace(player.BaseURL) == "" {
		return AgentTurnResponse{}, fmt.Errorf("%s has no base_url", player.Name)
	}
	endpoint, err := normalizeWebSocketURL(player.BaseURL)
	if err != nil {
		return AgentTurnResponse{}, err
	}

	key := wsConnKey(match.ID, match.State.Side)
	conn, err := a.websocketConnFor(key, endpoint)
	if err != nil {
		return AgentTurnResponse{}, err
	}

	req := buildAgentTurnRequest(match, room, TransportModeWebSocket, player, arenaState, nil)
	deadline := time.Now().Add(15 * time.Second)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		a.closeWebSocketConn(key)
		return AgentTurnResponse{}, err
	}
	if err := conn.WriteJSON(req); err != nil {
		a.closeWebSocketConn(key)
		return AgentTurnResponse{}, err
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		a.closeWebSocketConn(key)
		return AgentTurnResponse{}, err
	}
	var resp AgentTurnResponse
	if err := conn.ReadJSON(&resp); err != nil {
		a.closeWebSocketConn(key)
		return AgentTurnResponse{}, err
	}
	select {
	case <-ctx.Done():
		return AgentTurnResponse{}, ctx.Err()
	default:
	}
	return resp, nil
}
