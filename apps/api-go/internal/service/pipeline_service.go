package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"feishu-pipeline/apps/api-go/internal/external"
	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/job"
	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/pipeline"
	"feishu-pipeline/apps/api-go/internal/repo"
	"feishu-pipeline/apps/api-go/internal/utils"
)

type PipelineQueue interface {
	EnqueuePipelineRun(job.PipelineRunJob)
}

type PipelineService struct {
	repository   *repo.Repository
	engine       *pipeline.Engine
	queue        PipelineQueue
	feishuClient *feishu.Client
}

type PipelineServiceOption func(*PipelineService)

func WithPipelineExecutor(executor pipeline.Executor) PipelineServiceOption {
	return func(service *PipelineService) {
		if executor != nil {
			service.engine = pipeline.NewEngine(service.repository, executor, service.feishuClient)
		}
	}
}

type PipelineRunDetail struct {
	Run         model.PipelineRun
	Stages      []model.StageRun
	Artifacts   []model.Artifact
	Checkpoints []model.Checkpoint
}

type PipelineRunTimelineSummary struct {
	TotalStages      int
	CompletedStages  int
	FailedStages     int
	WaitingApproval  bool
	CurrentStageKey  string
	LatestArtifactID string
	LatestDeliveryID string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	DurationMS       int64
}

type PipelineRunTimeline struct {
	Run         model.PipelineRun
	Current     *PipelineRunCurrent
	Stages      []model.StageRun
	Artifacts   []model.Artifact
	Checkpoints []model.Checkpoint
	AgentRuns   []model.AgentRun
	Deliveries  []model.GitDelivery
	Summary     PipelineRunTimelineSummary
}

type PipelineRunCurrent struct {
	Run        model.PipelineRun
	Stage      *model.StageRun
	Artifact   *model.Artifact
	Checkpoint *model.Checkpoint
	AgentRun   *model.AgentRun
	Delivery   *model.GitDelivery
	NextAction string
}

type CreatePipelineRunInput struct {
	TemplateID      string
	Title           string
	RequirementText string
	TargetRepo      string
	TargetBranch    string
	SourceSessionID string
	CreatedBy       string
	SelectedDocUrls []string
}

