package pipeline

import (
	"context"
	"fmt"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
)

type DeliveryHandler struct{}

func (DeliveryHandler) Execute(_ context.Context, ctx StageContext) (StageExecutionResult, error) {
	artifacts := artifactTitles(ctx.Artifacts)
	changeSet := nestedMapSlice(ctx.Input, "latestArtifacts", model.ArtifactCodeDiff, "changeSet")
	changedFiles := nestedStringSlice(ctx.Input, "latestArtifacts", model.ArtifactCodeDiff, SchemaFieldChangedFiles)
	payload := baseStagePayload(ctx)
	payload[SchemaFieldSummary] = "Pipeline 已完成需求、方案、变更计划、测试、评审和交付摘要产物。"
	payload[SchemaFieldChangedFiles] = changedFiles
	payload[SchemaFieldValidation] = []string{"已生成结构化需求", "已生成飞书上下文", "已生成技术方案", "已生成代码变更计划", "已生成测试计划", "已生成测试执行结果", "已生成评审报告"}
	payload[SchemaFieldPRTitle] = ctx.Run.Title
	payload[SchemaFieldPRBody] = "包含 Pipeline 阶段产物、测试报告、评审报告和交付摘要。"
	payload[SchemaFieldManualReleaseNotes] = []string{"审批确认后可执行 execute-changes，提交到 GitHub 工作分支并创建远程 PR"}
	payload["inputs"] = []string{"评审报告", "测试报告", "代码变更计划"}
	payload["outputs"] = []string{"交付摘要", "GitHub 提交参数", "演示检查清单"}
	payload["risks"] = []string{"远程提交依赖用户已绑定 GitHub 且目标仓库有写权限"}
	payload["nextActions"] = []string{"人工确认变更", "执行 execute-changes 完成 commit、push 和 PR 创建"}
	payload["deliverySummary"] = payload[SchemaFieldSummary]
	payload["workBranch"] = ctx.Run.WorkBranch
	payload["artifacts"] = artifacts
	payload["changeSet"] = changeSet
	payload["manualPRSuggestion"] = map[string]any{"title": ctx.Run.Title, "branch": ctx.Run.WorkBranch, "includeArtifacts": artifacts}
	return newStageResult(model.ArtifactDeliverySummary, "交付摘要", payload, fmt.Sprintf("交付分支：%s\n已生成产物：%s\n下一步：人工确认后执行 execute-changes，提交到 GitHub 并创建 PR。", ctx.Run.WorkBranch, strings.Join(artifacts, ", "))), nil
}
