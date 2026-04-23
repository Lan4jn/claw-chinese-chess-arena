# Pico Xiangqi Arena

一个单进程的中国象棋对战服务，负责：

- 房间管理
- 主持人与观战视图
- 人类选手浏览器走子
- 受管 agent 对局
- 服务级 agent transport 模式切换

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
- 查看托管 `picoclaw` runtime 诊断（`preferred_mode`、`active_mode`、`session_state`、邀请/心跳时间等）
- 按席位切换托管 `picoclaw` 的 `preferred_mode`（`auto` / `prefer_session` / `prefer_message`）
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

当前只支持 `picoclaw`。走子链路采用 hybrid runtime 模型（session + message），邀请链路目前仍保持 `/message` 兼容。

主持人在席位配置里填写的 `Base URL`，就是 picoclaw 服务监听地址，例如：

- `http://192.168.31.160:18800`
- `http://192.168.31.130:18888`

arena 会自动请求：

```text
POST {base_url}/message
```

如果你填的是根地址，程序会自动补成 `/message`；如果已经带了 `/message`，则直接使用。

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

邀请流程（当前版本）：

- 主持人触发托管 `picoclaw` 邀请时，arena 同样调用 `POST {base_url}/message`
- `message` 内容会明确标注“邀请”语义，并要求对方确认收到邀请
- 邀请诊断会记录到席位 runtime（`last_invite_at`、`last_invite_status`）

预留接口（仅文档说明，当前不会调用）：

```text
POST {base_url}/invite
```

`/invite` 会在后续版本评估为独立邀请通道；当前发布版本仍只使用 `/message`。

### Picoclaw Runtime 模型（Hybrid）

- 每个托管 `picoclaw` 比赛席位都有独立 runtime 状态（host room `runtime` 字段）。
- `preferred_mode` 由主持人控制：`auto`、`prefer_session`、`prefer_message`。
- `active_mode` 为当前实际走子模式，可能因同回合回退而变化。
- session 相关状态通过 participant 级 API 维护（open / heartbeat / close），并写入 `session_state`、`session_id`、`lease_expires_at`。
- 对局走子支持同回合双通道回退：主模式失败时自动尝试另一路径，并记录模式切换原因与失败计数。
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
- `picoclaw /message` 托管对局
- `picoclaw /message` 托管邀请
- 托管 `picoclaw` runtime 诊断（host room `runtime`）
- participant 级 `preferred_mode` 控制与 session open/heartbeat/close
- 静态前端打包进二进制

这版暂未支持：

- 其他 AI agent 协议适配
- `picoclaw /invite` 独立通道（当前仅保留兼容预留）
