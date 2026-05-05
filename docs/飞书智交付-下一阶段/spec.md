# 飞书智交付 · 下一阶段项目范围（spec）

最近核验：2026-04-26（Asia/Shanghai）

当前基线：基于工作区现有未提交代码与文档核验；`cd apps/api-go && go test ./...` 已通过。

## 1. 文档定位

本文档是飞书智交付项目下一阶段开发的全局范围说明。它用于回答：

- 当前项目已经完成了什么；
- 赛题和产品设计还要求什么；
- 下一阶段为什么要做这些改动；
- 哪些能力在本阶段范围内；
- 哪些能力明确不在本阶段范围内；
- 本阶段开发时应坚持哪些技术决策和权衡。

后续 `tasks.md` 和 `checklist.md` 都以本文档为北极星。若实现过程中范围发生变化，应先更新本文档，再同步更新任务拆解与验证清单。

---

## 2. 赛题要求对齐

《飞书智交付赛题》要求构建一套 AI 驱动的研发流程引擎，核心是：

> 用户输入需求描述，平台将其拆解为多阶段 Pipeline，每个阶段由 AI Agent 执行，阶段间产物流转，人类在关键检查点 Approve/Reject，最终产出可验证、可交付的代码变更。

赛题功能一的 must-have 能力包括：

1. Pipeline 引擎
   - 阶段定义、排序与依赖管理
   - 每个阶段绑定一个或多个 AI Agent
   - 阶段间数据流转
   - 启动、暂停、恢复、终止等生命周期管理

2. Agent 编排与执行
   - 每个阶段有明确 Agent 角色定义和输入输出契约
   - Agent 能感知代码库上下文
   - LLM Provider 可配置，至少支持两个模型提供方，运行时可切换

3. Human-in-the-Loop 检查点
   - 至少两个检查点
   - 支持 Approve / Reject
   - Reject 后能回退上一阶段并携带原因重做
   - API 或 UI 能展示当前阶段产物供人决策

4. API-First 架构
   - Pipeline CRUD、执行触发、状态查询、检查点操作全部 API 化
   - API 设计规范、文档完整

5. 可运行端到端演示
   - 从需求输入到最终可交付代码变更
   - 目标代码库可以是平台自身

功能二要求页面注入、圈选元素、对话修改、热更新预览和 MR 创建。本阶段仍以功能一可演示闭环为主，功能二只保留模型和交付扩展点，不展开完整实现。

---

## 3. 当前开发状态

### 3.1 已完成能力

截至 2026-04-26，本次核验确认：当前后端已经完成了较完整的 Pipeline 基座，并完成了第二阶段第一批稳定性与架构收口工作。

已完成：

1. Pipeline 核心领域模型
   - `PipelineTemplate`
   - `PipelineRun`
   - `StageRun`
   - `Artifact`
   - `Checkpoint`
   - `AgentRun`
   - `GitDelivery`
   - `InPageEditSession`

2. 默认研发流程模板
   - 需求分析：`requirement_analysis`
   - 方案设计：`solution_design`
   - 方案审批：`checkpoint_design`
   - 代码生成：`code_generation`
   - 测试生成：`test_generation`
   - 代码评审：`code_review`
   - 评审确认：`checkpoint_review`
   - 交付集成：`delivery`

3. Pipeline 基础 API
   - 模板列表
   - PipelineRun 列表
   - 创建 PipelineRun
   - 从 Session 创建 PipelineRun
   - PipelineRun 详情
   - 阶段列表
   - 产物列表
   - 检查点列表
   - AgentRun 列表
   - start / pause / resume / terminate
   - checkpoint approve / reject

4. Pipeline 状态机与事务一致性
   - PipelineRun 聚合创建已经改为事务创建
   - start / pause / resume / terminate 增加状态合法性校验
   - checkpoint approve / reject 增加上下文校验
   - 重复审批已被拒绝
   - reject 后可回退到上一可执行阶段并携带 reject reason

5. Stage Handler 拆分
   - 原先过重的 executor 已拆为多个 stage handler
   - 各阶段职责更清晰，便于接入真实 Agent Provider

6. 阶段结构化输出契约
   - 已建立阶段输出 schema 字段常量
   - 需求、方案、代码、测试、评审、交付各阶段都输出结构化 JSON
   - 下游阶段可消费上游 Artifact / OutputJSON

