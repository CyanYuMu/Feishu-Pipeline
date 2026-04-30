package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/pipeline"
	agenttype "feishu-pipeline/apps/api-go/internal/type/agent"
	"feishu-pipeline/apps/api-go/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SessionSummary struct {
	Session      model.Session
	OwnerName    string
	MessageCount int
}

type SessionAggregate = agenttype.SessionAggregate

type PublishResult struct {
	Requirement model.Requirement
	Tasks       []model.Task
	Deliveries  []model.MessageDelivery
}

type FeishuLoginState struct {
	User       model.User
	Credential model.FeishuCredential
	Session    model.LoginSession
}

type Repository struct {
	db *gorm.DB
}

func NewSQLiteRepository(databasePath string) (*Repository, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite with gorm: %w", err)
	}

	repository := &Repository{db: db}
	if err := repository.AutoMigrate(context.Background()); err != nil {
		return nil, err
	}
	if err := repository.Seed(context.Background()); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (r *Repository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(
		&model.User{},
		&model.FeishuCredential{},
		&model.LoginSession{},
		&model.Session{},
		&model.Message{},
		&model.Requirement{},
		&model.Task{},
		&model.RoleMapping{},
		&model.RoleOwner{},
		&model.KnowledgeSource{},
		&model.MessageDelivery{},
		&model.OpenAPISpec{},
		&model.Project{},
		&model.PipelineTemplate{},
		&model.PipelineRun{},
		&model.StageRun{},
		&model.Artifact{},
		&model.Checkpoint{},
		&model.AgentRun{},
		&model.GitDelivery{},
		&model.InPageEditSession{},
	)
}

func (r *Repository) Seed(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.RoleMapping{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			mappings := []model.RoleMapping{
				{ID: "rm_product", Name: "产品角色", Keyword: "产品", Role: model.RoleProduct, Departments: []string{"产品部"}},
				{ID: "rm_frontend", Name: "前端角色", Keyword: "前端", Role: model.RoleFrontend, Departments: []string{"前端部"}},
				{ID: "rm_backend", Name: "后端角色", Keyword: "后端", Role: model.RoleBackend, Departments: []string{"后端部"}},
			}
			if err := tx.Create(&mappings).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&model.RoleOwner{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			now := time.Now().UTC()
			owners := []model.RoleOwner{
				{ID: "ro_product", Role: model.RoleProduct, OwnerName: "产品负责人", FeishuIDType: "user_id", Enabled: false, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
				{ID: "ro_frontend", Role: model.RoleFrontend, OwnerName: "前端负责人", FeishuIDType: "user_id", Enabled: false, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
				{ID: "ro_backend", Role: model.RoleBackend, OwnerName: "后端负责人", FeishuIDType: "user_id", Enabled: false, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
				{ID: "ro_admin", Role: model.RoleAdmin, OwnerName: "管理员", FeishuIDType: "user_id", Enabled: false, BaseModel: model.BaseModel{CreatedAt: now, UpdatedAt: now}},
			}
			if err := tx.Create(&owners).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&model.KnowledgeSource{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			now := time.Now().UTC()
			items := []model.KnowledgeSource{
				{ID: "ks_api", Title: "接口规范", Content: "接口返回统一 JSON 包装，错误不暴露内部堆栈，状态码与业务状态分离。", UpdatedAt: now},
				{ID: "ks_ui", Title: "UI 规范", Content: "涉及前端任务时，优先保证列表检索、表单反馈、状态标签与空态说明。", UpdatedAt: now},
				{ID: "ks_delivery", Title: "提测流程", Content: "开发完成后进入已提测状态，并同步测试负责人和需求会话状态。", UpdatedAt: now},
			}
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}

		if err := r.seedPipelineTemplatesTx(ctx, tx); err != nil {
			return err
		}

		return nil
	})
}

func (r *Repository) FindUserByID(ctx context.Context, userID string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", userID).Error
	return user, err
}

func (r *Repository) FindLatestUserByRole(ctx context.Context, role model.Role) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("role = ? AND feishu_open_id <> ''", role).
		Order("updated_at DESC").
		First(&user).Error
	return user, err
}

func (r *Repository) UpsertUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *Repository) SaveFeishuLoginState(ctx context.Context, state *FeishuLoginState) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&state.User).Error; err != nil {
			return err
		}
		if err := tx.Save(&state.Credential).Error; err != nil {
			return err
		}
		if err := tx.Save(&state.Session).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) FindCredentialByUserID(ctx context.Context, userID string) (model.FeishuCredential, error) {
	var credential model.FeishuCredential
	err := r.db.WithContext(ctx).First(&credential, "user_id = ?", userID).Error
	return credential, err
}

