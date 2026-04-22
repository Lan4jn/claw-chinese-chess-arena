package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func deliverHTTPSessionTurn(ctx context.Context, client *http.Client, match *Match, room *ArenaRoom, player PlayerConfig, arenaState PromptArenaState) (AgentTurnResponse, error) {
	if strings.TrimSpace(player.BaseURL) == "" {
		return AgentTurnResponse{}, fmt.Errorf("%s has no base_url", player.Name)
	}

	side := match.State.Side
	session := match.AgentSessions[side]
	if session.SessionID == "" || (!session.LeaseExpiresAt.IsZero() && time.Now().After(session.LeaseExpiresAt)) {
		opened, err := openHTTPSession(ctx, client, player)
		if err != nil {
			return AgentTurnResponse{}, err
		}
		session.SessionID = opened.SessionID
		session.ResumeToken = opened.ResumeToken
		session.ConnectionState = AgentConnectionStateConnected
		session.LastHeartbeatAt = time.Now()
		if opened.LeaseTTLMS > 0 {
			session.LeaseExpiresAt = time.Now().Add(time.Duration(opened.LeaseTTLMS) * time.Millisecond)
		}
		match.AgentSessions[side] = session
	}

	req := buildAgentTurnRequest(match, room, TransportModeHTTPSession, player, arenaState, &session)
	resp, err := postJSON[AgentTurnResponse](ctx, client, normalizeHTTPSessionURL(player.BaseURL, "/session/turn"), req)
	if err != nil {
		return AgentTurnResponse{}, err
	}
	if resp.SessionID != "" {
		session.SessionID = resp.SessionID
	}
	session.ConnectionState = AgentConnectionStateConnected
	session.LastHeartbeatAt = time.Now()
	match.AgentSessions[side] = session
	return resp, nil
}

func openHTTPSession(ctx context.Context, client *http.Client, player PlayerConfig) (HTTPSessionOpenResponse, error) {
	payload := map[string]string{
		"player_name": player.Name,
		"player_type": player.Type,
	}
	return postJSON[HTTPSessionOpenResponse](ctx, client, normalizeHTTPSessionURL(player.BaseURL, "/session/open"), payload)
}

func normalizeHTTPSessionURL(baseURL string, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func postJSON[T any](ctx context.Context, client *http.Client, endpoint string, payload any) (T, error) {
	var zero T
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded T
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return zero, err
	}
	return decoded, nil
}
