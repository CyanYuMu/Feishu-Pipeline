# 飞书智交付 · 下一阶段任务拆解（tasks）

## 1. 使用方式

本文档把 `spec.md` 中的下一阶段范围拆解为可执行任务。后续开发时，Agent 应按本文档顺序推进，并实时更新勾选状态。

状态约定：

- `[ ]` 未开始
- `[~]` 进行中
- `[x]` 已完成
- `[!]` 阻塞或需要重新确认

本次文档更新已完成的基线核验：

- [x] 已阅读赛题文档、产品技术设计、第一阶段计划、第二阶段计划
- [x] 已核对后端 Pipeline 模型、Service、Repository、Engine、Stage Handler、Controller、Router
- [x] 已核对前端工作流、审批、交付页面当前仍以静态 Demo 数据为主
- [x] 已执行 `cd apps/api-go && go test ./...`，当前后端测试通过
- [x] 已确认下一阶段从 S3：Agent 可插拔、调用可观测、交付可查询 开始推进
- [x] 已执行真实 Ark 端到端 smoke run，确认主 Pipeline 闭环可跑通
- [x] 已执行 `pnpm --filter web build`，确认前端工作台接入后可构建

---

## 2. 阶段总览

当前下一阶段建议拆为 6 个模块：

1. S3-1：Agent Provider Adapter
2. S3-2：Prompt Registry 与阶段 Agent 化
3. S3-3：AgentRun 可观测增强
4. S3-4：GitDelivery 基础交付闭环
5. S3-5：工作台聚合 API 增强
6. S3-6：Swagger、测试与演示验收

推荐顺序：

```text
S3-1 -> S3-2 -> S3-3 -> S3-4 -> S3-5 -> S3-6
```

原因：

- Provider Adapter 是阶段 Agent 化的前提
- AgentRun 可观测依赖 Agent 调用链路
- GitDelivery 依赖 delivery 阶段输出稳定
- 工作台增强依赖前面数据结构成型
- Swagger 和测试放在最后做整体收口

---

## 3. S3-1：Agent Provider Adapter

### 3.1 目标

建立统一 Agent Provider 抽象，使 Pipeline 阶段可以在 deterministic provider 和真实 LLM provider 之间切换。

### 3.2 范围

- Provider 接口定义
- Provider 配置结构
- deterministic provider 适配
- 至少一个真实 provider 的接入点或最小实现
- provider 选择逻辑
- fallback 策略

### 3.3 子任务

- [x] S3-1.1 梳理当前 Agent/AI 相关代码
  - [x] 检查 `apps/api-go/internal/agent`
  - [x] 检查 `apps/api-go/internal/external/ai`
  - [x] 确认是否已有 provider/client 可复用
  - [x] 避免重复造轮子
  - 结论：`internal/agent` 当前偏 Session 需求拆解 workflow；`internal/external/ai/ark.go` 可作为真实 provider 的包装入口；Pipeline Engine 当前直接写入 `internal/deterministic` AgentRun，下一步应抽出统一 Agent Runner。

- [x] S3-1.2 定义 Provider 接口
  - [x] 新增或复用 agent provider package
  - [x] 定义统一请求结构
  - [x] 定义统一响应结构
  - [x] 字段至少包含 prompt、input JSON、model、temperature、max tokens、metadata
  - [x] 响应至少包含 content、raw JSON、token usage、latency、finish reason
  - 结论：新增 `internal/pipeline/provider.go`，以 `AgentProvider` / `AgentProviderRequest` / `AgentProviderResponse` 统一真实模型与 fallback 观测。

- [x] S3-1.3 实现 deterministic provider
  - [x] 把当前 Stage Handler 的确定性逻辑纳入 fallback 路径
  - [x] 确保无外部配置时默认可运行
  - [x] AgentRun 中明确记录 provider=`internal` 或 `deterministic`
  - 结论：无 provider、provider 错误、JSON 解析失败、schema 校验失败时均回退 deterministic handler，并在 AgentRun tokenUsageJSON 中记录 fallbackReason。

