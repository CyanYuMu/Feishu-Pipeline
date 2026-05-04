package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/service"
	pipelinetype "feishu-pipeline/apps/api-go/internal/type/pipeline"

	"github.com/gin-gonic/gin"
)

type PipelineController struct {
	pipelineService *service.PipelineService
}

func NewPipelineController(pipelineService *service.PipelineService) *PipelineController {
	return &PipelineController{pipelineService: pipelineService}
}

// ListTemplates
// @tags Pipeline
// @summary 获取流水线模板列表
// @description 返回当前系统可用的 PipelineTemplate。模板定义了赛题要求的研发流水线阶段、顺序和 checkpoint，用于创建 PipelineRun 时初始化 StageRun、Checkpoint 和初始需求产物。
// @router /api/pipeline-templates [GET]
// @produce application/json
// @success 200 {object} pipelinetype.PipelineTemplateListEnvelope
// @failure 500 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListTemplates(ctx *gin.Context) {
	items, err := c.pipelineService.ListPipelineTemplates(ctx.Request.Context())
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}
	response := make([]pipelinetype.PipelineTemplateResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewPipelineTemplateResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, response)
}

// ListRuns
// @tags Pipeline
// @summary 获取流水线运行列表
// @description 按创建时间倒序返回 PipelineRun 列表。前端工作台可用该接口展示所有研发流水线运行记录、当前阶段、运行状态、目标仓库和工作分支。
// @router /api/pipeline-runs [GET]
// @produce application/json
// @success 200 {object} pipelinetype.PipelineRunListEnvelope
// @failure 500 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListRuns(ctx *gin.Context) {
	items, err := c.pipelineService.ListPipelineRuns(ctx.Request.Context())
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}
	response := make([]pipelinetype.PipelineRunResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewPipelineRunResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, response)
}

// CreateRun
// @tags Pipeline
// @summary 创建流水线运行
// @description 通过自然语言需求创建一个新的 PipelineRun。服务端会按模板事务化初始化 StageRun、Checkpoint 和初始 Artifact，但不会自动开始执行；创建后需调用 start 接口进入队列。targetRepo 为空时默认 self，targetBranch 为空时默认 main。
// @router /api/pipeline-runs [POST]
// @accept application/json
// @produce application/json
// @param req body pipelinetype.CreatePipelineRunRequest true "json入参"
// @success 201 {object} pipelinetype.PipelineRunDetailEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) CreateRun(ctx *gin.Context) {
	var req pipelinetype.CreatePipelineRunRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	// 验证：需求描述和飞书文档至少选一
	if strings.TrimSpace(req.RequirementText) == "" && len(req.SelectedDocUrls) == 0 {
		writeError(ctx, http.StatusBadRequest, errors.New("需求描述和飞书文档至少选一"))
		return
	}
	item, err := c.pipelineService.CreatePipelineRun(ctx.Request.Context(), service.CreatePipelineRunInput{TemplateID: req.TemplateID, Title: req.Title, RequirementText: req.RequirementText, TargetRepo: req.TargetRepo, TargetBranch: req.TargetBranch, SelectedDocUrls: req.SelectedDocUrls, CreatedBy: currentUserID(ctx)})
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusCreated, mapPipelineRunDetail(item))
}

// CreateRunFromSession
// @tags Pipeline
// @summary 从会话创建流水线运行
// @description 将已有需求会话中的消息和摘要汇总为 requirementText，并创建 PipelineRun。该接口用于把需求对话链路桥接到 DevFlow Pipeline，不会自动启动执行。
// @router /api/pipeline-runs/from-session [POST]
// @accept application/json
// @produce application/json
// @param req body pipelinetype.CreatePipelineRunFromSessionRequest true "json入参"
// @success 201 {object} pipelinetype.PipelineRunDetailEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) CreateRunFromSession(ctx *gin.Context) {
	var req pipelinetype.CreatePipelineRunFromSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	item, err := c.pipelineService.CreatePipelineRunFromSession(ctx.Request.Context(), req.SessionID, req.TemplateID, req.TargetRepo, req.TargetBranch, currentUserID(ctx))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusCreated, mapPipelineRunDetail(item))
}

