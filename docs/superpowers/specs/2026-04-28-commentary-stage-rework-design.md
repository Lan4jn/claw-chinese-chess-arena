# Commentary Stage Rework Design

## Goal

把当前“日志人类化显示”升级为真正面向观众的中国象棋解说台。

本设计替代 [2026-04-24-commentary-humanized-display-design.md](/Users/mewcapoo/Project/claw-chinese-chess-arena/docs/superpowers/specs/2026-04-24-commentary-humanized-display-design.md) 中较窄的“仅做人类化文案映射”方案。

新解说台必须满足：

- 每条解说先给出走子播报，再给出局势分析
- 走子播报允许切换为 `棋谱式`、`口语式`、`混合式`
- 局势分析允许切换为 `自动`、`保守`、`中度`、`强分析`
- `自动` 模式下，系统可根据局面事件自动切换分析强度
- 未勾选“显示原始回复”时，观众看到的是正常解说语言，而不是调试日志

## Problem

当前解说台虽然已经把部分技术文本改写成较自然的中文，但本质仍然是“日志换皮”：

- 走子信息仍然接近坐标或底层响应，不适合观众直接理解
- 缺少局势分析，无法形成“真人在解说棋局”的观感
- 所有回合几乎使用同一类表达，信息层次不明显
- 开局、中盘、关键手、失误手、胜负手没有语气区分

这使得解说台更像调试窗口，而不是赛事观赛界面。

## Scope

In scope:

- 前端解说生成逻辑重构
- 走子播报三种样式切换
- 解说强度四种模式切换
- 自动强度切换规则
- 基于当前局面和最近一步的局势分析生成
- 低置信度时的保守回退文案

Out of scope:

- 后端新增 AI 解说服务
- 引入外部大模型实时生成文案
- 完整职业裁判级深度棋理分析
- 多回合长链路战略复盘
- 比赛规则改动

## Product Behavior

### 1. 解说输出结构

每条解说固定分成两段，顺序不变：

1. 走子播报
2. 局势分析

示意：

- 走子播报：`黑方走出车8平9，把右路的车横到边线。`
- 局势分析：`这步先抢边路节奏，继续给红方后排施压。`

如果系统对局势判断不足，允许只输出较短分析，但仍然保留两段结构。

### 2. 走子播报样式

新增 `move commentary style`，提供三个选项：

- `notation`
  - 例：`黑方 车8平9。`
- `plain`
  - 例：`黑方把右路的车横到边线。`
- `hybrid`
  - 例：`黑方走出车8平9，把右路的车横到边线。`

默认值为 `hybrid`。

### 3. 解说强度

新增 `analysis intensity`，提供四个选项：

- `auto`
- `conservative`
- `balanced`
- `aggressive`

默认值为 `auto`。

各档位定义：

- `conservative`
  - 只解释这步棋在做什么，不轻易下结论
- `balanced`
  - 在目的说明之外，再补一句局势倾向
- `aggressive`
  - 强调计划、收益、风险、先手转换或胜负压力
- `auto`
  - 由系统根据当前回合事件自动选择以上三种之一

## Overall Architecture

继续采用纯前端生成，不改后端 API。

前端解说逻辑拆成四层：

1. `move event extraction`
   - 从 `publicMatch.board_rows`、上一帧棋盘、`last_move` 中提取结构化走子事件
2. `notation + plain rendering`
   - 把结构化走子事件分别渲染成棋谱式与口语式
3. `position analysis`
   - 基于当前局面和最近一步生成局势标签与分析结论
4. `commentary composition`
   - 根据用户所选播报样式和解说强度拼装最终解说文本

现有 `static/commentary.mjs` 将从“简单文案映射器”升级为“解说台生成器”。

## Move Event Extraction

系统需要把原始坐标走法转成结构化走子事件，至少包含：

- `side`
- `piece`
- `from`
- `to`
- `action`
  - `advance`
  - `retreat`
  - `horizontal`
- `capture`
- `check`
- `resolvedCheck`
- `mateThreat`
- `disambiguation`
  - 例如 `front` / `middle` / `rear`

这里的事件提取只服务于解说，不改比赛合法性判断。

## Chinese Notation Rendering

### 1. 棋谱式输出

棋谱式必须优先符合中文象棋观赛习惯，而不是继续显示 `a0-b0` 这种底层坐标。

规则：

- 红方使用中文数字：`一二三四五六七八九`
- 黑方使用阿拉伯数字：`1 2 3 4 5 6 7 8 9`
- 使用常见棋子名称：
  - 红：`车马相仕帅炮兵`
  - 黑：`车马象士将炮卒`
- 使用标准动作词：
  - `平`
  - `进`
  - `退`
- 同类棋子冲突时优先生成：
  - `前车`
  - `后车`
  - `前炮`
  - `后炮`
  - 必要时支持 `中兵` 一类表达

第一版目标不是覆盖所有职业记谱边角案例，而是正确覆盖当前对局中绝大多数正常着法。

### 2. 口语式输出

口语式面向不懂棋谱的观众，应优先表达“这步在干什么”。

示例风格：

- `黑方把右路的车横到边线。`
- `红方先把马跳出来，往中路靠。`
- `红方中炮继续往前顶，想把压力给到黑方正面。`

规则：

- 尽量使用“边路 / 中路 / 后排 / 前线 / 中腹 / 右翼 / 左翼”这类空间词
- 尽量说出棋子的功能意图，而不是只描述坐标变化
- 不要求每步都生成复杂修辞，优先自然、短句、清楚

