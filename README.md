# Pico Xiangqi Arena

一个单进程的中国象棋对战服务，负责：

- 房间管理
- 主持人与观战视图
- 人类选手浏览器走子
- 受管 agent 对局
- 服务级 agent transport 模式切换

当前后端兼容目标是 Go 1.20 这一档，适合继续做 Windows 7 / 2008 R2 方向的实机验证，同时也可以正常编译 Linux 版本。

## 启动

默认监听 `:8080`：

```bash
GOTOOLCHAIN=local go run .
```

自定义端口：

```bash
PORT=9090 GOTOOLCHAIN=local go run .
```

启动后默认访问：

```text
http://127.0.0.1:8080
```

运行时快照默认写入：

```text
runtime/arena-snapshot.json
```

## 基本使用

### 1. 进入房间

浏览器打开首页后，输入比赛码进入房间。

- 第一个进入的人会成为主持人
- 第二个抢比赛席的人会进入另一侧席位
- 后续进入者默认转为观众

### 2. 主持人控制

主持人可以在页面中：

- 修改步间隔
- 修改默认视图
- 配置红黑席位
- 开始、暂停、恢复、重开比赛
- 查看 transport 运行状态

### 3. 人类选手走子

轮到人类席位时：

- 先点起点
- 再点目标格
- 前端会提交到 `/api/arena/{code}/move`

## Service 级 Transport 切换

当前实现支持两种服务默认 transport 模式：

- `http_session`
- `websocket`

切换接口：

### 查看当前默认模式

```bash
curl http://127.0.0.1:8080/api/admin/transport
```

示例返回：

```json
{
  "default_mode": "http_session",
  "config_version": 0,
  "updated_at": "0001-01-01T00:00:00Z"
}
```

### 切到 WebSocket

```bash
curl -X POST http://127.0.0.1:8080/api/admin/transport \
  -H 'Content-Type: application/json' \
  -d '{"default_mode":"websocket"}'
```

### 切到 HTTP Session

```bash
curl -X POST http://127.0.0.1:8080/api/admin/transport \
  -H 'Content-Type: application/json' \
  -d '{"default_mode":"http_session"}'
```

注意：

- 这是服务级默认值
- 只影响之后新开的对局
- 已经开始的对局会保持开局时锁定的 transport mode
- 如果某局以 `websocket` 开局，但 WebSocket 失败，服务端会尝试自动降级到 `http_session`

## Agent 接入说明

当前实现里，受管 agent 仍然通过席位配置里的 `base_url` 接入。

### HTTP Session 模式

当对局使用 `http_session` 时，arena 会主动请求 agent：

- `POST {base_url}/session/open`
- `POST {base_url}/session/turn`

#### `/session/open`

请求体示例：

```json
{
  "player_name": "托管黑方",
  "player_type": "pico"
}
```

响应体示例：

```json
{
  "session_id": "sess-1",
  "resume_token": "resume-1",
  "lease_ttl_ms": 30000,
  "connection_state": "connected"
}
```

#### `/session/turn`

请求体示例：

```json
{
  "protocol_version": 1,
  "match_id": "abcd1234",
  "room_code": "demo-room",
  "seat": "black_player",
  "side": "black",
  "transport_mode": "http_session",
  "turn_id": "abcd1234-black-1",
  "move_count": 1,
  "step_interval_ms": 1500,
  "opponent_alias": "玻璃杯",
  "board_rows": ["rnbakabnr", ".........", ".....c..."],
  "board_text": "...",
  "legal_moves": ["a3-a4", "c3-c4"],
  "prompt": "..."
}
```

响应体示例：

```json
{
  "turn_id": "abcd1234-black-1",
  "move": "a3-a4",
  "reply": "MOVE: a3-a4",
  "agent_state": "ok",
  "session_id": "sess-1"
}
```

### WebSocket 模式

当对局使用 `websocket` 时，arena 会主动连：

```text
ws://host:port/ws
```

如果 `base_url` 是 `http://127.0.0.1:9000`，arena 会自动转成：

```text
ws://127.0.0.1:9000/ws
```

消息内容和 HTTP session 的 `AgentTurnRequest / AgentTurnResponse` 一致，只是通过 WebSocket 收发。

如果 WebSocket 建连或读写失败，服务端会尝试降级到 `http_session`。

## 编译

### 本机编译

```bash
GOTOOLCHAIN=local go build .
```

### 编译 Linux amd64 版本

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -o dist/pico-xiangqi-arena-linux-amd64 .
```

## 测试

运行全部测试：

```bash
GOTOOLCHAIN=local go test ./...
```

## 当前实现边界

这版已经支持：

- 服务级默认 transport 模式切换
- 对局启动时锁定 transport mode
- `http_session` transport
- `websocket` transport
- WebSocket 失败后自动降级到 HTTP session

这版还没有完全做满的内容：

- 独立 heartbeat API
- 完整 resume 恢复流程
- 更细的前端 transport 状态展示

所以它现在更适合当“第一版可用实现”，后续可以继续把 keepalive 细节补齐。
