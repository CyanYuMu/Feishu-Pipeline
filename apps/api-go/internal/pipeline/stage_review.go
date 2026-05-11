package pipeline

import (
	"context"

	"feishu-pipeline/apps/api-go/internal/model"
)

type CodeReviewHandler struct{}

func (CodeReviewHandler) Execute(_ context.Context, ctx StageContext) (StageExecutionResult, error) {
	changeSet := nestedMapSlice(ctx.Input, "latestArtifacts", model.ArtifactCodeDiff, "changeSet")
	testStatus := nestedString(ctx.Input, "latestArtifacts", model.ArtifactTestExecution, SchemaFieldStatus)
	if testStatus == "" {
		testStatus = nestedString(ctx.Input, "latestArtifacts", model.ArtifactTestReport, SchemaFieldStatus)
	}
	issues := []map[string]any{{"severity": "medium", "filePath": "apps/api-go/internal/pipeline/executor.go", "message": "当前代码生成阶段仍为计划产物，尚未应用真实 patch。", "suggestion": "下一阶段接入受控文件执行器前继续保留人工确认。"}}
	payload := baseStagePayload(ctx)
	payload[SchemaFieldSummary] = "从正确性、安全性、可维护性维度完成 AI 预审。"
	payload[SchemaFieldConclusion] = "needs_fix"
	payload[SchemaFieldIssues] = issues
	payload[SchemaFieldSecurityNotes] = []string{"本阶段未执行写文件、提交、推送等高风险操作", "测试命令限制在 Go 后端 pipeline 相关包"}
	payload[SchemaFieldMaintainabilityNotes] = []string{"阶段执行逻辑已按处理器拆分，便于后续替换为真实 Agent Provider"}
	payload[SchemaFieldTestCoverageNotes] = []string{"当前测试范围聚焦 pipeline 和 service，后续需要扩展前端与集成测试"}
	payload["inputs"] = []string{"代码变更计划", "测试报告", "技术方案"}
	payload["outputs"] = []string{"评审结论", "问题列表", "安全说明", "维护性说明"}
	payload["risks"] = []string{"真实写文件与 Git 交付尚未启用，需要人工确认后进入下一阶段"}
	payload["nextActions"] = []string{"等待评审确认 checkpoint", "确认通过后生成交付摘要"}
	payload["changeSet"] = changeSet
	payload["testStatus"] = testStatus
	payload["checkpoints"] = checkpointSummaries(ctx.Checkpoints)
	return newStageResult(model.ArtifactReviewReport, "评审报告", payload, "评审结论：待人工确认\n问题列表：\n- 当前代码生成阶段仍为计划产物，尚未应用真实 patch。\n安全说明：未执行写文件、提交、推送等高风险操作。"), nil
}