func NewPipelineService(repository *repo.Repository, feishuClient *feishu.Client, options ...PipelineServiceOption) *PipelineService {
	service := &PipelineService{
		repository:   repository,
		feishuClient: feishuClient,
		engine:       pipeline.NewEngine(repository, pipeline.NewSequentialExecutor(pipeline.WithAgentRunner(pipeline.NewAgentRunner(nil, pipeline.DefaultPromptRegistry()))), feishuClient),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *PipelineService) SetQueue(queue PipelineQueue) {
	s.queue = queue
}

func (s *PipelineService) ListPipelineTemplates(ctx context.Context) ([]model.PipelineTemplate, error) {
	return s.repository.ListPipelineTemplates(ctx)
}

func (s *PipelineService) ListPipelineRuns(ctx context.Context) ([]model.PipelineRun, error) {
	return s.repository.ListPipelineRuns(ctx)
}

func (s *PipelineService) GetPipelineRunDetail(ctx context.Context, runID string) (*PipelineRunDetail, error) {
	run, stages, artifacts, checkpoints, _, _, err := s.loadPipelineRunParts(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &PipelineRunDetail{Run: run, Stages: stages, Artifacts: artifacts, Checkpoints: checkpoints}, nil
}

func (s *PipelineService) GetPipelineRunTimeline(ctx context.Context, runID string) (*PipelineRunTimeline, error) {
	run, stages, artifacts, checkpoints, agentRuns, deliveries, err := s.loadPipelineRunParts(ctx, runID)
	if err != nil {
		return nil, err
	}
	current := buildPipelineRunCurrent(run, stages, artifacts, checkpoints, agentRuns, deliveries)
	return &PipelineRunTimeline{Run: run, Current: current, Stages: stages, Artifacts: artifacts, Checkpoints: checkpoints, AgentRuns: agentRuns, Deliveries: deliveries, Summary: buildPipelineRunTimelineSummary(run, stages, artifacts, deliveries)}, nil
}

func (s *PipelineService) GetPipelineRunCurrent(ctx context.Context, runID string) (*PipelineRunCurrent, error) {
	run, stages, artifacts, checkpoints, agentRuns, deliveries, err := s.loadPipelineRunParts(ctx, runID)
	if err != nil {
		return nil, err
	}
	return buildPipelineRunCurrent(run, stages, artifacts, checkpoints, agentRuns, deliveries), nil
}

func (s *PipelineService) loadPipelineRunParts(ctx context.Context, runID string) (model.PipelineRun, []model.StageRun, []model.Artifact, []model.Checkpoint, []model.AgentRun, []model.GitDelivery, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	stages, err := s.repository.ListStageRunsByPipelineRunID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	artifacts, err := s.repository.ListArtifactsByPipelineRunID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	checkpoints, err := s.repository.ListCheckpointsByPipelineRunID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	agentRuns, err := s.repository.ListAgentRunsByPipelineRunID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	deliveries, err := s.repository.ListGitDeliveriesByPipelineRunID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, nil, nil, nil, nil, nil, err
	}
	return run, stages, artifacts, checkpoints, agentRuns, deliveries, nil
}

func buildPipelineRunCurrent(run model.PipelineRun, stages []model.StageRun, artifacts []model.Artifact, checkpoints []model.Checkpoint, agentRuns []model.AgentRun, deliveries []model.GitDelivery) *PipelineRunCurrent {
	current := &PipelineRunCurrent{Run: run}
	for idx := range stages {
		if stages[idx].StageKey == run.CurrentStageKey {
			current.Stage = &stages[idx]
			break
		}
	}
	if current.Stage == nil && len(stages) > 0 {
		current.Stage = &stages[0]
	}

	// 优先获取当前 Stage 的 artifact
	if current.Stage != nil {
		for idx := len(artifacts) - 1; idx >= 0; idx-- {
			if artifacts[idx].StageRunID == current.Stage.ID {
				current.Artifact = &artifacts[idx]
				break
			}
		}
		for idx := range checkpoints {
			if checkpoints[idx].StageRunID == current.Stage.ID {
				current.Checkpoint = &checkpoints[idx]
				break
			}
		}
		for idx := len(agentRuns) - 1; idx >= 0; idx-- {
			if agentRuns[idx].StageRunID == current.Stage.ID {
				current.AgentRun = &agentRuns[idx]
				break
			}
		}
	}

	// 如果当前 Stage 没有 artifact（审批阶段），获取最后一个有效的 artifact
	if current.Artifact == nil && len(artifacts) > 0 {
		for idx := len(artifacts) - 1; idx >= 0; idx-- {
			if artifacts[idx].ContentText != "" || artifacts[idx].ContentJSON != "" {
				current.Artifact = &artifacts[idx]
				break
			}
		}
		// 仍没有就取最后一个
		if current.Artifact == nil {
			current.Artifact = &artifacts[len(artifacts)-1]
		}
	}

	// 如果当前 Stage 没有 checkpoint（审批阶段），获取最后一个 pending checkpoint
	if current.Checkpoint == nil && len(checkpoints) > 0 {
		for idx := len(checkpoints) - 1; idx >= 0; idx-- {
			if checkpoints[idx].Status == model.CheckpointPending {
				current.Checkpoint = &checkpoints[idx]
				break
			}
		}
		// 仍没有就取最后一个
		if current.Checkpoint == nil {
			current.Checkpoint = &checkpoints[len(checkpoints)-1]
		}
	}

	if latestDelivery := latestGitDelivery(deliveries); latestDelivery != nil {
		current.Delivery = latestDelivery
	}
	current.NextAction = pipelineRunNextAction(run, current)
	return current
}

func buildPipelineRunTimelineSummary(run model.PipelineRun, stages []model.StageRun, artifacts []model.Artifact, deliveries []model.GitDelivery) PipelineRunTimelineSummary {
	summary := PipelineRunTimelineSummary{TotalStages: len(stages), CurrentStageKey: run.CurrentStageKey, WaitingApproval: run.Status == model.PipelineRunWaitingApproval, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt}
	for _, stage := range stages {
		switch stage.Status {
		case model.StageRunSucceeded:
			summary.CompletedStages++
		case model.StageRunFailed:
			summary.FailedStages++
		}
	}
	for idx := len(artifacts) - 1; idx >= 0; idx-- {
		if artifacts[idx].ID != "" {
			summary.LatestArtifactID = artifacts[idx].ID
			break
		}
	}
	if latestDelivery := latestGitDelivery(deliveries); latestDelivery != nil {
		summary.LatestDeliveryID = latestDelivery.ID
	}
	if run.StartedAt != nil {
		finishedAt := time.Now().UTC()
		if run.FinishedAt != nil {
			finishedAt = *run.FinishedAt
		}
		summary.DurationMS = finishedAt.Sub(*run.StartedAt).Milliseconds()
	}
	return summary
}

func latestGitDelivery(deliveries []model.GitDelivery) *model.GitDelivery {
	for idx := len(deliveries) - 1; idx >= 0; idx-- {
		if deliveries[idx].ID != "" {
			return &deliveries[idx]
		}
	}
	return nil
}

func pipelineRunNextAction(run model.PipelineRun, current *PipelineRunCurrent) string {
	switch run.Status {
	case model.PipelineRunDraft:
		return "start_run"
	case model.PipelineRunQueued, model.PipelineRunRunning:
		return "wait_execution"
	case model.PipelineRunWaitingApproval:
		return "approve_checkpoint"
	case model.PipelineRunFailed:
		return "inspect_failure"
	case model.PipelineRunPaused:
		return "resume_run"
	case model.PipelineRunTerminated:
		return "terminated"
	case model.PipelineRunCompleted:
		if current != nil && current.Delivery != nil && (current.Delivery.Status == model.GitDeliveryDraft || current.Delivery.Status == model.GitDeliveryReady) {
			return "review_delivery"
		}
		return "completed"
	default:
		return "wait_execution"
	}
}

func (s *PipelineService) CreatePipelineRun(ctx context.Context, input CreatePipelineRunInput) (*PipelineRunDetail, error) {
	templateID := strings.TrimSpace(input.TemplateID)
	if templateID == "" {
		templateID = pipeline.DefaultTemplateID
	}
	template, err := s.repository.GetPipelineTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	definition, err := pipeline.ParseTemplateDefinition(template.DefinitionJSON)
	if err != nil {
		return nil, err
	}
	if len(definition.Stages) == 0 {
		return nil, fmt.Errorf("pipeline template stages are required")
	}

	targetRepo := strings.TrimSpace(input.TargetRepo)
	if targetRepo == "" {
		targetRepo = "self"
	}
	targetBranch := strings.TrimSpace(input.TargetBranch)
	if targetBranch == "" {
		targetBranch = "main"
	}
	now := time.Now().UTC()
	// 序列化飞书文档 URL
	selectedDocUrlsJSON := ""
	if len(input.SelectedDocUrls) > 0 {
		if data, err := json.Marshal(input.SelectedDocUrls); err == nil {
			selectedDocUrlsJSON = string(data)
		}
	}
	run := model.PipelineRun{
		ID:              utils.NewID("run"),
		TemplateID:      templateID,
		Title:           strings.TrimSpace(input.Title),
		RequirementText: strings.TrimSpace(input.RequirementText),
		SourceSessionID: strings.TrimSpace(input.SourceSessionID),
		TargetRepo:      targetRepo,
		TargetBranch:    targetBranch,
		WorkBranch:      fmt.Sprintf("devflow/%s", strings.TrimPrefix(utils.NewID("branch"), "branch_")),
		Status:          model.PipelineRunDraft,
		CurrentStageKey: definition.Stages[0].Key,
		CreatedBy:       input.CreatedBy,
		SelectedDocUrls: selectedDocUrlsJSON,
		BaseModel:       model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	stageRuns := make([]model.StageRun, 0, len(definition.Stages))
	checkpoints := make([]model.Checkpoint, 0, 2)
	for idx, stage := range definition.Stages {
		status := model.StageRunPending
		if idx == 0 {
			status = model.StageRunQueued
		}
		stageRun := model.StageRun{
			ID:            utils.NewID("stage"),
			PipelineRunID: run.ID,
			StageKey:      stage.Key,
			StageType:     stage.Type,
			Status:        status,
			Attempt:       1,
			BaseModel:     model.BaseModel{CreatedAt: now, UpdatedAt: now},
		}
		stageRuns = append(stageRuns, stageRun)
		if stage.IsCheckpoint {
			checkpoints = append(checkpoints, model.Checkpoint{
				ID:             utils.NewID("checkpoint"),
				PipelineRunID:  run.ID,
				StageRunID:     stageRun.ID,
				CheckpointType: checkpointTypeForStage(stage.Key),
				Status:         model.CheckpointPending,
				BaseModel:      model.BaseModel{CreatedAt: now, UpdatedAt: now},
			})
		}
	}
	artifacts := []model.Artifact{{
		ID:            utils.NewID("artifact"),
		PipelineRunID: run.ID,
		ArtifactType:  model.ArtifactStructuredRequirement,
		Title:         "结构化需求输入",
		ContentText:   run.RequirementText,
		MetaJSON:      "{}",
		BaseModel:     model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}}
	if err := s.repository.CreatePipelineRunAggregate(ctx, &run, stageRuns, checkpoints, artifacts); err != nil {
		return nil, err
	}

	return s.GetPipelineRunDetail(ctx, run.ID)
}

func (s *PipelineService) CreatePipelineRunFromSession(ctx context.Context, sessionID string, templateID string, targetRepo string, targetBranch string, createdBy string) (*PipelineRunDetail, error) {
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages, err := s.repository.ListMessagesBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	requirementText := session.Summary
	if len(messages) > 0 {
		parts := make([]string, 0, len(messages))
		for _, message := range messages {
			parts = append(parts, fmt.Sprintf("[%s] %s", message.Role, message.Content))
		}
		requirementText = strings.Join(parts, "\n")
	}
	if strings.TrimSpace(requirementText) == "" {
		requirementText = session.Title
	}
	return s.CreatePipelineRun(ctx, CreatePipelineRunInput{TemplateID: templateID, Title: session.Title, RequirementText: requirementText, TargetRepo: targetRepo, TargetBranch: targetBranch, SourceSessionID: session.ID, CreatedBy: createdBy})
}

func (s *PipelineService) ListStageRuns(ctx context.Context, runID string) ([]model.StageRun, error) {
	return s.repository.ListStageRunsByPipelineRunID(ctx, runID)
}

func (s *PipelineService) ListArtifacts(ctx context.Context, runID string) ([]model.Artifact, error) {
	return s.repository.ListArtifactsByPipelineRunID(ctx, runID)
}

func (s *PipelineService) ListCheckpoints(ctx context.Context, runID string) ([]model.Checkpoint, error) {
	return s.repository.ListCheckpointsByPipelineRunID(ctx, runID)
}

func (s *PipelineService) ListAgentRuns(ctx context.Context, runID string) ([]model.AgentRun, error) {
	return s.repository.ListAgentRunsByPipelineRunID(ctx, runID)
}

func (s *PipelineService) ListGitDeliveries(ctx context.Context, runID string) ([]model.GitDelivery, error) {
	if _, err := s.repository.GetPipelineRunByID(ctx, runID); err != nil {
		return nil, err
	}
	return s.repository.ListGitDeliveriesByPipelineRunID(ctx, runID)
}

func (s *PipelineService) GetGitDelivery(ctx context.Context, deliveryID string) (model.GitDelivery, error) {
	return s.repository.GetGitDeliveryByID(ctx, deliveryID)
}

func (s *PipelineService) StartPipelineRun(ctx context.Context, runID string) (model.PipelineRun, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, err
	}
	if !pipeline.CanStartRun(run.Status) {
		return model.PipelineRun{}, fmt.Errorf("cannot start pipeline run from status %s", run.Status)
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunQueued); err != nil {
		return model.PipelineRun{}, err
	}
	if s.queue != nil {
		s.queue.EnqueuePipelineRun(job.PipelineRunJob{RunID: runID})
	}
	return s.repository.GetPipelineRunByID(ctx, runID)
}

func (s *PipelineService) PausePipelineRun(ctx context.Context, runID string) (model.PipelineRun, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, err
	}
	if !pipeline.CanPauseRun(run.Status) {
		return model.PipelineRun{}, fmt.Errorf("cannot pause pipeline run from status %s", run.Status)
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunPaused); err != nil {
		return model.PipelineRun{}, err
	}
	return s.repository.GetPipelineRunByID(ctx, runID)
}

