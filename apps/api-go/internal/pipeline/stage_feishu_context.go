package pipeline

import (
	"context"
	"fmt"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
)

type FeishuContextBuildHandler struct{}

func (FeishuContextBuildHandler) Execute(_ context.Context, ctx StageContext) (StageExecutionResult, error) {
	requirement := buildRunRequirement(ctx.Run)
	docURLs, _ := requirement["selectedDocUrls"].([]string)
	contextSources := []string{"requirement_text", "pipeline_run"}
	if len(docURLs) > 0 {
		contextSources = append(contextSources, "selected_feishu_docs")
	}

	payload := baseStagePayload(ctx)
	payload[SchemaFieldSummary] = "汇总飞书入口、文档链接和研发台账字段，形成方案设计可消费的组织上下文。"
	payload[SchemaFieldContextSources] = contextSources
	payload[SchemaFieldFeishuDocs] = docURLs
	payload[SchemaFieldBitableFields] = []string{"需求标题", "来源", "状态", "当前阶段", "发起人", "审批人", "目标仓库", "目标分支", "风险等级", "测试结果", "MR 链接"}
	payload[SchemaFieldConversationSignals] = []string{"需求来自飞书协作入口", "后续审批需要通过方案审批和评审确认两个检查点", "所有阶段产物需要沉淀为 Artifact 供飞书文档和控制台展示"}
	payload["feishuDocContents"] = ctx.Input["feishuDocContents"]
	payload["inputs"] = []string{"结构化需求", "飞书文档链接", "Pipeline Run 元数据"}
	payload["outputs"] = []string{"飞书上下文摘要", "台账字段映射", "审批与通知线索"}
	payload["risks"] = []string{"飞书文档读取依赖应用具备 docx/wiki 读取权限；读取失败时保留链接并继续推进"}
	payload["nextActions"] = []string{"进入方案设计阶段", "将 contextSources 作为技术方案输入"}

	return newStageResult(model.ArtifactFeishuContext, "飞书上下文", payload, formatFeishuContext(contextSources, docURLs)), nil
}

func formatFeishuContext(contextSources []string, docURLs []string) string {
	docLine := "暂无"
	if len(docURLs) > 0 {
		docLine = strings.Join(docURLs, "\n- ")
	}
	return fmt.Sprintf("上下文来源：%s\n飞书文档：\n- %s", strings.Join(contextSources, ", "), docLine)
}
