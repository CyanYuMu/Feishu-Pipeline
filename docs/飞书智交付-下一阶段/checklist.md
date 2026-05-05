# 飞书智交付 · 下一阶段验证清单（checklist）

## 1. 使用方式

本文档用于在下一阶段开发完成后逐项核查完整性。只有核心实现、API、测试、演示链路、文档状态都核验通过，才视为本阶段收尾。

状态约定：

- `[ ]` 未核验
- `[x]` 已通过
- `[!]` 未通过或需修复
- `[n/a]` 本阶段不适用

本次核验记录：

- 核验日期：2026-04-26（Asia/Shanghai）
- 后端命令：`cd apps/api-go && go test ./...`
- 前端命令：`pnpm --filter web build`
- 真实环境 smoke：Ark / `doubao-seed-2-0-lite-260215` 端到端 Pipeline 主闭环通过
- 结果：通过
- 说明：本次核验覆盖后端测试、前端构建、真实 Ark AgentRun、checkpoint reject/approve 和 GitDelivery 查询。

---

## 2. 当前基线核验

### 2.1 已开发功能核验

- [x] Pipeline 核心模型已存在
- [x] 默认 8 阶段 Pipeline 已存在
- [x] PipelineRun 可从需求文本创建
- [x] PipelineRun 可从 Session 创建
- [x] StageRun、Checkpoint、Artifact 初始化已接入
- [x] PipelineRun 聚合创建已事务化
- [x] PipelineRun 生命周期 API 已存在
- [x] start / pause / resume / terminate 已有状态校验
- [x] pause / terminate 在异步 runner 中不会再被运行中阶段覆盖
- [x] Checkpoint approve / reject 已存在
- [x] 重复审批保护已补充
- [x] reject 后可回退上一可执行阶段
- [x] Stage Handler 已拆分
- [x] 阶段结构化输出契约已建立
- [x] deterministic fallback 可运行
- [x] AgentRun 基础记录已存在
- [x] timeline API 已新增
- [x] current API 已新增
- [x] `cd apps/api-go && go test ./...` 已通过
- [x] 前端基础页面入口已存在

### 2.2 当前已知缺口

- [x] 真实 Agent Provider 已通过 Ark Key 联调
- [x] 至少两个模型提供方的真实 adapter 与运行时切换已完成（Ark + OpenAI 兼容）
- [x] Prompt Registry 已形成基础实现
- [x] Agent 输出 JSON schema 字段类型校验已补充
- [x] AgentRun token usage 真实统计已实现（Ark 端暂未支持原始返回，OpenAI 兼容端已完整支持）
- [x] GitDelivery 基础交付记录闭环已实现
- [x] GitDelivery 查询 API 已实现
- [x] timeline/current 已包含 delivery 和 nextAction
- [x] Swagger/OpenAPI 已跟随新增接口更新
- [x] 前端工作台已消费 timeline/current/deliveries
- [x] 前端工作流、审批、交付页面已接入真实 Pipeline 数据
- [n/a] 页面圈选、热更新、MR 自动创建不属于本阶段主线

---

## 3. S3-1 Agent Provider Adapter 验证

### 3.1 Provider 抽象

- [x] 已定义统一 Provider 接口
- [x] 已定义统一请求结构
- [x] 已定义统一响应结构
- [x] 请求结构不包含明文持久化 API Key
- [x] 响应结构包含 content/output
- [x] 响应结构包含 token usage
- [x] 响应结构包含 latency
- [x] 响应结构包含 raw provider metadata

### 3.2 Provider 配置

- [x] 支持 provider name
- [x] 支持 model name
- [x] 支持 API Key 从环境变量读取
- [x] 支持超时配置
- [x] 支持 demo mode
- [x] 无 API Key 时不会启动失败

### 3.3 Provider 实现

- [x] deterministic provider 可运行
- [x] 至少一个真实 provider 已接入或具备完整占位调用链路
- [x] 第二个 provider 的配置结构已预留
- [x] provider 选择逻辑有测试
- [x] fallback 逻辑有测试