func (s *PipelineService) ResumePipelineRun(ctx context.Context, runID string) (model.PipelineRun, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, err
	}
	if !pipeline.CanResumeRun(run.Status) {
		return model.PipelineRun{}, fmt.Errorf("cannot resume pipeline run from status %s", run.Status)
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunQueued); err != nil {
		return model.PipelineRun{}, err
	}
	if s.queue != nil {
		s.queue.EnqueuePipelineRun(job.PipelineRunJob{RunID: runID})
	}
	return s.repository.GetPipelineRunByID(ctx, runID)
}

func (s *PipelineService) TerminatePipelineRun(ctx context.Context, runID string) (model.PipelineRun, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return model.PipelineRun{}, err
	}
	if !pipeline.CanTerminateRun(run.Status) {
		return model.PipelineRun{}, fmt.Errorf("cannot terminate pipeline run from status %s", run.Status)
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunTerminated); err != nil {
		return model.PipelineRun{}, err
	}
	return s.repository.GetPipelineRunByID(ctx, runID)
}

func (s *PipelineService) ApproveCheckpoint(ctx context.Context, checkpointID string, comment string, approverID string) (model.Checkpoint, error) {
	checkpoint, run, stage, err := s.getCheckpointDecisionContext(ctx, checkpointID)
	if err != nil {
		return model.Checkpoint{}, err
	}
	if !pipeline.CanApproveCheckpoint(run, stage, checkpoint) {
		return model.Checkpoint{}, fmt.Errorf("cannot approve checkpoint %s from run status %s, stage status %s, checkpoint status %s", checkpointID, run.Status, stage.Status, checkpoint.Status)
	}
	if err := s.repository.UpdateCheckpointDecision(ctx, checkpointID, model.CheckpointApproved, "approve", strings.TrimSpace(comment), approverID); err != nil {
		return model.Checkpoint{}, err
	}
	checkpoint, err = s.repository.GetCheckpointByID(ctx, checkpointID)
	if err != nil {
		return model.Checkpoint{}, err
	}
	if err := s.repository.UpdateStageRunStatus(ctx, checkpoint.StageRunID, model.StageRunSucceeded); err != nil {
		return model.Checkpoint{}, err
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, checkpoint.PipelineRunID, model.PipelineRunQueued); err != nil {
		return model.Checkpoint{}, err
	}
	if s.queue != nil {
		s.queue.EnqueuePipelineRun(job.PipelineRunJob{RunID: checkpoint.PipelineRunID})
	}
	return checkpoint, nil
}