7. Pipeline 工作台聚合 API
   - 新增 `GET /api/pipeline-runs/:id/timeline`
   - 新增 `GET /api/pipeline-runs/:id/current`
   - timeline 聚合 run、current、stages、artifacts、checkpoints、agentRuns、summary
   - current 聚合当前 stage、artifact、checkpoint、agentRun

8. 测试状态
   - 本次已执行 `cd apps/api-go && go test ./...`
   - 当前后端 Go 全量测试通过

9. 前端工作台现状
   - 已存在首页、需求新建、工作流、审批、监控、交付等页面入口
   - `Workflows` 已接入 PipelineRun 列表、timeline/current、阶段进度、AgentRun、Artifact、Delivery 与 start/pause/resume/terminate 操作
   - `Approvals` 已接入 current/timeline，能筛选 pending checkpoint 并调用 approve/reject
   - `Delivery` 已接入 deliveries、GitDelivery 详情和关联 timeline，能展示 PR/MR 草稿、变更文件与验证摘要

10. 本轮 S3 第一批开发完成能力
   - 新增统一 `AgentProvider` / `AgentRunner` / `PromptRegistry` 结构
   - Pipeline 可执行阶段已具备 AgentKey、System Prompt、User Prompt、输出必填字段校验
   - 有真实 AI client 时，Pipeline 启动会通过统一 provider adapter 调用模型
   - 无 provider、provider 调用失败、JSON 解析失败、schema 校验失败时，自动 fallback 到 deterministic handler
   - AgentRun 由 Engine 统一落库，记录 provider、model、promptSnapshot、inputJSON、outputJSON、tokenUsageJSON、latencyMS、status、errorMessage
   - `test_generation` 阶段即使 provider 输出合法，也保留后端白名单测试命令结果，避免模型绕过真实验证
   - 已移除不再使用的 `UpdateAgentRunStatus` repository 方法，避免 AgentRun 状态写入存在两条路径

11. 本轮 S3 第二批开发完成能力
   - delivery 阶段完成后自动创建 `GitDelivery` 本地交付草稿记录
   - `GitDelivery` 增加 PR/MR 标题、正文、变更文件、验证摘要等字段
   - 新增 `GET /api/pipeline-runs/:id/deliveries`
   - 新增 `GET /api/git-deliveries/:deliveryID`
   - timeline/current 已聚合 delivery 数据
   - timeline summary 已包含 latestDeliveryID、startedAt、finishedAt、durationMs
   - current 已包含 nextAction，支持 start_run、wait_execution、approve_checkpoint、inspect_failure、resume_run、review_delivery、completed、terminated
   - 已重新生成 Swagger/OpenAPI 文档
   - 已清理旧 TS 辅助服务中硬编码的飞书/OpenAI 默认密钥

12. 本轮工作台接入与真实环境核验完成能力
   - 已使用真实 Ark 配置完成端到端 smoke run，7 条 AgentRun 均为 `provider=ark` 且 `status=succeeded`
   - 已验证 checkpoint reject 后回退上一可执行阶段，并在重跑输入中携带驳回原因
   - 已验证 GitDelivery 本地草稿生成与查询，默认不 push、不创建远程 PR/MR
   - 修复异步 runner 中 pause/terminate 被运行中阶段覆盖的问题
   - 修复 timeline durationMs 因每个阶段重新写入 startedAt 而失真的问题
   - 已补充 Agent 输出字段类型校验，防止真实 provider 返回字段名正确但类型错误的 JSON 污染下游阶段
   - 已预留第二个 `openai_compatible` provider 配置结构，后续可在 adapter 完成后接入第二真实模型提供方
   - 已执行 `cd apps/api-go && go test ./...` 与 `pnpm --filter web build`，均通过

### 3.2 当前功能是否正常

当前功能状态判断：

