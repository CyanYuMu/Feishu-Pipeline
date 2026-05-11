package pipeline_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/pipeline"
	"feishu-pipeline/apps/api-go/internal/repo"
)

func TestEngineRunBuildsStructuredStageInput(t *testing.T) {
	repository := newTestRepository(t)
	run := seedPipelineRun(t, repository)
	checkpoints, err := repository.ListCheckpointsByPipelineRunID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list checkpoints: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}

	engine := pipeline.NewEngine(repository, pipeline.NewSequentialExecutor(), nil)
	if err := engine.Run(context.Background(), run.ID); err != nil {
		t.Fatalf("run engine: %v", err)
	}

	updatedRun, err := repository.GetPipelineRunByID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updatedRun.Status != model.PipelineRunWaitingApproval {
		t.Fatalf("expected waiting approval, got %s", updatedRun.Status)
	}
	if updatedRun.CurrentStageKey != pipeline.StageCheckpointDesign {
		t.Fatalf("expected current stage %s, got %s", pipeline.StageCheckpointDesign, updatedRun.CurrentStageKey)
	}

	designStage, err := repository.GetStageRunByKey(context.Background(), run.ID, pipeline.StageSolutionDesign)
	if err != nil {
		t.Fatalf("get design stage: %v", err)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(designStage.InputJSON), &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	latestArtifacts, ok := input["latestArtifacts"].(map[string]any)
	if !ok {
		t.Fatalf("latestArtifacts missing")
	}
	requirementArtifact, ok := latestArtifacts[string(model.ArtifactStructuredRequirement)].(map[string]any)
	if !ok {
		t.Fatalf("structured requirement artifact missing")
	}
	if requirementArtifact["title"] != "结构化需求" {
		t.Fatalf("unexpected requirement artifact title: %v", requirementArtifact["title"])
	}
}

func TestEngineRunCompletesAfterCheckpointApprovals(t *testing.T) {
	repository := newTestRepository(t)
	run := seedFullPipelineRun(t, repository)
	engine := pipeline.NewEngine(repository, pipeline.NewSequentialExecutor(), nil)
	ctx := context.Background()

	if err := engine.Run(ctx, run.ID); err != nil {
		t.Fatalf("run to design checkpoint: %v", err)
	}
	designCheckpoint, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageCheckpointDesign)
	if err != nil {
		t.Fatalf("get design checkpoint stage: %v", err)
	}
	if designCheckpoint.Status != model.StageRunWaitingApproval {
		t.Fatalf("expected design checkpoint waiting, got %s", designCheckpoint.Status)
	}
	if err := repository.UpdateStageRunStatus(ctx, designCheckpoint.ID, model.StageRunSucceeded); err != nil {
		t.Fatalf("approve design stage: %v", err)
	}
	if err := repository.UpdatePipelineRunStatus(ctx, run.ID, model.PipelineRunQueued); err != nil {
		t.Fatalf("queue after design approval: %v", err)
	}

	if err := engine.Run(ctx, run.ID); err != nil {
		t.Fatalf("run to review checkpoint: %v", err)
	}
	reviewCheckpoint, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageCheckpointReview)
	if err != nil {
		t.Fatalf("get review checkpoint stage: %v", err)
	}
	if reviewCheckpoint.Status != model.StageRunWaitingApproval {
		t.Fatalf("expected review checkpoint waiting, got %s", reviewCheckpoint.Status)
	}
	if err := repository.UpdateStageRunStatus(ctx, reviewCheckpoint.ID, model.StageRunSucceeded); err != nil {
		t.Fatalf("approve review stage: %v", err)
	}
	if err := repository.UpdatePipelineRunStatus(ctx, run.ID, model.PipelineRunQueued); err != nil {
		t.Fatalf("queue after review approval: %v", err)
	}

	if err := engine.Run(ctx, run.ID); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	updatedRun, err := repository.GetPipelineRunByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("get completed run: %v", err)
	}
	if updatedRun.Status != model.PipelineRunCompleted {
		t.Fatalf("expected completed run, got %s", updatedRun.Status)
	}
	artifacts, err := repository.ListArtifactsByPipelineRunID(ctx, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if !hasArtifactType(artifacts, model.ArtifactDeliverySummary) {
		t.Fatalf("delivery summary artifact missing")
	}
	deliveries, err := repository.ListGitDeliveriesByPipelineRunID(ctx, run.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].Status != model.GitDeliveryReady {
		t.Fatalf("expected ready delivery, got %s", deliveries[0].Status)
	}
	if deliveries[0].PRMRTitle == "" || deliveries[0].PRMRBody == "" || deliveries[0].SummaryMarkdown == "" {
		t.Fatalf("expected delivery pr draft fields: %+v", deliveries[0])
	}
	agentRuns, err := repository.ListAgentRunsByPipelineRunID(ctx, run.ID)
	if err != nil {
		t.Fatalf("list agent runs: %v", err)
	}
	expectedAgentRuns := executableStageCount()
	if len(agentRuns) != expectedAgentRuns {
		t.Fatalf("expected %d executable agent runs, got %d", expectedAgentRuns, len(agentRuns))
	}
}

