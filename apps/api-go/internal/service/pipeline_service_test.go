package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/pipeline"
	"feishu-pipeline/apps/api-go/internal/repo"
)

func TestGetPipelineRunTimelineAggregatesWaitingApprovalState(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{ID: "run_timeline", TemplateID: pipeline.DefaultTemplateID, Title: "Timeline", RequirementText: "展示 pipeline 工作台", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/timeline", Status: model.PipelineRunWaitingApproval, CurrentStageKey: pipeline.StageCheckpointDesign, CreatedBy: "tester", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stages := []model.StageRun{
		{ID: "stage_timeline_req", PipelineRunID: run.ID, StageKey: pipeline.StageRequirementAnalysis, StageType: model.StageTypeAnalysis, Status: model.StageRunSucceeded, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_timeline_checkpoint", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointDesign, StageType: model.StageTypeCheckpoint, Status: model.StageRunWaitingApproval, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := repository.CreateStageRuns(ctx, stages); err != nil {
		t.Fatalf("create stages: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "cp_timeline", PipelineRunID: run.ID, StageRunID: "stage_timeline_checkpoint", CheckpointType: model.CheckpointDesignReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	artifact := model.Artifact{ID: "artifact_timeline", PipelineRunID: run.ID, StageRunID: "stage_timeline_req", ArtifactType: model.ArtifactStructuredRequirement, Title: "结构化需求", MetaJSON: "{}", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateArtifact(ctx, &artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	timeline, err := service.GetPipelineRunTimeline(ctx, run.ID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if timeline.Summary.TotalStages != 2 || timeline.Summary.CompletedStages != 1 || !timeline.Summary.WaitingApproval {
		t.Fatalf("unexpected summary: %+v", timeline.Summary)
	}
	if timeline.Current == nil || timeline.Current.Stage == nil || timeline.Current.Stage.ID != "stage_timeline_checkpoint" {
		t.Fatalf("unexpected current stage: %+v", timeline.Current)
	}
	if timeline.Current.Checkpoint == nil || timeline.Current.Checkpoint.ID != checkpoint.ID {
		t.Fatalf("expected current checkpoint")
	}
}

func TestGetPipelineRunTimelineAggregatesCreatedRun(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	detail, err := service.CreatePipelineRun(ctx, CreatePipelineRunInput{Title: "Timeline test", RequirementText: "需要展示 pipeline 工作台聚合数据", TargetRepo: "self", TargetBranch: "main", CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create pipeline run: %v", err)
	}
	timeline, err := service.GetPipelineRunTimeline(ctx, detail.Run.ID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if timeline.Summary.TotalStages != len(pipeline.DefaultStageDefinitions) {
		t.Fatalf("expected %d stages, got %d", len(pipeline.DefaultStageDefinitions), timeline.Summary.TotalStages)
	}
	if timeline.Current == nil || timeline.Current.Stage == nil {
		t.Fatalf("expected current stage")
	}
	if timeline.Current.Stage.StageKey != pipeline.StageRequirementAnalysis {
		t.Fatalf("expected current stage %s, got %s", pipeline.StageRequirementAnalysis, timeline.Current.Stage.StageKey)
	}
	if len(timeline.Artifacts) != 1 || timeline.Summary.LatestArtifactID == "" {
		t.Fatalf("expected initial artifact in timeline")
	}
	current, err := service.GetPipelineRunCurrent(ctx, detail.Run.ID)
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if current.Stage == nil || current.Stage.ID != timeline.Current.Stage.ID {
		t.Fatalf("current endpoint mismatch")
	}
	if current.NextAction != "start_run" {
		t.Fatalf("expected start_run next action, got %s", current.NextAction)
	}
}

func TestGetPipelineRunTimelineAggregatesDeliveryAndNextAction(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	startedAt := now.Add(-2 * time.Minute)
	finishedAt := now
	run := model.PipelineRun{ID: "run_timeline_delivery", TemplateID: pipeline.DefaultTemplateID, Title: "Delivery", RequirementText: "需要展示交付记录", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/delivery", Status: model.PipelineRunCompleted, CurrentStageKey: pipeline.StageDelivery, CreatedBy: "tester", StartedAt: &startedAt, FinishedAt: &finishedAt, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stage := model.StageRun{ID: "stage_timeline_delivery", PipelineRunID: run.ID, StageKey: pipeline.StageDelivery, StageType: model.StageTypeDelivery, Status: model.StageRunSucceeded, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateStageRuns(ctx, []model.StageRun{stage}); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	delivery := model.GitDelivery{ID: "delivery_timeline", PipelineRunID: run.ID, Provider: "local", Repo: "self", BaseBranch: "main", HeadBranch: "devflow/delivery", PRMRTitle: "Delivery", PRMRBody: "Body", SummaryMarkdown: "Summary", Status: model.GitDeliveryReady, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateGitDelivery(ctx, &delivery); err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	timeline, err := service.GetPipelineRunTimeline(ctx, run.ID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if timeline.Summary.LatestDeliveryID != delivery.ID {
		t.Fatalf("expected latest delivery id, got %s", timeline.Summary.LatestDeliveryID)
	}
	if timeline.Summary.DurationMS <= 0 {
		t.Fatalf("expected duration, got %d", timeline.Summary.DurationMS)
	}
	if timeline.Current == nil || timeline.Current.Delivery == nil || timeline.Current.Delivery.ID != delivery.ID {
		t.Fatalf("expected current delivery")
	}
	if timeline.Current.NextAction != "review_delivery" {
		t.Fatalf("expected review_delivery next action, got %s", timeline.Current.NextAction)
	}
	deliveries, err := service.ListGitDeliveries(ctx, run.ID)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected one delivery, got %d", len(deliveries))
	}
	item, err := service.GetGitDelivery(ctx, delivery.ID)
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if item.PRMRTitle != delivery.PRMRTitle {
		t.Fatalf("unexpected delivery title: %s", item.PRMRTitle)
	}
}

func TestGetPipelineRunTimelineAggregatesWorkspaceData(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	detail, err := service.CreatePipelineRun(ctx, CreatePipelineRunInput{Title: "Timeline test", RequirementText: "需要展示流水线工作台", TargetRepo: "self", TargetBranch: "main", CreatedBy: "tester"})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	timeline, err := service.GetPipelineRunTimeline(ctx, detail.Run.ID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if timeline.Run.ID != detail.Run.ID {
		t.Fatalf("unexpected run id: %s", timeline.Run.ID)
	}
	if timeline.Summary.TotalStages != len(pipeline.DefaultStageDefinitions) {
		t.Fatalf("unexpected total stages: %d", timeline.Summary.TotalStages)
	}
	if timeline.Current == nil || timeline.Current.Stage == nil {
		t.Fatalf("expected current stage")
	}
	if timeline.Current.Stage.StageKey != pipeline.StageRequirementAnalysis {
		t.Fatalf("unexpected current stage: %s", timeline.Current.Stage.StageKey)
	}
	if len(timeline.Artifacts) != 1 {
		t.Fatalf("expected initial artifact, got %d", len(timeline.Artifacts))
	}
	if timeline.Summary.LatestArtifactID != timeline.Artifacts[0].ID {
		t.Fatalf("latest artifact mismatch")
	}
}

func TestGetPipelineRunTimelineReturnsAggregatedCurrentState(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{ID: "run_test_timeline", TemplateID: pipeline.DefaultTemplateID, Title: "Timeline", RequirementText: "需要聚合工作台数据", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/timeline", Status: model.PipelineRunWaitingApproval, CurrentStageKey: pipeline.StageCheckpointDesign, CreatedBy: "tester", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stages := []model.StageRun{
		{ID: "timeline_design", PipelineRunID: run.ID, StageKey: pipeline.StageSolutionDesign, StageType: model.StageTypeDesign, Status: model.StageRunSucceeded, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "timeline_checkpoint", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointDesign, StageType: model.StageTypeCheckpoint, Status: model.StageRunWaitingApproval, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := repository.CreateStageRuns(ctx, stages); err != nil {
		t.Fatalf("create stages: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "timeline_cp", PipelineRunID: run.ID, StageRunID: "timeline_checkpoint", CheckpointType: model.CheckpointDesignReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	artifact := model.Artifact{ID: "timeline_artifact", PipelineRunID: run.ID, StageRunID: "timeline_design", ArtifactType: model.ArtifactSolutionDesign, Title: "技术方案", MetaJSON: "{}", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateArtifact(ctx, &artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	timeline, err := service.GetPipelineRunTimeline(ctx, run.ID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if timeline.Summary.TotalStages != 2 || timeline.Summary.CompletedStages != 1 || !timeline.Summary.WaitingApproval {
		t.Fatalf("unexpected summary: %+v", timeline.Summary)
	}
	if timeline.Current == nil || timeline.Current.Stage == nil || timeline.Current.Stage.ID != "timeline_checkpoint" {
		t.Fatalf("unexpected current stage: %+v", timeline.Current)
	}
	if timeline.Current.Checkpoint == nil || timeline.Current.Checkpoint.ID != checkpoint.ID {
		t.Fatalf("expected current checkpoint")
	}
}

func TestPipelineRunLifecycleRejectsInvalidTransitions(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{ID: "run_lifecycle_done", TemplateID: pipeline.DefaultTemplateID, Title: "done", RequirementText: "done", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/done", Status: model.PipelineRunCompleted, CurrentStageKey: pipeline.StageDelivery, CreatedBy: "tester", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create completed run: %v", err)
	}
	if _, err := service.ResumePipelineRun(ctx, run.ID); err == nil {
		t.Fatalf("expected completed run resume to fail")
	}
	if _, err := service.TerminatePipelineRun(ctx, run.ID); err == nil {
		t.Fatalf("expected completed run terminate to fail")
	}
}

func TestApproveCheckpointRejectsDuplicateDecision(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{ID: "run_test_duplicate_approve", TemplateID: pipeline.DefaultTemplateID, Title: "Duplicate approval", RequirementText: "需要防止重复审批", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/duplicate-approve", Status: model.PipelineRunWaitingApproval, CurrentStageKey: pipeline.StageCheckpointDesign, CreatedBy: "tester", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stage := model.StageRun{ID: "stage_duplicate_checkpoint", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointDesign, StageType: model.StageTypeCheckpoint, Status: model.StageRunWaitingApproval, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateStageRuns(ctx, []model.StageRun{stage}); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "cp_duplicate", PipelineRunID: run.ID, StageRunID: stage.ID, CheckpointType: model.CheckpointDesignReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	if _, err := service.ApproveCheckpoint(ctx, checkpoint.ID, "ok", "reviewer"); err != nil {
		t.Fatalf("approve checkpoint: %v", err)
	}
	if _, err := service.ApproveCheckpoint(ctx, checkpoint.ID, "again", "reviewer"); err == nil {
		t.Fatalf("expected duplicate approval to fail")
	}
}

func TestGetCheckpointApprovalDetailSelectsLatestRelevantArtifact(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{ID: "run_test_approval_detail", TemplateID: pipeline.DefaultTemplateID, Title: "Approval detail", RequirementText: "需要展示审批上下文", TargetRepo: "self", TargetBranch: "main", WorkBranch: "devflow/approval-detail", Status: model.PipelineRunWaitingApproval, CurrentStageKey: pipeline.StageCheckpointReview, CreatedBy: "tester", BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stage := model.StageRun{ID: "stage_approval_detail_checkpoint", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointReview, StageType: model.StageTypeCheckpoint, Status: model.StageRunWaitingApproval, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateStageRuns(ctx, []model.StageRun{stage}); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "cp_approval_detail", PipelineRunID: run.ID, StageRunID: stage.ID, CheckpointType: model.CheckpointCodeReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	supersededReview := model.Artifact{ID: "artifact_review_old", PipelineRunID: run.ID, StageRunID: "stage_review_old", ArtifactType: model.ArtifactReviewReport, Title: "旧评审报告", ContentJSON: `{"summary":"old"}`, MetaJSON: `{"superseded":true}`, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	codeDiff := model.Artifact{ID: "artifact_code_diff", PipelineRunID: run.ID, StageRunID: "stage_codegen", ArtifactType: model.ArtifactCodeDiff, Title: "代码变更计划", ContentJSON: `{"summary":"diff"}`, MetaJSON: `{}`, BaseModel: model.BaseModel{CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}}
	review := model.Artifact{ID: "artifact_review_new", PipelineRunID: run.ID, StageRunID: "stage_review", ArtifactType: model.ArtifactReviewReport, Title: "AI Review 报告", ContentJSON: `{"summary":"review"}`, MetaJSON: `{}`, BaseModel: model.BaseModel{CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)}}
	for _, artifact := range []model.Artifact{supersededReview, codeDiff, review} {
		item := artifact
		if err := repository.CreateArtifact(ctx, &item); err != nil {
			t.Fatalf("create artifact %s: %v", artifact.ID, err)
		}
	}

	detail, err := service.GetCheckpointApprovalDetail(ctx, checkpoint.ID)
	if err != nil {
		t.Fatalf("get approval detail: %v", err)
	}
	if !detail.CanDecide {
		t.Fatalf("expected checkpoint to be decidable")
	}
	if detail.ApprovalArtifact == nil || detail.ApprovalArtifact.ID != review.ID {
		t.Fatalf("expected latest review artifact, got %#v", detail.ApprovalArtifact)
	}
	if _, ok := detail.LatestArtifacts[model.ArtifactReviewReport]; !ok {
		t.Fatalf("expected latest artifacts to include review report")
	}
	if len(detail.RecentArtifacts) != 2 {
		t.Fatalf("expected 2 non-superseded recent artifacts, got %d", len(detail.RecentArtifacts))
	}
}

func TestRejectCheckpointResetsPreviousStage(t *testing.T) {
	repository := newPipelineTestRepository(t)
	service := NewPipelineService(repository, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	run := model.PipelineRun{
		ID:              "run_test_reject",
		TemplateID:      pipeline.DefaultTemplateID,
		Title:           "Reject checkpoint test",
		RequirementText: "需要在 reject 后回退到方案阶段",
		TargetRepo:      "self",
		TargetBranch:    "main",
		WorkBranch:      "devflow/test-reject",
		Status:          model.PipelineRunWaitingApproval,
		CurrentStageKey: pipeline.StageCheckpointDesign,
		CreatedBy:       "tester",
		BaseModel:       model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := repository.CreatePipelineRun(ctx, &run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	stages := []model.StageRun{
		{ID: "stage_req_2", PipelineRunID: run.ID, StageKey: pipeline.StageRequirementAnalysis, StageType: model.StageTypeAnalysis, Status: model.StageRunSucceeded, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_design_2", PipelineRunID: run.ID, StageKey: pipeline.StageSolutionDesign, StageType: model.StageTypeDesign, Status: model.StageRunSucceeded, Attempt: 1, OutputJSON: `{"summary":"old design"}`, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_checkpoint_2", PipelineRunID: run.ID, StageKey: pipeline.StageCheckpointDesign, StageType: model.StageTypeCheckpoint, Status: model.StageRunWaitingApproval, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "stage_codegen_2", PipelineRunID: run.ID, StageKey: pipeline.StageCodeGeneration, StageType: model.StageTypeCodegen, Status: model.StageRunPending, Attempt: 1, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := repository.CreateStageRuns(ctx, stages); err != nil {
		t.Fatalf("create stages: %v", err)
	}
	checkpoint := model.Checkpoint{ID: "cp_design_2", PipelineRunID: run.ID, StageRunID: "stage_checkpoint_2", CheckpointType: model.CheckpointDesignReview, Status: model.CheckpointPending, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateCheckpoint(ctx, &checkpoint); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	artifact := model.Artifact{ID: "artifact_design_2", PipelineRunID: run.ID, StageRunID: "stage_design_2", ArtifactType: model.ArtifactSolutionDesign, Title: "技术方案", ContentJSON: `{"summary":"old design"}`, MetaJSON: `{}`, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}}
	if err := repository.CreateArtifact(ctx, &artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	updatedCheckpoint, err := service.RejectCheckpoint(ctx, checkpoint.ID, "方案需要补充回退信息", "reviewer")
	if err != nil {
		t.Fatalf("reject checkpoint: %v", err)
	}
	if updatedCheckpoint.Status != model.CheckpointRejected {
		t.Fatalf("expected rejected checkpoint, got %s", updatedCheckpoint.Status)
	}

	designStage, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageSolutionDesign)
	if err != nil {
		t.Fatalf("get design stage: %v", err)
	}
	if designStage.Status != model.StageRunQueued {
		t.Fatalf("expected design stage queued, got %s", designStage.Status)
	}
	if designStage.Attempt != 2 {
		t.Fatalf("expected design stage attempt 2, got %d", designStage.Attempt)
	}
	if designStage.InputJSON == "" {
		t.Fatalf("expected reject context on design stage input")
	}
	if !strings.Contains(designStage.InputJSON, "方案需要补充回退信息") {
		t.Fatalf("expected reject reason in design stage input: %s", designStage.InputJSON)
	}

	checkpointStage, err := repository.GetStageRunByKey(ctx, run.ID, pipeline.StageCheckpointDesign)
	if err != nil {
		t.Fatalf("get checkpoint stage: %v", err)
	}
	if checkpointStage.Status != model.StageRunPending {
		t.Fatalf("expected checkpoint stage pending, got %s", checkpointStage.Status)
	}

	updatedRun, err := repository.GetPipelineRunByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if updatedRun.Status != model.PipelineRunQueued {
		t.Fatalf("expected queued run, got %s", updatedRun.Status)
	}
	if updatedRun.CurrentStageKey != pipeline.StageSolutionDesign {
		t.Fatalf("expected current stage %s, got %s", pipeline.StageSolutionDesign, updatedRun.CurrentStageKey)
	}
}

func newPipelineTestRepository(t *testing.T) *repo.Repository {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "pipeline-service-test.db")
	repository, err := repo.NewSQLiteRepository(databasePath)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() {
		_ = repository.Close()
	})
	return repository
}