// GetRun
// @tags Pipeline
// @summary 获取流水线运行详情
// @description 返回单个 PipelineRun 的基础详情，以及该运行下的 StageRun、Artifact 和 Checkpoint 列表。适合详情页初始化使用；若需要 AgentRun、GitDelivery、当前动作提示，请使用 timeline/current 接口。
// @router /api/pipeline-runs/{id} [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.PipelineRunDetailEnvelope
// @failure 404 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) GetRun(ctx *gin.Context) {
	item, err := c.pipelineService.GetPipelineRunDetail(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusNotFound, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapPipelineRunDetail(item))
}

// GetRunTimeline
// @tags Pipeline
// @summary 获取流水线工作台时间线
// @description 聚合返回 PipelineRun、所有 StageRun、Artifact、Checkpoint、AgentRun、GitDelivery、当前阶段和 summary。summary 包含阶段统计、等待审批状态、最新产物、最新交付记录和耗时信息，是前端工作台的主数据接口。
// @router /api/pipeline-runs/{id}/timeline [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.PipelineRunTimelineEnvelope
// @failure 404 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) GetRunTimeline(ctx *gin.Context) {
	item, err := c.pipelineService.GetPipelineRunTimeline(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusNotFound, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapPipelineRunTimeline(item))
}

// GetRunCurrent
// @tags Pipeline
// @summary 获取流水线当前阶段详情
// @description 返回当前 PipelineRun 的主操作上下文，包括当前阶段、当前产物、当前 checkpoint、当前 AgentRun、最新 GitDelivery 和 nextAction。nextAction 可用于驱动前端主按钮和审批/交付面板。
// @router /api/pipeline-runs/{id}/current [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.PipelineRunCurrentEnvelope
// @failure 404 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) GetRunCurrent(ctx *gin.Context) {
	item, err := c.pipelineService.GetPipelineRunCurrent(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusNotFound, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapPipelineRunCurrent(item))
}

// ListStages
// @tags Pipeline
// @summary 获取流水线阶段列表
// @description 返回指定 PipelineRun 的全部 StageRun，包含阶段 key、类型、状态、尝试次数、输入输出 JSON、错误信息和开始/结束时间。用于展示阶段流转和调试单个阶段状态。
// @router /api/pipeline-runs/{id}/stages [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunStageListEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListStages(ctx *gin.Context) {
	items, err := c.pipelineService.ListStageRuns(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	response := make([]pipelinetype.StageRunResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewStageRunResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.RunStageListResponse{Stages: response})
}

// ListArtifacts
// @tags Pipeline
// @summary 获取流水线产物列表
// @description 返回指定 PipelineRun 的全部 Artifact。Artifact 是阶段间数据流转的核心载体，包含结构化需求、技术方案、代码变更计划、测试报告、评审报告和交付摘要等。
// @router /api/pipeline-runs/{id}/artifacts [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunArtifactListEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListArtifacts(ctx *gin.Context) {
	items, err := c.pipelineService.ListArtifacts(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	response := make([]pipelinetype.ArtifactResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewArtifactResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.RunArtifactListResponse{Artifacts: response})
}

// GetCodeDiff
// @tags Pipeline
// @summary 获取流水线的代码变更集
// @description 返回指定 PipelineRun 最新的代码变更计划，包含结构化的changeSet、标准git格式diff和变更摘要。
// @router /api/pipeline-runs/{id}/code-diff [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} object{changeSet=[]map[string]any, summary=string, updatedAt=time.Time}
// @failure 404 {object} pipelinetype.ErrorEnvelope
// @failure 500 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) GetCodeDiff(ctx *gin.Context) {
	runID := ctx.Param("id")

	// 获取该流水线的所有产物
	artifacts, err := c.pipelineService.ListArtifacts(ctx.Request.Context(), runID)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	// 找到最新的code_diff类型产物
	var codeDiffArtifact *model.Artifact
	for i := len(artifacts) - 1; i >= 0; i-- {
		if artifacts[i].ArtifactType == model.ArtifactCodeDiff {
			codeDiffArtifact = &artifacts[i]
			break
		}
	}

	if codeDiffArtifact == nil {
		writeError(ctx, http.StatusNotFound, errors.New("未找到代码变更记录"))
		return
	}

	// 解析结构化内容
	var content map[string]any
	if err := json.Unmarshal([]byte(codeDiffArtifact.ContentJSON), &content); err != nil {
		writeError(ctx, http.StatusInternalServerError, errors.New("解析变更内容失败"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"changeSet": content["changeSet"],
		"summary":   codeDiffArtifact.ContentText,
		"updatedAt": codeDiffArtifact.UpdatedAt,
	})
}

