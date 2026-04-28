# Commentary Humanized Display Design

## Goal

在解说台未勾选“显示原始回复”时，把当前偏技术化的日志、回复和错误展示为人类可读的赛事播报文案；勾选后继续显示原始文本。

## Scope

In scope:

- 前端解说台展示层的人类化文案转换
- 原始回复开关与展示切换
- 常见 `reply` / `error` 的赛事播报型改写

Out of scope:

- 后端日志结构调整
- 比赛规则变更
- 新增后端 API 字段

## Design

当前解说台直接渲染 `log.message`、`log.reply`、`log.error`。这会导致：

- 技术错误直接暴露给观众
- `MOVE: a3-a4` 这类原始回复缺少解说语气
- 未勾选原始回复时，界面仍像调试面板

改造方式限定在前端：

1. 新增一个独立的前端文案格式化模块，负责：
   - 根据 `log` 生成赛事播报型 `message`
   - 在未显示原始回复时，将 `reply` 和 `error` 改写为播报文案
   - 在显示原始回复时，保留原始 `reply` / `error`
2. `static/app.js` 中的 `renderEvents()` 改为通过该模块生成展示模型
3. `static/index.html` 继续复用已有“显示原始回复”复选框

## Commentary Rules

优先覆盖以下高频场景：

- `MOVE: a3-a4`
  - 改写为：`选手示意将走 a3-a4。`
- `picoclaw reply did not contain a legal move`
  - 改写为：`选手已经作答，但给出的着法不在当前允许范围内，本回合未被接受。`
- `move causes forbidden long-check repetition`
  - 改写为：`这步棋会形成长将重复，裁判系统已驳回。`
- `move causes forbidden long-chase repetition`
  - 改写为：`这步棋会形成长捉重复，裁判系统已驳回。`
- `move causes forbidden idle repetition`
  - 改写为：`这步棋会形成闲着循环，裁判系统已驳回。`
- 其他请求失败
  - 改写为：`选手响应暂时异常，本回合请求未成功完成。`

## File Changes

- Create: `static/commentary.mjs`
  - 纯文案和展示模型格式化逻辑
- Create: `static/commentary.test.mjs`
  - Node 原生测试，覆盖关键文案映射
- Modify: `static/app.js`
  - 接入格式化模块与复选框状态
- Modify: `static/index.html`
  - 改为模块脚本加载

## Testing

使用 Node 原生测试运行前端纯函数：

- 原始回复关闭时，`MOVE:` 会被改写成播报文案
- 原始回复关闭时，重复规则错误会被改写成可读播报
- 原始回复开启时，保留原始 `reply` / `error`
