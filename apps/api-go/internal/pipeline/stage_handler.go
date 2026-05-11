package pipeline

import (
	"context"

	"feishu-pipeline/apps/api-go/internal/model"
)

type StageHandler interface {
	Execute(context.Context, StageContext) (StageExecutionResult, error)
}

type SequentialExecutor struct {
	handlers    map[string]StageHandler
	agentRunner *AgentRunner
}

type SequentialExecutorOption func(*SequentialExecutor)

func WithAgentRunner(agentRunner *AgentRunner) SequentialExecutorOption {
	return func(executor *SequentialExecutor) {
		executor.agentRunner = agentRunner
	}
}

func NewSequentialExecutor(options ...SequentialExecutorOption) *SequentialExecutor {
	executor := &SequentialExecutor{handlers: map[string]StageHandler{
		StageRequirementAnalysis: RequirementAnalysisHandler{},
		StageFeishuContextBuild:  FeishuContextBuildHandler{},
		StageSolutionDesign:      SolutionDesignHandler{},
		StageCodeGeneration:      NewCodeGenerationHandler(nil),
		StageTestGeneration:      TestGenerationHandler{},
		StageTestExecution:       TestExecutionHandler{},
		StageCodeReview:          CodeReviewHandler{},
		StageDelivery:            DeliveryHandler{},
	}}
	for _, option := range options {
		option(executor)
	}
	return executor
}

func (e *SequentialExecutor) Execute(ctx context.Context, stageContext StageContext) (StageExecutionResult, error) {
	handler, ok := e.handlers[stageContext.Stage.StageKey]
	if !ok {
		return DefaultStageHandler{}.Execute(ctx, stageContext)
	}
	if e.agentRunner != nil {
		return e.agentRunner.Execute(ctx, stageContext, handler)
	}
	return handler.Execute(ctx, stageContext)
}

type DefaultStageHandler struct{}

func (DefaultStageHandler) Execute(_ context.Context, ctx StageContext) (StageExecutionResult, error) {
	payload := baseStagePayload(ctx)
	payload[SchemaFieldSummary] = "阶段执行完成。"
	return newStageResult(model.ArtifactDeliverySummary, ctx.Stage.StageKey, payload, "阶段执行完成。"), nil
}