// ListCheckpoints
// @tags Pipeline
// @summary 获取流水线检查点列表
// @description 返回指定 PipelineRun 的人工检查点列表。当前默认模板包含方案审批和评审确认两个 Human-in-the-Loop 节点，支持 approve/reject 决策。
// @router /api/pipeline-runs/{id}/checkpoints [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunCheckpointListEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListCheckpoints(ctx *gin.Context) {
	items, err := c.pipelineService.ListCheckpoints(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	response := make([]pipelinetype.CheckpointResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewCheckpointResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.RunCheckpointListResponse{Checkpoints: response})
}

// ListAgentRuns
// @tags Pipeline
// @summary 获取流水线 Agent 执行记录
// @description 返回指定 PipelineRun 的 AgentRun 列表。每条记录包含 AgentKey、Provider、Model、PromptSnapshot、输入输出、token usage 占位、耗时、状态和错误信息，用于审计真实 provider 调用、fallback 和阶段执行结果。
// @router /api/pipeline-runs/{id}/agent-runs [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunAgentRunListEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListAgentRuns(ctx *gin.Context) {
	items, err := c.pipelineService.ListAgentRuns(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	response := make([]pipelinetype.AgentRunResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewAgentRunResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.RunAgentRunListResponse{AgentRuns: response})
}

// ListGitDeliveries
// @tags Pipeline
// @summary 获取流水线交付记录列表
// @description 返回指定 PipelineRun 的 GitDelivery 交付草稿列表。当前实现只创建本地可审查的交付记录和 PR/MR 草稿，不执行 git push，也不创建远程 PR/MR。
// @router /api/pipeline-runs/{id}/deliveries [GET]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunGitDeliveryListEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ListGitDeliveries(ctx *gin.Context) {
	items, err := c.pipelineService.ListGitDeliveries(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.RunGitDeliveryListResponse{Deliveries: mapGitDeliveryResponses(items)})
}

// GetGitDelivery
// @tags Pipeline
// @summary 获取单个交付记录详情
// @description 按 deliveryID 查询 GitDelivery 详情，包含目标仓库、基础分支、工作分支、PR/MR 标题草稿、正文草稿、变更文件 JSON、验证摘要和交付状态。该接口用于交付审查页展示。
// @router /api/git-deliveries/{deliveryID} [GET]
// @produce application/json
// @param deliveryID path string true "交付记录ID"
// @success 200 {object} pipelinetype.GitDeliveryEnvelope
// @failure 404 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) GetGitDelivery(ctx *gin.Context) {
	item, err := c.pipelineService.GetGitDelivery(ctx.Request.Context(), ctx.Param("deliveryID"))
	if err != nil {
		writeError(ctx, http.StatusNotFound, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.NewGitDeliveryResponse(item))
}

// StartRun
// @tags Pipeline
// @summary 启动流水线运行
// @description 将 draft 或 failed 状态的 PipelineRun 置为 queued 并投递后台 runner。执行器会顺序运行可执行阶段，遇到 checkpoint 时自动进入 waiting_approval。
// @router /api/pipeline-runs/{id}/start [POST]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunStatusEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) StartRun(ctx *gin.Context) {
	item, err := c.pipelineService.StartPipelineRun(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapRunStatus(item))
}

// PauseRun
// @tags Pipeline
// @summary 暂停流水线运行
// @description 将 queued 或 running 状态的 PipelineRun 暂停。暂停后后台执行器会停止继续推进后续阶段；已完成的阶段和产物不会被删除。
// @router /api/pipeline-runs/{id}/pause [POST]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunStatusEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) PauseRun(ctx *gin.Context) {
	item, err := c.pipelineService.PausePipelineRun(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapRunStatus(item))
}

// ResumeRun
// @tags Pipeline
// @summary 恢复流水线运行
// @description 将 paused 或 failed 状态的 PipelineRun 重新置为 queued 并投递后台 runner。waiting_approval 状态不能通过 resume 绕过审批，必须调用 checkpoint approve/reject。
// @router /api/pipeline-runs/{id}/resume [POST]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunStatusEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ResumeRun(ctx *gin.Context) {
	item, err := c.pipelineService.ResumePipelineRun(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapRunStatus(item))
}