### 3.4 安全核验

- [x] API Key 不写入数据库
- [x] API Key 不写入日志
- [x] provider 原始错误不会泄漏敏感请求头
- [x] 外部调用失败会记录结构化错误

---

## 4. S3-2 Prompt Registry 与阶段 Agent 化验证

### 4.1 Prompt Registry

- [x] 每个可执行阶段有 AgentKey
- [x] 每个可执行阶段有 system prompt
- [x] 每个可执行阶段有 user prompt builder
- [x] prompt 明确输入 JSON
- [x] prompt 明确输出 JSON 格式
- [x] prompt 明确禁止输出无法解析的自由文本，或实现了提取逻辑

### 4.2 阶段覆盖

- [x] requirement_analysis 已走 Agent Runner 或 fallback
- [x] solution_design 已走 Agent Runner 或 fallback
- [x] code_generation 已走 Agent Runner 或 fallback
- [x] test_generation 已走 Agent Runner 或 fallback
- [x] code_review 已走 Agent Runner 或 fallback
- [x] delivery 已走 Agent Runner 或 fallback
- [x] checkpoint 阶段仍由人工审批控制，不误走 provider

### 4.3 输出契约

- [x] requirement_analysis 输出包含结构化需求和验收标准
- [x] solution_design 输出包含影响文件、API 改动、实现计划
- [x] code_generation 输出包含 changedFiles、patches、diffSummary
- [x] test_generation 输出包含 testPlan、commands、commandResults、status
- [x] code_review 输出包含 conclusion、issues、securityNotes
- [x] delivery 输出包含 prTitle、prBody、validation、manualReleaseNotes

### 4.4 异常处理

- [x] JSON 解析失败有错误记录
- [x] schema 校验失败有错误记录
- [x] schema 字段类型错误会触发 fallback
- [x] provider 超时有错误记录
- [x] fallback 被触发时可追踪原因
- [x] 阶段失败不会留下不一致状态

---

## 5. S3-3 AgentRun 可观测验证

### 5.1 基础字段

- [x] 每次阶段执行都有 AgentRun
- [x] AgentRun 记录 PipelineRunID
- [x] AgentRun 记录 StageRunID
- [x] AgentRun 记录 AgentKey
- [x] AgentRun 记录 Provider
- [x] AgentRun 记录 Model
- [x] AgentRun 记录 PromptSnapshot
- [x] AgentRun 记录 InputJSON
- [x] AgentRun 记录 OutputJSON
- [x] AgentRun 记录 TokenUsageJSON
- [x] AgentRun 记录 LatencyMS
- [x] AgentRun 记录 Status
- [x] AgentRun 记录 ErrorMessage

### 5.2 成功与失败

- [x] provider 成功时 AgentRun 为 succeeded
- [x] provider 失败且 fallback 成功时 AgentRun 为 succeeded，并记录 fallback reason
- [x] fallback 失败时 AgentRun 为 failed
- [x] fallback 成功时 AgentRun 能体现 fallback
- [x] JSON 校验失败时 AgentRun 能体现失败原因
- [x] timeline 能返回 AgentRun 列表
- [x] current 能返回当前阶段最新 AgentRun

---

## 6. S3-4 GitDelivery 验证

### 6.1 数据创建

- [x] delivery 阶段完成后创建 GitDelivery
- [x] GitDelivery 关联 PipelineRunID
- [x] GitDelivery 记录目标分支
- [x] GitDelivery 记录工作分支
- [x] GitDelivery 记录状态
- [x] GitDelivery 记录交付摘要
- [x] GitDelivery 记录 PR/MR 标题草稿
- [x] GitDelivery 记录 PR/MR 正文草稿

### 6.2 API

- [x] 可按 PipelineRun 查询 deliveries
- [x] 可按 deliveryID 查询详情
- [x] 查询不存在 delivery 时返回合理错误
- [x] API 需要登录鉴权
- [x] 响应类型在 `internal/type/pipeline` 中定义