### 3. 混合式输出

混合式先给棋谱，再补一句白话定位。

示例：

- `黑方走出车8平9，把右路的车横到边线。`
- `红方马二进三，先把马跳出来保护中路。`

混合式是默认样式，因为它兼顾懂棋观众和普通观众。

## Position Analysis

第一版局势分析只做“单步之后的局势解释”，不做多回合深度推演。

### 1. 局势标签

前端在当前步后提取以下标签：

- `isCapture`
- `isCheck`
- `resolvedCheck`
- `isExchange`
- `developsMajorPiece`
- `controlsCenter`
- `pressesFlank`
- `opensLine`
- `materialSwing`
- `enteringEndgame`
- `repetitionRisk`
- `tempoGain`
- `positionStable`

这些标签由当前局面、上一局面、最近一步和现有日志共同推断。

### 2. 分析模板策略

分析文案不直接固定成一个句子，而是采用“标签 -> 话术池”的方式。

示意：

- `isCapture`
  - `这步先把子力换掉，场上结构开始出现变化。`
  - `这步直接吃进去了，黑方在子力交换上先拿到实惠。`
- `isCheck`
  - `这一将很直接，红方必须先处理眼前压力。`
  - `先手已经抢到了，黑方这步把将军节奏打出来了。`
- `pressesFlank + tempoGain`
  - `这步主要是在边路抢节奏，逼对手先应。`
  - `边路压力继续叠上来，红方短时间内不太好轻松腾挪。`

同一标签至少准备 3 组表达，轮换输出，避免明显模板感。

### 3. 强度定义

`conservative`

- 只说目的
- 不轻易判断优劣
- 不轻易说“明显优势”

`balanced`

- 说目的
- 再补一句主动权、节奏、结构或压力判断

`aggressive`

- 说目的
- 说收益
- 说潜在风险或后续手段

示例：

- `conservative`
  - `这步先把车横出来，主要是把边路通开。`
- `balanced`
  - `这步先把车横出来，继续给红方边路施压，目前黑方的出子更主动一些。`
- `aggressive`
  - `黑方这步横车就是在抢先手，边路压力已经形成，但如果后续跟不上，自己后排也会露出空当。`

## Auto Switching Rules

当 `analysis intensity = auto` 时，按以下优先级决定档位。

切到 `aggressive`：

- 吃子
- 将军
- 解将
- 失子明显
- 触发重复走子风险
- 进入残局
- 这一步后胜负倾向明显变化

切到 `balanced`：

- 普通中盘推进
- 兑子
- 抢先手
- 压边或控中
- 形成可见威胁但还不是决定性一手

切到 `conservative`：

- 常规开局出子
- 局面变化很小
- 标签不足以支持更强判断

系统宁可保守，也不输出高置信度假分析。

## Confidence And Fallback

为了避免“像真人一样胡说”，系统需要引入置信度回退。

规则：

- 如果棋谱转换失败，则回退到更朴素的口语播报
- 如果口语定位也不充分，则至少保证能输出正确的棋谱式或坐标后备文案
- 如果局势标签不足，则分析段只输出低风险说明
- 如果当前日志属于错误、超时、非法走法、重复规则拒绝等非正常落子事件，则优先输出事件说明，不强行做棋理分析

例如：

- `黑方尝试走出车8平9，但这步棋因长捉重复被裁判系统驳回。`
- `红方已经给出回应，不过这步着法当前不能成立。`

## User Controls

前端新增并持久化两个设置：

- `commentaryMoveStyle`
  - `notation`
  - `plain`
  - `hybrid`
- `commentaryAnalysisIntensity`
  - `auto`
  - `conservative`
  - `balanced`
  - `aggressive`

设置保存在现有本地存储体系中，和当前 `showRawLog` 一样按浏览器持久化。

默认值：

- `commentaryMoveStyle = hybrid`
- `commentaryAnalysisIntensity = auto`

## File Changes

- Modify: `static/commentary.mjs`
  - 升级为完整解说生成模块
- Modify: `static/commentary.test.mjs`
  - 改写测试，覆盖播报样式与分析强度
- Modify: `static/app.js`
  - 读取新设置并接入渲染
- Modify: `static/index.html`
  - 增加两个用户切换控件
- Modify: `static/style.css`
  - 为解说控制区补样式

如果当前模块过大，允许将其进一步拆分为：

- `static/commentary-move.mjs`
- `static/commentary-analysis.mjs`
- `static/commentary-format.mjs`

但这属于实现阶段决策，不属于本 spec 的强制要求。

## Testing

必须覆盖以下验证点：

1. 同一手走法可分别输出棋谱式、口语式、混合式
2. 红黑双方数字、棋子称谓和方向表达正确
3. 同类重子场景能生成可区分播报
4. `auto` 模式能根据关键事件切换强度
5. 吃子、将军、重复规则拒绝等关键事件能输出合理分析
6. 低置信度场景不会输出过度武断结论
7. 勾选原始回复后，仍可回看底层日志文本

## Non-Goals

这版不追求：

- 达到职业象棋直播解说员水准
- 每一步都给出高质量深度战略判断
- 完整替代原始日志调试价值

第一版的成功标准是：

- 观众看到的是“能看懂、像解说、不过分胡说”的内容
- 坐标走法不再直接暴露为主要播报文本
- 同一解说台能同时服务懂棋观众和普通观众
