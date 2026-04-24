# Pico Xiangqi Arena

一个单进程的中国象棋对战服务，负责：

- 房间管理
- 主持人与观战视图
- 人类选手浏览器走子
- 受管 `picoclaw` 对局
- participant 级 `session + /message` hybrid 走子

当前后端兼容目标是 Go 1.20 这一档，适合继续做 Windows 7 / 2008 R2 方向的实机验证，同时也可以正常编译 Linux 版本。

发布说明：

- 编译后的可执行文件会内嵌 `static/` 前端资源
- 发布包可以脱离项目源码目录单独启动
- 不需要再额外拷贝 `static/index.html`、`static/app.js`、`static/style.css`

## 启动

默认监听 `:8080`：

```bash
GOTOOLCHAIN=local go run .
```

查看启动参数：

```bash
GOTOOLCHAIN=local go run . --help
```

自定义端口：

```bash
GOTOOLCHAIN=local go run . --port 9090
```

监听指定 IP：

```bash
GOTOOLCHAIN=local go run . --host 0.0.0.0 --port 8080
```

监听 IPv6：

```bash
GOTOOLCHAIN=local go run . --host :: --port 8080
```

直接指定完整监听地址：

```bash
GOTOOLCHAIN=local go run . --listen 192.168.1.20:8080
```

直接指定 IPv6 监听地址：

```bash
GOTOOLCHAIN=local go run . --listen [::]:8080
```

后台启动：

```bash
GOTOOLCHAIN=local go run . --host 0.0.0.0 --port 8080 --background
```

后台模式日志默认写入：

```text
runtime/arena.log
```

启动后本机默认访问：

```text
http://127.0.0.1:8080
```

运行时快照默认写入：

```text
runtime/arena-snapshot.json
```

参数优先级：

- 命令行参数优先
- 其次是环境变量 `PORT` / `HOST` / `LISTEN` / `SNAPSHOT_PATH`
- 最后才是内置默认值

IPv6 说明：

- `--host :: --port 8080` 会监听 IPv6 地址 `[::]:8080`
- `--host [::] --port 8080` 也支持，程序会自动归一化
- 如果你想完整控制监听地址，直接用 `--listen [::]:8080`

如果你希望其他机器通过 IP 访问这台服务，建议显式使用：

```bash
GOTOOLCHAIN=local go run . --host 0.0.0.0 --port 8080
```

如果仍然不能访问，通常就不是程序监听地址的问题，而是操作系统防火墙或云主机安全组没有放行对应端口。

二进制启动说明：

- 直接运行编译产物即可，前端页面和静态资源已经打进包内
- 这也是为了避免从 `dist/`、计划任务、服务管理器或其他工作目录启动时出现首页 `404 page not found`

## 基本使用

### 1. 创建或加入房间

浏览器打开首页后，输入比赛码，然后：

- 点击“创建比赛”会新建一个房间，并让你成为主持人
- 点击“加入比赛”只会进入已有房间，不会自动创建新房间
- 如果主持人选择抢比赛席，仍然会按当前入场意图占位
- 后续进入者会根据入场意图尝试占位或成为观众

### 2. 主持人控制

主持人可以在页面中：

- 修改步间隔
- 修改默认视图
- 配置红黑席位
- 查看托管 `picoclaw` runtime 诊断（`preferred_mode`、`active_mode`、`session_state`、`ws_state`、邀请/心跳时间等）
- 按席位切换托管 `picoclaw` 的 `preferred_mode`（`auto` / `prefer_pico_ws` / `prefer_session` / `prefer_message`）
- 开始、暂停、恢复、重开比赛

当前席位类型只支持：

- `human`
- `picoclaw`

其他 AI agent 暂时不会进入实际对局流程，前端会显示“其他 AI agent 等待适配中”。

### 3. 人类选手走子

轮到人类席位时：

- 先点起点
- 再点目标格
- 前端会提交到 `/api/arena/{code}/move`

## Agent 接入说明

