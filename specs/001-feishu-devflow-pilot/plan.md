# Plan: 分阶段开发路线

## Phase 0: Spec Baseline

建立 constitution、feature spec、implementation plan 和 task list。所有后续改动先更新任务状态，再实现和验证。

## Phase 1: Pipeline Core Alignment

将默认模板对齐产品设计主线，补齐飞书上下文构建和测试执行阶段，确保每个阶段都有 handler、prompt contract、artifact type 和统计名称。

## Phase 2: Approval And Rework

完善 checkpoint 详情 API/UI，展示待审批产物、Approve/Reject 操作、拒绝原因和后续阶段重置效果。

当前状态：已提供 `GET /api/checkpoints/{checkpointID}/detail` 作为统一审批上下文接口，返回主审批产物、按类型归档的最新产物、近期相关产物和 AgentRun；Web 已增加 `/approvals/{runId}` 落地页，并让审批聊天面板优先消费该聚合响应。Reject 后上游重跑阶段会携带 `rejectReason`，工作台阶段时间线可直接展示回退原因。

## Phase 3: Feishu Native Entry

把 api-ts 的文档、多维表格、消息能力收敛为 Feishu Connector，并让 Go Core 消费文档正文、台账字段和卡片回调。

当前状态：Go Feishu Client 已补 Pipeline 专用同步能力。FeishuContextBuild 会读取已选择的飞书文档 raw content 并写入阶段输入；PipelineRun 创建、启动、暂停、恢复、终止、审批和阶段推进时会 upsert 多维表格台账记录；每个检查点会发送交互卡片，卡片 intent 通过 WebSocket 回调接回 `ApproveCheckpoint` / `RejectCheckpoint`。阶段产物会以飞书文档形式沉淀，并把 mock/真实 URL 写回 Artifact。

## Phase 4: Delivery Execution

在人工确认后应用受控 changeSet，创建工作分支、提交 commit、生成 PR/MR 草稿，并把链接回填到飞书和 Web 控制台。

## Phase 5: Demo Hardening

准备以自身仓库为目标的端到端演示脚本，覆盖一次 Reject 重做、一次测试执行、一次交付摘要生成。

## Verification

- Go: `go test ./internal/pipeline ./internal/service`
- Frontend: `pnpm --dir apps/web build`
- Demo: 创建 self target Pipeline Run，审批两次检查点，确认最终 GitDelivery 草稿。
