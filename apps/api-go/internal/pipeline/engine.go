package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/utils"
)

type EngineRepository interface {
	GetPipelineRunByID(context.Context, string) (model.PipelineRun, error)
	ListStageRunsByPipelineRunID(context.Context, string) ([]model.StageRun, error)
	ListArtifactsByPipelineRunID(context.Context, string) ([]model.Artifact, error)
	ListCheckpointsByPipelineRunID(context.Context, string) ([]model.Checkpoint, error)
	SaveStageRunInput(context.Context, string, string) error
	UpdateStageRunStatus(context.Context, string, model.StageRunStatus) error
	UpdatePipelineRunCurrentStage(context.Context, string, string) error
	UpdatePipelineRunStatus(context.Context, string, model.PipelineRunStatus) error
	CreateAgentRun(context.Context, *model.AgentRun) error
	UpdateAgentRun(context.Context, *model.AgentRun) error
	SaveStageRunOutput(context.Context, string, string, string) error
	CreateArtifact(context.Context, *model.Artifact) error
	CreateGitDelivery(context.Context, *model.GitDelivery) error
	ResetCheckpointByStageRunID(context.Context, string) error
}

type Engine struct {
	repository   EngineRepository
	executor     Executor
	feishuClient *feishu.Client
}

func NewEngine(repository EngineRepository, executor Executor, feishuClient *feishu.Client) *Engine {
	return &Engine{repository: repository, executor: executor, feishuClient: feishuClient}
}

func (e *Engine) Run(ctx context.Context, runID string) error {
	run, err := e.repository.GetPipelineRunByID(ctx, runID)
	if err != nil {
		return err
	}
	stages, err := e.repository.ListStageRunsByPipelineRunID(ctx, runID)
	if err != nil {
		return err
	}
	artifacts, err := e.repository.ListArtifactsByPipelineRunID(ctx, runID)
	if err != nil {
		return err
	}
	checkpoints, err := e.repository.ListCheckpointsByPipelineRunID(ctx, runID)
	if err != nil {
		return err
	}

	for _, stage := range stages {
		run, err = e.repository.GetPipelineRunByID(ctx, runID)
		if err != nil {
			return err
		}
		if run.Status == model.PipelineRunPaused || run.Status == model.PipelineRunTerminated || run.Status == model.PipelineRunCompleted {
			return nil
		}
		if stage.Status == model.StageRunSucceeded || stage.Status == model.StageRunWaitingApproval {
			continue
		}
		if !IsRunnableStageStatus(stage.Status) {
			continue
		}
		if stage.StageType == model.StageTypeCheckpoint || ShouldStageWaitForApproval(stage.StageKey) {
			if err := e.repository.UpdateStageRunStatus(ctx, stage.ID, model.StageRunWaitingApproval); err != nil {
				return err
			}
			if err := e.repository.ResetCheckpointByStageRunID(ctx, stage.ID); err != nil {
				return err
			}
			if err := e.repository.UpdatePipelineRunCurrentStage(ctx, runID, stage.StageKey); err != nil {
				return err
			}
			if err := e.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunWaitingApproval); err != nil {
				return err
			}

			// Send Feishu notification when reaching checkpoint
			if e.feishuClient != nil && e.feishuClient.Enabled() {
				// Find the corresponding checkpoint
				var checkpoint *model.Checkpoint
				for _, cp := range checkpoints {
					if cp.StageRunID == stage.ID {
						checkpoint = &cp
						break
					}
				}

				if checkpoint != nil {
					// Build notification message
					checkpointName := "方案审批"
					if stage.StageKey == StageCheckpointReview {
						checkpointName = "代码评审确认"
					}

					approvalURL := fmt.Sprintf("%s/approvals/%s", e.feishuClient.BaseURL(), runID)

					// Create a task to send notification
					task := model.Task{
						ID:            utils.NewID("task"),
						Title:         fmt.Sprintf("流水线审批通知: %s", run.Title),
						Description:   fmt.Sprintf("流水线已到达【%s】检查点，等待您的审批。\n\n需求描述: %s\n流水线ID: %s\n当前阶段: %s\n审批链接: %s", checkpointName, run.RequirementText, runID, stage.StageKey, approvalURL),
						AssigneeID:    run.CreatedBy,
						NotifyContent: fmt.Sprintf("您有一条流水线需要审批：%s", run.Title),
						DocURL:        approvalURL,
					}

					// Send Feishu message
					_, err := e.feishuClient.SendTaskMessage(ctx, task)
					if err != nil {
						// Log error but don't fail the pipeline
						fmt.Printf("failed to send feishu notification for checkpoint: %v\n", err)
					}
				}
			}

			return nil
		}

		input := buildStageInput(run, stage, artifacts, checkpoints)
		inputJSON, _ := json.Marshal(input)
		if err := e.repository.SaveStageRunInput(ctx, stage.ID, string(inputJSON)); err != nil {
			return err
		}
		if err := e.repository.UpdateStageRunStatus(ctx, stage.ID, model.StageRunRunning); err != nil {
			return err
		}
		if err := e.repository.UpdatePipelineRunCurrentStage(ctx, runID, stage.StageKey); err != nil {
			return err
		}
		if err := e.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunRunning); err != nil {
			return err
		}

		result, execErr := e.executor.Execute(ctx, StageContext{Run: run, Stage: stage, Artifacts: artifacts, Checkpoints: checkpoints, Input: input})
		if err := e.repository.CreateAgentRun(ctx, buildAgentRun(run, stage, string(inputJSON), result.AgentRun, execErr)); err != nil {
			return err
		}
		if execErr != nil {
			_ = e.repository.UpdateStageRunStatus(ctx, stage.ID, model.StageRunFailed)
			_ = e.repository.SaveStageRunOutput(ctx, stage.ID, "", execErr.Error())
			_ = e.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunFailed)
			return execErr
		}

		if err := e.repository.SaveStageRunOutput(ctx, stage.ID, result.OutputJSON, ""); err != nil {
			return err
		}
		if err := e.repository.UpdateStageRunStatus(ctx, stage.ID, model.StageRunSucceeded); err != nil {
			return err
		}
		artifact := model.Artifact{
			ID:            utils.NewID("artifact"),
			PipelineRunID: run.ID,
			StageRunID:    stage.ID,
			ArtifactType:  result.ArtifactType,
			Title:         result.Title,
			ContentText:   result.ContentText,
			ContentJSON:   result.ContentJSON,
			BaseModel:     model.BaseModel{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		}
		if err := e.repository.CreateArtifact(ctx, &artifact); err != nil {
			return err
		}
		artifacts = append(artifacts, artifact)
		if stage.StageKey == StageDelivery {
			delivery := buildGitDeliveryDraft(run, result.OutputJSON)
			if err := e.repository.CreateGitDelivery(ctx, &delivery); err != nil {
				return err
			}
		}
	}

	if err := e.repository.UpdatePipelineRunStatus(ctx, runID, model.PipelineRunCompleted); err != nil {
		return err
	}
	if err := e.repository.UpdatePipelineRunCurrentStage(ctx, runID, StageDelivery); err != nil {
		return err
	}
	return nil
}

