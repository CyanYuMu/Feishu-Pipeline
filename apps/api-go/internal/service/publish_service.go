package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"feishu-pipeline/apps/api-go/internal/agent"
	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/job"
	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/repo"
	"feishu-pipeline/apps/api-go/internal/utils"

	"gorm.io/gorm"
)

type PublishQueue interface {
	Enqueue(job.PublishJob)
}

type PublishService struct {
	repository      *repo.Repository
	authService     *AuthService
	agentEngine     *agent.Engine
	feishuClient    *feishu.Client
	queue           PublishQueue
	pipelineService *PipelineService
}

func NewPublishService(repository *repo.Repository, authService *AuthService, agentEngine *agent.Engine, feishuClient *feishu.Client, pipelineService *PipelineService) *PublishService {
	return &PublishService{
		repository:      repository,
		authService:     authService,
		agentEngine:     agentEngine,
		feishuClient:    feishuClient,
		pipelineService: pipelineService,
	}
}

func (s *PublishService) SetQueue(queue PublishQueue) {
	s.queue = queue
}

func (s *PublishService) SetPipelineService(ps *PipelineService) {
	s.pipelineService = ps
}

func (s *PublishService) PublishSession(ctx context.Context, userID string, sessionID string) error {
	user, err := s.authService.CurrentUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.Role != model.RoleProduct && user.Role != model.RoleAdmin {
		return errors.New("only product or admin can publish requirement")
	}

	aggregate, err := s.repository.GetSessionAggregate(ctx, sessionID)
	if err != nil {
		return err
	}
	if aggregate.Session.Status != model.SessionDraft {
		return errors.New("session is not in draft status")
	}

	if err := s.repository.MarkSessionPublished(ctx, sessionID); err != nil {
		return err
	}
	if s.queue == nil {
		return errors.New("publish queue not configured")
	}

	s.queue.Enqueue(job.PublishJob{SessionID: sessionID})
	return nil
}

func (s *PublishService) TryAutoPublishByMessage(ctx context.Context, userID string, sessionID string, content string) (bool, string, error) {
	user, err := s.authService.CurrentUser(ctx, userID)
	if err != nil {
		return false, "authentication required", err
	}
	if user.Role != model.RoleProduct && user.Role != model.RoleAdmin {
		return false, "user role is not product/admin", nil
	}
	if !containsScheduleSignal(content) {
		return false, "message does not include schedule signal", nil
	}

	aggregate, err := s.repository.GetSessionAggregate(ctx, sessionID)
	if err != nil {
		return false, "session not found", err
	}
	if aggregate.Session.Status != model.SessionDraft {
		return false, "session is not in draft status", nil
	}

	if err := s.PublishSession(ctx, userID, sessionID); err != nil {
		return false, "auto publish failed", err
	}
	return true, "auto publish accepted", nil
}