// TerminateRun
// @tags Pipeline
// @summary 终止流水线运行
// @description 将未完成的 PipelineRun 标记为 terminated。terminated 和 completed 都是终态，不能再次 start/resume；该接口不会删除已有产物、AgentRun 或交付记录。
// @router /api/pipeline-runs/{id}/terminate [POST]
// @produce application/json
// @param id path string true "流水线运行ID"
// @success 200 {object} pipelinetype.RunStatusEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) TerminateRun(ctx *gin.Context) {
	item, err := c.pipelineService.TerminatePipelineRun(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, mapRunStatus(item))
}

// ApproveCheckpoint
// @tags Checkpoint
// @summary 审批通过检查点
// @description 对 pending 且绑定到 waiting_approval Run/Stage 的 checkpoint 做 approve 决策。审批通过后 checkpoint stage 会标记 succeeded，PipelineRun 重新 queued 并继续后续阶段；重复审批会被拒绝。
// @router /api/checkpoints/{checkpointID}/approve [POST]
// @accept application/json
// @produce application/json
// @param checkpointID path string true "检查点ID"
// @param req body pipelinetype.UpdateCheckpointDecisionRequest true "json入参"
// @success 200 {object} pipelinetype.CheckpointEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ApproveCheckpoint(ctx *gin.Context) {
	var req pipelinetype.UpdateCheckpointDecisionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	item, err := c.pipelineService.ApproveCheckpoint(ctx.Request.Context(), ctx.Param("checkpointID"), req.Comment, currentUserID(ctx))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.NewCheckpointResponse(item))
}

// RejectCheckpoint
// @tags Checkpoint
// @summary 驳回检查点
// @description 对 pending 且绑定到 waiting_approval Run/Stage 的 checkpoint 做 reject 决策。驳回后会回退到上一可执行阶段，携带 reject comment 作为重做上下文，并将后续阶段与产物标记为待重跑/已 superseded。
// @router /api/checkpoints/{checkpointID}/reject [POST]
// @accept application/json
// @produce application/json
// @param checkpointID path string true "检查点ID"
// @param req body pipelinetype.UpdateCheckpointDecisionRequest true "json入参"
// @success 200 {object} pipelinetype.CheckpointEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) RejectCheckpoint(ctx *gin.Context) {
	var req pipelinetype.UpdateCheckpointDecisionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	item, err := c.pipelineService.RejectCheckpoint(ctx.Request.Context(), ctx.Param("checkpointID"), req.Comment, currentUserID(ctx))
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.NewCheckpointResponse(item))
}

// ExecuteChanges
// @tags Pipeline
// @summary 执行代码变更
// @description 审批通过后执行变更计划，将代码变更写入 GitHub 仓库。
// @router /api/pipeline-runs/{id}/execute-changes [POST]
// @accept application/json
// @produce application/json
// @param id path string true "流水线运行ID"
// @param req body pipelinetype.ExecuteChangesRequest true "变更执行参数"
// @success 200 {object} pipelinetype.ExecuteChangesEnvelope
// @failure 400 {object} pipelinetype.ErrorEnvelope
func (c *PipelineController) ExecuteChanges(ctx *gin.Context) {
	var req pipelinetype.ExecuteChangesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	changeSet := make([]map[string]any, 0, len(req.ChangeSet))
	for _, item := range req.ChangeSet {
		changeSet = append(changeSet, map[string]any{
			"filePath":        item.FilePath,
			"changeType":      item.ChangeType,
			"reason":          item.Reason,
			"proposedPatch":   item.ProposedPatch,
			"contextIncluded": item.ContextIncluded,
			"originalContent": item.OriginalContent,
			"proposedDiff":    item.ProposedDiff,
		})
	}
	params := service.ExecuteChangesParams{
		ChangeSet:   changeSet,
		CommitterID: currentUserID(ctx),
		Token:       req.Token,
	}
	result, err := c.pipelineService.ExecuteChanges(ctx.Request.Context(), ctx.Param("id"), params)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, pipelinetype.ExecuteChangesResponse{
		AppliedFiles: result.AppliedFiles,
		FailedFiles:  result.FailedFiles,
		Summary:      result.Summary,
	})
}

