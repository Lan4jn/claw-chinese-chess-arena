# 需求文档

## 简介

Pico Xiangqi Arena 是一个面向“赛博斗蛐蛐”场景的象棋比赛观战系统。系统允许任意进入者通过比赛码加入同一个比赛场地，在无登录体系的前提下形成主持席、两个比赛席和无限观战席。系统核心目标不是传统象棋工具，而是让人类观众能够看懂、围观并管理一场 1v1 的人类或 AI Agent 对抗赛，同时在比赛过程中默认隐藏选手真实身份，只公开伪装昵称和比赛行为。

## 术语表

- **System**: Pico Xiangqi Arena 系统
- **User**: 进入比赛场地的人类观众、主持人或接入系统的 AI Agent
- **Room**: 通过比赛码标识的单个比赛场地
- **Host Seat**: 仅房主可进入的主持席，拥有比赛配置与席位管理权限
- **Player Seat**: 红方或黑方比赛席，仅允许两名参赛方占用
- **Spectator Seat**: 观战席，无数量限制
- **Match Code**: 用户输入或随机生成的比赛码，用于进入特定 Room
- **Client Token**: 系统在本地为每个进入者持久化的随机身份标识
- **Public Alias**: 面向他人展示的伪装昵称，要求偏现实物品风格
- **Real Type**: 选手真实身份类型，如 human、pico、claw 或其他 agent
- **Reveal State**: 身份揭晓状态，支持全部隐藏、单边揭晓或全部揭晓
- **Step Interval**: 比赛自动推进时的步间隔，单位为毫秒
- **Arena View**: 观众端的比赛展示视图，包含棋局中心型和解说型
- **Managed Agent**: 由主持人预配置并托管到比赛席的 AI 选手
- **Phase**: 当前比赛推进阶段，如 waiting_match、waiting_human、waiting_agent、paused、finished

## 需求

### 需求 1: 比赛码房间接入

**用户故事：** 作为用户，我想通过比赛码进入一个比赛场地，以便快速发起或加入一场比赛而不需要注册登录。

#### 验收标准

1. WHEN User 提交一个不存在的 Match Code THEN THE System SHALL 创建一个新的 Room 并将该进入者视为房主。
2. WHEN User 提交一个已存在的 Match Code THEN THE System SHALL 让该进入者加入已存在的 Room。
3. THE System SHALL 为每个进入者使用 Client Token 识别其本地身份，而不是要求注册登录。
4. WHEN User 重复使用同一个 Client Token 进入同一个 Room THEN THE System SHALL 识别为同一参与者而不是重复创建身份。
5. THE System SHALL 支持 User 在进入时指定展示名和入场意图。
6. THE System SHALL 允许 User 编辑 Match Code，也允许系统生成随机 Match Code。

### 需求 2: 房间席位模型

**用户故事：** 作为主持人或观众，我想明确区分主持席、比赛席和观战席，以便理解场地角色和权限。

#### 验收标准

1. THE System SHALL 在每个 Room 中提供 1 个 Host Seat、2 个 Player Seat 和无限个 Spectator Seat。
2. WHEN 首个进入者创建 Room THEN THE System SHALL 将其标记为 Host Seat 持有者。
3. WHEN User 以自动分配或抢比赛席方式进入且比赛席未满 THEN THE System SHALL 按先到先得分配红方、黑方比赛席。
4. WHEN 两个 Player Seat 已满 THEN THE System SHALL 将后续进入者置于 Spectator Seat。
5. THE System SHALL 在公开房间视图中展示每个座位的当前占用情况。
6. THE System SHALL 允许主持人手动调整 Player Seat 的归属。

### 需求 3: 身份隐藏与揭晓

**用户故事：** 作为观众或参赛者，我想在比赛中默认不知道对方真实身份，以便形成“隐藏身份对抗”的观赛体验。

#### 验收标准

1. THE System SHALL 为每个参与者分配或保留一个 Public Alias。
2. THE System SHALL 默认在公开视图中隐藏 Player Seat 对应参与者的 Real Type。
3. WHEN Host 选择揭晓红方、黑方或全部身份 THEN THE System SHALL 在公开视图中仅暴露被授权揭晓的身份信息。
4. WHEN Host 恢复隐藏状态 THEN THE System SHALL 再次在公开视图中隐藏真实身份。
5. THE System SHALL 在提示词和前端展示中明确“对手真实身份未知”，除非当前揭晓状态允许公开。

### 需求 4: 比赛配置与主持控制

**用户故事：** 作为主持人，我想在比赛开始前配置规则、席位和选手绑定，以便组织一场可控的正式对抗。

#### 验收标准

1. THE System SHALL 允许 Host 设置 Step Interval。
2. THE System SHALL 允许 Host 设置默认观战视图。
3. THE System SHALL 允许 Host 将红方或黑方席位绑定为 human、pico、claw 或 custom_agent。
4. THE System SHALL 允许 Host 为托管选手设置展示名、Public Alias、Base URL 和 API Key。
5. THE System SHALL 允许 Host 清空指定比赛席。
6. THE System SHALL 仅允许 Host 调用房间设置、席位分配、身份揭晓和比赛控制接口。