func TestEngineStopsAfterInFlightStageWhenRunIsPaused(t *testing.T) {
	repository := newTestRepository(t)
	run := seedFullPipelineRun(t, repository)
	ctx := context.Background()
	engine := pipeline.NewEngine(repository, lifecycleExecutor{
		onStage: func(ctx context.Context, stage model.StageRun) error {
			if stage.StageKey == pipeline.StageRequirementAnalysis {
				return repository.UpdatePipelineRunStatus(ctx, run.ID, model.PipelineRunPaused)
			}
			return nil
		},
	}, nil)

	if err := engine.Run(ctx, run.ID); err != nil {
		t.Fatalf("run engine: %v", err)
	}
	updatedRun, err := repository.GetPipelineRunByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updatedRun.Status != model.PipelineRunPaused {
		t.Fatalf("expected run to remain paused, got %s", updatedRun.Status)
	}
	nextStage, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageSolutionDesign)
	if err != nil {
		t.Fatalf("get next stage: %v", err)
	}
	if nextStage.Status != model.StageRunPending {
		t.Fatalf("expected next stage to stay pending, got %s", nextStage.Status)
	}
}

func TestEngineStopsAfterInFlightStageWhenRunIsTerminated(t *testing.T) {
	repository := newTestRepository(t)
	run := seedFullPipelineRun(t, repository)
	ctx := context.Background()
	engine := pipeline.NewEngine(repository, lifecycleExecutor{
		onStage: func(ctx context.Context, stage model.StageRun) error {
			if stage.StageKey == pipeline.StageSolutionDesign {
				return repository.UpdatePipelineRunStatus(ctx, run.ID, model.PipelineRunTerminated)
			}
			return nil
		},
	}, nil)

	if err := engine.Run(ctx, run.ID); err != nil {
		t.Fatalf("run engine: %v", err)
	}
	updatedRun, err := repository.GetPipelineRunByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updatedRun.Status != model.PipelineRunTerminated {
		t.Fatalf("expected run to remain terminated, got %s", updatedRun.Status)
	}
	checkpointStage, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageCheckpointDesign)
	if err != nil {
		t.Fatalf("get checkpoint stage: %v", err)
	}
	if checkpointStage.Status != model.StageRunPending {
		t.Fatalf("expected checkpoint stage to stay pending, got %s", checkpointStage.Status)
	}
}

func newTestRepository(t *testing.T) *repo.Repository {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "pipeline-test.db")
	repository, err := repo.NewSQLiteRepository(databasePath)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() {
		_ = repository.Close()
	})
	return repository
}

