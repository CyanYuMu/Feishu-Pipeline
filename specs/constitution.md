# Feishu DevFlow Constitution

## 1. Spec First

每个可演示能力先沉淀规格，再进入实现。规格包含用户目标、验收标准、阶段契约、风险和验证方式；代码变更必须能回溯到规格与任务。

## 2. Pipeline Is The Product

本项目优先做深「需求输入 -> 方案设计 -> 编码 -> 测试 -> 评审 -> 交付」闭环。飞书机器人、文档、多维表格、Web 控制台都服务于同一条可追踪 Pipeline。

## 3. Agent Contracts Over Freeform Chat

Agent 输出必须是结构化 JSON，字段契约由阶段定义管理。Agent 可以失败、降级或回退到 deterministic handler，但不能跳过契约、伪造外部状态或直接执行高风险操作。

## 4. Human Gates For Irreversible Steps

方案审批和评审确认是强制检查点。Reject 必须携带原因回写上下文，并重置后续阶段；Approve 才能继续进入下一阶段。

## 5. Observable Delivery

每次运行必须留下 StageRun、AgentRun、Artifact、Checkpoint 和 GitDelivery 草稿。演示优先证明状态、产物、成本、测试和交付记录可审计。

## 6. Conservative Execution

MVP 中代码生成先产出受控变更计划；测试执行走后端白名单；Git 提交和 PR 创建必须在人工确认后触发。
