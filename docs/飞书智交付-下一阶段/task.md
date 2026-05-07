# 飞书智交付 · 下一阶段任务索引（task）

本文件用于兼容后续按 `task.md` 查找任务的开发上下文；详细任务仍维护在 `tasks.md`。

## 当前优先级

- [x] 对话确认需求后自动进入 Pipeline
- [x] 结构化需求文档通过飞书发送给会话归属用户
- [x] 会话页展示绑定 Pipeline 的状态与入口
- [x] 修复 assistant 确认回复未触发发布的问题
- [x] 会话所有者本人可发布自己的需求
- [x] 会话页短轮询展示异步创建的 Pipeline
- [x] 新增 Bug Fix / Feature / Refactor 三类预置 Pipeline 模板
- [x] Workflows 新建 Pipeline 支持选择模板
- [x] Workflows 在存在代码变更上下文时展示 Diff 对话入口，不再只依赖审批 checkpoint
- [x] Workflows 增加横向执行轨道，强化流水线阶段可视化
- [x] Diff 对话优先展示结构化 `code_diff` 产物，并保留 AgentRun diff 兜底
- [x] Diff 对话复用 Workflows timeline 并缓存 `code-diff`，避免反复请求慢 `/current` 接口
- [x] GitHub 绑定改为读取后端 OAuth 配置，移除前端硬编码 clientId
- [x] Reject 时携带用户输入的回退原因，并在 Workflows 展示“回退重做”上下文
- [x] execute-changes 使用绑定 GitHub token 完成远程 commit/push，并自动创建 PR
- [x] GitDelivery 记录 commitSha 与 prmrUrl，前端执行变更后提示 commit/PR 结果
- [ ] 真实飞书租户 smoke：docx 创建与消息送达
- [ ] 真实 GitHub OAuth App smoke：账号绑定、仓库列表、分支列表
- [ ] 真实演示 smoke：Pipeline 自动流转到首个 checkpoint

## 参考文件

- `spec.md`：下一阶段范围和当前基线
- `tasks.md`：完整任务拆解
- `checklist.md`：验证清单
