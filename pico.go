package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type PlayerConfig struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

type picoMessageRequest struct {
	SessionID         string `json:"session_id,omitempty"`
	SenderID          string `json:"sender_id,omitempty"`
	SenderDisplayName string `json:"sender_display_name,omitempty"`
	Message           string `json:"message,omitempty"`
	APIKey            string `json:"api_key,omitempty"`
}

type picoMessageResponse struct {
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

var movePattern = regexp.MustCompile(`[a-i][0-9]-[a-i][0-9]`)

func askPicoForMove(ctx context.Context, client *http.Client, matchID string, player PlayerConfig, state GameState, legal []string) (string, string, error) {
	if strings.TrimSpace(player.BaseURL) == "" {
		return "", "", fmt.Errorf("%s has no base_url", player.Name)
	}
	endpoint, err := normalizePicoMessageURL(player.BaseURL)
	if err != nil {
		return "", "", err
	}
	prompt := buildMovePrompt(matchID, player, state, legal)
	payload := picoMessageRequest{
		SessionID:         "xiangqi-" + matchID,
		SenderID:          "pico-xiangqi-arena",
		SenderDisplayName: "Pico Xiangqi Arena",
		Message:           prompt,
		APIKey:            strings.TrimSpace(player.APIKey),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if payload.APIKey != "" {
		req.Header.Set("X-PicoClaw-API-Key", payload.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("contact %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", string(respBody), fmt.Errorf("pico returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded picoMessageResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", string(respBody), fmt.Errorf("decode pico response: %w", err)
	}
	if decoded.Error != "" {
		return "", decoded.Reply, fmt.Errorf("%s", decoded.Error)
	}
	move := extractMove(decoded.Reply, legal)
	if move == "" {
		return "", decoded.Reply, fmt.Errorf("pico reply did not contain a legal move")
	}
	return move, decoded.Reply, nil
}

func buildMovePrompt(matchID string, player PlayerConfig, state GameState, legal []string) string {
	sideName := "红方"
	if state.Side == SideBlack {
		sideName = "黑方"
	}
	return fmt.Sprintf(`你正在参加一场中国象棋对局，比赛 ID：%s。
你是：%s（%s）。

棋盘坐标固定为：左到右 a-i，上到下 0-9。红方在下方，黑方在上方。
棋子用英文缩写：K/k 将帅，A/a 士，B/b 象相，N/n 马，R/r 车，C/c 炮，P/p 兵卒；大写是红方，小写是黑方。

当前棋盘：
%s
轮到你走。只能从下面合法走法中选择一个：
%s

请只给出一步棋，格式必须包含 MOVE: a0-a1，例如：
MOVE: h9-g7
不要执行命令，不要解释长篇推理。`, matchID, player.Name, sideName, BoardText(state.Board), strings.Join(legal, ", "))
}

func extractMove(reply string, legal []string) string {
	legalSet := make(map[string]struct{}, len(legal))
	for _, mv := range legal {
		legalSet[mv] = struct{}{}
	}
	matches := movePattern.FindAllString(strings.ToLower(reply), -1)
	for _, mv := range matches {
		if _, ok := legalSet[mv]; ok {
			return mv
		}
	}
	return ""
}

func normalizePicoMessageURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
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
		parsed.Path = "/message"
	} else if !strings.HasSuffix(parsed.Path, "/message") {
		parsed.Path += "/message"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}
