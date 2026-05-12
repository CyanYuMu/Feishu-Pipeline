# Plan: 分阶段开发路线

## Phase 0: Spec Baseline

建立 constitution、feature spec、implementation plan 和 task list。所有后续改动先更新任务状态，再实现和验证。

当前状态：已完成基础 spec、plan、tasks，并按产品形态 5.1/5.2 将开发路线重构为「Bot 入口优先、Bitable 台账贯穿、Pipeline Core 执行」。

## Phase 1: Feishu Bot Entry First

目标：让飞书机器人成为主入口，接收自然语言需求并启动同一套 Pipeline Run。

开发重点：

- 接收飞书机器人事件和群消息。
- 解析飞书文档链接、仓库、分支、审批人、上线确认人和来源群聊。
- 复用 `POST /api/pipeline-runs` 或内部 service 创建 Pipeline Run，避免飞书入口和 Web/API 入口状态分叉。
- 首次回复状态卡片，展示需求摘要、目标仓库、当前阶段和多维表格台账入口。

验收口径：评委在飞书群 @DevFlow Bot 后，可以看到需求被创建、Pipeline Run ID 被返回、台账记录被绑定。

## Phase 2: Bitable Ledger First-Class

目标：多维表格成为管理者真实可用的研发流水线数据库，而不是演示截图。

开发重点：

- 统一 PipelineRun -> Bitable record 字段映射。
- 创建、启动、暂停、恢复、等待审批、Reject、失败、重试、测试完成、MR 创建、交付完成时都 upsert 同一条记录。
- 字段覆盖需求标题、来源、状态、当前阶段、发起人、审批人、目标仓库、目标分支、风险等级、测试结果、MR 链接、Token 成本、最近失败原因。
- 将 `bitable_record_id` 固化到 Feishu binding，保证重复同步不会产生多行。

当前状态：Go Feishu Client 已具备 Pipeline 专用同步能力，PipelineRun 生命周期和阶段推进时已有 upsert 多维表格台账记录的基础。

## Phase 3: Pipeline Core Alignment

目标：Pipeline Core 承接机器人入口后的完整研发执行链路。

开发重点：

- 默认模板对齐：RequirementAnalysis -> FeishuContextBuild -> SolutionDesign -> DesignApprovalCheckpoint -> CodeGeneration -> TestGeneration -> TestExecution -> AIReview -> HumanReviewCheckpoint -> MRCreation -> FeishuDeliverySummary。
- 每个阶段都有 handler、prompt contract、artifact type、统计名称和 JSON 输出契约。
- FeishuContextBuild 读取飞书文档正文、多维表格历史记录和群聊元信息。
- Reject 后携带原因回退到 SolutionDesign、CodeGeneration 或 TestGeneration，并重置后续阶段。

当前状态：默认模板、飞书上下文构建、测试执行阶段、Artifact、Checkpoint、AgentRun、阶段统计和生命周期测试已完成基础对齐。

## Phase 4: Feishu Approval And Rework

目标：Human-in-the-Loop 真正在飞书消息卡片完成。

开发重点：

- 方案审批卡片展示需求摘要、影响范围、技术方案、风险、测试计划和回滚方案。
- 代码评审确认卡片展示 Diff 摘要、测试结果、AI Review 问题和 MR 草稿。
- Approve/Reject 回调调用统一 Checkpoint API。
- Reject 原因进入后续重做阶段输入，并同步到多维表格最近失败原因。

当前状态：已提供 `GET /api/checkpoints/{checkpointID}/detail` 统一审批上下文接口；Web 已增加 `/approvals/{runId}` 落地页；检查点已可发送飞书交互卡片，并通过回调触发 approve/reject。

## Phase 5: Delivery Execution

目标：人工确认后从变更计划走到真实 MR/PR，并回填飞书。

开发重点：

- 校验并应用已审批 changeSet。
- 创建工作分支和 commit。
- 通过 Git provider 创建 PR/MR。
- 持久化 MR/PR URL、提交 SHA、测试结果和交付摘要。
- 生成飞书交付总结文档，并回写多维表格和群消息。

当前状态：交付阶段已有 GitDelivery 草稿能力，真实写文件、提交和远程 MR/PR 创建仍是主要缺口。

## Phase 6: Demo Hardening

目标：形成可现场演示的自举闭环。

开发重点：

- 准备以自身仓库为目标的固定需求。
- 覆盖一次飞书机器人创建、一次多维表格记录更新、一次方案 Reject 重做、一次测试执行、一次代码评审确认和一次 MR/PR 交付。
- 准备演示脚本、测试报告、截图和网络异常降级方案。

## Verification

- Go: `go test ./internal/pipeline ./internal/service`
- Frontend: `pnpm --dir apps/web build`
- Feishu smoke: 机器人消息创建 Run，检查台账行、审批卡片、Reject/Approve 回调和交付总结。
- Demo: 创建 self target Pipeline Run，审批两个检查点，确认最终 GitDelivery 或 MR/PR URL 回填到多维表格。