func (s *PipelineService) RejectCheckpoint(ctx context.Context, checkpointID string, comment string, approverID string) (model.Checkpoint, error) {
	trimmedComment := strings.TrimSpace(comment)
	checkpoint, run, checkpointStageRun, err := s.getCheckpointDecisionContext(ctx, checkpointID)
	if err != nil {
		return model.Checkpoint{}, err
	}
	if !pipeline.CanRejectCheckpoint(run, checkpointStageRun, checkpoint) {
		return model.Checkpoint{}, fmt.Errorf("cannot reject checkpoint %s from run status %s, stage status %s, checkpoint status %s", checkpointID, run.Status, checkpointStageRun.Status, checkpoint.Status)
	}
	if err := s.repository.UpdateCheckpointDecision(ctx, checkpointID, model.CheckpointRejected, "reject", trimmedComment, approverID); err != nil {
		return model.Checkpoint{}, err
	}
	checkpoint, err = s.repository.GetCheckpointByID(ctx, checkpointID)
	if err != nil {
		return model.Checkpoint{}, err
	}
	if err := s.repository.ResetStageRun(ctx, checkpointStageRun.ID, model.StageRunPending, checkpointStageRun.Attempt, ""); err != nil {
		return model.Checkpoint{}, err
	}

	prevKey := pipeline.PreviousExecutableStage(checkpointStageRun.StageKey)
	if prevKey == "" {
		return checkpoint, nil
	}
	prevStage, err := s.repository.GetStageRunByKey(ctx, checkpoint.PipelineRunID, prevKey)
	if err != nil {
		return model.Checkpoint{}, err
	}

	if err := s.repository.MarkArtifactsSupersededByStageRunID(ctx, prevStage.ID); err != nil {
		return model.Checkpoint{}, err
	}
	if err := s.repository.ResetStageRun(ctx, prevStage.ID, model.StageRunQueued, prevStage.Attempt+1, pipeline.BuildRejectContext(trimmedComment, prevStage.OutputJSON)); err != nil {
		return model.Checkpoint{}, err
	}

	for _, stageKey := range pipeline.NextStagesForReset(prevKey) {
		stage, stageErr := s.repository.GetStageRunByKey(ctx, checkpoint.PipelineRunID, stageKey)
		if stageErr != nil {
			continue
		}
		if stage.StageKey == checkpointStageRun.StageKey {
			if err := s.repository.ResetStageRun(ctx, stage.ID, model.StageRunPending, stage.Attempt, ""); err != nil {
				return model.Checkpoint{}, err
			}
			continue
		}
		if err := s.repository.ResetStageRun(ctx, stage.ID, model.StageRunPending, stage.Attempt, ""); err != nil {
			return model.Checkpoint{}, err
		}
		if err := s.repository.MarkArtifactsSupersededByStageRunID(ctx, stage.ID); err != nil {
			return model.Checkpoint{}, err
		}
	}

	if err := s.repository.UpdatePipelineRunCurrentStage(ctx, checkpoint.PipelineRunID, prevKey); err != nil {
		return model.Checkpoint{}, err
	}
	if err := s.repository.UpdatePipelineRunStatus(ctx, checkpoint.PipelineRunID, model.PipelineRunQueued); err != nil {
		return model.Checkpoint{}, err
	}
	if s.queue != nil {
		s.queue.EnqueuePipelineRun(job.PipelineRunJob{RunID: checkpoint.PipelineRunID})
	}
	return checkpoint, nil
}

