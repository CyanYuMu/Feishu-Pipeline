package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OpenAPIController struct {
	repo *repo.Repository
}

func NewOpenAPIController(repo *repo.Repository) *OpenAPIController {
	return &OpenAPIController{repo: repo}
}

// SaveSpec
// @tags OpenAPI
// @summary 保存 OpenAPI 规范
// @router /api/openapi/specs [POST]
// @produce application/json
// @accept application/json
// @param body body SaveSpecRequest true "OpenAPI 规范请求"
func (c *OpenAPIController) SaveSpec(ctx *gin.Context) {
	var req SaveSpecRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	specID := "spec_" + uuid.New().String()[:8]
	
	// 获取前端 URL
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	swaggerURL := frontendURL + "/swagger?specId=" + specID

	spec := &model.OpenAPISpec{
		ID:         specID,
		ProjectID:  req.ProjectID,
		Title:      req.Title,
		Description: req.Description,
		SpecJSON:   req.SpecJSON,
		SwaggerURL: swaggerURL,
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := c.repo.CreateOpenAPISpec(ctx.Request.Context(), spec); err != nil {
		writeError(ctx, http.StatusInternalServerError, errors.New("failed to save spec"))
		return
	}

	writeSuccess(ctx, http.StatusOK, gin.H{
		"specId":     specID,
		"swaggerUrl": swaggerURL,
	})
}

// GetSpec
// @tags OpenAPI
// @summary 获取 OpenAPI 规范
// @router /api/openapi/specs/:specId [GET]
// @produce json
// @param specId path string true "Spec ID"
func (c *OpenAPIController) GetSpec(ctx *gin.Context) {
	specID := ctx.Param("specId")

	spec, err := c.repo.GetOpenAPISpec(ctx.Request.Context(), specID)
	if err != nil {
		writeError(ctx, http.StatusNotFound, errors.New("spec not found"))
		return
	}

	writeSuccess(ctx, http.StatusOK, spec.SpecJSON)
}

// SaveSpecPublic 不需要认证的保存接口（供 AI 工具调用）
func (c *OpenAPIController) SaveSpecPublic(ctx *gin.Context) {
	var req SaveSpecRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	specID := "spec_" + uuid.New().String()[:8]

	// 获取前端 URL
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	swaggerURL := frontendURL + "/swagger?specId=" + specID

	spec := &model.OpenAPISpec{
		ID:         specID,
		ProjectID:  req.ProjectID,
		Title:      req.Title,
		Description: req.Description,
		SpecJSON:   req.SpecJSON,
		SwaggerURL: swaggerURL,
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := c.repo.CreateOpenAPISpec(ctx.Request.Context(), spec); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to save spec"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"specId":     specID,
			"swaggerUrl": swaggerURL,
			"projectId":  req.ProjectID,
		},
	})
}

// GetSpecPublic 不需要认证的获取接口（供 Swagger UI 调用）
func (c *OpenAPIController) GetSpecPublic(ctx *gin.Context) {
	specID := ctx.Param("specId")

	spec, err := c.repo.GetOpenAPISpec(ctx.Request.Context(), specID)
	if err != nil {
		writeError(ctx, http.StatusNotFound, errors.New("spec not found"))
		return
	}

	// 直接返回 JSON，不包装
	ctx.Header("Content-Type", "application/json")
	ctx.String(http.StatusOK, spec.SpecJSON)
}

// ListSpecsPublic 不需要认证的列表接口（供前端资产页面调用）
func (c *OpenAPIController) ListSpecsPublic(ctx *gin.Context) {
	specs, err := c.repo.ListOpenAPISpecs(ctx.Request.Context())
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, errors.New("failed to list specs"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": specs,
	})
}

// UpdateSpecPublic 更新规范的关联信息（不需要认证，供前端调用）
func (c *OpenAPIController) UpdateSpecPublic(ctx *gin.Context) {
	specID := ctx.Param("specId")
	
	var req UpdateSpecRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}
	
	// 构建更新数据
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.DocUrls != nil {
		// 将数组转为 JSON 字符串存储
		docUrlsJSON, _ := json.Marshal(req.DocUrls)
		updates["doc_urls"] = string(docUrlsJSON)
	}
	
	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "no fields to update"})
		return
	}
	
	if err := c.repo.UpdateOpenAPISpec(ctx.Request.Context(), specID, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to update spec"})
		return
	}
	
	ctx.JSON(http.StatusOK, gin.H{"success": true, "message": "spec updated"})
}

// ─── Project 项目接口 ─────────────────────────────────────────────────────────

// CreateProject 创建项目
func (c *OpenAPIController) CreateProject(ctx *gin.Context) {
	var req CreateProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	projectID := "proj_" + uuid.New().String()[:8]
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	swaggerURL := frontendURL + "/swagger?projectId=" + projectID

	project := &model.Project{
		ID:         projectID,
		Title:      req.Title,
		Description: req.Description,
		SwaggerURL: swaggerURL,
		GitHubRepo: req.GitHubRepo,
		BaseModel: model.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := c.repo.CreateProject(ctx.Request.Context(), project); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create project"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"projectId":  projectID,
			"swaggerUrl": swaggerURL,
		},
	})
}

// ListProjects 列出所有项目
func (c *OpenAPIController) ListProjects(ctx *gin.Context) {
	projects, err := c.repo.ListProjects(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to list projects"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": projects,
	})
}

// GetProject 获取项目详情
func (c *OpenAPIController) GetProject(ctx *gin.Context) {
	projectID := ctx.Param("projectId")

	project, err := c.repo.GetProject(ctx.Request.Context(), projectID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"success": false, "error": "project not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": project,
	})
}

// UpdateProject 更新项目
func (c *OpenAPIController) UpdateProject(ctx *gin.Context) {
	projectID := ctx.Param("projectId")

	var req UpdateProjectRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.DocUrls != nil {
		docUrlsJSON, _ := json.Marshal(req.DocUrls)
		updates["doc_urls"] = string(docUrlsJSON)
	}
	// GitHubRepo 允许空字符串（解绑）
	updates["github_repo"] = req.GitHubRepo

	if len(updates) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "no fields to update"})
		return
	}

	if err := c.repo.UpdateProject(ctx.Request.Context(), projectID, updates); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to update project"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true, "message": "project updated"})
}

// GetProjectSpecs 获取项目下的所有 API 文档
func (c *OpenAPIController) GetProjectSpecs(ctx *gin.Context) {
	projectID := ctx.Param("projectId")

	specs, err := c.repo.ListOpenAPISpecsByProject(ctx.Request.Context(), projectID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to list specs"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": specs,
	})
}

type CreateProjectRequest struct {
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	DocUrls     []string `json:"doc_urls"`
	GitHubRepo  string   `json:"github_repo"`
}

type UpdateProjectRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DocUrls     []string `json:"doc_urls"`
	GitHubRepo  string   `json:"github_repo"`
}

// SaveSpecRequest 保存规范的请求
type SaveSpecRequest struct {
	Title       string `json:"title"`
	SpecJSON    string `json:"spec_json" binding:"required"`
	ProjectID   string `json:"project_id"`
	Description string `json:"description"`
	DocUrls     string `json:"doc_urls"`
}

// UpdateSpecRequest 更新规范的请求
type UpdateSpecRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	ProjectID   string   `json:"project_id"`
	DocUrls     []string `json:"doc_urls"`
}