func (r *Repository) SaveCredential(ctx context.Context, credential *model.FeishuCredential) error {
	return r.db.WithContext(ctx).Save(credential).Error
}

func (r *Repository) FindLoginSessionByID(ctx context.Context, sessionID string) (model.LoginSession, error) {
	var session model.LoginSession
	err := r.db.WithContext(ctx).First(&session, "id = ?", sessionID).Error
	return session, err
}

func (r *Repository) CreateLoginSession(ctx context.Context, session *model.LoginSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *Repository) DeleteLoginSessionByID(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Delete(&model.LoginSession{}, "id = ?", sessionID).Error
}

func (r *Repository) DeleteExpiredLoginSessions(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Delete(&model.LoginSession{}, "expires_at <= ?", now).Error
}

func (r *Repository) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	var sessions []model.Session
	if err := r.db.WithContext(ctx).Preload("Owner").Order("updated_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}

	items := make([]SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		var count int64
		if err := r.db.WithContext(ctx).Model(&model.Message{}).Where("session_id = ?", session.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		items = append(items, SessionSummary{
			Session:      session,
			OwnerName:    session.Owner.Name,
			MessageCount: int(count),
		})
	}
	return items, nil
}

func (r *Repository) CreateSession(ctx context.Context, owner model.User, title string, prompt string) (model.Session, error) {
	session := model.Session{
		ID:      utils.NewID("sess"),
		Title:   title,
		Summary: prompt,
		Status:  model.SessionDraft,
		OwnerID: owner.ID,
	}
	return session, r.db.WithContext(ctx).Create(&session).Error
}

func (r *Repository) AddMessage(ctx context.Context, sessionID string, role model.MessageRole, content string) (model.Message, error) {
	message := model.Message{
		ID:        utils.NewID("msg"),
		SessionID: sessionID,
		Role:      role,
		Content:   strings.TrimSpace(content),
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&message).Error; err != nil {
		return model.Message{}, err
	}
	if err := r.db.WithContext(ctx).Model(&model.Session{}).Where("id = ?", sessionID).Updates(map[string]any{
		"summary":    utils.Summarize(message.Content, 120),
		"updated_at": message.CreatedAt,
	}).Error; err != nil {
		return model.Message{}, err
	}
	return message, nil
}

func (r *Repository) GetSessionAggregate(ctx context.Context, sessionID string) (*agenttype.SessionAggregate, error) {
	var session model.Session
	if err := r.db.WithContext(ctx).Preload("Owner").First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	var messages []model.Message
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&messages, "session_id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	var requirement model.Requirement
	reqErr := r.db.WithContext(ctx).First(&requirement, "session_id = ?", sessionID).Error
	var reqPtr *model.Requirement
	if reqErr == nil {
		reqPtr = &requirement
	} else if reqErr != gorm.ErrRecordNotFound {
		return nil, reqErr
	}

	var tasks []model.Task
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&tasks, "session_id = ?", sessionID).Error; err != nil {
		return nil, err
	}

	var messageCount int64
	if err := r.db.WithContext(ctx).Model(&model.Message{}).Where("session_id = ?", sessionID).Count(&messageCount).Error; err != nil {
		return nil, err
	}

	return &agenttype.SessionAggregate{
		Session:      session,
		Owner:        session.Owner,
		MessageCount: int(messageCount),
		Messages:     messages,
		Requirement:  reqPtr,
		Tasks:        tasks,
	}, nil
}

