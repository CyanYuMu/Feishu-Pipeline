package pipeline

import "feishu-pipeline/apps/api-go/internal/model"

type StageDefinition struct {
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Type         model.StageType `json:"type"`
	Order        int             `json:"order"`
	IsCheckpoint bool            `json:"isCheckpoint"`
}

const (
	StageRequirementAnalysis = "requirement_analysis"
	StageFeishuContextBuild  = "feishu_context_build"
	StageSolutionDesign      = "solution_design"
	StageCheckpointDesign    = "checkpoint_design"
	StageCodeGeneration      = "code_generation"
	StageTestGeneration      = "test_generation"
	StageTestExecution       = "test_execution"
	StageCodeReview          = "code_review"
	StageCheckpointReview    = "checkpoint_review"
	StageDelivery            = "delivery"
)

var DefaultStageDefinitions = []StageDefinition{
	{Key: StageRequirementAnalysis, Name: "需求分析", Type: model.StageTypeAnalysis, Order: 1},
	{Key: StageFeishuContextBuild, Name: "飞书上下文构建", Type: model.StageTypeAnalysis, Order: 2},
	{Key: StageSolutionDesign, Name: "方案设计", Type: model.StageTypeDesign, Order: 3},
	{Key: StageCheckpointDesign, Name: "方案审批", Type: model.StageTypeCheckpoint, Order: 4, IsCheckpoint: true},
	{Key: StageCodeGeneration, Name: "代码生成", Type: model.StageTypeCodegen, Order: 5},
	{Key: StageTestGeneration, Name: "测试生成", Type: model.StageTypeTest, Order: 6},
	{Key: StageTestExecution, Name: "测试执行", Type: model.StageTypeTest, Order: 7},
	{Key: StageCodeReview, Name: "代码评审", Type: model.StageTypeReview, Order: 8},
	{Key: StageCheckpointReview, Name: "评审确认", Type: model.StageTypeCheckpoint, Order: 9, IsCheckpoint: true},
	{Key: StageDelivery, Name: "交付集成", Type: model.StageTypeDelivery, Order: 10},
}

func IsRunnableStageStatus(status model.StageRunStatus) bool {
	switch status {
	case model.StageRunPending, model.StageRunQueued, model.StageRunFailed:
		return true
	default:
		return false
	}
}

func NextStageKey(stageKey string) string {
	for idx, stage := range DefaultStageDefinitions {
		if stage.Key == stageKey && idx+1 < len(DefaultStageDefinitions) {
			return DefaultStageDefinitions[idx+1].Key
		}
	}
	return ""
}