- [x] S3-1.4 设计真实 provider 配置
  - [x] 支持 provider name
  - [x] 支持 model name
  - [x] 支持 API Key 从环境变量读取
  - [x] 支持超时配置
  - [x] 支持 demo mode 开关
  - [x] 预留第二个 `openai_compatible` provider 配置结构
  - 结论：现有 `ai.provider`、`ai.ark.*` 配置已被 Pipeline provider adapter 复用；新增 `ai.openai_compatible.*` 配置结构用于后续第二真实 provider adapter 联调。demo mode 目前表现为“无 key 自动 fallback”，后续可补显式 `demo_mode` 配置项。

- [x] S3-1.5 接入至少一个真实 provider 的最小调用链路
  - [x] 如果使用已有 AI client，则包装为 Provider
  - [x] 如果暂不直接调用外部 API，则至少完成接口和配置占位
  - [x] 外部调用失败时返回结构化错误
  - [x] 不把 API Key 写入数据库或日志
  - 结论：启动时如 Ark client 可用，会通过 `TextGenerationProvider` 接入 Pipeline AgentRunner；API Key 仍只从配置/环境变量进入 client，不进入 AgentRun。

- [x] S3-1.6 Provider fallback 策略
  - [x] 无 API Key 时 fallback deterministic
  - [x] provider 调用失败时记录失败原因
  - [x] 明确哪些错误允许 fallback，哪些错误应中断阶段
  - [x] fallback 行为写入 AgentRun metadata 或 output

### 3.4 验收标准

- [x] 可以通过配置选择 deterministic provider
- [x] 可以通过配置选择真实 provider 或真实 provider 占位
- [x] 无 key 时 Pipeline 仍可跑通
- [x] provider 错误不会导致数据丢失
- [x] 测试覆盖 provider 选择和 fallback

---

## 4. S3-2：Prompt Registry 与阶段 Agent 化

### 4.1 目标

将各 Stage Handler 的执行逻辑升级为“Prompt + Provider + 输出校验 + fallback”的结构。

### 4.2 范围

覆盖以下阶段：

- requirement_analysis
- solution_design
- code_generation
- test_generation
- code_review
- delivery

Checkpoint 阶段仍由引擎和人工审批控制，不走 provider。

### 4.3 子任务

- [x] S3-2.1 定义 AgentKey
  - [x] requirement analyst agent
  - [x] solution designer agent
  - [x] code generator agent
  - [x] test generator agent
  - [x] code reviewer agent
  - [x] delivery integrator agent

- [x] S3-2.2 建立 Prompt Registry
  - [x] 每个阶段一个 system prompt
  - [x] 每个阶段一个 user prompt builder
  - [x] prompt 中明确输入 JSON
  - [x] prompt 中明确输出 JSON 字段
  - [x] prompt 中要求不得输出非 JSON 或需包裹可解析 JSON

- [x] S3-2.3 建立输出结构校验
  - [x] 校验 required fields
  - [x] 校验字段类型
  - [x] 校验空输出
  - [x] 校验 JSON 解析失败
  - [x] 输出不合法时记录错误并 fallback 或 fail
  - 说明：字段类型由 PromptRegistry 中的 schema 定义驱动，字符串字段必须为 JSON string，数组字段必须为 JSON array；未知扩展字段暂不拦截。

- [x] S3-2.4 改造 RequirementAnalysisHandler
  - [x] 构造需求分析输入
  - [x] 调用 Agent Runner
  - [x] 解析结构化需求输出
  - [x] fallback 到当前 deterministic 输出

- [x] S3-2.5 改造 SolutionDesignHandler
  - [x] 注入代码库上下文
  - [x] 调用 Agent Runner
  - [x] 输出影响文件、API 改动、数据模型改动、实现计划
  - [x] fallback 到当前 deterministic 输出

- [x] S3-2.6 改造 CodeGenerationHandler
  - [x] 输入技术方案和上下文
  - [x] 输出 changedFiles、patches、diffSummary
  - [x] 保持 patch 为受控结构，不直接危险写文件
  - [x] fallback 到当前 deterministic 输出

- [x] S3-2.7 改造 TestGenerationHandler
  - [x] 输入变更集和需求
  - [x] 保留受控命令执行
  - [x] Agent 可生成 testPlan，命令执行仍由后端白名单控制
  - [x] fallback 到当前 deterministic 输出
  - 说明：`test_generation` provider 输出不会覆盖后端白名单执行出的 commands、commandResults、status。

- [x] S3-2.8 改造 CodeReviewHandler
  - [x] 输入方案、变更集、测试结果
  - [x] 输出 conclusion、issues、securityNotes、maintainabilityNotes
  - [x] fallback 到当前 deterministic 输出