func (r *Repository) MarkSessionPublished(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Model(&model.Session{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":     model.SessionPublished,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *Repository) SavePublishResult(ctx context.Context, result PublishResult) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result.Requirement.Status = model.SessionInDelivery
		if err := tx.Where("session_id = ?", result.Requirement.SessionID).Delete(&model.Task{}).Error; err != nil {
			return err
		}

		if err := tx.Where("session_id = ?", result.Requirement.SessionID).
			Assign(result.Requirement).
			FirstOrCreate(&model.Requirement{SessionID: result.Requirement.SessionID}).Error; err != nil {
			return err
		}

		if len(result.Tasks) > 0 {
			if err := tx.Create(&result.Tasks).Error; err != nil {
				return err
			}
		}
		if len(result.Deliveries) > 0 {
			if err := tx.Create(&result.Deliveries).Error; err != nil {
				return err
			}
		}

		return tx.Model(&model.Session{}).Where("id = ?", result.Requirement.SessionID).Updates(map[string]any{
			"status":     model.SessionInDelivery,
			"summary":    utils.Summarize(result.Requirement.Summary, 120),
			"updated_at": time.Now().UTC(),
		}).Error
	})
}

func (r *Repository) GetTaskByID(ctx context.Context, taskID string) (model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).First(&task, "id = ?", taskID).Error
	return task, err
}

func (r *Repository) UpdateTaskStatus(ctx context.Context, taskID string, status model.TaskStatus) (model.Task, error) {
	if err := r.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return model.Task{}, err
	}

	task, err := r.GetTaskByID(ctx, taskID)
	if err != nil {
		return model.Task{}, err
	}
	if err := r.RefreshSessionStatus(ctx, task.SessionID); err != nil {
		return model.Task{}, err
	}
	return task, nil
}

func (r *Repository) UpdateTaskLinks(ctx context.Context, taskID string, docURL string, bitableURL string, bitableRecordID string, bitableAppToken string, bitableTableID string) error {
	return r.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"doc_url":            docURL,
		"bitable_record_url": bitableURL,
		"bitable_record_id":  bitableRecordID,
		"bitable_app_token":  bitableAppToken,
		"bitable_table_id":   bitableTableID,
		"updated_at":         time.Now().UTC(),
	}).Error
}

