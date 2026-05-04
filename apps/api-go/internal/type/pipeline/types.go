package pipelinetype

import (
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
)

type CreatePipelineRunRequest struct {
	TemplateID      string   `json:"templateId"`
	Title           string   `json:"title" binding:"required"`
	RequirementText string   `json:"requirementText"`
	TargetRepo      string   `json:"targetRepo"`
	TargetBranch    string   `json:"targetBranch"`
	SelectedDocUrls []string `json:"selectedDocUrls"`
}

type CreatePipelineRunFromSessionRequest struct {
	SessionID    string `json:"sessionId" binding:"required"`
	TemplateID   string `json:"templateId"`
	TargetRepo   string `json:"targetRepo"`
	TargetBranch string `json:"targetBranch"`
}

type UpdateCheckpointDecisionRequest struct {
	Comment string `json:"comment"`
}

type UpdateRunStatusRequest struct {
	Status string `json:"status"`
}

// ExecuteChangesRequest 变更执行请求
type ExecuteChangesRequest struct {
	ChangeSet []ChangeItemRequest `json:"changeSet" binding:"required"`
	Token     string              `json:"token"`
}

type ChangeItemRequest struct {
	FilePath        string `json:"filePath" binding:"required"`
	ChangeType      string `json:"changeType"`
	Reason          string `json:"reason"`
	ProposedPatch   string `json:"proposedPatch"`
	ContextIncluded bool   `json:"contextIncluded"`
	OriginalContent string `json:"originalContent"`
	ProposedDiff    string `json:"proposedDiff"`
}

type ExecuteChangesResponse struct {
	AppliedFiles []string           `json:"appliedFiles"`
	FailedFiles  []map[string]any   `json:"failedFiles"`
	Summary      string             `json:"summary"`
}

type ExecuteChangesEnvelope struct {
	Data  ExecuteChangesResponse `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

type RunStageListResponse struct {
	Stages []StageRunResponse `json:"stages"`
}

type RunArtifactListResponse struct {
	Artifacts []ArtifactResponse `json:"artifacts"`
}

type RunCheckpointListResponse struct {
	Checkpoints []CheckpointResponse `json:"checkpoints"`
}

type RunAgentRunListResponse struct {
	AgentRuns []AgentRunResponse `json:"agentRuns"`
}

type RunGitDeliveryListResponse struct {
	Deliveries []GitDeliveryResponse `json:"deliveries"`
}

type RunStatusResponse struct {
	ID              string                  `json:"id"`
	Status          model.PipelineRunStatus `json:"status"`
	CurrentStageKey string                  `json:"currentStageKey"`
	StartedAt       *time.Time              `json:"startedAt,omitempty"`
	FinishedAt      *time.Time              `json:"finishedAt,omitempty"`
	UpdatedAt       time.Time               `json:"updatedAt"`
}

type PipelineTemplateResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type PipelineRunResponse struct {
	ID              string                  `json:"id"`
	TemplateID      string                  `json:"templateId"`
	Title           string                  `json:"title"`
	RequirementText string                  `json:"requirementText"`
	SourceSessionID string                  `json:"sourceSessionId,omitempty"`
	TargetRepo      string                  `json:"targetRepo"`
	TargetBranch    string                  `json:"targetBranch"`
	WorkBranch      string                  `json:"workBranch"`
	Status          model.PipelineRunStatus `json:"status"`
	CurrentStageKey string                  `json:"currentStageKey"`
	CreatedBy       string                  `json:"createdBy"`
	StartedAt       *time.Time              `json:"startedAt,omitempty"`
	FinishedAt      *time.Time              `json:"finishedAt,omitempty"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
}

type StageRunResponse struct {
	ID            string               `json:"id"`
	PipelineRunID string               `json:"pipelineRunId"`
	StageKey      string               `json:"stageKey"`
	StageType     model.StageType      `json:"stageType"`
	Status        model.StageRunStatus `json:"status"`
	Attempt       int                  `json:"attempt"`
	InputJSON     string               `json:"inputJson,omitempty"`
	OutputJSON    string               `json:"outputJson,omitempty"`
	ErrorMessage  string               `json:"errorMessage,omitempty"`
	StartedAt     *time.Time           `json:"startedAt,omitempty"`
	FinishedAt    *time.Time           `json:"finishedAt,omitempty"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
}

type ArtifactResponse struct {
	ID            string             `json:"id"`
	PipelineRunID string             `json:"pipelineRunId"`
	StageRunID    string             `json:"stageRunId,omitempty"`
	ArtifactType  model.ArtifactType `json:"artifactType"`
	Title         string             `json:"title"`
	ContentText   string             `json:"contentText,omitempty"`
	ContentJSON   string             `json:"contentJson,omitempty"`
	FilePath      string             `json:"filePath,omitempty"`
	MetaJSON      string             `json:"metaJson,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
}