- Pipeline 数据底座：正常
- 默认模板创建：正常
- 状态机校验：正常
- Checkpoint approve/reject：正常
- deterministic Pipeline 执行：正常
- Stage Handler 拆分后测试：正常
- timeline/current 聚合 API：测试通过
- 真实 AI Agent 执行：已用 Ark / `doubao-seed-2-0-lite-260215` 完成真实配置联调
- 多 Provider 切换：已支持统一 provider 抽象，已实现 Ark + OpenAI 兼容双 Provider，运行时可配置切换
- GitDelivery 交付闭环：基础记录与查询 API 已完成；远程 push/PR 已通过 `execute-changes` 接口实现，需手动触发
- timeline/current nextAction：已完成
- 前端工作台消费 timeline/current：`Workflows`、`Approvals`、`Delivery` 已接入真实 Pipeline API
- 前端工作流、审批、交付页面：已从静态 Demo 数据升级为真实后端闭环展示，并通过前端构建
- 页面圈选与热更新：未完成，属于功能二选做范围

### 3.3 功能一可选加分项完成情况

- 多 Agent 协作：未实现，当前仅支持单 Agent 顺序执行
- 自动回归：未实现，Reject 后需人工触发重跑
- 可观测性面板：基本完成，已支持阶段耗时、Token 消耗、Agent 推理过程展示，缺少统计聚合面板
- 代码库索引：未实现，仅支持手动提供文件路径上下文
- Pipeline 模板：仅支持单一默认模板，不支持多模板和自定义编辑
- Git 集成：已实现，支持自动创建分支、提交代码、发起 PR/ MR，需手动触发执行

### 3.4 功能二（选做）完成情况

- 前端UI和官网：已完成，TS+React 单页应用，包含完整的工作台页面
- 注入悬浮对话框与圈选功能：未实现
- 支持圈选并定位修改：未实现
- 预览链接支持热更新：未实现
- 自动创建 MR 并生成摘要：部分完成，摘要生成已实现，MR 创建需手动触发

### 3.3 本次核验依据

本次文档判断主要依据：

- 赛题文档：`docs/飞书智交付赛题.md`
- 产品技术设计：`docs/飞书智交付-产品开发设计.md`
- 第一、第二阶段计划：`docs/飞书智交付-第一阶段实施计划.md`、`docs/飞书智交付-第二阶段实施计划.md`
- 后端关键实现：`apps/api-go/internal/model`、`repo`、`service`、`pipeline`、`controller`、`router`
- 前端关键页面：`apps/web/src/pages/Workflows.tsx`、`Approvals.tsx`、`Delivery.tsx`
- 后端测试命令：`cd apps/api-go && go test ./...`

---

## 4. 当前阶段主要问题

结合赛题要求和当前代码状态，下一阶段最关键的问题不是继续增加更多确定性 Demo 输出，而是把“看起来像 Pipeline 的确定性执行器”升级为“可接入真实 AI Agent 的研发流程引擎”。

### 4.1 Agent Provider 已有最小接入，仍需真实联调

当前 Pipeline 已新增统一 provider adapter：

- 无 AI client 时 fallback 到 deterministic
- 有 Ark AI client 时通过 `TextGenerationProvider` 调用统一接口
- AgentRun 会记录 provider、model、prompt、input、output、latency、fallback reason

仍未完成的是：真实 Ark Key 联调（Token Usage 受 eino 库限制暂未支持）；第二个模型提供方的 adapter 已实现（OpenAI 兼容）。

### 4.2 Prompt 与 Agent 角色定义还不成体系

当前各 Stage Handler 已经拆分，但阶段内的执行逻辑仍主要是 Go 代码生成确定性结构。下一阶段需要让每个阶段拥有：

- AgentKey
- System Prompt
- User Prompt Builder
- 输入 JSON 契约
- 输出 JSON 契约
- 输出校验与 fallback 策略

这样才能在答辩中清楚说明 Agent 编排策略和 AI Native 设计，而不是只展示硬编码模拟。

本轮已完成 Prompt Registry 基础骨架。后续仍需继续增强：

- 根据真实 provider 输出质量微调 prompt
- 引入更完整的 JSON Schema 校验，包括数组元素结构和枚举值校验
- 给 prompt 增加代码库上下文压缩策略，避免输入过长

### 4.3 AgentRun 可观测信息还不够完整

赛题强调可观测性面板是加分项，而产品设计也要求每个 Stage 的输入、输出、日志、耗时、token、错误可追踪。

当前已经有 AgentRun 基础模型，但下一阶段仍需增强：

- provider 原始响应摘要
- token usage
- latency
- fallback reason
- JSON parse error
- schema validation error
- provider error