func mapRunStatus(item model.PipelineRun) pipelinetype.RunStatusResponse {
	return pipelinetype.RunStatusResponse{ID: item.ID, Status: item.Status, CurrentStageKey: item.CurrentStageKey, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, UpdatedAt: item.UpdatedAt}
}

func mapPipelineRunDetail(item *service.PipelineRunDetail) pipelinetype.PipelineRunDetailResponse {
	stages := mapStageRunResponses(item.Stages)
	artifacts := mapArtifactResponses(item.Artifacts)
	checkpoints := mapCheckpointResponses(item.Checkpoints)
	return pipelinetype.PipelineRunDetailResponse{Run: pipelinetype.NewPipelineRunResponse(item.Run), Stages: stages, Artifacts: artifacts, Checkpoints: checkpoints}
}

func mapPipelineRunTimeline(item *service.PipelineRunTimeline) pipelinetype.PipelineRunTimelineResponse {
	agentRuns := make([]pipelinetype.AgentRunResponse, 0, len(item.AgentRuns))
	for _, agentRun := range item.AgentRuns {
		agentRuns = append(agentRuns, pipelinetype.NewAgentRunResponse(agentRun))
	}
	return pipelinetype.PipelineRunTimelineResponse{
		Run:         pipelinetype.NewPipelineRunResponse(item.Run),
		Current:     mapPipelineRunCurrent(item.Current),
		Stages:      mapStageRunResponses(item.Stages),
		Artifacts:   mapArtifactResponses(item.Artifacts),
		Checkpoints: mapCheckpointResponses(item.Checkpoints),
		AgentRuns:   agentRuns,
		Deliveries:  mapGitDeliveryResponses(item.Deliveries),
		Summary:     pipelinetype.PipelineRunTimelineSummaryResponse{TotalStages: item.Summary.TotalStages, CompletedStages: item.Summary.CompletedStages, FailedStages: item.Summary.FailedStages, WaitingApproval: item.Summary.WaitingApproval, CurrentStageKey: item.Summary.CurrentStageKey, LatestArtifactID: item.Summary.LatestArtifactID, LatestDeliveryID: item.Summary.LatestDeliveryID, StartedAt: item.Summary.StartedAt, FinishedAt: item.Summary.FinishedAt, DurationMS: item.Summary.DurationMS},
	}
}

func mapPipelineRunCurrent(item *service.PipelineRunCurrent) *pipelinetype.PipelineRunCurrentResponse {
	if item == nil {
		return nil
	}
	response := &pipelinetype.PipelineRunCurrentResponse{Run: pipelinetype.NewPipelineRunResponse(item.Run)}
	if item.Stage != nil {
		stage := pipelinetype.NewStageRunResponse(*item.Stage)
		response.Stage = &stage
	}
	if item.Artifact != nil {
		artifact := pipelinetype.NewArtifactResponse(*item.Artifact)
		response.Artifact = &artifact
	}
	if item.Checkpoint != nil {
		checkpoint := pipelinetype.NewCheckpointResponse(*item.Checkpoint)
		response.Checkpoint = &checkpoint
	}
	if item.AgentRun != nil {
		agentRun := pipelinetype.NewAgentRunResponse(*item.AgentRun)
		response.AgentRun = &agentRun
	}
	if item.Delivery != nil {
		delivery := pipelinetype.NewGitDeliveryResponse(*item.Delivery)
		response.Delivery = &delivery
	}
	response.NextAction = item.NextAction
	return response
}

func mapStageRunResponses(items []model.StageRun) []pipelinetype.StageRunResponse {
	response := make([]pipelinetype.StageRunResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewStageRunResponse(item))
	}
	return response
}

func mapArtifactResponses(items []model.Artifact) []pipelinetype.ArtifactResponse {
	response := make([]pipelinetype.ArtifactResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewArtifactResponse(item))
	}
	return response
}

func mapCheckpointResponses(items []model.Checkpoint) []pipelinetype.CheckpointResponse {
	response := make([]pipelinetype.CheckpointResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewCheckpointResponse(item))
	}
	return response
}

func mapGitDeliveryResponses(items []model.GitDelivery) []pipelinetype.GitDeliveryResponse {
	response := make([]pipelinetype.GitDeliveryResponse, 0, len(items))
	for _, item := range items {
		response = append(response, pipelinetype.NewGitDeliveryResponse(item))
	}
	return response
}
