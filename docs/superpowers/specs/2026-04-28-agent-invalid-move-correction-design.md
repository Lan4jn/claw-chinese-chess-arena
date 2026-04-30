# Managed Agent Invalid Move Correction Design

## Goal

当托管 `picoclaw` 选手在 `message`、`session`、`pico_ws` 任一通道中提交了违反规则的走法时，比赛不应直接暂停，而应在同一回合内明确通知该 agent 这步棋被驳回，并要求其重新给出一步新棋。

第一版要求：

- 三种托管通道统一支持
- 同一回合最多允许 3 次违规重走
- 仅对“可纠正违规”启用重走
- 网络、超时、协议错误仍走现有失败链路
- 主持端日志和观众解说都能看懂这个过程

## Problem

当前实现里，托管 agent 的走子流程只区分：

- 请求成功并给出可落子的走法
- 请求失败

这会带来一个问题：

- 如果 agent 回复了一步“格式正确，但违反规则”的棋，例如长将、长捉、闲着循环或其他非法着法，系统会直接把它当成失败并暂停比赛
- agent 没有机会在同一回合改走
- 主持人和观众看到的是“请求失败”，而不是“裁判驳回后要求重走”

这不符合正常赛事行为，也不利于后续和 `picoclaw` 形成稳定交互。

## Scope

In scope:

- 托管 agent 同回合违规步重走机制
- 三种通道的统一违规反馈语义
- 同回合 3 次违规上限
- 主持端技术日志补充
- 观众端可读解说补充

Out of scope:

- 网络重试策略重构
- 比赛判负规则升级
- 长期累计警告计数
- 非托管 human 走子行为改变

## Correction Policy

### 1. 允许进入重走机制的错误

仅以下“可纠正违规”进入重走机制：

- 非法着法
- 长将
- 长捉
- 闲着循环
- 其他引擎明确拒绝、但语义上属于“换一步即可继续”的错误

### 2. 不进入重走机制的错误

以下错误仍走现有失败或回退链路：

- 网络失败
- 请求超时
- 连接中断
- 协议错误
- 返回内容无法解析
- 非法空回复
- 服务端内部错误

## Product Behavior

## Same-Turn Retry Loop

同一回合内统一采用如下流程：

1. 系统向 agent 请求一步棋
2. agent 返回走法
3. 系统校验这步棋
4. 如果合法，则正常落子并结束回合
5. 如果属于可纠正违规：
   - 记录“被驳回”日志
   - 通知 agent 这步不允许，并附原因
   - 要求其重新选择一步合法走法
   - 回到步骤 1
6. 如果同一回合累计 3 次违规仍未给出有效走法：
   - 记录本回合失败
   - 走现有暂停或失败链路

## Attempt Limit

第一版统一上限：

- 每回合最多 `3` 次违规重走机会

语义：

- 第 1 次违规：继续重走
- 第 2 次违规：继续重走
- 第 3 次违规：本回合失败，不再继续重走

这里的计数只统计“可纠正违规”，不把网络错误和协议错误算进这 3 次里。

## Transport Semantics

三种通道的底层传输格式可以不同，但必须满足同一语义：

- 明确指出“上一手被驳回”
- 明确给出被驳回的走法
- 明确给出驳回原因
- 明确要求重新走一步
- 给出当前仍可选择的合法走法
- 给出当前是第几次违规重走机会

## Message Mode

`message` 模式中：

- 第一次正常通过 `POST /message` 请求走子
- 如果这步棋被驳回，arena 再发一次新的 `POST /message`
- 第二次请求的内容是 correction prompt，不是普通的初始求步 prompt

该 correction prompt 需要包含：

- 上一步走法
- 驳回原因
- 当前局面
- 当前合法走法
- “请重新选择一步合法着法”的明确要求

## Session Mode

`session` 模式中：

- 同一 `session_id` 内允许多次 turn 请求
- 每次违规后，arena 在同一回合继续生成新的 `turn`
- 新 `turn` 使用新的 `turn_id`
- 新 `turn.prompt` 变为 correction prompt

这样对接方不需要重新开 session，只需要把它视为“裁判反馈后的再次求步”。

## Pico WS Mode

`pico_ws` 模式中：

- 保持现有 ws 连接
- 违规后在同一连接中再次发送 correction prompt
- 不断开、不重建连接
- 回合内允许多次消息往返，直到得到合法走法或超过 3 次上限

## Logging

## Host Logs

主持端需要新增两类技术事件：

1. `选手走子被驳回`
   - 包含原始回复
   - 包含被驳回走法
   - 包含驳回原因
   - 包含当前第几次重走

2. `系统要求选手重新走子`
   - 包含当前使用的通道
   - 可选包含剩余可用次数

示意：

- `黑方走子被驳回：e2-e1，原因：move causes forbidden long-check repetition（第 1/3 次）`
- `系统已要求黑方重新走子（message 模式）`

## Public Commentary

观众端不应直接暴露技术细节，但需要知道比赛仍在继续。

推荐文案方向：

- `黑方刚才这步棋违反重复规则，裁判没有允许落子。`
- `系统已经要求黑方重新选择下一手。`
- `黑方再次给出违规着法，裁判继续要求其改走。`
- `黑方连续三次提交违规着法，这一回合未能形成有效落子。`

这部分由前端解说系统统一渲染，不要求后端直接输出最终中文。

## Error Classification

需要引入明确分类函数，把 agent 走子错误分成两类：

1. `correctable_invalid_move`
   - 可继续重走
2. `terminal_request_failure`
   - 不重走，直接失败

分类必须同时适用于：

- `message` 模式下由 HTTP 响应解析出来的错误
- `session` 模式下由 turn 结果解析出来的错误
- `pico_ws` 模式下由 ws 消息解析出来的错误
- `ApplyAgentMove` / `GameState.Apply` 返回的规则错误

## Architecture

重走机制应放在 arena 层统一控制，而不是分别散落到三个通道中。

建议职责拆分：

- 通道层
  - 发一次请求
  - 收一次回复
  - 返回原始 reply / move / transport error
- arena 层
  - 校验走子
  - 判断是否属于可纠正违规
  - 发送 correction
  - 控制同回合最多 3 次重走
  - 统一记录日志和回合结果

这样三种通道只保留“传递消息”的职责，规则语义集中在 arena 层。

## File Changes

- Modify: `arena.go`
  - 引入统一的违规步重走循环
- Modify: `match.go`
  - 增加更细的日志记录接口
- Modify: `pico.go`
  - 支持 correction prompt 的 message 请求
- Modify: `pico_ws.go`
  - 支持 ws correction prompt 请求
- Modify: `picoclaw_runtime.go`
  - 如有必要，可补充本回合诊断字段
- Modify: `arena_test.go`
  - 补充 message / session / pico_ws 三种模式的重走测试
- Modify: `static/commentary.mjs`
  - 补充被驳回、要求重走等解说语义

## Testing

必须覆盖：

1. `message` 模式下第一次违规、第二次合法，可继续比赛
2. `session` 模式下第一次违规、第二次合法，可继续比赛
3. `pico_ws` 模式下第一次违规、第二次合法，可继续比赛
4. 同一回合连续 3 次违规后，回合失败并进入现有暂停链路
5. 网络错误不会误走到“违规步重走”分支
6. 日志能准确记录违规原因、次数和重走动作
7. 观众端解说可区分“被驳回”和“请求失败”

## Non-Goals

这版不做：

- 跨回合累计警告次数
- 自动判负
- 对 human 选手自动弹出重走引导
- 复杂仲裁优先级系统