本轮已能记录上述所有字段：prompt、input、output、latency、status、error、fallback reason、token usage。Token Usage 完整结构已实现，OpenAI 兼容 Provider 已支持真实 Token 统计；现有 Ark client wrapper 受 eino 库版本限制暂时无法取得真实 token 统计，后续库升级后可无缝对接。

### 4.4 GitDelivery 基础闭环已完成，远程交付仍未启用

当前 delivery 阶段已生成可查询的 GitDelivery 本地交付草稿记录，并提供稳定查询 API。

赛题不要求本阶段必须默认 push 或创建远程 PR，但需要至少展示：

- 最终变更摘要
- 可合并代码变更或变更计划
- PR/MR 标题和正文草稿
- 交付状态
- 与 PipelineRun 的关联

本轮已满足上述基础展示能力。仍未做的是自动创建远程分支、push、提交远程 PR/MR。

### 4.5 工作台 API 已具备下一步动作语义

timeline/current 已经能返回聚合数据，并通过 `nextAction` 表达：

- `start_run`
- `wait_execution`
- `approve_checkpoint`
- `inspect_failure`
- `resume_run`
- `review_delivery`
- `completed`
- `terminated`

前端工作台已开始消费该字段驱动主按钮、审批面板和交付审查区。后续可继续增强筛选、搜索、AgentRun 详情抽屉和更细粒度的失败诊断。

---

## 5. 下一阶段目标

下一阶段目标是完成：

> 从 deterministic Pipeline 底座升级到“Agent 可插拔、调用可观测、交付可查询”的 DevFlow Engine 演示闭环。

具体目标：

1. 支持统一 Agent Provider Adapter
   - deterministic provider 作为默认 fallback
   - 真实 provider 可配置接入
   - 至少预留两个 provider 的切换结构

2. 建立阶段 Prompt Registry
   - 每个可执行阶段都有明确 Agent 角色
   - 每个阶段都有输入输出契约
   - 每个阶段都支持 provider 调用和 fallback

3. 增强 AgentRun 可观测性
   - 记录 prompt、input、output、token、latency、status、error
   - timeline/current 可展示 AgentRun 详情

4. 实现 GitDelivery 基础交付闭环
   - delivery 阶段创建 GitDelivery 记录
   - 提供 GitDelivery 查询 API
   - 不默认执行远程 push 或 PR 创建

5. 增强工作台聚合 API
   - summary 增加交付与耗时信息
   - current 增加 nextAction
   - 支持前端按单个 current 响应渲染主操作区

6. 完成 Swagger、测试与演示链路收口
   - 更新 API 文档
   - 增加关键测试
   - 验证 demo mode 无外部 key 可跑通

---

## 6. 本阶段范围内

本阶段明确要做：

1. Agent Provider Adapter
   - 定义 provider 接口
   - 定义请求/响应结构
   - deterministic provider 接入统一接口
   - 真实 provider 的最小接入或配置占位
   - provider 选择与 fallback

2. Prompt Registry
   - 阶段 AgentKey 定义
   - 阶段 system prompt 定义
   - 阶段 user prompt 构造
   - 阶段输出 JSON schema 校验

3. Stage Handler Agent 化
   - requirement analysis
   - solution design
   - code generation
   - test generation
   - code review
   - delivery

4. AgentRun 增强
   - 成功记录
   - 失败记录
   - fallback 记录
   - token/latency 记录

5. GitDelivery 基础能力
   - delivery 阶段创建 GitDelivery
   - 查询 PipelineRun 的 deliveries
   - 查询单个 GitDelivery
   - timeline/current 关联最新交付

6. 工作台聚合增强
   - nextAction
   - latestDeliveryID
   - duration 信息
   - 当前阶段展示数据补齐

7. 验证与文档
   - Go 全量测试
   - Swagger 注释更新
   - spec/tasks/checklist 更新
   - 演示链路核验

---

## 7. 本阶段范围外

本阶段明确不做：

1. 不默认自动 push 代码
2. 不默认自动创建远程 PR/MR
3. 不实现完整浏览器扩展
4. 不实现 DOM 到源码定位
5. 不实现页面圈选元素修改闭环
6. 不实现完整语义向量索引
7. 不引入 Redis 或外部队列作为必需依赖
8. 不重写当前 controller/service/repo/pipeline 分层
9. 不做完整前端重设计；当前只接入 Workflows / Approvals / Delivery 的真实工作台数据
10. 不引入不可本地运行的强外部依赖