当前只支持 `picoclaw`。走子链路采用 hybrid runtime 模型（`pico_ws` + `session` + `message`），邀请链路目前仍保持 `/message` 兼容。

主持人在席位配置里填写的 `Base URL`，就是 picoclaw 服务监听地址，例如：

- `http://127.0.0.1:18790`
- `http://192.168.31.160:18790`

如果你没有手动改过 picoclaw 配置，默认监听端口通常就是 `18790`。这里的 `Base URL` 指的是 PicoClaw 服务根地址，不是 launcher 登录页，也不是手工补好的 `/message` 或 `/pico/ws` 完整路径。

arena 会自动请求：

```text
POST {base_url}/message
```

如果你填的是根地址，程序会自动补成 `/message`；如果已经带了 `/message`，则直接使用。

如果启用了 `pico_ws`，arena 也会从同一个 `Base URL` 自动推导：

```text
WS {base_url}/pico/ws?session_id=xiangqi-{room_code}-{participant_id}
```

当前 `pico_ws` 对接目标是 PicoClaw gateway 原生 `/pico/ws`，不是 launcher 代理过的 `/pico/ws`。

请求体格式：

```json
{
  "session_id": "xiangqi-abcd1234",
  "sender_id": "picoclaw-xiangqi-arena",
  "sender_display_name": "Picoclaw Xiangqi Arena",
  "message": "你正在参加一场中国象棋对局......\nMOVE: h9-g7",
  "api_key": "optional"
}
```

说明：

- `session_id`：同一局棋会复用同一个比赛级会话标识
- `message`：包含当前棋盘、合法走法、己方身份、对手公开身份等上下文
- `api_key`：如果席位里填写了 API Key，会同时写入 JSON 体，并额外带上请求头 `X-PicoClaw-API-Key`

响应体格式：

```json
{
  "reply": "MOVE: a3-a4",
  "error": ""
}
```

约定：

- arena 会从 `reply` 中提取形如 `MOVE: a3-a4` 的走法
- 如果响应不是 JSON，或者 `reply` 里没有合法走法，这一手会记为失败并暂停比赛
- `picoclaw` 席位在开局前必须配置 `Base URL`

Message 模式说明：

- `message` 模式每回合最多重试 `3` 次
- 只对临时传输失败重试，例如超时、连接失败、HTTP `502/503/504`
- 如果返回体不是合法 JSON，或 `reply` 没有合法走法，则不会继续重试

Session 模式说明：

- `/message` 仍然保留，兼容邀请与 fallback 走子
- `session` 是 arena 本地提供的 participant 级长期会话通道，不再复用远端 `/message`
- 当席位 `preferred_mode=prefer_session`，且 session 心跳健康时，arena 会优先走 session turn 通道
- 如果 session 失败，会同回合自动回退到 `/message`
- 如果 `/message` 失败，但 session 健康，也会反向回退到 session

Pico WS 模式说明：

- `prefer_pico_ws` 会优先连接 PicoClaw gateway 的原生 `/pico/ws`
- arena 作为 WebSocket client 主动发 `message.send`
- arena 从 `message.create` / `message.update` 中提取 `MOVE: a0-a1`
- `API Key` 字段会被复用为 `pico_ws` 的 Bearer Token
- 如果 `pico_ws` 当回合失败，arena 会按当前可用状态自动回退到 `session` 或 `message`

arena 本地 session 接口：

```text
POST /api/arena/{code}/picoclaw/{participant_id}/invite
POST /api/arena/{code}/picoclaw/{participant_id}/session/open
POST /api/arena/{code}/picoclaw/{participant_id}/session/heartbeat
POST /api/arena/{code}/picoclaw/{participant_id}/session/close
POST /api/arena/{code}/picoclaw/{participant_id}/turn
```

`session/open` 请求：

```json
{
  "host_token": "host-token"
}
```

`session/open` 响应会返回：

```json
{
  "participant_id": "abcd1234",
  "session_id": "f3b8...",
  "session_token": "9c1e...",
  "session_state": "opening"
}
```

`session/heartbeat` 请求：