func containsScheduleSignal(content string) bool {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return false
	}

	keywords := []string{
		"排期", "上线", "截止", "ddl", "deadline", "milestone", "里程碑", "交付时间", "本周", "下周",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (s *PublishService) HandlePublish(ctx context.Context, payload job.PublishJob) (err error) {
	startedAt := time.Now().UTC()
	log.Printf("publish workflow started: session_id=%s", payload.SessionID)
	defer func() {
		if err != nil {
			log.Printf("publish workflow failed: session_id=%s elapsed_ms=%d err=%v", payload.SessionID, time.Since(startedAt).Milliseconds(), err)
			return
		}
		log.Printf("publish workflow completed: session_id=%s elapsed_ms=%d", payload.SessionID, time.Since(startedAt).Milliseconds())
	}()

	aggregate, err := s.repository.GetSessionAggregate(ctx, payload.SessionID)
	if err != nil {
		return err
	}

	mappings, err := s.repository.ListRoleMappings(ctx)
	if err != nil {
		return err
	}
	roleOwners, err := s.repository.ListRoleOwners(ctx)
	if err != nil {
		return err
	}
	roleOwners, err = s.fillRoleOwnersFromUsers(ctx, roleOwners)
	if err != nil {
		return err
	}
	knowledge, err := s.repository.SearchKnowledgeSources(ctx, aggregate.Session.Summary, 5)
	if err != nil {
		return err
	}

	output, err := s.agentEngine.Execute(ctx, agent.PublishInput{
		Session:      aggregate,
		RoleMappings: mappings,
		RoleOwners:   roleOwners,
		Knowledge:    knowledge,
	})
	if err != nil {
		return err
	}

	requirementDocURL := ""
	if s.feishuClient != nil {
		requirementDocURL, err = s.feishuClient.CreateRequirementDoc(ctx, output.Requirement, output.Tasks)
		if err != nil {
			log.Printf("publish session %s: create requirement doc failed: %v", payload.SessionID, err)
			requirementDocURL = ""
		}
	}

	deliveries := make([]model.MessageDelivery, 0, len(output.Tasks))
	for idx := range output.Tasks {
		docURL, err := s.feishuClient.CreateTaskDoc(ctx, aggregate.Session.Title, output.Tasks[idx])
		if err != nil {
			log.Printf("publish session %s: create task doc failed for task %s: %v", payload.SessionID, output.Tasks[idx].ID, err)
			docURL = ""
		}
		recordResult, err := s.feishuClient.UpsertTaskRecord(ctx, output.Tasks[idx])
		if err != nil {
			log.Printf("publish session %s: upsert bitable record failed for task %s: %v", payload.SessionID, output.Tasks[idx].ID, err)
			recordResult = feishu.TaskRecordResult{}
		}
		output.Tasks[idx].DocURL = docURL
		output.Tasks[idx].BitableAppToken = recordResult.AppToken
		output.Tasks[idx].BitableTableID = recordResult.TableID
		output.Tasks[idx].BitableRecordID = recordResult.RecordID
		output.Tasks[idx].BitableRecordURL = recordResult.RecordURL

		sendResult, err := s.feishuClient.SendTaskMessage(ctx, output.Tasks[idx])
		if err != nil {
			log.Printf("publish session %s: send task message failed for task %s: %v", payload.SessionID, output.Tasks[idx].ID, err)
			sendResult = feishu.SendResult{
				Channel:    "feishu-bot",
				Receiver:   output.Tasks[idx].AssigneeName,
				Status:     "failed",
				RemoteID:   "",
				RawPayload: err.Error(),
			}
		}
		deliveries = append(deliveries, model.MessageDelivery{
			ID:         utils.NewID("delivery"),
			TaskID:     output.Tasks[idx].ID,
			Channel:    sendResult.Channel,
			Receiver:   sendResult.Receiver,
			Status:     sendResult.Status,
			RemoteID:   sendResult.RemoteID,
			RawPayload: sendResult.RawPayload,
			CreatedAt:  time.Now().UTC(),
		})
	}

	err = s.repository.SavePublishResult(ctx, repo.PublishResult{
		Requirement: output.Requirement,
		Tasks:       output.Tasks,
		Deliveries:  deliveries,
	})
	if err != nil {
		return err
	}
	log.Printf("publish workflow persisted: session_id=%s requirement_id=%s tasks=%d deliveries=%d", payload.SessionID, output.Requirement.ID, len(output.Tasks), len(deliveries))

	// 自动创建对应的研发流水线
	runID := ""
	if s.pipelineService != nil {
		// 获取默认模板
		templates, err := s.pipelineService.ListPipelineTemplates(ctx)
		if err == nil && len(templates) > 0 {
			// 使用第一个激活的模板
			var defaultTemplate *model.PipelineTemplate
			for _, t := range templates {
				if t.IsActive {
					defaultTemplate = &t
					break
				}
			}

			if defaultTemplate != nil {
				// 构建需求描述
				requirementText := buildPipelineRequirementText(output.Requirement, output.Tasks, requirementDocURL)
				if requirementText == "" {
					requirementText = aggregate.Session.Title
				}

				// 创建流水线
				detail, err := s.pipelineService.CreatePipelineRun(ctx, CreatePipelineRunInput{
					TemplateID:      defaultTemplate.ID,
					Title:           output.Requirement.Title,
					RequirementText: requirementText,
					TargetRepo:      "self", // 会话发布默认当前仓库
					TargetBranch:    "main",
					CreatedBy:       aggregate.Session.OwnerID, // 使用会话创建者
					SourceSessionID: aggregate.Session.ID,
				})

				if err != nil {
					log.Printf("自动创建流水线失败: session_id=%s err=%v", payload.SessionID, err)
				} else {
					runID = detail.Run.ID
					if _, startErr := s.pipelineService.StartPipelineRun(ctx, detail.Run.ID); startErr != nil {
						log.Printf("自动启动流水线失败: session_id=%s run_id=%s err=%v", payload.SessionID, detail.Run.ID, startErr)
					} else {
						log.Printf("自动创建并启动流水线成功: session_id=%s run_id=%s", payload.SessionID, detail.Run.ID)
					}
				}
			}
		}
	}

	s.sendRequirementDocMessage(ctx, aggregate, output.Requirement, requirementDocURL, runID)
	_, _ = s.repository.AddMessage(ctx, payload.SessionID, model.MessageAssistant, buildPipelineStartedAssistantReply(output.Requirement, requirementDocURL, runID))

	return nil
}

func buildPipelineRequirementText(requirement model.Requirement, tasks []model.Task, docURL string) string {
	parts := []string{
		"需求标题：" + requirement.Title,
		"需求摘要：\n" + requirement.Summary,
		"交付目标：\n" + requirement.DeliverySummary,
	}
	if strings.TrimSpace(docURL) != "" {
		parts = append(parts, "飞书结构化需求文档："+docURL)
	}
	if len(requirement.ReferencedKnowledge) > 0 {
		parts = append(parts, "参考知识：\n- "+strings.Join(requirement.ReferencedKnowledge, "\n- "))
	}
	if len(tasks) > 0 {
		taskLines := make([]string, 0, len(tasks))
		for _, task := range tasks {
			taskLines = append(taskLines, strings.TrimSpace(task.Title+"："+task.Description))
		}
		parts = append(parts, "初始任务拆解：\n- "+strings.Join(taskLines, "\n- "))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildPipelineStartedAssistantReply(requirement model.Requirement, docURL string, runID string) string {
	lines := []string{
		"需求已确认，结构化需求文档已生成并发送到飞书。",
		"标题：" + requirement.Title,
	}
	if strings.TrimSpace(docURL) != "" {
		lines = append(lines, "文档："+docURL)
	}
	if strings.TrimSpace(runID) != "" {
		lines = append(lines, "Pipeline 已自动启动："+runID)
	}
	return strings.Join(lines, "\n")
}

func (s *PublishService) sendRequirementDocMessage(ctx context.Context, aggregate *repo.SessionAggregate, requirement model.Requirement, docURL string, runID string) {
	if s.feishuClient == nil {
		log.Printf("sendRequirementDocMessage skipped: feishu client not available")
		return
	}
	ownerOpenID := aggregate.Owner.FeishuOpenID
	if ownerOpenID == "" {
		log.Printf("sendRequirementDocMessage skipped: owner open_id is empty")
		return
	}
	sendResult, err := s.feishuClient.SendRequirementDocMessage(ctx, ownerOpenID, requirement, docURL, runID)
	if err != nil {
		log.Printf("sendRequirementDocMessage failed: session_id=%s open_id=%s err=%v", aggregate.Session.ID, ownerOpenID, err)
		return
	}
	log.Printf("sendRequirementDocMessage success: session_id=%s open_id=%s message_id=%s", aggregate.Session.ID, ownerOpenID, sendResult.RemoteID)
}

// sendApprovalCard 向发布者发送需求确认卡片
func (s *PublishService) sendApprovalCard(ctx context.Context, aggregate *repo.SessionAggregate, requirement model.Requirement) {
	if s.feishuClient == nil {
		log.Printf("sendApprovalCard skipped: feishu client not available")
		return
	}

	// 获取发布者的 openID
	ownerOpenID := aggregate.Owner.FeishuOpenID
	if ownerOpenID == "" {
		log.Printf("sendApprovalCard skipped: owner open_id is empty")
		return
	}

	// 构建卡片 payload
	summary := utils.Summarize(aggregate.Session.Summary, 200)
	cardPayload := feishu.ApprovalCardPayload{
		Title:       aggregate.Session.Title,
		Summary:     summary,
		Requirement: aggregate.Session.Summary,
		SessionID:   aggregate.Session.ID,
		RunID:       "", // 将在后续创建 pipeline run 后填充
	}

	sendResult, err := s.feishuClient.SendApprovalCardMessage(ctx, ownerOpenID, cardPayload)
	if err != nil {
		log.Printf("sendApprovalCard failed: session_id=%s open_id=%s err=%v", aggregate.Session.ID, ownerOpenID, err)
		return
	}

	log.Printf("sendApprovalCard success: session_id=%s open_id=%s message_id=%s", aggregate.Session.ID, ownerOpenID, sendResult.RemoteID)
}

func (s *PublishService) fillRoleOwnersFromUsers(ctx context.Context, existing []model.RoleOwner) ([]model.RoleOwner, error) {
	hasEnabledOwner := make(map[model.Role]bool, len(existing))
	for _, owner := range existing {
		if owner.Enabled && strings.TrimSpace(owner.FeishuID) != "" {
			hasEnabledOwner[owner.Role] = true
		}
	}

	enrichRoles := []model.Role{model.RoleProduct, model.RoleFrontend, model.RoleBackend}
	result := make([]model.RoleOwner, 0, len(existing)+len(enrichRoles))
	result = append(result, existing...)

	for _, role := range enrichRoles {
		if hasEnabledOwner[role] {
			continue
		}
		user, err := s.repository.FindLatestUserByRole(ctx, role)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		result = append(result, model.RoleOwner{
			ID:           "",
			Role:         role,
			OwnerName:    user.Name,
			FeishuID:     user.FeishuOpenID,
			FeishuIDType: "open_id",
			Enabled:      true,
		})
		log.Printf("publish role owner auto-matched: role=%s user_id=%s name=%s", role, user.ID, user.Name)
	}

	return result, nil
}