- [x] S3-2.9 改造 DeliveryHandler
  - [x] 输入评审结果和变更集
  - [x] 输出 prTitle、prBody、manualReleaseNotes、validation
  - [x] 为 GitDelivery 创建准备数据

### 4.4 验收标准

- [x] 每个可执行阶段都有 AgentKey
- [x] 每个可执行阶段都有 prompt 定义
- [x] 每个可执行阶段输出仍符合现有 schema
- [x] provider 关闭时测试仍通过
- [x] provider 开启时可记录真实调用结果或结构化错误

---

## 5. S3-3：AgentRun 可观测增强

### 5.1 目标

让每一次阶段 Agent 执行都可追踪、可审计、可展示。

### 5.2 子任务

- [x] S3-3.1 检查 AgentRun 模型字段
  - [x] provider
  - [x] model
  - [x] promptSnapshot
  - [x] inputJSON
  - [x] outputJSON
  - [x] tokenUsageJSON
  - [x] latencyMS
  - [x] status
  - [x] errorMessage
  - 结论：字段已存在；本轮已接入写入 promptSnapshot、tokenUsageJSON、latencyMS、fallback reason。

- [x] S3-3.2 统一 AgentRun 创建时机
  - [x] 调用 provider 前记录 input/prompt
  - [x] 调用成功后记录 output/token/latency/status
  - [x] 调用失败后记录 error/status/latency
  - 说明：当前为阶段执行结束后由 Engine 一次性落库 AgentRun，避免状态写入分散；实时运行中状态展示可作为后续优化项。

- [x] S3-3.3 增强 token usage 记录
  - [x] input tokens
  - [x] output tokens
  - [x] total tokens
  - [x] provider 原始 usage 字段
  - 说明：Token Usage 完整结构已实现，OpenAI 兼容 Provider 已支持真实 Token 统计；Ark 端受限于当前 eino 库版本，暂使用占位符，待后续库升级后可无缝对接。

- [x] S3-3.4 增强错误记录
  - [x] provider error
  - [x] JSON parse error
  - [x] schema validation error
  - [x] fallback reason

- [x] S3-3.5 增加 service 层聚合支持
  - [x] timeline 返回 AgentRun 已可用，确认字段完整
  - [x] current 返回当前阶段最新 AgentRun
  - [x] 确认现有 AgentRun 查询 API 是否足够
  - 说明：已有 `GET /api/pipeline-runs/:id/agent-runs`，本轮未新增单条 AgentRun 详情 API。

### 5.3 验收标准

- [x] 每个执行阶段至少有一条 AgentRun
- [x] 失败阶段也有 AgentRun 错误记录
- [x] timeline/current 能展示当前阶段 AgentRun
- [x] 测试覆盖成功、失败、fallback 三类情况

---

## 6. S3-4：GitDelivery 基础交付闭环

### 6.1 目标

让 delivery 阶段从“只生成交付摘要”升级为“创建可查询的 GitDelivery 交付记录”。

### 6.2 子任务

- [x] S3-4.1 检查 GitDelivery 模型与 repository 方法
  - [x] 确认字段是否覆盖 runID、branch、commit、PR/MR URL、status、summary
  - [x] 补充 create/list/get/update 方法
  - 结论：`GitDelivery` 已补 PR/MR 标题、正文、changedFilesJSON、validationJSON；repository 已补 create/list/get/updateStatus。

- [x] S3-4.2 DeliveryHandler 生成交付草稿
  - [x] prTitle
  - [x] prBody
  - [x] changedFiles
  - [x] validation summary
  - [x] manual release notes

- [x] S3-4.3 Pipeline Engine 在 delivery 阶段创建 GitDelivery
  - [x] 与 Artifact 创建保持一致性
  - [x] 不执行 push
  - [x] 不调用远程 Git API
  - [x] status 使用 draft/ready 等安全状态

- [x] S3-4.4 新增 GitDelivery 查询 API
  - [x] `GET /api/pipeline-runs/:id/deliveries`
  - [x] `GET /api/git-deliveries/:deliveryID`
  - [x] 返回交付摘要、PR 草稿、状态、关联 run

- [x] S3-4.5 timeline 关联最新 GitDelivery
  - [x] summary 增加 latestDeliveryID
  - [x] timeline 返回 deliveries 或 latestDelivery
  - [x] current 在 delivery 阶段展示交付信息