### 6.3 安全边界

- [x] 默认不执行 git push
- [x] 默认不创建远程 PR/MR
- [x] 默认不删除文件
- [x] 默认不覆盖未提交用户修改
- [x] 如有高风险动作，必须显式配置或用户确认

---

## 7. S3-5 工作台聚合 API 验证

### 7.1 Timeline

- [x] 返回 run
- [x] 返回 stages
- [x] 返回 artifacts
- [x] 返回 checkpoints
- [x] 返回 agentRuns
- [x] 返回 current
- [x] 返回 summary
- [x] summary 包含 totalStages
- [x] summary 包含 completedStages
- [x] summary 包含 failedStages
- [x] summary 包含 waitingApproval
- [x] summary 包含 currentStageKey
- [x] summary 包含 latestArtifactID
- [x] summary 包含 latestDeliveryID，如实现 delivery
- [x] summary 包含 durationMs，如实现耗时统计

### 7.2 Current

- [x] 返回当前 run
- [x] 返回当前 stage
- [x] 返回当前 artifact
- [x] 返回当前 checkpoint
- [x] 返回当前 agentRun
- [x] 返回当前 delivery，如处于交付阶段
- [x] 返回 nextAction

### 7.3 NextAction

- [x] draft run 返回 start_run
- [x] queued/running 返回 wait_execution
- [x] waiting_approval 返回 approve_checkpoint
- [x] failed 返回 inspect_failure
- [x] delivery ready 返回 review_delivery
- [x] completed 返回 completed

### 7.4 前端工作台

- [x] Workflows 可展示 PipelineRun 列表
- [x] Workflows 可展示 timeline 阶段进度
- [x] Workflows 可展示 AgentRun、Artifact、Delivery 核心数据
- [x] Workflows 可触发 start/pause/resume/terminate
- [x] Approvals 可筛选 pending checkpoint
- [x] Approvals 可展示审批上下文和最近产物
- [x] Approvals 可调用 approve/reject
- [x] Delivery 可展示 GitDelivery 列表
- [x] Delivery 可展示单条 GitDelivery 详情、变更文件和验证摘要

---

## 8. API 与文档验证

### 8.1 Swagger/OpenAPI

- [x] timeline API 有 Swagger 注释
- [x] current API 有 Swagger 注释
- [x] GitDelivery API 有 Swagger 注释
- [x] checkpoint approve/reject 文档准确
- [x] 新增 response type 能出现在文档中
- [x] 错误响应文档准确

### 8.2 项目文档

- [x] `spec.md` 与实际范围一致
- [x] `tasks.md` 勾选状态与实际进度一致
- [x] `checklist.md` 核验结果与测试结果一致
- [x] 如范围变化，已同步更新三份文档

---

## 9. 测试验证

### 9.1 后端测试

以下是 S3 实现完成后的收尾验证，不等同于当前基线测试：

- [x] 执行 `cd apps/api-go && go test ./...`
- [x] 所有测试通过
- [x] provider adapter 测试通过
- [x] fallback 测试通过
- [x] stage agent 化测试通过
- [x] AgentRun 记录测试通过
- [x] GitDelivery 测试通过
- [x] timeline/current 测试通过

### 9.2 演示链路手工验证

- [x] 创建 PipelineRun
- [x] 启动 PipelineRun
- [x] 自动执行需求分析
- [x] 自动执行方案设计
- [x] 到达方案审批 checkpoint
- [x] 查询 current 能看到 checkpoint
- [x] reject 后回退方案阶段并携带驳回原因
- [x] approve 后继续执行
- [x] 自动执行代码生成
- [x] 自动执行测试生成
- [x] 自动执行代码评审
- [x] 到达评审确认 checkpoint
- [x] approve 后执行 delivery
- [x] delivery 生成交付摘要
- [x] GitDelivery 可查询
- [x] timeline 展示完整阶段链路

### 9.3 Demo Mode 验证