```json
{
  "session_id": "f3b8...",
  "session_token": "9c1e...",
  "lease_ttl_ms": 45000
}
```

`turn` 轮询请求：

```json
{
  "session_id": "f3b8...",
  "session_token": "9c1e...",
  "wait_ms": 25000
}
```

如果当前有待处理回合，arena 会返回：

```json
{
  "status": "turn",
  "turn": {
    "turn_id": "match-red-3",
    "match_id": "match",
    "room_code": "demo",
    "seat": "red_player",
    "side": "red",
    "move_count": 3,
    "step_interval_ms": 3000,
    "opponent_alias": "黑雨伞",
    "board_rows": ["..."],
    "board_text": "...",
    "legal_moves": ["a3-a4"],
    "prompt": "你正在参加一场中国象棋对局......",
    "session_id": "f3b8..."
  }
}
```

`turn` 提交走子请求：

```json
{
  "session_id": "f3b8...",
  "session_token": "9c1e...",
  "turn_id": "match-red-3",
  "move": "a3-a4",
  "reply": "MOVE: a3-a4"
}
```

成功时返回：

```json
{
  "status": "accepted"
}
```

邀请流程（当前版本）：

- 主持人触发托管 `picoclaw` 邀请时，arena 同样调用 `POST {base_url}/message`
- arena 会先在本地为该 participant 自动准备 `session_id` 和 `session_token`
- `message` 内容会明确标注“邀请”语义，并附带：
  - `arena_base_url`
  - `participant_id`
  - `preferred_mode`
  - `session_id`
  - `session_token`
  - `heartbeat_url`
  - `turn_url`
- 邀请诊断会记录到席位 runtime（`last_invite_at`、`last_invite_status`）
- 当前邀请消息不会把 `/session/open` 当作 picoclaw 的接入口，因为该接口仍是主持人权限接口

预留接口（仅文档说明，当前不会调用）：

```text
POST {base_url}/invite
```

`/invite` 会在后续版本评估为独立邀请通道；当前发布版本仍只使用 `/message`。

### Picoclaw Runtime 模型（Hybrid）

- 每个托管 `picoclaw` 比赛席位都有独立 runtime 状态（host room `runtime` 字段）。
- `preferred_mode` 由主持人控制：`auto`、`prefer_pico_ws`、`prefer_session`、`prefer_message`。
- `active_mode` 为当前实际走子模式，可能因同回合回退而变化。
- session 相关状态通过 participant 级 API 维护（open / heartbeat / close），并写入 `session_state`、`session_id`、`lease_expires_at`。
- `pico_ws` 相关状态会写入 `ws_state`、`ws_session_id`、`ws_connected_at`、`ws_last_recv_at`。
- host 也可以通过 `POST /api/arena/{code}/picoclaw/{participant_id}/invite` 主动触发一次 picoclaw 邀请；该接口会基于当前 HTTP 请求自动推断 `arena_base_url`。
- 对局走子支持同回合同 participant 多通道回退：主模式失败时自动尝试其他可用路径，并记录模式切换原因与失败计数。
- 邀请链路当前保持 `/message` 兼容，`/invite` 仍是预留能力。

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

### 编译 Windows amd64 版本

```bash
mkdir -p dist
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -o dist/pico-xiangqi-arena-windows-amd64.exe .
```

## 测试

运行全部测试：

```bash
GOTOOLCHAIN=local go test ./...
```

## 当前实现边界

这版已经支持：

- 房间与主持人流程
- 浏览器端 human 选手走子
- `picoclaw /pico/ws` 托管走子
- `picoclaw /message` 托管对局
- `picoclaw /message` 托管邀请
- 托管 `picoclaw` runtime 诊断（host room `runtime`）
- participant 级 `preferred_mode` 控制与 `pico_ws` / session / message 自动回退
- participant 级 session open/heartbeat/close
- 静态前端打包进二进制

这版暂未支持：

- 其他 AI agent 协议适配
- `picoclaw /invite` 独立通道（当前仅保留兼容预留）