func seedPipelineRun(t *testing.T, repository *repo.Repository) model.PipelineRun {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	run := model.PipelineRun{
		ID:              "run_test_engine",
		TemplateID:      pipeline.DefaultTemplateID,
		Title:           "Pipeline engine test",
		RequirementText: "需要补齐 pipeline 阶段输入输出",
		TargetRepo:      "self",
		TargetBranch:    "main",
		WorkBranch:      "devflow/test-engine",
		Status:          model.PipelineRunQueued,
		CurrentStageKey: pipeline.StageRequirementAnalysis,
		CreatedBy:       "tester",
		BaseModel:       model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stages := []model.StageRun{
		{ID: "stage_req", PipelineRunID: run.ID, StageKey: pipeline.StageRequirementAnalysis, StageType: model.StageTypeAnalysis, Status: model.StageRunQueued, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_design", PipelineRunID: run.ID, StageKey: pipeline.StageSolutionDesign, StageType: model.StageTypeDesign, Status: model.StageRunPending, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_checkpoint", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointDesign, StageType: model.StageTypeCheckpoint, Status: model.StageRunPending, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := repository.CreateStageRuns(ctx, stages); err != nil {
		t.Fatalf("create stages: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "cp_design", PipelineRunID: run.ID, StageRunID: "stage_checkpoint", CheckpointType: model.CheckpointDesignReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	return run
}

func seedFullPipelineRun(t *testing.T, repository *repo.Repository) model.PipelineRun {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	run := model.PipelineRun{
		ID:              "run_test_full_engine",
		TemplateID:      pipeline.DefaultTemplateID,
		Title:           "Full pipeline engine test",
		RequirementText: "根据赛题要求补齐完整 pipeline 可演示闭环",
		TargetRepo:      "self",
		TargetBranch:    "main",
		WorkBranch:      "devflow/test-full-engine",
		Status:          model.PipelineRunQueued,
		CurrentStageKey: pipeline.StageRequirementAnalysis,
		CreatedBy:       "tester",
		BaseModel:       model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create full run: %v", err)
	}
	stageRuns := make([]model.StageRun, 0, len(pipeline.DefaultStageDefinitions))
	for idx, definition := range pipeline.DefaultStageDefinitions {
		status := model.StageRunPending
		if idx == 0 {
			status = model.StageRunQueued
		}
		stageRuns = append(stageRuns, model.StageRun{ID: "full_" + definition.Key, PipelineRunID: run.ID, StageKey: definition.Key, StageType: definition.Type, Status: status, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}})
	}
	if err := repository.CreateStageRuns(ctx, stageRuns); err != nil {
		t.Fatalf("create full stages: %v", err)
	}
	for _, stageRun := range stageRuns {
		if stageRun.StageKey == pipeline.StageCheckpointDesign || stageRun.StageKey == pipeline.StageCheckpointReview {
			checkpointType := model.CheckpointCodeReview
			if stageRun.StageKey == pipeline.StageCheckpointDesign {
				checkpointType = model.CheckpointDesignReview
			}
			checkpoint := model.Checkpoint{ID: "cp_" + stageRun.StageKey, PipelineRunID: run.ID, StageRunID: stageRun.ID, CheckpointType: checkpointType, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
			if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
				t.Fatalf("create full checkpoint: %v", err)
			}
		}
	}
	return run
}

func hasArtifactType(items []model.Artifact, artifactType model.ArtifactType) bool {
	for _, item := range items {
		if item.ArtifactType == artifactType {
			return true
		}
	}
	return false
}

func executableStageCount() int {
	count := 0
	for _, definition := range pipeline.DefaultStageDefinitions {
		if definition.Type != model.StageTypeCheckpoint {
			count++
		}
	}
	return count
}

type lifecycleExecutor struct {
	onStage func(context.Context, model.StageRun) error
}

func (e lifecycleExecutor) Execute(ctx context.Context, stageContext pipeline.StageContext) (pipeline.StageExecutionResult, error) {
	if e.onStage != nil {
		if err := e.onStage(ctx, stageContext.Stage); err != nil {
			return pipeline.StageExecutionResult{}, err
		}
	}
	artifactType := model.ArtifactStructuredRequirement
	switch stageContext.Stage.StageKey {
	case pipeline.StageFeishuContextBuild:
		artifactType = model.ArtifactFeishuContext
	case pipeline.StageSolutionDesign:
		artifactType = model.ArtifactSolutionDesign
	case pipeline.StageCodeGeneration:
		artifactType = model.ArtifactCodeDiff
	case pipeline.StageTestGeneration:
		artifactType = model.ArtifactTestReport
	case pipeline.StageTestExecution:
		artifactType = model.ArtifactTestExecution
	case pipeline.StageCodeReview:
		artifactType = model.ArtifactReviewReport
	case pipeline.StageDelivery:
		artifactType = model.ArtifactDeliverySummary
	}
	outputJSON := `{"summary":"lifecycle test"}`
	return pipeline.StageExecutionResult{
		ArtifactType: artifactType,
		Title:        "Lifecycle test",
		ContentText:  "Lifecycle test",
		ContentJSON:  outputJSON,
		OutputJSON:   outputJSON,
	}, nil
}