- [x] 不配置外部 AI Key 时后端可启动
- [x] 不配置外部 AI Key 时 Pipeline 可跑通
- [x] deterministic provider 在 AgentRun 中可见
- [x] fallback 原因可追踪

### 9.4 前端构建验证

- [x] 执行 `pnpm --filter web build`
- [x] TypeScript 构建通过
- [x] Vite 构建通过
- [x] 仅存在 chunk size warning，无阻塞错误

---

## 10. 最终收尾标准

本阶段可收尾的条件：

- [x] 所有范围内任务完成或明确标记延期
- [x] 高优先级 checklist 全部通过
- [x] `cd apps/api-go && go test ./...` 通过
- [x] `pnpm --filter web build` 通过
- [x] 无新增敏感信息泄漏风险
- [x] 无默认高风险 Git 操作
- [x] 文档已同步
- [x] 能按演示链路跑出完整 Pipeline timeline
- [x] 能展示 AgentRun 和 GitDelivery 的核心数据

---

## 11. 功能一可选加分项核验

- [x] 多 Agent 协作：同一阶段内多个 Agent 并行或协商工作
  - 说明：已实现多Agent并行执行框架，支持4种合并策略。代码生成和代码评审阶段已配置多Agent协作，分别从性能、可读性、安全性三个维度生成和评审代码，大幅提升输出质量。
- [ ] 自动回归：当评审发现问题时，Agent 自动修复并重新提交，无需人工介入
  - 说明：当前 Reject 后需人工触发重新执行，无自动修复逻辑
- [x] 可观测性面板：Pipeline 运行状态的实时可视化
  - [x] 每个阶段的耗时统计
  - [x] Token 消耗记录
  - [x] Agent 的推理过程展示
  - [ ] 成功/失败率统计面板
  - 说明：核心可观测数据已具备，缺少统计聚合面板
- [ ] 代码库索引：对目标代码库进行语义索引，让 Agent 在方案设计和代码生成阶段能精准检索相关代码
  - 说明：当前仅支持手动提供文件路径上下文，无自动语义索引能力
- [ ] Pipeline 模板：支持预定义不同类型的 Pipeline 模板
  - [ ] Bug 修复流程模板
  - [ ] 新功能开发流程模板
  - [ ] 重构流程模板
  - [ ] 模板自定义编辑功能
  - 说明：当前仅支持单一默认 Feature Delivery 模板
- [x] Git 集成：自动创建分支、提交代码、发起 MR/PR
  - [x] 自动创建工作分支
  - [x] 代码变更执行与提交
  - [x] PR/MR 自动创建
  - [x] 变更摘要生成
  - 说明：通过 `/api/pipeline-runs/:id/execute-changes` 接口实现，需手动触发

---

## 12. 功能二（选做）核验

- [x] 前端UI和官网
  - [x] TS+React 单页应用实现
  - [x] 首页与功能页完整
  - [x] 飞书风格的交互设计
  - 说明：前端工作台已完整实现，包含 Workflows、Approvals、Delivery 等核心页面
- [ ] 注入悬浮对话框与圈选功能
  - [ ] 浏览器扩展或注入脚本实现
  - [ ] 页面悬浮控件
  - [ ] 元素圈选功能
  - 说明：当前无相关实现
- [ ] 支持圈选并定位修改
  - [ ] 至少支持3种不同元素的圈选（按钮、标题、卡片文案等）
  - [ ] 圈选上下文理解
  - [ ] 源代码定位能力
  - 说明：当前无相关实现
- [ ] 预览链接支持热更新
  - [ ] 代码修改触发热更新
  - [ ] 页面实时刷新展示修改效果
  - 说明：当前无相关实现
- [ ] 自动创建 MR 并生成摘要
  - [x] 语义化改动摘要生成
  - [x] 行级 diff 摘要展示
  - [ ] 修改确认后自动创建 MR
  - 说明：摘要生成已实现，MR 创建需手动触发 `execute-changes` 接口