func (s *PipelineService) HandlePipelineRun(ctx context.Context, payload job.PipelineRunJob) error {
	if err := pipeline.RequireRunID(payload.RunID); err != nil {
		return err
	}
	return s.engine.Run(ctx, payload.RunID)
}

func (s *PipelineService) getCheckpointDecisionContext(ctx context.Context, checkpointID string) (model.Checkpoint, model.PipelineRun, model.StageRun, error) {
	checkpoint, err := s.repository.GetCheckpointByID(ctx, checkpointID)
	if err != nil {
		return model.Checkpoint{}, model.PipelineRun{}, model.StageRun{}, err
	}
	if checkpoint.PipelineRunID == "" || checkpoint.StageRunID == "" {
		return model.Checkpoint{}, model.PipelineRun{}, model.StageRun{}, fmt.Errorf("checkpoint %s is not bound to pipeline run and stage run", checkpointID)
	}
	run, err := s.repository.GetPipelineRunByID(ctx, checkpoint.PipelineRunID)
	if err != nil {
		return model.Checkpoint{}, model.PipelineRun{}, model.StageRun{}, err
	}
	stage, err := s.repository.GetStageRunByID(ctx, checkpoint.StageRunID)
	if err != nil {
		return model.Checkpoint{}, model.PipelineRun{}, model.StageRun{}, err
	}
	return checkpoint, run, stage, nil
}