func buildGitDeliveryDraft(run model.PipelineRun, outputJSON string) model.GitDelivery {
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(outputJSON), &payload)
	prTitle := stringField(payload, SchemaFieldPRTitle)
	if prTitle == "" {
		prTitle = run.Title
	}
	prBody := stringField(payload, SchemaFieldPRBody)
	if prBody == "" {
		prBody = "包含 Pipeline 阶段产物、测试报告、评审报告和交付摘要。"
	}
	changedFiles := stringSliceField(payload, SchemaFieldChangedFiles)
	validation := stringSliceField(payload, SchemaFieldValidation)
	releaseNotes := stringSliceField(payload, SchemaFieldManualReleaseNotes)
	changedFilesJSON := marshalJSONString(changedFiles)
	validationJSON := marshalJSONString(validation)
	summary := buildGitDeliverySummaryMarkdown(run, prTitle, prBody, changedFiles, validation, releaseNotes)
	now := time.Now().UTC()
	return model.GitDelivery{
		ID:               utils.NewID("delivery"),
		PipelineRunID:    run.ID,
		Provider:         "local",
		Repo:             run.TargetRepo,
		BaseBranch:       run.TargetBranch,
		HeadBranch:       run.WorkBranch,
		PRMRTitle:        prTitle,
		PRMRBody:         prBody,
		ChangedFilesJSON: changedFilesJSON,
		ValidationJSON:   validationJSON,
		SummaryMarkdown:  summary,
		Status:           model.GitDeliveryReady,
		BaseModel:        model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
}

func buildGitDeliverySummaryMarkdown(run model.PipelineRun, prTitle string, prBody string, changedFiles []string, validation []string, releaseNotes []string) string {
	lines := []string{
		"# " + prTitle,
		"",
		"## PR/MR Body",
		prBody,
		"",
		"## Branches",
		fmt.Sprintf("- Base: `%s`", run.TargetBranch),
		fmt.Sprintf("- Head: `%s`", run.WorkBranch),
		"",
		"## Changed Files",
	}
	lines = appendMarkdownList(lines, changedFiles)
	lines = append(lines, "", "## Validation")
	lines = appendMarkdownList(lines, validation)
	lines = append(lines, "", "## Release Notes")
	lines = appendMarkdownList(lines, releaseNotes)
	lines = append(lines, "", "> Generated as a local delivery draft. No push or remote PR/MR was created.")
	return joinMarkdownLines(lines)
}

