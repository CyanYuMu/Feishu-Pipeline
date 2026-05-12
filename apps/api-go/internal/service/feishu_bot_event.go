package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/pipeline"
	"feishu-pipeline/apps/api-go/internal/utils"

	"gorm.io/gorm"
)

type FeishuBotEventRequest struct {
	Challenge string `json:"challenge,omitempty"`
	Type      string `json:"type"`
	Header    struct {
		EventType string `json:"event_type"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID  string `json:"open_id"`
				UserID  string `json:"user_id"`
				UnionID string `json:"union_id"`
			} `json:"sender_id"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			ChatID      string `json:"chat_id"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
	} `json:"event"`
}

type FeishuBotEventResult struct {
	Challenge        string                  `json:"challenge,omitempty"`
	Ignored          bool                    `json:"ignored,omitempty"`
	PipelineRunID    string                  `json:"pipelineRunId,omitempty"`
	Status           model.PipelineRunStatus `json:"status,omitempty"`
	CurrentStageKey  string                  `json:"currentStageKey,omitempty"`
	BitableRecordURL string                  `json:"bitableRecordUrl,omitempty"`
	Message          string                  `json:"message,omitempty"`
}

type parsedBotRequirement struct {
	Title            string
	RequirementText  string
	TargetRepo       string
	TargetBranch     string
	Approver         string
	Reviewer         string
	ReleaseConfirmer string
	SelectedDocURLs  []string
}

func (s *PipelineService) HandleFeishuBotEvent(ctx context.Context, req FeishuBotEventRequest) (FeishuBotEventResult, error) {
	if strings.TrimSpace(req.Challenge) != "" {
		return FeishuBotEventResult{Challenge: req.Challenge}, nil
	}
	eventType := strings.TrimSpace(req.Header.EventType)
	if eventType != "" && eventType != "im.message.receive_v1" {
		return FeishuBotEventResult{Ignored: true, Message: "ignored unsupported event type"}, nil
	}

	messageID := strings.TrimSpace(req.Event.Message.MessageID)
	text := extractFeishuMessageText(req.Event.Message.Content)
	if strings.TrimSpace(text) == "" {
		return FeishuBotEventResult{Ignored: true, Message: "ignored empty message"}, nil
	}

	sourceSessionID := "feishu:" + utils.Coalesce(messageID, stableFeishuMessageSource(text))
	if existing, err := s.repository.GetPipelineRunBySourceSessionID(ctx, sourceSessionID); err == nil {
		return FeishuBotEventResult{
			PipelineRunID:    existing.ID,
			Status:           existing.Status,
			CurrentStageKey:  existing.CurrentStageKey,
			BitableRecordURL: existing.BitableRecordURL,
			Message:          "pipeline run already exists for this Feishu message",
		}, nil
	} else if !errorsIsRecordNotFound(err) {
		return FeishuBotEventResult{}, err
	}

	parsed := parseFeishuBotRequirement(text)
	createdBy := firstNonEmpty(req.Event.Sender.SenderID.OpenID, req.Event.Sender.SenderID.UserID, "feishu_user")
	detail, err := s.CreatePipelineRun(ctx, CreatePipelineRunInput{
		TemplateID:      pipeline.DefaultTemplateID,
		Title:           parsed.Title,
		RequirementText: parsed.RequirementText,
		TargetRepo:      parsed.TargetRepo,
		TargetBranch:    parsed.TargetBranch,
		SourceSessionID: sourceSessionID,
		CreatedBy:       createdBy,
		SelectedDocUrls: parsed.SelectedDocURLs,
	})
	if err != nil {
		return FeishuBotEventResult{}, err
	}
	run, err := s.StartPipelineRun(ctx, detail.Run.ID)
	if err != nil {
		return FeishuBotEventResult{}, err
	}
	if s.feishuClient != nil {
		receiverID := strings.TrimSpace(req.Event.Message.ChatID)
		receiverIDType := "chat_id"
		if receiverID == "" {
			receiverID = createdBy
			receiverIDType = "open_id"
		}
		if sendResult, sendErr := s.feishuClient.SendPipelineRunStatusCard(ctx, receiverID, receiverIDType, run); sendErr != nil {
			log.Printf("send feishu pipeline status card failed: receiver=%s receive_id_type=%s error=%v", receiverID, receiverIDType, sendErr)
		} else {
			log.Printf("send feishu pipeline status card accepted: receiver=%s receive_id_type=%s status=%s remote_id=%s", receiverID, receiverIDType, sendResult.Status, sendResult.RemoteID)
		}
	}
	return FeishuBotEventResult{
		PipelineRunID:    run.ID,
		Status:           run.Status,
		CurrentStageKey:  run.CurrentStageKey,
		BitableRecordURL: run.BitableRecordURL,
		Message:          "pipeline run created from Feishu bot message",
	}, nil
}

func parseFeishuBotRequirement(text string) parsedBotRequirement {
	cleaned := cleanBotMentionText(text)
	docURLs := extractFeishuDocURLs(cleaned)
	targetRepo := parseLabeledValue(cleaned, []string{"目标仓库", "仓库", "repo", "repository"})
	targetBranch := parseLabeledValue(cleaned, []string{"目标分支", "分支", "branch", "base branch"})
	approver := parseLabeledValue(cleaned, []string{"审批人", "approver"})
	reviewer := parseLabeledValue(cleaned, []string{"评审人", "代码评审人", "reviewer"})
	releaseConfirmer := parseLabeledValue(cleaned, []string{"上线确认人", "发布确认人", "release confirmer"})

	title := firstSentence(cleaned)
	if title == "" && len(docURLs) > 0 {
		title = "基于飞书文档创建研发流水线"
	}
	if title == "" {
		title = "飞书机器人需求"
	}
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80])
	}
	requirementText := cleaned
	if approver != "" {
		requirementText += "\n审批人：" + approver
	}
	if reviewer != "" {
		requirementText += "\n评审人：" + reviewer
	}
	if releaseConfirmer != "" {
		requirementText += "\n上线确认人：" + releaseConfirmer
	}

	return parsedBotRequirement{
		Title:            title,
		RequirementText:  strings.TrimSpace(requirementText),
		TargetRepo:       targetRepo,
		TargetBranch:     targetBranch,
		Approver:         approver,
		Reviewer:         reviewer,
		ReleaseConfirmer: releaseConfirmer,
		SelectedDocURLs:  docURLs,
	}
}

func extractFeishuMessageText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err == nil {
		if text, ok := payload["text"].(string); ok {
			return strings.TrimSpace(text)
		}
		if title, ok := payload["title"].(string); ok {
			return strings.TrimSpace(title)
		}
	}
	return content
}

func cleanBotMentionText(text string) string {
	text = regexp.MustCompile(`<at[^>]*>.*?</at>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)@(?:DevFlow|需求交付机器人|Feishu\s*DevFlow|_user_\d+)`).ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\u00a0", " ")
	return strings.TrimSpace(regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " "))
}

func extractFeishuDocURLs(text string) []string {
	matches := regexp.MustCompile(`https?://[^\s，。；,;]+(?:feishu|larksuite)[^\s，。；,;]+`).FindAllString(text, -1)
	seen := map[string]struct{}{}
	result := make([]string, 0, len(matches))
	for _, item := range matches {
		item = strings.TrimRight(item, ".)）]")
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func parseLabeledValue(text string, labels []string) string {
	for _, label := range labels {
		pattern := fmt.Sprintf(`(?i)%s\s*(?:是|为|:|：|=)?\s*([A-Za-z0-9_./:@-]+)`, regexp.QuoteMeta(label))
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			return strings.Trim(match[1], " ，。；;,")
		}
	}
	return ""
}

func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, sep := range []string{"\n", "。", "；", ";"} {
		if idx := strings.Index(text, sep); idx > 0 {
			return strings.TrimSpace(text[:idx])
		}
	}
	return strings.TrimSpace(text)
}

func stableFeishuMessageSource(text string) string {
	value := strings.TrimSpace(text)
	if len(value) > 48 {
		value = value[:48]
	}
	value = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func errorsIsRecordNotFound(err error) bool {
	return err == nil || errors.Is(err, gorm.ErrRecordNotFound)
}