### 6.3 验收标准

- [x] delivery 阶段完成后有 GitDelivery 记录
- [x] 可通过 API 查询交付记录
- [x] timeline 能显示交付结果
- [x] 不执行远程 push/PR 创建
- [x] 测试覆盖 delivery 创建与查询

---

## 7. S3-5：工作台聚合 API 增强

### 7.1 目标

让前端工作台可以用少量 API 渲染完整 Pipeline 状态和下一步动作。

### 7.2 子任务

- [x] S3-5.1 增强 timeline summary
  - [x] totalStages
  - [x] completedStages
  - [x] failedStages
  - [x] waitingApproval
  - [x] currentStageKey
  - [x] latestArtifactID
  - [x] latestDeliveryID
  - [x] startedAt / finishedAt / durationMs

- [x] S3-5.2 增强 current 响应
  - [x] 当前 run
  - [x] 当前 stage
  - [x] 当前 artifact
  - [x] 当前 checkpoint
  - [x] 当前 agentRun
  - [x] 当前 delivery
  - [x] nextAction

- [x] S3-5.3 设计 nextAction
  - [x] `start_run`
  - [x] `wait_execution`
  - [x] `approve_checkpoint`
  - [x] `inspect_failure`
  - [x] `review_delivery`
  - [x] `completed`
  - [x] `resume_run`
  - [x] `terminated`

- [x] S3-5.4 测试聚合逻辑
  - [x] draft run
  - [x] running run
  - [x] waiting approval run
  - [x] failed run
  - [x] completed delivery run

- [x] S3-5.5 前端工作台消费聚合 API
  - [x] `Workflows` 消费 PipelineRun 列表、timeline/current、AgentRun、Artifact、Delivery
  - [x] `Workflows` 接入 start/pause/resume/terminate 操作
  - [x] `Approvals` 消费 current/timeline，展示 pending checkpoint 和审批上下文
  - [x] `Approvals` 接入 checkpoint approve/reject
  - [x] `Delivery` 消费 deliveries、GitDelivery 详情和关联 timeline
  - [x] 新增前端 Pipeline API client 与状态映射工具

### 7.3 验收标准

- [x] 前端可以通过 timeline/current 判断页面主状态
- [x] waiting approval 时能拿到 checkpoint 和审批动作提示
- [x] delivery 完成后能拿到交付记录
- [x] 测试覆盖关键状态
- [x] Workflows / Approvals / Delivery 已从静态 Demo 数据升级为真实 API 数据

---

## 8. S3-6：Swagger、测试与演示验收

### 8.1 目标

完成文档、测试和演示链路收口。

### 8.2 子任务

- [x] S3-6.1 Swagger 注释更新
  - [x] provider 相关接口，如有，本轮无新增 provider HTTP API
  - [x] GitDelivery 查询接口
  - [x] timeline/current 新字段
  - [x] checkpoint 审批接口保持准确

- [x] S3-6.2 生成或校验 API 文档
  - [x] 确认 docs/api 或 swagger 输出可用
  - [x] 确认新增类型在 OpenAPI 中出现

- [x] S3-6.3 后端测试
  - [x] `cd apps/api-go && go test ./...`
  - [x] provider fallback 测试
  - [x] stage agent 化测试
  - [x] delivery API 测试
  - [x] timeline/current 聚合测试
  - [x] pause/terminate 异步 runner 回归测试

- [x] S3-6.4 演示脚本核验
  - [x] 创建 PipelineRun
  - [x] start
  - [x] 到达方案审批
  - [x] reject 后回退方案阶段并携带原因
  - [x] approve
  - [x] 到达评审确认
  - [x] approve
  - [x] delivery 完成
  - [x] 查询 timeline/current/delivery

- [x] S3-6.5 更新 checklist
  - [x] 把已完成项勾选
  - [x] 标记仍未完成项
  - [x] 记录测试结果

- [x] S3-6.6 前端构建验证
  - [x] `pnpm --filter web build`
  - [x] Workflows / Approvals / Delivery 类型检查通过

### 8.3 验收标准

- [x] 全量测试通过
- [x] API 文档没有明显缺失
- [x] demo mode 无外部 key 可跑通
- [x] 如果配置真实 provider，错误可观测且不破坏数据
- [x] 文档与实现状态一致
- [x] 前端工作台可展示后端闭环数据