### 需求 5: 单局 1v1 比赛生命周期

**用户故事：** 作为观众，我想看到一场明确的 1v1 单局比赛从开始、暂停、恢复到结束的全过程，以便理解比赛当前进度。

#### 验收标准

1. THE System SHALL 仅支持单个 Room 内同时存在一场 1v1 比赛。
2. WHEN 红方和黑方席位均被占用 THEN THE System SHALL 允许 Host 开始比赛。
3. WHEN Match 开始 THEN THE System SHALL 初始化棋盘、走子方、步数、日志和调度状态。
4. THE System SHALL 支持 Host 暂停比赛、恢复比赛和重开一局。
5. WHEN 比赛结束 THEN THE System SHALL 在公开视图中展示胜方、结束原因和最终状态。
6. WHEN Host 重开一局 THEN THE System SHALL 重置比赛状态但保留 Room 和席位结构。

### 需求 6: 人类与 Agent 的回合推进

**用户故事：** 作为比赛组织者，我想让系统自动驱动 Agent 回合，但保留人类手动落子的控制权，以便兼顾真人和 AI 参赛。

#### 验收标准

1. WHEN 当前轮到 human 选手 THEN THE System SHALL 等待该 human 自主提交走子，而不是由 Host 代为单步推进。
2. WHEN 当前轮到 AI Agent 选手 THEN THE System SHALL 根据 Step Interval 自动触发请求并推进比赛。
3. THE System SHALL 将当前 Step Interval 和 Room 上下文传递给 Agent 提示词。
4. WHEN Agent 返回非法走法或调用失败 THEN THE System SHALL 记录错误日志并暂停比赛。
5. WHEN 非当前回合持有者提交走子 THEN THE System SHALL 拒绝该请求。
6. WHEN 当前回合不是 human THEN THE System SHALL 拒绝手动走子接口。

### 需求 7: 观赛优先的公开展示

**用户故事：** 作为观众，我想优先看到清晰易懂的棋局和解说信息，以便把这场比赛当成一个“可看”的 demo。

#### 验收标准

1. THE System SHALL 提供棋局中心型默认视图。
2. THE System SHALL 提供可切换的解说型视图。
3. THE System SHALL 为未来的游戏型视图预留入口，但当前阶段可以不实现完整功能。
4. THE System SHALL 在公开比赛视图中返回棋盘文本、棋盘二维行数据、走子日志、当前轮次、步数和比赛阶段。
5. THE System SHALL 让观众能够看到当前比赛码、房间状态、身份揭晓状态和 Step Interval。
6. THE System SHALL 让人类选手在可操作时通过直观方式完成落子。

### 需求 8: Agent 接入与托管

**用户故事：** 作为系统集成人员，我想让 pico 或其他 agent 能通过接口加入和参与比赛，以便后续扩展更多对战形态。

#### 验收标准

1. THE System SHALL 提供 Agent 注册接口，使外部 Agent 可以带着 Client Token 和绑定信息进入 Room。
2. THE System SHALL 提供 Agent 提交走子接口，供外部 Agent 在轮到自己时上报走法。
3. THE System SHALL 支持通过 Base URL 和 API Key 调用兼容的远程 Agent。
4. THE System SHALL 在 Match 中记录 Agent 的原始回复，并区分公开日志与主持人可见日志。
5. THE System SHALL 允许当前阶段只接入两个 pico 进行 1v1 对战。

### 需求 9: 状态持久化与恢复

**用户故事：** 作为部署者，我想在不引入数据库的前提下保留房间和比赛快照，以便服务重启后还能恢复现场。

#### 验收标准

1. THE System SHALL 使用内存作为运行态主存储。
2. THE System SHALL 支持将 Arena Snapshot 写入本地文件。
3. WHEN 服务启动且快照文件存在 THEN THE System SHALL 尝试恢复已保存的 Room 状态。
4. THE System SHALL 以原子方式写入快照文件，避免部分写入造成文件损坏。
5. THE System SHALL 不要求引入数据库作为当前 demo 的必要前置。

### 需求 10: 可交接的工程化基础

**用户故事：** 作为后续开发者，我想获得清晰的接口、状态模型、错误边界和测试入口，以便在现有基础上继续开发完整 demo。

#### 验收标准

1. THE System SHALL 提供清晰的 HTTP API 路由用于房间进入、比赛查询、主持控制、Agent 接入和静态资源访问。
2. THE System SHALL 使用结构化 JSON 请求与响应作为前后端交互格式。
3. THE System SHALL 为核心行为提供自动化测试，包括房间创建、席位分配、身份揭晓、比赛启动、人类走子约束和 Agent 自动推进。
4. THE System SHALL 提供最小可运行前端骨架，用于后续补全正式 demo。
5. THE System SHALL 让后续开发者能够基于文档继续补全前端交互、后端保护逻辑和集成测试。
