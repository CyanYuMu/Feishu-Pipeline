package controller

import (
	"errors"
	"net/http"

	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/service"
	admintype "feishu-pipeline/apps/api-go/internal/type/admin"

	"github.com/gin-gonic/gin"
)

type AdminController struct {
	adminService *service.AdminService
	feishuClient *feishu.Client
}

func NewAdminController(adminService *service.AdminService) *AdminController {
	return &AdminController{adminService: adminService}
}

func (c *AdminController) SetFeishuClient(client *feishu.Client) {
	c.feishuClient = client
}

// CreateRoleMapping
// @tags 后台管理
// @summary 创建或保存角色映射规则
// @router /api/admin/role-mappings [POST]
// @accept application/json
// @produce application/json
// @param req body admintype.CreateRoleMappingRequest true "json入参"
func (c *AdminController) CreateRoleMapping(ctx *gin.Context) {
	var request admintype.CreateRoleMappingRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	err := c.adminService.SaveRoleMapping(ctx.Request.Context(), &model.RoleMapping{
		Name:        request.Name,
		Keyword:     request.Keyword,
		Role:        request.Role,
		Departments: request.Departments,
	})
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusCreated, admintype.CreateRoleMappingResponse{Status: "saved"})
}

// SaveRoleOwner
// @tags 后台管理
// @summary 保存角色负责人
// @router /api/admin/role-owners [POST]
// @accept application/json
// @produce application/json
// @param req body admintype.SaveRoleOwnerRequest true "json入参"
func (c *AdminController) SaveRoleOwner(ctx *gin.Context) {
	var request admintype.SaveRoleOwnerRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	err := c.adminService.SaveRoleOwner(ctx.Request.Context(), &model.RoleOwner{
		Role:         request.Role,
		OwnerName:    request.OwnerName,
		FeishuID:     request.FeishuID,
		FeishuIDType: request.FeishuIDType,
		Enabled:      request.Enabled,
	})
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusCreated, map[string]string{"status": "saved"})
}

// ListRoleOwners
// @tags 后台管理
// @summary 角色负责人列表
// @router /api/admin/role-owners [GET]
// @produce application/json
func (c *AdminController) ListRoleOwners(ctx *gin.Context) {
	items, err := c.adminService.ListRoleOwners(ctx.Request.Context())
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	response := make([]admintype.RoleOwnerResponse, 0, len(items))
	for _, item := range items {
		response = append(response, admintype.NewRoleOwnerResponse(item))
	}
	writeSuccess(ctx, http.StatusOK, response)
}

// SyncKnowledge
// @tags 后台管理
// @summary 同步知识库来源
// @router /api/admin/knowledge/sync [POST]
// @accept application/json
// @produce application/json
// @param req body admintype.SyncKnowledgeRequest true "json入参"
func (c *AdminController) SyncKnowledge(ctx *gin.Context) {
	var request admintype.SyncKnowledgeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	items := admintype.NewKnowledgeSourceModels(request.Sources)
	if err := c.adminService.SyncKnowledgeSources(ctx.Request.Context(), items); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}
	writeSuccess(ctx, http.StatusCreated, admintype.SyncKnowledgeResponse{Count: len(items)})
}

// TestApprovalCard 测试发送需求确认卡片
// @tags 后台管理
// @summary 测试发送需求确认卡片
// @router /api/admin/test-approval-card [POST]
// @accept application/json
// @produce application/json
// @param req body TestApprovalCardRequest true "测试卡片请求"
func (c *AdminController) TestApprovalCard(ctx *gin.Context) {
	var req TestApprovalCardRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	if c.feishuClient == nil {
		writeError(ctx, http.StatusServiceUnavailable, errors.New("feishu client not available"))
		return
	}

	if req.OpenID == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("open_id is required"))
		return
	}

	// 构建测试卡片内容
	payload := feishu.ApprovalCardPayload{
		Title:       req.Title,
		Summary:     req.Summary,
		Requirement: req.Requirement,
		SessionID:   req.SessionID,
		RunID:       req.RunID,
	}

	result, err := c.feishuClient.SendApprovalCardMessage(ctx.Request.Context(), req.OpenID, payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, gin.H{
		"message_id": result.RemoteID,
		"status":     result.Status,
	})
}

type TestApprovalCardRequest struct {
	OpenID      string `json:"open_id" binding:"required"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Requirement string `json:"requirement"`
	SessionID   string `json:"session_id"`
	RunID       string `json:"run_id"`
}