这些能力留到后续阶段。

---

## 8. 关键技术决策与权衡

### 8.1 保留 deterministic fallback

决策：真实 provider 接入后，仍保留 deterministic provider。

原因：

- 比赛演示时外部 API Key、网络、模型稳定性不可控
- 本地开发和 CI 不应依赖外部模型
- fallback 能保证端到端链路一直可演示

权衡：

- deterministic 输出智能度有限
- 但它能作为稳定基线，真实 AI 能力作为增强层逐步加入

### 8.2 高风险 Git 操作默认不执行

决策：本阶段只创建 GitDelivery 交付记录和 PR/MR 草稿，不默认 push 或创建远程 PR。

原因：

- 自动 push/PR 是外部可见动作，风险更高
- 本阶段目标是后端演示闭环，不是完整 GitOps 自动化
- 保持安全边界有利于答辩解释

权衡：

- 演示中不能直接展示远程 PR 创建
- 但可以展示交付摘要、PR 草稿、变更计划，后续再接真实 Git 平台

### 8.3 Stage Handler 不推翻，只做 Agent 化

决策：保留当前拆分后的 Stage Handler 结构，在 handler 内接入 Agent Runner。

原因：

- 当前拆分已通过测试
- 每个阶段边界清晰
- 可以最小化改动风险

权衡：

- 短期仍是顺序执行
- 并行、多 Agent 协商、模板依赖图调度后续再做

### 8.4 输出必须结构化

决策：真实 provider 输出必须被解析为结构化 JSON，不能只保存自然语言文本。

原因：

- 下游阶段依赖上游阶段数据
- 前端工作台需要稳定字段展示
- Checkpoint 审批需要展示明确产物

权衡：

- Prompt 和校验会更复杂
- 但可显著降低 AI 输出漂移对流程的破坏

---

## 9. 推荐开发顺序

下一阶段建议按以下顺序推进：

1. Agent Provider Adapter
2. Prompt Registry 基础骨架
3. AgentRun 可观测增强
4. Stage Handler Agent 化
5. GitDelivery 基础交付闭环
6. timeline/current 增强
7. Swagger 与测试收口

第一批建议先实现：

```text
Agent Provider Adapter + Prompt Registry 骨架 + AgentRun 可观测增强
```

理由：

- 这是满足赛题 Agent 编排要求的核心缺口
- 也是后续 GitDelivery 和工作台增强的数据基础
- 风险集中但边界清晰，适合下一轮开发

---

## 10. 本阶段完成定义

本阶段完成时，应满足：

1. 无外部 AI Key 时，demo mode 仍可完整跑通 Pipeline
2. 配置真实 provider 时，阶段可通过统一 provider 接口调用
3. provider 调用成功、失败、fallback 都能形成 AgentRun 记录
4. 每个可执行阶段有明确 Agent 角色和 prompt 定义
5. delivery 阶段能创建 GitDelivery 记录
6. GitDelivery 可通过 API 查询
7. timeline/current 能展示执行、审批、交付的核心状态
8. `cd apps/api-go && go test ./...` 通过
9. Swagger 注释与新增 API 对齐
10. `spec.md`、`tasks.md`、`checklist.md` 与实际实现状态一致

---

## 11. 当前需要用户提供或确认的信息

为继续推进下一批开发，需要确认：

1. 真实 Ark provider 联调所需配置
   - `FEISHU_PIPELINE_AI_ARK_API_KEY`
   - Ark 模型名是否继续使用 `doubao-seed-2-0-lite-260215`

2. 第二个模型提供方选择
   - 赛题要求至少两个 provider 可配置切换
   - 建议在 OpenAI、通义、Anthropic 中选择一个作为第二 adapter

3. 下一批优先级
   - 建议优先接前端：让工作台消费 timeline/current/AgentRun/GitDelivery
   - 后端可继续增强：第二个模型 provider、真实 token usage、Git 远程 PR/MR

真实配置已由用户提供，但本轮未把密钥写入仓库。`apps/api-go/config/config.yaml` 已被 `.gitignore` 忽略，可以用于本地联调；在公开仓库前建议轮换已经出现在对话中的密钥。