func appendMarkdownList(lines []string, items []string) []string {
	if len(items) == 0 {
		return append(lines, "- 暂无")
	}
	for _, item := range items {
		if item != "" {
			lines = append(lines, "- "+item)
		}
	}
	return lines
}

func joinMarkdownLines(lines []string) string {
	result := ""
	for idx, line := range lines {
		if idx > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func stringSliceField(payload map[string]any, key string) []string {
	items, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

func buildAgentRun(run model.PipelineRun, stage model.StageRun, inputJSON string, observation *AgentObservation, execErr error) *model.AgentRun {
	now := time.Now().UTC()
	status := model.AgentRunSucceeded
	errorMessage := ""
	outputJSON := ""
	provider := AgentProviderDeterministic
	modelName := AgentModelDeterministic
	agentKey := stage.StageKey
	promptSnapshot := ""
	tokenUsageJSON := "{}"
	latencyMS := int64(0)
	if execErr != nil {
		status = model.AgentRunFailed
		errorMessage = execErr.Error()
	}
	if observation != nil {
		if observation.AgentKey != "" {
			agentKey = observation.AgentKey
		}
		if observation.Provider != "" {
			provider = observation.Provider
		}
		if observation.Model != "" {
			modelName = observation.Model
		}
		promptSnapshot = observation.PromptSnapshot
		if observation.InputJSON != "" {
			inputJSON = observation.InputJSON
		}
		outputJSON = observation.OutputJSON
		if observation.TokenUsageJSON != "" {
			tokenUsageJSON = observation.TokenUsageJSON
		}
		latencyMS = observation.LatencyMS
		status = observation.Status
		if observation.ErrorMessage != "" {
			errorMessage = observation.ErrorMessage
		}
	}
	return &model.AgentRun{
		ID:             utils.NewID("agentrun"),
		PipelineRunID:  run.ID,
		StageRunID:     stage.ID,
		AgentKey:       agentKey,
		Provider:       provider,
		Model:          modelName,
		PromptSnapshot: promptSnapshot,
		InputJSON:      inputJSON,
		OutputJSON:     outputJSON,
		TokenUsageJSON: tokenUsageJSON,
		LatencyMS:      latencyMS,
		Status:         status,
		ErrorMessage:   errorMessage,
		BaseModel:      model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
}

func buildStageInput(run model.PipelineRun, stage model.StageRun, artifacts []model.Artifact, checkpoints []model.Checkpoint) map[string]any {
	input := map[string]any{
		"runId":           run.ID,
		"stageKey":        stage.StageKey,
		"attempt":         stage.Attempt,
		"startedAt":       time.Now().UTC().Format(time.RFC3339),
		"requirement":     buildRunRequirement(run),
		"latestArtifacts": latestArtifactsByType(artifacts),
		"checkpoints":     checkpointInputs(checkpoints),
	}
	mergeJSONMap(input, stage.InputJSON)
	return input
}

func buildRunRequirement(run model.PipelineRun) map[string]any {
	req := map[string]any{
		"title":           run.Title,
		"requirementText": run.RequirementText,
		"targetRepo":      run.TargetRepo,
		"targetBranch":    run.TargetBranch,
		"workBranch":      run.WorkBranch,
	}
	// 解析飞书文档 URL
	if run.SelectedDocUrls != "" {
		var urls []string
		if err := json.Unmarshal([]byte(run.SelectedDocUrls), &urls); err == nil && len(urls) > 0 {
			req["selectedDocUrls"] = urls
		}
	}
	return req
}

func latestArtifactsByType(artifacts []model.Artifact) map[string]any {
	result := map[string]any{}
	for _, artifact := range artifacts {
		entry := map[string]any{
			"id":          artifact.ID,
			"title":       artifact.Title,
			"contentText": artifact.ContentText,
			"stageRunId":  artifact.StageRunID,
		}
		mergeJSONMap(entry, artifact.ContentJSON)
		result[string(artifact.ArtifactType)] = entry
	}
	return result
}

func checkpointInputs(checkpoints []model.Checkpoint) []map[string]any {
	items := make([]map[string]any, 0, len(checkpoints))
	for _, checkpoint := range checkpoints {
		items = append(items, map[string]any{
			"id":         checkpoint.ID,
			"stageRunId": checkpoint.StageRunID,
			"status":     checkpoint.Status,
			"decision":   checkpoint.Decision,
			"comment":    checkpoint.Comment,
		})
	}
	return items
}

func mergeJSONMap(target map[string]any, raw string) {
	if raw == "" {
		return
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return
	}
	for key, value := range payload {
		target[key] = value
	}
}

func RequireRunID(runID string) error {
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	return nil
}