func checkpointTypeForStage(stageKey string) model.CheckpointType {
	switch stageKey {
	case pipeline.StageCheckpointDesign:
		return model.CheckpointDesignReview
	case pipeline.StageCheckpointReview:
		return model.CheckpointCodeReview
	default:
		return model.CheckpointCodeReview
	}
}

// ExecuteChangesParams 变更执行参数
type ExecuteChangesParams struct {
	ChangeSet   []map[string]any
	CommitterID string
	Token       string
}

// ExecuteChangesResult 变更执行结果
type ExecuteChangesResult struct {
	AppliedFiles []string         `json:"appliedFiles"`
	FailedFiles  []map[string]any `json:"failedFiles"`
	Summary      string           `json:"summary"`
}

// ExecuteChanges 审批通过后执行变更计划
func (s *PipelineService) ExecuteChanges(ctx context.Context, runID string, params ExecuteChangesParams) (*ExecuteChangesResult, error) {
	run, err := s.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return nil, err
	}

	if run.Status != model.PipelineRunCompleted {
		return nil, fmt.Errorf("只能在完成状态下执行变更，当前状态: %s", run.Status)
	}

	// 查找 code_diff 类型的 artifact
	artifacts, err := s.repository.ListArtifactsByPipelineRunID(ctx, runID)
	if err != nil {
		return nil, err
	}

	var codeDiffArtifact *model.Artifact
	for idx := len(artifacts) - 1; idx >= 0; idx-- {
		if artifacts[idx].ArtifactType == model.ArtifactCodeDiff {
			codeDiffArtifact = &artifacts[idx]
			break
		}
	}

	if codeDiffArtifact == nil {
		return nil, fmt.Errorf("未找到代码变更计划 artifact")
	}

	// 转换变更项
	execChangeSet := make([]pipeline.ChangeItem, 0, len(params.ChangeSet))
	for _, item := range params.ChangeSet {
		if filePath, ok := item["filePath"].(string); ok {
			changeItem := pipeline.ChangeItem{FilePath: filePath}
			if proposedPatch, ok := item["proposedPatch"].(string); ok {
				changeItem.NewContent = proposedPatch
			}
			if sha, ok := item["sha"].(string); ok {
				changeItem.SHA = sha
			}
			execChangeSet = append(execChangeSet, changeItem)
		}
	}

	// 执行变更
	gh := external.NewGitHubService()
	execParams := pipeline.ExecuteChangesParams{
		RunID:       runID,
		ChangeSet:   execChangeSet,
		CommitterID: params.CommitterID,
		Token:       params.Token,
	}
	result, err := pipeline.ExecuteChanges(ctx, gh, execParams, run)
	if err != nil {
		return nil, err
	}

	// 记录执行结果到 GitDelivery
	changedFilesJSON, _ := json.Marshal(result.AppliedFiles)
	delivery := model.GitDelivery{
		ID:               utils.NewID("delivery"),
		PipelineRunID:    runID,
		Provider:         "github",
		Repo:             run.TargetRepo,
		BaseBranch:       run.TargetBranch,
		HeadBranch:       run.WorkBranch,
		ChangedFilesJSON: string(changedFilesJSON),
		ValidationJSON:   string(changedFilesJSON),
		SummaryMarkdown:  result.Summary,
		Status:           model.GitDeliveryCompleted,
		BaseModel:        model.BaseModel{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	if err := s.repository.CreateGitDelivery(ctx, &delivery); err != nil {
		return nil, err
	}

	// 创建 Pull Request
	if params.Token != "" {
		gh := external.NewGitHubService()
		owner, repo, ok := external.ParseRepoPath(run.TargetRepo)
		if ok {
			_, _, prErr := gh.CreatePullRequest(
				ctx,
				params.Token,
				owner, repo,
				run.WorkBranch,
				run.TargetBranch,
				run.Title,
				result.Summary,
			)
			if prErr != nil {
				// PR 创建失败不影响执行结果
			}
		}
	}

	return &ExecuteChangesResult{
		AppliedFiles: result.AppliedFiles,
		FailedFiles:  result.FailedFiles,
		Summary:      result.Summary,
	}, nil
}