func (r *Repository) RefreshSessionStatus(ctx context.Context, sessionID string) error {
	var tasks []model.Task
	if err := r.db.WithContext(ctx).Find(&tasks, "session_id = ?", sessionID).Error; err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	nextStatus := model.SessionInDelivery
	allDone := true
	allTestingOrDone := true
	for _, task := range tasks {
		if task.Status != model.TaskDone {
			allDone = false
		}
		if task.Status != model.TaskDone && task.Status != model.TaskTesting {
			allTestingOrDone = false
		}
	}
	if allDone {
		nextStatus = model.SessionDone
	} else if allTestingOrDone {
		nextStatus = model.SessionTesting
	}

	return r.db.WithContext(ctx).Model(&model.Session{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":     nextStatus,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *Repository) SaveRoleMapping(ctx context.Context, mapping *model.RoleMapping) error {
	if mapping.ID == "" {
		mapping.ID = utils.NewID("rm")
	}
	return r.db.WithContext(ctx).Save(mapping).Error
}

func (r *Repository) ListRoleMappings(ctx context.Context) ([]model.RoleMapping, error) {
	var items []model.RoleMapping
	err := r.db.WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}

func (r *Repository) SaveRoleOwner(ctx context.Context, owner *model.RoleOwner) error {
	if owner.ID == "" {
		var existing model.RoleOwner
		err := r.db.WithContext(ctx).First(&existing, "role = ?", owner.Role).Error
		if err == nil {
			owner.ID = existing.ID
			owner.CreatedAt = existing.CreatedAt
		} else {
			owner.ID = utils.NewID("ro")
		}
	}
	return r.db.WithContext(ctx).Save(owner).Error
}

func (r *Repository) ListRoleOwners(ctx context.Context) ([]model.RoleOwner, error) {
	var items []model.RoleOwner
	err := r.db.WithContext(ctx).Order("role ASC").Find(&items).Error
	return items, err
}

func (r *Repository) SaveKnowledgeSources(ctx context.Context, items []model.KnowledgeSource) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for idx := range items {
			if items[idx].ID == "" {
				items[idx].ID = utils.NewID("ks")
			}
			if items[idx].UpdatedAt.IsZero() {
				items[idx].UpdatedAt = time.Now().UTC()
			}
			if err := tx.Save(&items[idx]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SearchKnowledgeSources(ctx context.Context, query string, limit int) ([]model.KnowledgeSource, error) {
	var items []model.KnowledgeSource
	if limit <= 0 {
		limit = 5
	}
	err := r.db.WithContext(ctx).
		Where("title LIKE ? OR content LIKE ?", "%"+query+"%", "%"+query+"%").
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// OpenAPISpec CRUD

func (r *Repository) CreateOpenAPISpec(ctx context.Context, spec *model.OpenAPISpec) error {
	return r.db.WithContext(ctx).Create(spec).Error
}

func (r *Repository) GetOpenAPISpec(ctx context.Context, id string) (model.OpenAPISpec, error) {
	var spec model.OpenAPISpec
	err := r.db.WithContext(ctx).First(&spec, "id = ?", id).Error
	return spec, err
}

func (r *Repository) ListOpenAPISpecs(ctx context.Context) ([]model.OpenAPISpec, error) {
	var specs []model.OpenAPISpec
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&specs).Error
	return specs, err
}

func (r *Repository) ListOpenAPISpecsByProject(ctx context.Context, projectID string) ([]model.OpenAPISpec, error) {
	var specs []model.OpenAPISpec
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&specs).Error
	return specs, err
}

func (r *Repository) UpdateOpenAPISpec(ctx context.Context, id string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.OpenAPISpec{}).Where("id = ?", id).Updates(updates).Error
}

// Project CRUD

func (r *Repository) CreateProject(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *Repository) GetProject(ctx context.Context, id string) (model.Project, error) {
	var project model.Project
	err := r.db.WithContext(ctx).First(&project, "id = ?", id).Error
	return project, err
}

func (r *Repository) ListProjects(ctx context.Context) ([]model.Project, error) {
	var projects []model.Project
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&projects).Error
	return projects, err
}

func (r *Repository) UpdateProject(ctx context.Context, id string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Project{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) seedPipelineTemplatesTx(ctx context.Context, tx *gorm.DB) error {
	var count int64
	if err := tx.WithContext(ctx).Model(&model.PipelineTemplate{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	template := model.PipelineTemplate{
		ID:             pipeline.DefaultTemplateID,
		Name:           "Feature Delivery",
		Description:    "默认新功能研发交付流水线模板",
		Version:        "v1",
		DefinitionJSON: pipeline.DefaultTemplateDefinitionJSON(),
		IsActive:       true,
		BaseModel:      model.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	return tx.WithContext(ctx).Create(&template).Error
}

func (r *Repository) ListPipelineTemplates(ctx context.Context) ([]model.PipelineTemplate, error) {
	var items []model.PipelineTemplate
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) GetPipelineTemplateByID(ctx context.Context, templateID string) (model.PipelineTemplate, error) {
	var item model.PipelineTemplate
	err := r.db.WithContext(ctx).First(&item, "id = ?", templateID).Error
	return item, err
}

func (r *Repository) SavePipelineTemplate(ctx context.Context, template *model.PipelineTemplate) error {
	if template.ID == "" {
		template.ID = utils.NewID("plt")
	}
	return r.db.WithContext(ctx).Save(template).Error
}

func (r *Repository) CreatePipelineRun(ctx context.Context, run *model.PipelineRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *Repository) CreatePipelineRunAggregate(ctx context.Context, run *model.PipelineRun, stages []model.StageRun, checkpoints []model.Checkpoint, artifacts []model.Artifact) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		if len(stages) > 0 {
			if err := tx.Create(&stages).Error; err != nil {
				return err
			}
		}
		if len(checkpoints) > 0 {
			if err := tx.Create(&checkpoints).Error; err != nil {
				return err
			}
		}
		if len(artifacts) > 0 {
			if err := tx.Create(&artifacts).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) GetPipelineRunByID(ctx context.Context, runID string) (model.PipelineRun, error) {
	var item model.PipelineRun
	err := r.db.WithContext(ctx).First(&item, "id = ?", runID).Error
	return item, err
}

func (r *Repository) ListPipelineRuns(ctx context.Context) ([]model.PipelineRun, error) {
	var items []model.PipelineRun
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (r *Repository) UpdatePipelineRunStatus(ctx context.Context, runID string, status model.PipelineRunStatus) error {
	updates := map[string]any{"status": status, "updated_at": time.Now().UTC()}
	if status == model.PipelineRunRunning {
		now := time.Now().UTC()
		updates["started_at"] = gorm.Expr("COALESCE(started_at, ?)", now)
	}
	if status == model.PipelineRunCompleted || status == model.PipelineRunFailed || status == model.PipelineRunTerminated {
		now := time.Now().UTC()
		updates["finished_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&model.PipelineRun{}).Where("id = ?", runID).Updates(updates).Error
}

func (r *Repository) UpdatePipelineRunCurrentStage(ctx context.Context, runID string, stageKey string) error {
	return r.db.WithContext(ctx).Model(&model.PipelineRun{}).Where("id = ?", runID).Updates(map[string]any{
		"current_stage_key": stageKey,
		"updated_at":        time.Now().UTC(),
	}).Error
}

func (r *Repository) CreateStageRuns(ctx context.Context, items []model.StageRun) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *Repository) ListStageRunsByPipelineRunID(ctx context.Context, runID string) ([]model.StageRun, error) {
	var items []model.StageRun
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ?", runID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) GetStageRunByID(ctx context.Context, stageRunID string) (model.StageRun, error) {
	var item model.StageRun
	err := r.db.WithContext(ctx).First(&item, "id = ?", stageRunID).Error
	return item, err
}

func (r *Repository) GetStageRunByKey(ctx context.Context, runID string, stageKey string) (model.StageRun, error) {
	var item model.StageRun
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ? AND stage_key = ?", runID, stageKey).First(&item).Error
	return item, err
}

func (r *Repository) UpdateStageRunStatus(ctx context.Context, stageRunID string, status model.StageRunStatus) error {
	updates := map[string]any{"status": status, "updated_at": time.Now().UTC()}
	if status == model.StageRunRunning {
		now := time.Now().UTC()
		updates["started_at"] = &now
	}
	if status == model.StageRunSucceeded || status == model.StageRunFailed || status == model.StageRunSkipped {
		now := time.Now().UTC()
		updates["finished_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&model.StageRun{}).Where("id = ?", stageRunID).Updates(updates).Error
}

func (r *Repository) QueueStageRun(ctx context.Context, runID string, stageKey string) error {
	return r.db.WithContext(ctx).Model(&model.StageRun{}).Where("pipeline_run_id = ? AND stage_key = ?", runID, stageKey).Updates(map[string]any{
		"status":     model.StageRunQueued,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *Repository) SaveStageRunInput(ctx context.Context, stageRunID string, inputJSON string) error {
	return r.db.WithContext(ctx).Model(&model.StageRun{}).Where("id = ?", stageRunID).Updates(map[string]any{
		"input_json": inputJSON,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *Repository) SaveStageRunOutput(ctx context.Context, stageRunID string, outputJSON string, errorMessage string) error {
	return r.db.WithContext(ctx).Model(&model.StageRun{}).Where("id = ?", stageRunID).Updates(map[string]any{
		"output_json":   outputJSON,
		"error_message": errorMessage,
		"updated_at":    time.Now().UTC(),
	}).Error
}

func (r *Repository) ResetStageRun(ctx context.Context, stageRunID string, status model.StageRunStatus, attempt int, inputJSON string) error {
	updates := map[string]any{
		"status":        status,
		"attempt":       attempt,
		"input_json":    inputJSON,
		"output_json":   "",
		"error_message": "",
		"started_at":    nil,
		"finished_at":   nil,
		"updated_at":    time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Model(&model.StageRun{}).Where("id = ?", stageRunID).Updates(updates).Error
}

func (r *Repository) CreateArtifact(ctx context.Context, item *model.Artifact) error {
	if item.MetaJSON == "" {
		item.MetaJSON = "{}"
	}
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) MarkArtifactsSupersededByStageRunID(ctx context.Context, stageRunID string) error {
	var items []model.Artifact
	if err := r.db.WithContext(ctx).Where("stage_run_id = ?", stageRunID).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		meta := map[string]any{}
		if strings.TrimSpace(item.MetaJSON) != "" {
			_ = json.Unmarshal([]byte(item.MetaJSON), &meta)
		}
		meta["superseded"] = true
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		if err := r.db.WithContext(ctx).Model(&model.Artifact{}).Where("id = ?", item.ID).Updates(map[string]any{"meta_json": string(metaJSON), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListArtifactsByPipelineRunID(ctx context.Context, runID string) ([]model.Artifact, error) {
	var items []model.Artifact
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ?", runID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) CreateCheckpoint(ctx context.Context, item *model.Checkpoint) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) GetCheckpointByID(ctx context.Context, checkpointID string) (model.Checkpoint, error) {
	var item model.Checkpoint
	err := r.db.WithContext(ctx).First(&item, "id = ?", checkpointID).Error
	return item, err
}

func (r *Repository) UpdateCheckpointDecision(ctx context.Context, checkpointID string, status model.CheckpointStatus, decision string, comment string, approverID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&model.Checkpoint{}).Where("id = ?", checkpointID).Updates(map[string]any{
		"status":      status,
		"decision":    decision,
		"comment":     comment,
		"approver_id": approverID,
		"decided_at":  &now,
		"updated_at":  now,
	}).Error
}

func (r *Repository) ResetCheckpoint(ctx context.Context, checkpointID string) error {
	return r.db.WithContext(ctx).Model(&model.Checkpoint{}).Where("id = ?", checkpointID).Updates(map[string]any{
		"status":      model.CheckpointPending,
		"decision":    "",
		"comment":     "",
		"approver_id": "",
		"decided_at":  nil,
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (r *Repository) ResetCheckpointByStageRunID(ctx context.Context, stageRunID string) error {
	return r.db.WithContext(ctx).Model(&model.Checkpoint{}).Where("stage_run_id = ?", stageRunID).Updates(map[string]any{
		"status":      model.CheckpointPending,
		"decision":    "",
		"comment":     "",
		"approver_id": "",
		"decided_at":  nil,
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (r *Repository) ListCheckpointsByPipelineRunID(ctx context.Context, runID string) ([]model.Checkpoint, error) {
	var items []model.Checkpoint
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ?", runID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) CreateAgentRun(ctx context.Context, item *model.AgentRun) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) UpdateAgentRun(ctx context.Context, item *model.AgentRun) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *Repository) ListAgentRunsByPipelineRunID(ctx context.Context, runID string) ([]model.AgentRun, error) {
	var items []model.AgentRun
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ?", runID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) ListAgentRunsByStageRunID(ctx context.Context, stageRunID string) ([]model.AgentRun, error) {
	var items []model.AgentRun
	err := r.db.WithContext(ctx).Where("stage_run_id = ?", stageRunID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) CreateGitDelivery(ctx context.Context, item *model.GitDelivery) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *Repository) GetGitDeliveryByID(ctx context.Context, deliveryID string) (model.GitDelivery, error) {
	var item model.GitDelivery
	err := r.db.WithContext(ctx).First(&item, "id = ?", deliveryID).Error
	return item, err
}

func (r *Repository) ListGitDeliveriesByPipelineRunID(ctx context.Context, runID string) ([]model.GitDelivery, error) {
	var items []model.GitDelivery
	err := r.db.WithContext(ctx).Where("pipeline_run_id = ?", runID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) UpdateGitDeliveryStatus(ctx context.Context, deliveryID string, status model.GitDeliveryStatus, prmrURL string, commitSHA string) error {
	return r.db.WithContext(ctx).Model(&model.GitDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{
		"status":     status,
		"prmr_url":   prmrURL,
		"commit_sha": commitSHA,
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *Repository) GetSessionByID(ctx context.Context, sessionID string) (model.Session, error) {
	var session model.Session
	err := r.db.WithContext(ctx).Preload("Owner").First(&session, "id = ?", sessionID).Error
	return session, err
}

func (r *Repository) ListMessagesBySessionID(ctx context.Context, sessionID string) ([]model.Message, error) {
	var items []model.Message
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *Repository) CountMessagesBySessionID(ctx context.Context, sessionID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Message{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}

func (r *Repository) CountTasksBySessionID(ctx context.Context, sessionID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Task{}).Where("session_id = ?", sessionID).Count(&count).Error
	return count, err
}
