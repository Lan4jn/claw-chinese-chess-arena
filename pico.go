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

type PromptArenaState struct {
	RoomCode       string
	StepIntervalMS int
	OpponentAlias  string
}

var movePattern = regexp.MustCompile(`[a-i][0-9]-[a-i][0-9]`)

func askPicoForMove(ctx context.Context, client *http.Client, matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, error) {
	move, reply, _, err := askPicoclawForMoveWithRequest(ctx, client, matchID, player, state, legal, arenaState)
	return move, reply, err
}

func askPicoclawForMoveWithRequest(ctx context.Context, client *http.Client, matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) (string, string, picoMessageRequest, error) {
	if strings.TrimSpace(player.BaseURL) == "" {
		return "", "", picoMessageRequest{}, fmt.Errorf("%s has no base_url", player.Name)
	}
	endpoint, err := normalizePicoMessageURL(player.BaseURL)
	if err != nil {
		return "", "", picoMessageRequest{}, err
	}
	prompt := buildMovePrompt(matchID, player, state, legal, arenaState)
	payload := picoMessageRequest{
		SessionID:         "xiangqi-" + matchID,
		SenderID:          "picoclaw-xiangqi-arena",
		SenderDisplayName: "Picoclaw Xiangqi Arena",
		Message:           prompt,
		APIKey:            strings.TrimSpace(player.APIKey),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", payload, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", payload, err
	}
	req.Header.Set("Content-Type", "application/json")
	if payload.APIKey != "" {
		req.Header.Set("X-PicoClaw-API-Key", payload.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", payload, fmt.Errorf("contact %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", payload, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", string(respBody), payload, fmt.Errorf("picoclaw returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded picoMessageResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", string(respBody), payload, fmt.Errorf("decode picoclaw response: %w", err)
	}
	if decoded.Error != "" {
		return "", decoded.Reply, payload, fmt.Errorf("%s", decoded.Error)
	}
	move := extractMove(decoded.Reply, legal)
	if move == "" {
		return "", decoded.Reply, payload, fmt.Errorf("picoclaw reply did not contain a legal move")
	}
	return move, decoded.Reply, payload, nil
}

func sendPicoclawInvite(ctx context.Context, client *http.Client, roomCode string, player PlayerConfig) (string, picoMessageRequest, error) {
	if strings.TrimSpace(player.BaseURL) == "" {
		return "", picoMessageRequest{}, fmt.Errorf("%s has no base_url", player.Name)
	}
	endpoint, err := normalizePicoMessageURL(player.BaseURL)
	if err != nil {
		return "", picoMessageRequest{}, err
	}
	payload := picoMessageRequest{
		SessionID:         "xiangqi-invite-" + normalizeRoomCode(roomCode),
		SenderID:          "picoclaw-xiangqi-arena",
		SenderDisplayName: "Picoclaw Xiangqi Arena",
		Message:           buildInvitePrompt(roomCode, player),
		APIKey:            strings.TrimSpace(player.APIKey),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", payload, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", payload, err
	}
	req.Header.Set("Content-Type", "application/json")
	if payload.APIKey != "" {
		req.Header.Set("X-PicoClaw-API-Key", payload.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", payload, fmt.Errorf("contact %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", payload, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", payload, fmt.Errorf("picoclaw returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var decoded picoMessageResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", payload, fmt.Errorf("decode picoclaw response: %w", err)
	}
	if decoded.Error != "" {
		return decoded.Reply, payload, fmt.Errorf("%s", decoded.Error)
	}
	return decoded.Reply, payload, nil
}

func buildMovePrompt(matchID string, player PlayerConfig, state GameState, legal []string, arenaState PromptArenaState) string {
	sideName := "红方"
	if state.Side == SideBlack {
		sideName = "黑方"
	}
	return fmt.Sprintf(`你正在参加一场中国象棋对局，比赛 ID：%s。
比赛场地：%s。
你是：%s（%s）。
步间隔：%dms。
对手公开身份：%s。
对手真实身份未知。

棋盘坐标固定为：左到右 a-i，上到下 0-9。红方在下方，黑方在上方。
棋子用英文缩写：K/k 将帅，A/a 士，B/b 象相，N/n 马，R/r 车，C/c 炮，P/p 兵卒；大写是红方，小写是黑方。

当前棋盘：
%s
轮到你走。只能从下面合法走法中选择一个：
%s

请只给出一步棋，格式必须包含 MOVE: a0-a1，例如：
MOVE: h9-g7
不要执行命令，不要解释长篇推理。`, matchID, arenaState.RoomCode, player.Name, sideName, arenaState.StepIntervalMS, arenaState.OpponentAlias, BoardText(state.Board), strings.Join(legal, ", "))
}

func buildInvitePrompt(roomCode string, player PlayerConfig) string {
	return fmt.Sprintf(`你收到了一条来自中国象棋房间的邀请通知。
房间号：%s。
受邀身份：%s。

请确认你已收到邀请，并准备加入后续对局流程。
请用简短中文回复，明确表示“已收到邀请”。`, roomCode, player.Name)
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