type CheckpointResponse struct {
	ID             string                 `json:"id"`
	PipelineRunID  string                 `json:"pipelineRunId"`
	StageRunID     string                 `json:"stageRunId,omitempty"`
	CheckpointType model.CheckpointType   `json:"checkpointType"`
	Status         model.CheckpointStatus `json:"status"`
	ApproverID     string                 `json:"approverId,omitempty"`
	Decision       string                 `json:"decision,omitempty"`
	Comment        string                 `json:"comment,omitempty"`
	DecidedAt      *time.Time             `json:"decidedAt,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
}

type AgentRunResponse struct {
	ID             string               `json:"id"`
	PipelineRunID  string               `json:"pipelineRunId"`
	StageRunID     string               `json:"stageRunId,omitempty"`
	AgentKey       string               `json:"agentKey"`
	Provider       string               `json:"provider,omitempty"`
	Model          string               `json:"model,omitempty"`
	PromptSnapshot string               `json:"promptSnapshot,omitempty"`
	InputJSON      string               `json:"inputJson,omitempty"`
	OutputJSON     string               `json:"outputJson,omitempty"`
	TokenUsageJSON string               `json:"tokenUsageJson,omitempty"`
	LatencyMS      int64                `json:"latencyMs"`
	Status         model.AgentRunStatus `json:"status"`
	ErrorMessage   string               `json:"errorMessage,omitempty"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

type GitDeliveryResponse struct {
	ID               string                  `json:"id"`
	PipelineRunID    string                  `json:"pipelineRunId"`
	Provider         string                  `json:"provider"`
	Repo             string                  `json:"repo"`
	BaseBranch       string                  `json:"baseBranch"`
	HeadBranch       string                  `json:"headBranch"`
	CommitSHA        string                  `json:"commitSha,omitempty"`
	PRMRURL          string                  `json:"prmrUrl,omitempty"`
	PRMRTitle        string                  `json:"prmrTitle,omitempty"`
	PRMRBody         string                  `json:"prmrBody,omitempty"`
	ChangedFilesJSON string                  `json:"changedFilesJson,omitempty"`
	ValidationJSON   string                  `json:"validationJson,omitempty"`
	SummaryMarkdown  string                  `json:"summaryMarkdown,omitempty"`
	Status           model.GitDeliveryStatus `json:"status"`
	CreatedAt        time.Time               `json:"createdAt"`
	UpdatedAt        time.Time               `json:"updatedAt"`
}

type PipelineRunDetailResponse struct {
	Run         PipelineRunResponse  `json:"run"`
	Stages      []StageRunResponse   `json:"stages"`
	Artifacts   []ArtifactResponse   `json:"artifacts"`
	Checkpoints []CheckpointResponse `json:"checkpoints"`
}

type PipelineRunTimelineSummaryResponse struct {
	TotalStages      int        `json:"totalStages"`
	CompletedStages  int        `json:"completedStages"`
	FailedStages     int        `json:"failedStages"`
	WaitingApproval  bool       `json:"waitingApproval"`
	CurrentStageKey  string     `json:"currentStageKey"`
	LatestArtifactID string     `json:"latestArtifactId,omitempty"`
	LatestDeliveryID string     `json:"latestDeliveryId,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	DurationMS       int64      `json:"durationMs,omitempty"`
}

type PipelineRunCurrentResponse struct {
	Run        PipelineRunResponse  `json:"run"`
	Stage      *StageRunResponse    `json:"stage,omitempty"`
	Artifact   *ArtifactResponse    `json:"artifact,omitempty"`
	Checkpoint *CheckpointResponse  `json:"checkpoint,omitempty"`
	AgentRun   *AgentRunResponse    `json:"agentRun,omitempty"`
	Delivery   *GitDeliveryResponse `json:"delivery,omitempty"`
	NextAction string               `json:"nextAction"`
}

type PipelineRunTimelineResponse struct {
	Run         PipelineRunResponse                `json:"run"`
	Current     *PipelineRunCurrentResponse        `json:"current,omitempty"`
	Stages      []StageRunResponse                 `json:"stages"`
	Artifacts   []ArtifactResponse                 `json:"artifacts"`
	Checkpoints []CheckpointResponse               `json:"checkpoints"`
	AgentRuns   []AgentRunResponse                 `json:"agentRuns"`
	Deliveries  []GitDeliveryResponse              `json:"deliveries"`
	Summary     PipelineRunTimelineSummaryResponse `json:"summary"`
}

type ErrorEnvelope struct {
	Error string `json:"error,omitempty"`
}

type PipelineTemplateListEnvelope struct {
	Data  []PipelineTemplateResponse `json:"data,omitempty"`
	Error string                     `json:"error,omitempty"`
}

type PipelineRunListEnvelope struct {
	Data  []PipelineRunResponse `json:"data,omitempty"`
	Error string                `json:"error,omitempty"`
}

type PipelineRunDetailEnvelope struct {
	Data  PipelineRunDetailResponse `json:"data,omitempty"`
	Error string                    `json:"error,omitempty"`
}

type PipelineRunTimelineEnvelope struct {
	Data  PipelineRunTimelineResponse `json:"data,omitempty"`
	Error string                      `json:"error,omitempty"`
}

type PipelineRunCurrentEnvelope struct {
	Data  PipelineRunCurrentResponse `json:"data,omitempty"`
	Error string                     `json:"error,omitempty"`
}

type RunStageListEnvelope struct {
	Data  RunStageListResponse `json:"data,omitempty"`
	Error string               `json:"error,omitempty"`
}

type RunArtifactListEnvelope struct {
	Data  RunArtifactListResponse `json:"data,omitempty"`
	Error string                  `json:"error,omitempty"`
}

type RunCheckpointListEnvelope struct {
	Data  RunCheckpointListResponse `json:"data,omitempty"`
	Error string                    `json:"error,omitempty"`
}

type RunAgentRunListEnvelope struct {
	Data  RunAgentRunListResponse `json:"data,omitempty"`
	Error string                  `json:"error,omitempty"`
}

type RunGitDeliveryListEnvelope struct {
	Data  RunGitDeliveryListResponse `json:"data,omitempty"`
	Error string                     `json:"error,omitempty"`
}

type GitDeliveryEnvelope struct {
	Data  GitDeliveryResponse `json:"data,omitempty"`
	Error string              `json:"error,omitempty"`
}

type RunStatusEnvelope struct {
	Data  RunStatusResponse `json:"data,omitempty"`
	Error string            `json:"error,omitempty"`
}

type CheckpointEnvelope struct {
	Data  CheckpointResponse `json:"data,omitempty"`
	Error string             `json:"error,omitempty"`
}

func NewPipelineTemplateResponse(item model.PipelineTemplate) PipelineTemplateResponse {
	return PipelineTemplateResponse{ID: item.ID, Name: item.Name, Description: item.Description, Version: item.Version, IsActive: item.IsActive, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func NewPipelineRunResponse(item model.PipelineRun) PipelineRunResponse {
	return PipelineRunResponse{ID: item.ID, TemplateID: item.TemplateID, Title: item.Title, RequirementText: item.RequirementText, SourceSessionID: item.SourceSessionID, TargetRepo: item.TargetRepo, TargetBranch: item.TargetBranch, WorkBranch: item.WorkBranch, Status: item.Status, CurrentStageKey: item.CurrentStageKey, CreatedBy: item.CreatedBy, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func NewStageRunResponse(item model.StageRun) StageRunResponse {
	return StageRunResponse{ID: item.ID, PipelineRunID: item.PipelineRunID, StageKey: item.StageKey, StageType: item.StageType, Status: item.Status, Attempt: item.Attempt, InputJSON: item.InputJSON, OutputJSON: item.OutputJSON, ErrorMessage: item.ErrorMessage, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func NewArtifactResponse(item model.Artifact) ArtifactResponse {
	return ArtifactResponse{ID: item.ID, PipelineRunID: item.PipelineRunID, StageRunID: item.StageRunID, ArtifactType: item.ArtifactType, Title: item.Title, ContentText: item.ContentText, ContentJSON: item.ContentJSON, FilePath: item.FilePath, MetaJSON: item.MetaJSON, CreatedAt: item.CreatedAt}
}

func NewCheckpointResponse(item model.Checkpoint) CheckpointResponse {
	return CheckpointResponse{ID: item.ID, PipelineRunID: item.PipelineRunID, StageRunID: item.StageRunID, CheckpointType: item.CheckpointType, Status: item.Status, ApproverID: item.ApproverID, Decision: item.Decision, Comment: item.Comment, DecidedAt: item.DecidedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func NewAgentRunResponse(item model.AgentRun) AgentRunResponse {
	return AgentRunResponse{ID: item.ID, PipelineRunID: item.PipelineRunID, StageRunID: item.StageRunID, AgentKey: item.AgentKey, Provider: item.Provider, Model: item.Model, PromptSnapshot: item.PromptSnapshot, InputJSON: item.InputJSON, OutputJSON: item.OutputJSON, TokenUsageJSON: item.TokenUsageJSON, LatencyMS: item.LatencyMS, Status: item.Status, ErrorMessage: item.ErrorMessage, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func NewGitDeliveryResponse(item model.GitDelivery) GitDeliveryResponse {
	return GitDeliveryResponse{ID: item.ID, PipelineRunID: item.PipelineRunID, Provider: item.Provider, Repo: item.Repo, BaseBranch: item.BaseBranch, HeadBranch: item.HeadBranch, CommitSHA: item.CommitSHA, PRMRURL: item.PRMRURL, PRMRTitle: item.PRMRTitle, PRMRBody: item.PRMRBody, ChangedFilesJSON: item.ChangedFilesJSON, ValidationJSON: item.ValidationJSON, SummaryMarkdown: item.SummaryMarkdown, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