---

## 9. 推荐第一批开发切片

为了降低风险，建议下一次实际编码先做第一批：

```text
S3-1 Agent Provider Adapter
S3-2 Prompt Registry 基础骨架
S3-3 AgentRun 可观测增强的最小闭环
```

第一批不急于实现 GitDelivery API。原因：

- provider 和 AgentRun 是后续所有真实 AI 能力的基础
- 当前 delivery 仍可使用 deterministic 输出支撑演示
- 先把 Agent 调用链路稳定下来，再做交付记录更安全

第一批完成后，再做第二批：

```text
S3-4 GitDelivery 基础交付闭环
S3-5 工作台聚合 API 增强
S3-6 Swagger、测试与演示验收
```

---

## 10. 后续待开发任务（功能一可选加分项）

### 10.1 S4-1：多 Agent 协作能力
- [x] 设计多 Agent 协商机制
- [x] 实现同一阶段内多个 Agent 并行执行
- [x] 支持 Agent 间的结果对比与合并
- [x] 代码生成阶段配置3个Agent（性能/可读性/安全专家）并行工作
- [x] 代码评审阶段配置3个Agent（安全/性能/可维护性专家）并行工作
- [x] 实现4种合并策略：投票、择优、汇总、优先
- 说明：多Agent协作功能已完整实现，代码生成和代码评审阶段已默认启用多Agent模式，可通过模板配置灵活调整Agent数量、角色和合并策略。

### 10.2 S4-2：自动回归能力
- [ ] 设计评审问题自动修复流程
- [ ] 实现 Reject 原因的自动解析与理解
- [ ] 支持 Agent 基于评审意见自动修复代码
- [ ] 增加最大重试次数控制
- [ ] 修复完成后自动重新提交评审

### 10.3 S4-3：可观测性面板增强
- [ ] 实现 Pipeline 成功/失败率统计
- [ ] 增加 Token 消耗统计与趋势展示
- [ ] 实现阶段耗时统计与性能分析
- [ ] 开发 Agent 推理过程详情展示面板
- [ ] 支持日志与错误的搜索与过滤

### 10.4 S4-4：代码库语义索引
- [ ] 设计代码库索引架构
- [ ] 实现代码文件的语义化解析
- [ ] 支持基于语义的代码检索功能
- [ ] 集成到方案设计和代码生成阶段
- [ ] 支持索引的增量更新

### 10.5 S4-5：Pipeline 模板系统
- [ ] 设计模板数据结构与存储
- [ ] 实现 Bug 修复流程模板
- [ ] 实现新功能开发流程模板
- [ ] 实现重构流程模板
- [ ] 支持模板的自定义编辑与保存
- [ ] 开发模板管理 UI

### 10.6 S4-6：Git 集成增强
- [ ] 实现代码变更的自动执行
- [ ] 支持 MR/PR 创建的自动触发
- [ ] 增加 MR/PR 状态的同步与展示
- [ ] 实现代码合并后的状态回调

---

## 11. 后续待开发任务（功能二选做）

### 11.1 S5-1：浏览器扩展与注入脚本
- [ ] 设计浏览器扩展架构
- [ ] 实现页面注入脚本
- [ ] 开发悬浮对话框控件
- [ ] 实现与后端 API 的通信机制

### 11.2 S5-2：元素圈选功能
- [ ] 实现页面元素的圈选交互
- [ ] 支持至少3种不同类型元素的识别
- [ ] 实现圈选元素的上下文信息提取
- [ ] 开发圈选状态的 UI 展示

### 11.3 S5-3：代码定位与修改
- [ ] 设计 DOM 元素到源代码的映射机制
- [ ] 实现圈选元素到源代码的定位功能
- [ ] 支持基于自然语言指令的代码修改
- [ ] 实现修改结果的验证与回滚机制

### 11.4 S5-4：热更新预览
- [ ] 设计热更新架构
- [ ] 实现代码修改后的自动构建
- [ ] 支持页面的无刷新热更新
- [ ] 开发修改效果的实时预览功能

### 11.5 S5-5：MR 自动创建
- [ ] 实现修改确认后的自动提交
- [ ] 支持 MR/PR 的自动创建
- [ ] 生成语义化的改动摘要
- [ ] 实现行级 diff 摘要的生成与展示
