package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/utils"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const (
	grantTypeAuthorizationCode = "authorization_code"
	openAPIAppAccessTokenPath  = "/open-apis/auth/v3/app_access_token/internal"
	openAPITenantTokenPath     = "/open-apis/auth/v3/tenant_access_token/internal"
	openAPIUserTokenPath       = "/open-apis/authen/v1/access_token"
	openAPIRefreshTokenPath    = "/open-apis/authen/v1/refresh_access_token"
	openAPIUserInfoPath        = "/open-apis/authen/v1/user_info"
)

type SendResult struct {
	Channel    string
	Receiver   string
	Status     string
	RemoteID   string
	RawPayload string
}

type UserToken struct {
	AccessToken           string
	RefreshToken          string
	AccessTokenExpiresAt  time.Time
	RefreshTokenExpiresAt time.Time
}

type UserProfile struct {
	OpenID          string
	UnionID         string
	FeishuUserID    string
	Name            string
	EnName          string
	Email           string
	EnterpriseEmail string
	AvatarURL       string
}

type Department struct {
	DepartmentID     string
	OpenDepartmentID string
	Name             string
	NameEN           string
}

type Config struct {
	Enabled              bool
	AppID                string
	AppSecret            string
	RedirectURL          string
	OpenBaseURL          string
	BotName              string
	ReceiveIDType        string
	DocFolderToken       string
	BitableName          string
	BitableFolderToken   string
	BitableAppToken      string
	BitableTableID       string
	BitableTemplateToken string
	BaseURL              string
	OAuthScope           string
}

type TaskRecordResult struct {
	AppToken  string
	TableID   string
	RecordID  string
	RecordURL string
}

type PipelineRunRecordResult struct {
	AppToken  string
	TableID   string
	RecordID  string
	RecordURL string
}

type PipelineRunSyncPayload struct {
	Run          model.PipelineRun
	Stages       []model.StageRun
	Artifacts    []model.Artifact
	Checkpoints  []model.Checkpoint
	AgentRuns    []model.AgentRun
	Deliveries   []model.GitDelivery
	FailureCause string
}

type bitableTarget struct {
	AppToken string
	TableID  string
}

type Client struct {
	cfg        Config
	sdk        *lark.Client
	httpClient *http.Client
	mu         sync.Mutex
	bitable    *bitableTarget
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		sdk: lark.NewClient(cfg.AppID, cfg.AppSecret),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c.cfg.Enabled && c.cfg.AppID != "" && c.cfg.AppSecret != ""
}

func (c *Client) AppID() string {
	return c.cfg.AppID
}

func (c *Client) AppSecret() string {
	return c.cfg.AppSecret
}

func (c *Client) OAuthScope() string {
	return c.cfg.OAuthScope
}

func (c *Client) GetAppAccessToken(ctx context.Context) (string, error) {
	if !c.Enabled() {
		return "", errors.New("feishu sso is not enabled")
	}

	body, err := json.Marshal(map[string]string{
		"app_id":     c.cfg.AppID,
		"app_secret": c.cfg.AppSecret,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openAPIURL(openAPIAppAccessTokenPath), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var response struct {
		Code           int    `json:"code"`
		Msg            string `json:"msg"`
		AppAccessToken string `json:"app_access_token"`
	}
	if err := c.doJSON(req, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("get feishu app_access_token failed: %s", response.Msg)
	}
	if strings.TrimSpace(response.AppAccessToken) == "" {
		return "", errors.New("feishu app_access_token is empty")
	}
	return response.AppAccessToken, nil
}

func (c *Client) GetTenantAccessToken(ctx context.Context) (string, error) {
	if !c.Enabled() {
		return "", errors.New("feishu sso is not enabled")
	}

	body, err := json.Marshal(map[string]string{
		"app_id":     c.cfg.AppID,
		"app_secret": c.cfg.AppSecret,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openAPIURL(openAPITenantTokenPath), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var response struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := c.doJSON(req, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("get feishu tenant_access_token failed: %s", response.Msg)
	}
	if strings.TrimSpace(response.TenantAccessToken) == "" {
		return "", errors.New("feishu tenant_access_token is empty")
	}
	return response.TenantAccessToken, nil
}

func (c *Client) ExchangeCodeForUserToken(ctx context.Context, code string) (UserToken, error) {
	if !c.Enabled() {
		return UserToken{}, errors.New("feishu sso is not enabled")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return UserToken{}, errors.New("feishu pre-auth code is required")
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return UserToken{}, err
	}

	return c.requestUserToken(ctx, openAPIUserTokenPath, appAccessToken, map[string]string{
		"grant_type": grantTypeAuthorizationCode,
		"code":       code,
	})
}

func (c *Client) RefreshUserToken(ctx context.Context, refreshToken string) (UserToken, error) {
	if !c.Enabled() {
		return UserToken{}, errors.New("feishu sso is not enabled")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return UserToken{}, errors.New("feishu refresh_token is required")
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return UserToken{}, err
	}

	return c.requestUserToken(ctx, openAPIRefreshTokenPath, appAccessToken, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	})
}

func (c *Client) GetUserInfo(ctx context.Context, userAccessToken string) (UserProfile, error) {
	userAccessToken = strings.TrimSpace(userAccessToken)
	if userAccessToken == "" {
		return UserProfile{}, errors.New("feishu user_access_token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.openAPIURL(openAPIUserInfoPath), nil)
	if err != nil {
		return UserProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+userAccessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OpenID          string `json:"open_id"`
			UnionID         string `json:"union_id"`
			UserID          string `json:"user_id"`
			Name            string `json:"name"`
			EnName          string `json:"en_name"`
			Email           string `json:"email"`
			EnterpriseEmail string `json:"enterprise_email"`
			AvatarURL       string `json:"avatar_url"`
		} `json:"data"`
	}
	if err := c.doJSON(req, &response); err != nil {
		return UserProfile{}, err
	}
	if response.Code != 0 {
		return UserProfile{}, fmt.Errorf("get feishu user info failed: %s", response.Msg)
	}

	return UserProfile{
		OpenID:          response.Data.OpenID,
		UnionID:         response.Data.UnionID,
		FeishuUserID:    response.Data.UserID,
		Name:            response.Data.Name,
		EnName:          response.Data.EnName,
		Email:           response.Data.Email,
		EnterpriseEmail: response.Data.EnterpriseEmail,
		AvatarURL:       response.Data.AvatarURL,
	}, nil
}

func (c *Client) ListUserDepartments(ctx context.Context, userAccessToken string, userIdentifier string, userIDType string) ([]Department, error) {
	userIdentifier = strings.TrimSpace(userIdentifier)
	if userIdentifier == "" {
		return nil, errors.New("feishu user identifier is required")
	}
	userAccessToken = strings.TrimSpace(userAccessToken)
	if userAccessToken == "" {
		return nil, errors.New("feishu user_access_token is required")
	}
	if !c.Enabled() {
		return nil, errors.New("feishu sso is not enabled")
	}
	switch strings.TrimSpace(userIDType) {
	case "open_id", "union_id":
	default:
		userIDType = "user_id"
	}

	path := fmt.Sprintf("/open-apis/contact/v3/users/%s?user_id_type=%s&department_id_type=department_id&user_fields=department_path,department_ids,name", url.PathEscape(userIdentifier), url.QueryEscape(userIDType))
	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			User struct {
				DepartmentIDs  []string `json:"department_ids"`
				DepartmentPath []struct {
					DepartmentID   string `json:"department_id"`
					DepartmentName struct {
						Name     string `json:"name"`
						I18nName struct {
							ZhCN string `json:"zh_cn"`
							EnUS string `json:"en_us"`
						} `json:"i18n_name"`
					} `json:"department_name"`
					DepartmentPath struct {
						DepartmentIDs []string `json:"department_ids"`
						PathName      struct {
							Name     string `json:"name"`
							I18nName struct {
								ZhCN string `json:"zh_cn"`
								EnUS string `json:"en_us"`
							} `json:"i18n_name"`
						} `json:"department_path_name"`
					} `json:"department_path"`
				} `json:"department_path"`
			} `json:"user"`
			DepartmentIDs []string `json:"department_ids"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodGet, path, userAccessToken, nil, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("get user profile for departments failed (code=%d): %s", response.Code, response.Msg)
	}

	orderedDepartmentIDs := make([]string, 0, len(response.Data.User.DepartmentIDs))
	seenDepartmentIDs := make(map[string]struct{}, len(response.Data.User.DepartmentIDs))
	fromDepartmentPath := make(map[string]Department, len(response.Data.User.DepartmentPath))
	orderedFromPath := make([]string, 0, len(response.Data.User.DepartmentPath))

	for _, pathItem := range response.Data.User.DepartmentPath {
		departmentID := strings.TrimSpace(pathItem.DepartmentID)
		if departmentID == "" {
			departmentID = firstNonEmpty(pathItem.DepartmentPath.DepartmentIDs...)
		}
		if departmentID == "" {
			continue
		}
		name := firstNonEmpty(
			strings.TrimSpace(pathItem.DepartmentName.I18nName.ZhCN),
			strings.TrimSpace(pathItem.DepartmentName.Name),
			strings.TrimSpace(pathItem.DepartmentName.I18nName.EnUS),
			strings.TrimSpace(pathItem.DepartmentPath.PathName.I18nName.ZhCN),
			strings.TrimSpace(pathItem.DepartmentPath.PathName.Name),
			strings.TrimSpace(pathItem.DepartmentPath.PathName.I18nName.EnUS),
		)
		if !isReadableDepartmentName(name) {
			name = ""
		}
		fromDepartmentPath[departmentID] = Department{
			DepartmentID: departmentID,
			Name:         name,
		}
		if _, ok := seenDepartmentIDs[departmentID]; !ok {
			seenDepartmentIDs[departmentID] = struct{}{}
			orderedFromPath = append(orderedFromPath, departmentID)
		}
	}

	for _, departmentID := range append(response.Data.User.DepartmentIDs, response.Data.DepartmentIDs...) {
		departmentID = strings.TrimSpace(departmentID)
		if departmentID == "" {
			continue
		}
		if _, ok := seenDepartmentIDs[departmentID]; ok {
			continue
		}
		seenDepartmentIDs[departmentID] = struct{}{}
		orderedDepartmentIDs = append(orderedDepartmentIDs, departmentID)
	}

	if len(orderedDepartmentIDs) == 0 && len(orderedFromPath) > 0 {
		orderedDepartmentIDs = append(orderedDepartmentIDs, orderedFromPath...)
	}

	if len(orderedDepartmentIDs) == 0 {
		return nil, nil
	}

	batchDetails := map[string]Department{}
	// 优先使用 department_path 中的中文名称；名称缺失时再回退批量查询部门详情。
	needBatchLookup := len(fromDepartmentPath) == 0
	if !needBatchLookup {
		for _, departmentID := range orderedDepartmentIDs {
			if !isReadableDepartmentName(fromDepartmentPath[departmentID].Name) {
				needBatchLookup = true
				break
			}
		}
	}
	if needBatchLookup {
		if tenantAccessToken, err := c.GetTenantAccessToken(ctx); err == nil {
			if details, detailErr := c.batchGetDepartmentDetails(ctx, tenantAccessToken, orderedDepartmentIDs); detailErr == nil {
				batchDetails = details
			}
		}
	}

	departments := make([]Department, 0, len(orderedDepartmentIDs))
	for _, departmentID := range orderedDepartmentIDs {
		department := batchDetails[departmentID]
		if fromPath, ok := fromDepartmentPath[departmentID]; ok {
			if !isReadableDepartmentName(department.Name) {
				department.Name = fromPath.Name
			}
			if department.OpenDepartmentID == "" {
				department.OpenDepartmentID = fromPath.OpenDepartmentID
			}
		}
		department.DepartmentID = departmentID
		department.Name = strings.TrimSpace(department.Name)
		department.NameEN = strings.TrimSpace(department.NameEN)
		if !isReadableDepartmentName(department.Name) {
			department.Name = strings.TrimSpace(department.NameEN)
		}
		if !isReadableDepartmentName(department.Name) {
			continue
		}
		departments = append(departments, department)
	}

	return departments, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isReadableDepartmentName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return !looksLikeDepartmentID(value)
}

func looksLikeDepartmentID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}

	odPattern := regexp.MustCompile(`^od-[a-z0-9]{8,}$`)
	rawPattern := regexp.MustCompile(`^[a-z0-9]{12,}$`)
	if odPattern.MatchString(value) {
		return true
	}
	if rawPattern.MatchString(value) {
		hasDigit := false
		hasLetter := false
		for _, r := range value {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
			if r >= 'a' && r <= 'z' {
				hasLetter = true
			}
		}
		return hasDigit && hasLetter
	}
	return false
}

func (c *Client) CreateTaskDoc(ctx context.Context, sessionTitle string, task model.Task) (string, error) {
	if !c.Enabled() || c.cfg.DocFolderToken == "" {
		return fmt.Sprintf("%s/mock/feishu/docs/%s", c.cfg.BaseURL, task.ID), nil
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Document struct {
				DocumentID string `json:"document_id"`
			} `json:"document"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodPost, "/open-apis/docx/v1/documents", appAccessToken, map[string]any{
		"folder_token": c.cfg.DocFolderToken,
		"title":        fmt.Sprintf("%s-%s", sessionTitle, task.Title),
	}, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("create feishu doc failed: %s", response.Msg)
	}
	if response.Data.Document.DocumentID == "" {
		return "", errors.New("feishu document id is empty")
	}

	if err := c.appendDocumentContent(ctx, appAccessToken, response.Data.Document.DocumentID, task.Description); err != nil {
		return "", fmt.Errorf("append doc content failed: %w", err)
	}

	return fmt.Sprintf("https://feishu.cn/docx/%s", url.PathEscape(response.Data.Document.DocumentID)), nil
}

func (c *Client) CreateRequirementDoc(ctx context.Context, requirement model.Requirement, tasks []model.Task) (string, error) {
	if !c.Enabled() || c.cfg.DocFolderToken == "" {
		return fmt.Sprintf("%s/mock/feishu/requirements/%s", c.cfg.BaseURL, requirement.ID), nil
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Document struct {
				DocumentID string `json:"document_id"`
			} `json:"document"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodPost, "/open-apis/docx/v1/documents", appAccessToken, map[string]any{
		"folder_token": c.cfg.DocFolderToken,
		"title":        fmt.Sprintf("需求文档-%s", requirement.Title),
	}, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("create feishu requirement doc failed: %s", response.Msg)
	}
	if response.Data.Document.DocumentID == "" {
		return "", errors.New("feishu requirement document id is empty")
	}

	if err := c.appendDocumentContent(ctx, appAccessToken, response.Data.Document.DocumentID, buildRequirementDocMarkdown(requirement, tasks)); err != nil {
		return "", fmt.Errorf("append requirement doc content failed: %w", err)
	}

	return fmt.Sprintf("https://feishu.cn/docx/%s", url.PathEscape(response.Data.Document.DocumentID)), nil
}

func (c *Client) CreatePipelineArtifactDoc(ctx context.Context, run model.PipelineRun, artifact model.Artifact) (string, error) {
	if !c.Enabled() || c.cfg.DocFolderToken == "" {
		return fmt.Sprintf("%s/mock/feishu/pipeline-runs/%s/artifacts/%s", c.cfg.BaseURL, run.ID, artifact.ID), nil
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Document struct {
				DocumentID string `json:"document_id"`
			} `json:"document"`
		} `json:"data"`
	}

	title := fmt.Sprintf("DevFlow-%s-%s", run.Title, artifact.Title)
	if err := c.doJSONWithToken(ctx, http.MethodPost, "/open-apis/docx/v1/documents", appAccessToken, map[string]any{
		"folder_token": c.cfg.DocFolderToken,
		"title":        title,
	}, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("create pipeline artifact doc failed: %s", response.Msg)
	}
	if response.Data.Document.DocumentID == "" {
		return "", errors.New("feishu pipeline artifact document id is empty")
	}

	if err := c.appendDocumentContent(ctx, appAccessToken, response.Data.Document.DocumentID, buildPipelineArtifactMarkdown(run, artifact)); err != nil {
		return "", fmt.Errorf("append pipeline artifact doc content failed: %w", err)
	}

	return fmt.Sprintf("https://feishu.cn/docx/%s", url.PathEscape(response.Data.Document.DocumentID)), nil
}

func (c *Client) GetDocumentRawContent(ctx context.Context, documentToken string) (string, error) {
	documentToken = strings.TrimSpace(documentToken)
	if documentToken == "" {
		return "", errors.New("document token is required")
	}
	if !c.Enabled() {
		return fmt.Sprintf("mock raw content for feishu document %s", documentToken), nil
	}
	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}
	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/raw_content?lang=0", url.PathEscape(documentToken))
	if err := c.doJSONWithToken(ctx, http.MethodGet, path, appAccessToken, nil, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("get feishu document raw content failed: %s", response.Msg)
	}
	return response.Data.Content, nil
}

func (c *Client) appendDocumentContent(ctx context.Context, appAccessToken string, documentID string, markdownContent string) error {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return errors.New("document id is required")
	}

	children := buildDocParagraphBlocks(markdownContent)
	if len(children) == 0 {
		return nil
	}

	for _, group := range chunkParagraphBlocks(children, 20) {
		var response struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/children", url.PathEscape(documentID), url.PathEscape(documentID))
		if err := c.doJSONWithToken(ctx, http.MethodPost, path, appAccessToken, map[string]any{
			"children": group,
		}, &response); err != nil {
			return err
		}
		if response.Code != 0 {
			return fmt.Errorf("append doc blocks failed: %s", response.Msg)
		}
	}
	return nil
}

func buildDocParagraphBlocks(markdownContent string) []map[string]any {
	lines := strings.Split(strings.ReplaceAll(markdownContent, "\r\n", "\n"), "\n")
	children := make([]map[string]any, 0, len(lines))
	for _, raw := range lines {
		content := strings.TrimSpace(raw)
		if content == "" {
			continue
		}
		content = strings.TrimLeft(content, "#")
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		children = append(children, map[string]any{
			"block_type": 2,
			"text": map[string]any{
				"elements": []map[string]any{
					{
						"text_run": map[string]any{
							"content":            content,
							"text_element_style": map[string]any{},
						},
					},
				},
				"style": map[string]any{},
			},
		})
	}
	return children
}

func chunkParagraphBlocks(items []map[string]any, size int) [][]map[string]any {
	if size <= 0 {
		return [][]map[string]any{items}
	}
	if len(items) == 0 {
		return nil
	}
	chunks := make([][]map[string]any, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func buildRequirementDocMarkdown(requirement model.Requirement, tasks []model.Task) string {
	lines := []string{
		"# " + requirement.Title,
		"## 需求摘要",
		requirement.Summary,
		"## 交付目标",
		requirement.DeliverySummary,
		"## 验收标准",
	}
	if len(tasks) == 0 {
		lines = append(lines, "- 待 Pipeline 需求分析阶段补全")
	}
	for _, task := range tasks {
		if len(task.AcceptanceCriteria) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("### %s", task.Title))
		for _, item := range task.AcceptanceCriteria {
			lines = append(lines, "- "+item)
		}
	}
	lines = append(lines, "## 任务拆解")
	for _, task := range tasks {
		lines = append(lines,
			fmt.Sprintf("### %s", task.Title),
			fmt.Sprintf("- 类型：%s", task.Type),
			fmt.Sprintf("- 优先级：%s", task.Priority),
			fmt.Sprintf("- 负责人：%s", utils.Coalesce(task.AssigneeName, string(task.AssigneeRole))),
			fmt.Sprintf("- 预估：%d 天", task.EstimateDays),
			task.Description,
		)
		if len(task.Risks) > 0 {
			lines = append(lines, "风险：")
			for _, risk := range task.Risks {
				lines = append(lines, "- "+risk)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func buildPipelineArtifactMarkdown(run model.PipelineRun, artifact model.Artifact) string {
	lines := []string{
		"# " + artifact.Title,
		"## Pipeline",
		"- Run ID: " + run.ID,
		"- 需求标题: " + run.Title,
		"- 当前阶段: " + run.CurrentStageKey,
		"- 目标仓库: " + run.TargetRepo,
		"- 工作分支: " + run.WorkBranch,
		"## 摘要",
	}
	if strings.TrimSpace(artifact.ContentText) != "" {
		lines = append(lines, artifact.ContentText)
	} else {
		lines = append(lines, "暂无文本摘要")
	}
	if strings.TrimSpace(artifact.ContentJSON) != "" {
		lines = append(lines, "## 结构化 JSON", artifact.ContentJSON)
	}
	return strings.Join(lines, "\n")
}

func (c *Client) UpsertTaskRecord(ctx context.Context, task model.Task) (TaskRecordResult, error) {
	target, enabled, err := c.ensureBitableTarget(ctx, task)
	if err != nil {
		return TaskRecordResult{}, err
	}
	if !enabled {
		return TaskRecordResult{
			AppToken:  task.BitableAppToken,
			TableID:   task.BitableTableID,
			RecordID:  task.BitableRecordID,
			RecordURL: fmt.Sprintf("%s/mock/feishu/bitable/%s", c.cfg.BaseURL, task.ID),
		}, nil
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return TaskRecordResult{}, err
	}

	fields := c.buildTaskRecordFields(task)
	recordID := strings.TrimSpace(task.BitableRecordID)
	if recordID == "" {
		var createResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Record struct {
					RecordID string `json:"record_id"`
				} `json:"record"`
				RecordID string `json:"record_id"`
			} `json:"data"`
		}

		if err := c.doJSONWithToken(ctx, http.MethodPost, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records", target.AppToken, target.TableID), appAccessToken, map[string]any{
			"fields": fields,
		}, &createResp); err != nil {
			return TaskRecordResult{}, err
		}
		if createResp.Code != 0 {
			return TaskRecordResult{}, fmt.Errorf("create bitable record failed: %s", createResp.Msg)
		}
		recordID = utils.Coalesce(createResp.Data.Record.RecordID, createResp.Data.RecordID)
	} else {
		var updateResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}

		if err := c.doJSONWithToken(ctx, http.MethodPut, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", target.AppToken, target.TableID, recordID), appAccessToken, map[string]any{
			"fields": fields,
		}, &updateResp); err != nil {
			return TaskRecordResult{}, err
		}
		if updateResp.Code != 0 {
			return TaskRecordResult{}, fmt.Errorf("update bitable record failed: %s", updateResp.Msg)
		}
	}

	return TaskRecordResult{
		AppToken:  target.AppToken,
		TableID:   target.TableID,
		RecordID:  recordID,
		RecordURL: fmt.Sprintf("https://feishu.cn/base/%s?table=%s", url.QueryEscape(target.AppToken), url.QueryEscape(target.TableID)),
	}, nil
}

func (c *Client) UpsertPipelineRunRecord(ctx context.Context, payload PipelineRunSyncPayload) (PipelineRunRecordResult, error) {
	target, enabled, err := c.ensurePipelineBitableTarget(ctx, payload.Run)
	if err != nil {
		return PipelineRunRecordResult{}, err
	}
	if !enabled {
		return PipelineRunRecordResult{
			AppToken:  payload.Run.BitableAppToken,
			TableID:   payload.Run.BitableTableID,
			RecordID:  payload.Run.BitableRecordID,
			RecordURL: fmt.Sprintf("%s/mock/feishu/bitable/pipeline-runs/%s", c.cfg.BaseURL, payload.Run.ID),
		}, nil
	}

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return PipelineRunRecordResult{}, err
	}

	fields := c.buildPipelineRunRecordFields(payload)
	recordID := strings.TrimSpace(payload.Run.BitableRecordID)
	if recordID == "" {
		var createResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Record struct {
					RecordID string `json:"record_id"`
				} `json:"record"`
				RecordID string `json:"record_id"`
			} `json:"data"`
		}
		if err := c.doJSONWithToken(ctx, http.MethodPost, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records", target.AppToken, target.TableID), appAccessToken, map[string]any{
			"fields": fields,
		}, &createResp); err != nil {
			return PipelineRunRecordResult{}, err
		}
		if createResp.Code != 0 {
			return PipelineRunRecordResult{}, fmt.Errorf("create pipeline bitable record failed: %s", createResp.Msg)
		}
		recordID = utils.Coalesce(createResp.Data.Record.RecordID, createResp.Data.RecordID)
	} else {
		var updateResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := c.doJSONWithToken(ctx, http.MethodPut, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/%s", target.AppToken, target.TableID, recordID), appAccessToken, map[string]any{
			"fields": fields,
		}, &updateResp); err != nil {
			return PipelineRunRecordResult{}, err
		}
		if updateResp.Code != 0 {
			return PipelineRunRecordResult{}, fmt.Errorf("update pipeline bitable record failed: %s", updateResp.Msg)
		}
	}

	return PipelineRunRecordResult{
		AppToken:  target.AppToken,
		TableID:   target.TableID,
		RecordID:  recordID,
		RecordURL: fmt.Sprintf("https://feishu.cn/base/%s?table=%s", url.QueryEscape(target.AppToken), url.QueryEscape(target.TableID)),
	}, nil
}

func (c *Client) SendTaskMessage(ctx context.Context, task model.Task) (SendResult, error) {
	receiveID := strings.TrimSpace(task.AssigneeID)
	if receiveID == "" {
		receiveID = task.AssigneeName
	}

	messageText := strings.TrimSpace(task.NotifyContent)
	if messageText == "" {
		messageText = fmt.Sprintf("您有一条新的需求任务：%s", task.Title)
	}
	messageText = fmt.Sprintf("%s\n文档：%s\n排期：%s - %s",
		messageText,
		utils.Coalesce(task.DocURL, "待生成"),
		formatTaskDate(task.PlannedStartAt),
		formatTaskDate(task.PlannedEndAt),
	)
	payload, _ := json.Marshal(map[string]string{"text": messageText})
	content := string(payload)

	if !c.Enabled() || strings.TrimSpace(task.AssigneeID) == "" {
		return SendResult{
			Channel:    "feishu-bot",
			Receiver:   receiveID,
			Status:     "mock_sent",
			RemoteID:   "mock_" + task.ID,
			RawPayload: content,
		}, nil
	}

	resp, err := c.sdk.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(utils.Coalesce(strings.TrimSpace(task.AssigneeIDType), c.cfg.ReceiveIDType)).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType("text").
			Content(content).
			Build()).
		Build())
	if err != nil {
		return SendResult{}, err
	}
	if !resp.Success() {
		return SendResult{}, fmt.Errorf("send feishu message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return SendResult{
		Channel:    "feishu-bot",
		Receiver:   receiveID,
		Status:     "accepted",
		RemoteID:   stringValue(resp.Data.MessageId),
		RawPayload: content,
	}, nil
}

func (c *Client) SendRequirementDocMessage(ctx context.Context, openID string, requirement model.Requirement, docURL string, runID string) (SendResult, error) {
	text := fmt.Sprintf("需求已确认并进入 DevFlow Pipeline：%s\n结构化需求文档：%s", requirement.Title, utils.Coalesce(docURL, "待生成"))
	if strings.TrimSpace(runID) != "" {
		text += "\nPipeline Run：" + runID
	}
	payload, _ := json.Marshal(map[string]string{"text": text})
	content := string(payload)

	if !c.Enabled() || strings.TrimSpace(openID) == "" {
		return SendResult{
			Channel:    "feishu-bot",
			Receiver:   openID,
			Status:     "mock_sent",
			RemoteID:   "mock_requirement_" + requirement.ID,
			RawPayload: content,
		}, nil
	}

	resp, err := c.sdk.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(c.cfg.ReceiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType("text").
			Content(content).
			Build()).
		Build())
	if err != nil {
		return SendResult{}, err
	}
	if !resp.Success() {
		return SendResult{}, fmt.Errorf("send requirement doc message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return SendResult{
		Channel:    "feishu-bot",
		Receiver:   openID,
		Status:     "accepted",
		RemoteID:   stringValue(resp.Data.MessageId),
		RawPayload: content,
	}, nil
}

// ApprovalCardPayload 需求确认卡片内容结构
type ApprovalCardPayload struct {
	Title       string // 需求标题
	Summary     string // 需求摘要
	Requirement string // 详细需求描述
	SessionID   string // 会话ID，用于跳转链接
	RunID       string // Pipeline Run ID
}

type PipelineCheckpointCardPayload struct {
	Run          model.PipelineRun
	Checkpoint   model.Checkpoint
	Stage        model.StageRun
	Artifact     *model.Artifact
	ApprovalURL  string
	RejectReason string
}

// SendApprovalCardMessage 发送需求确认卡片消息
// 卡片包含需求摘要信息和 Approve/Reject 按钮，用户点击后触发回调
func (c *Client) SendApprovalCardMessage(ctx context.Context, openID string, payload ApprovalCardPayload) (SendResult, error) {
	if !c.Enabled() || strings.TrimSpace(openID) == "" {
		return SendResult{
			Channel:    "feishu-bot",
			Receiver:   openID,
			Status:     "mock_sent",
			RemoteID:   "mock_card_" + payload.SessionID,
			RawPayload: fmt.Sprintf("mock card for requirement: %s", payload.Title),
		}, nil
	}

	// 获取前端 URL 用于跳转
	frontendURL := c.cfg.BaseURL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	detailURL := fmt.Sprintf("%s/sessions/%s", frontendURL, payload.SessionID)

	// 构建卡片 JSON（飞书交互卡片格式）
	cardContent := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"body": map[string]any{
			"elements": []any{
				// 标题区域
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("## 📋 **%s**", payload.Title),
				},
				// 分隔线
				map[string]any{
					"tag": "hr",
				},
				// 需求摘要
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("**📝 需求摘要**\n%s", payload.Summary),
				},
				// 详细描述（如果有）
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("**📄 详细需求**\n%s", payload.Requirement),
				},
				// 分隔线
				map[string]any{
					"tag": "hr",
				},
				map[string]any{
					"tag":     "markdown",
					"content": "⚠️ 请确认需求细节是否完整准确，并在页面内完成审批。",
				},
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("[查看详情](%s)", detailURL),
				},
			},
		},
	}

	cardJSON, err := json.Marshal(cardContent)
	if err != nil {
		return SendResult{}, fmt.Errorf("build card content failed: %v", err)
	}

	resp, err := c.sdk.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(c.cfg.ReceiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType("interactive").
			Content(string(cardJSON)).
			Build()).
		Build())
	if err != nil {
		return SendResult{}, err
	}
	if !resp.Success() {
		return SendResult{}, fmt.Errorf("send approval card failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return SendResult{
		Channel:    "feishu-bot",
		Receiver:   openID,
		Status:     "accepted",
		RemoteID:   stringValue(resp.Data.MessageId),
		RawPayload: string(cardJSON),
	}, nil
}

func (c *Client) SendPipelineCheckpointCard(ctx context.Context, receiverID string, payload PipelineCheckpointCardPayload) (SendResult, error) {
	approvalURL := strings.TrimSpace(payload.ApprovalURL)
	if approvalURL == "" {
		approvalURL = fmt.Sprintf("%s/approvals/%s", strings.TrimRight(c.cfg.BaseURL, "/"), payload.Run.ID)
	}
	artifactTitle := "待生成"
	artifactSummary := "暂无审批产物"
	if payload.Artifact != nil {
		artifactTitle = payload.Artifact.Title
		artifactSummary = utils.Summarize(utils.Coalesce(payload.Artifact.ContentText, payload.Artifact.ContentJSON), 400)
	}
	cardContent := map[string]any{
		"schema": "2.0",
		"config": map[string]any{"wide_screen_mode": true},
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("## DevFlow 审批：%s", payload.Run.Title)},
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("**当前阶段**：%s\n**目标仓库**：%s\n**工作分支**：%s", payload.Stage.StageKey, payload.Run.TargetRepo, payload.Run.WorkBranch)},
				map[string]any{"tag": "hr"},
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("**待审批产物**：%s\n%s", artifactTitle, artifactSummary)},
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("[打开审批页](%s)", approvalURL),
				},
			},
		},
	}
	if strings.TrimSpace(payload.RejectReason) != "" {
		body := cardContent["body"].(map[string]any)
		body["elements"] = append(body["elements"].([]any), map[string]any{"tag": "markdown", "content": "**上次驳回原因**：" + payload.RejectReason})
	}
	cardJSON, err := json.Marshal(cardContent)
	if err != nil {
		return SendResult{}, fmt.Errorf("build pipeline checkpoint card failed: %w", err)
	}
	if !c.Enabled() || strings.TrimSpace(receiverID) == "" {
		return SendResult{
			Channel:    "feishu-bot",
			Receiver:   receiverID,
			Status:     "mock_sent",
			RemoteID:   "mock_checkpoint_" + payload.Checkpoint.ID,
			RawPayload: string(cardJSON),
		}, nil
	}
	resp, err := c.sdk.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(c.cfg.ReceiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiverID).
			MsgType("interactive").
			Content(string(cardJSON)).
			Build()).
		Build())
	if err != nil {
		return SendResult{}, err
	}
	if !resp.Success() {
		return SendResult{}, fmt.Errorf("send pipeline checkpoint card failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return SendResult{
		Channel:    "feishu-bot",
		Receiver:   receiverID,
		Status:     "accepted",
		RemoteID:   stringValue(resp.Data.MessageId),
		RawPayload: string(cardJSON),
	}, nil
}

func (c *Client) SendPipelineRunStatusCard(ctx context.Context, receiverID string, receiveIDType string, run model.PipelineRun) (SendResult, error) {
	if strings.TrimSpace(receiveIDType) == "" {
		receiveIDType = c.cfg.ReceiveIDType
	}
	approvalURL := fmt.Sprintf("%s/approvals/%s", strings.TrimRight(c.cfg.BaseURL, "/"), run.ID)
	cardContent := map[string]any{
		"schema": "2.0",
		"config": map[string]any{"wide_screen_mode": true},
		"body": map[string]any{
			"elements": []any{
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("## DevFlow 已创建研发流水线\n**%s**", run.Title)},
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("**Pipeline Run**：%s\n**状态**：%s\n**当前阶段**：%s\n**目标仓库**：%s\n**目标分支**：%s", run.ID, run.Status, run.CurrentStageKey, run.TargetRepo, run.TargetBranch)},
				map[string]any{"tag": "hr"},
				map[string]any{"tag": "markdown", "content": fmt.Sprintf("**多维表格台账**：%s", utils.Coalesce(run.BitableRecordURL, "待同步"))},
				map[string]any{
					"tag":     "markdown",
					"content": fmt.Sprintf("[打开流水线](%s)", approvalURL),
				},
			},
		},
	}
	cardJSON, err := json.Marshal(cardContent)
	if err != nil {
		return SendResult{}, fmt.Errorf("build pipeline status card failed: %w", err)
	}
	if !c.Enabled() || strings.TrimSpace(receiverID) == "" {
		return SendResult{
			Channel:    "feishu-bot",
			Receiver:   receiverID,
			Status:     "mock_sent",
			RemoteID:   "mock_pipeline_status_" + run.ID,
			RawPayload: string(cardJSON),
		}, nil
	}
	resp, err := c.sdk.Im.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiverID).
			MsgType("interactive").
			Content(string(cardJSON)).
			Build()).
		Build())
	if err != nil {
		return SendResult{}, err
	}
	if !resp.Success() {
		return SendResult{}, fmt.Errorf("send pipeline status card failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return SendResult{
		Channel:    "feishu-bot",
		Receiver:   receiverID,
		Status:     "accepted",
		RemoteID:   stringValue(resp.Data.MessageId),
		RawPayload: string(cardJSON),
	}, nil
}

func (c *Client) requestUserToken(ctx context.Context, path string, appAccessToken string, payload map[string]string) (UserToken, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return UserToken{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openAPIURL(path), bytes.NewReader(body))
	if err != nil {
		return UserToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appAccessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken      string `json:"access_token"`
			RefreshToken     string `json:"refresh_token"`
			ExpiresIn        int64  `json:"expires_in"`
			RefreshExpiresIn int64  `json:"refresh_expires_in"`
		} `json:"data"`
	}
	if err := c.doJSON(req, &response); err != nil {
		return UserToken{}, err
	}
	if response.Code != 0 {
		return UserToken{}, fmt.Errorf("request feishu user token failed: %s", response.Msg)
	}

	now := time.Now().UTC()
	return UserToken{
		AccessToken:           response.Data.AccessToken,
		RefreshToken:          response.Data.RefreshToken,
		AccessTokenExpiresAt:  now.Add(time.Duration(response.Data.ExpiresIn) * time.Second),
		RefreshTokenExpiresAt: now.Add(time.Duration(response.Data.RefreshExpiresIn) * time.Second),
	}, nil
}

func (c *Client) batchGetDepartmentDetails(ctx context.Context, tenantAccessToken string, departmentIDs []string) (map[string]Department, error) {
	results := make(map[string]Department, len(departmentIDs))
	for _, group := range chunkStrings(departmentIDs, 50) {
		items, err := c.requestDepartmentBatch(ctx, tenantAccessToken, group)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			departmentID := strings.TrimSpace(item.DepartmentID)
			if departmentID == "" {
				continue
			}
			results[departmentID] = item
		}
	}
	return results, nil
}

func (c *Client) requestDepartmentBatch(ctx context.Context, tenantAccessToken string, departmentIDs []string) ([]Department, error) {
	if len(departmentIDs) == 0 {
		return nil, nil
	}

	paths := []string{
		"/open-apis/contact/v3/departments/batch_get?department_id_type=department_id",
		"/open-apis/contact/v3/departments/batch?department_id_type=department_id",
	}

	var lastErr error
	for _, path := range paths {
		var response struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items []struct {
					DepartmentID     string `json:"department_id"`
					OpenDepartmentID string `json:"open_department_id"`
					Name             string `json:"name"`
					NameEN           string `json:"name_en"`
				} `json:"items"`
			} `json:"data"`
		}

		err := c.doJSONWithToken(ctx, http.MethodPost, path, tenantAccessToken, map[string]any{
			"department_ids": departmentIDs,
		}, &response)
		if err != nil {
			if strings.Contains(err.Error(), "status=404") {
				lastErr = err
				continue
			}
			return nil, err
		}
		if response.Code != 0 {
			lastErr = fmt.Errorf("batch get departments failed: %s", response.Msg)
			continue
		}

		items := make([]Department, 0, len(response.Data.Items))
		for _, item := range response.Data.Items {
			items = append(items, Department{
				DepartmentID:     strings.TrimSpace(item.DepartmentID),
				OpenDepartmentID: strings.TrimSpace(item.OpenDepartmentID),
				Name:             strings.TrimSpace(item.Name),
				NameEN:           strings.TrimSpace(item.NameEN),
			})
		}
		return items, nil
	}

	if lastErr == nil {
		lastErr = errors.New("batch get departments failed")
	}
	return nil, lastErr
}

func (c *Client) ensureBitableTarget(ctx context.Context, task model.Task) (bitableTarget, bool, error) {
	if !c.Enabled() {
		return bitableTarget{}, false, nil
	}
	if task.BitableAppToken != "" && task.BitableTableID != "" {
		return bitableTarget{AppToken: task.BitableAppToken, TableID: task.BitableTableID}, true, nil
	}
	if c.cfg.BitableAppToken != "" && c.cfg.BitableTableID != "" {
		return bitableTarget{AppToken: c.cfg.BitableAppToken, TableID: c.cfg.BitableTableID}, true, nil
	}
	if c.cfg.BitableFolderToken == "" {
		return bitableTarget{}, false, nil
	}

	c.mu.Lock()
	if c.bitable != nil {
		target := *c.bitable
		c.mu.Unlock()
		return target, true, nil
	}
	c.mu.Unlock()

	appAccessToken, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return bitableTarget{}, false, err
	}

	var createAppResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			App struct {
				AppToken string `json:"app_token"`
			} `json:"app"`
			AppToken string `json:"app_token"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodPost, "/open-apis/bitable/v1/apps", appAccessToken, map[string]any{
		"name":         utils.Coalesce(c.cfg.BitableName, "需求排期"),
		"folder_token": c.cfg.BitableFolderToken,
		"time_zone":    "Asia/Shanghai",
	}, &createAppResp); err != nil {
		return bitableTarget{}, false, err
	}
	if createAppResp.Code != 0 {
		return bitableTarget{}, false, fmt.Errorf("create bitable app failed: %s", createAppResp.Msg)
	}

	appToken := utils.Coalesce(createAppResp.Data.App.AppToken, createAppResp.Data.AppToken)
	if appToken == "" {
		return bitableTarget{}, false, errors.New("bitable app token is empty")
	}

	tableID, err := c.ensureBitableTable(ctx, appAccessToken, appToken)
	if err != nil {
		return bitableTarget{}, false, err
	}

	target := bitableTarget{AppToken: appToken, TableID: tableID}
	c.mu.Lock()
	c.bitable = &target
	c.mu.Unlock()
	return target, true, nil
}

func (c *Client) ensurePipelineBitableTarget(ctx context.Context, run model.PipelineRun) (bitableTarget, bool, error) {
	if !c.Enabled() {
		return bitableTarget{}, false, nil
	}
	if run.BitableAppToken != "" && run.BitableTableID != "" {
		return bitableTarget{AppToken: run.BitableAppToken, TableID: run.BitableTableID}, true, nil
	}
	if c.cfg.BitableAppToken != "" && c.cfg.BitableTableID != "" {
		return bitableTarget{AppToken: c.cfg.BitableAppToken, TableID: c.cfg.BitableTableID}, true, nil
	}
	if c.cfg.BitableFolderToken == "" {
		return bitableTarget{}, false, nil
	}
	return c.ensureBitableTarget(ctx, model.Task{})
}

func (c *Client) ensureBitableTable(ctx context.Context, appAccessToken string, appToken string) (string, error) {
	var listResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				TableID string `json:"table_id"`
				Name    string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodGet, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables", appToken), appAccessToken, nil, &listResp); err != nil {
		return "", err
	}
	if listResp.Code != 0 {
		return "", fmt.Errorf("list bitable tables failed: %s", listResp.Msg)
	}

	for _, item := range listResp.Data.Items {
		if item.Name == utils.Coalesce(c.cfg.BitableName, "需求排期") {
			if err := c.ensureBitableFields(ctx, appAccessToken, appToken, item.TableID); err != nil {
				return "", err
			}
			return item.TableID, nil
		}
	}

	var createResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Table struct {
				TableID string `json:"table_id"`
			} `json:"table"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodPost, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables", appToken), appAccessToken, map[string]any{
		"table": map[string]any{
			"name": utils.Coalesce(c.cfg.BitableName, "需求排期"),
		},
	}, &createResp); err != nil {
		return "", err
	}
	if createResp.Code != 0 {
		return "", fmt.Errorf("create bitable table failed: %s", createResp.Msg)
	}
	if createResp.Data.Table.TableID == "" {
		return "", errors.New("bitable table id is empty")
	}

	if err := c.ensureBitableFields(ctx, appAccessToken, appToken, createResp.Data.Table.TableID); err != nil {
		return "", err
	}
	return createResp.Data.Table.TableID, nil
}

func (c *Client) ensureBitableFields(ctx context.Context, appAccessToken string, appToken string, tableID string) error {
	var listResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				FieldName string `json:"field_name"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := c.doJSONWithToken(ctx, http.MethodGet, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/fields", appToken, tableID), appAccessToken, nil, &listResp); err != nil {
		return err
	}
	if listResp.Code != 0 {
		return fmt.Errorf("list bitable fields failed: %s", listResp.Msg)
	}

	existing := make(map[string]struct{}, len(listResp.Data.Items))
	for _, item := range listResp.Data.Items {
		existing[item.FieldName] = struct{}{}
	}

	for _, field := range []struct {
		Name string
		Type int
	}{
		{Name: "PipelineRunID", Type: 1},
		{Name: "来源", Type: 1},
		{Name: "当前阶段", Type: 1},
		{Name: "发起人", Type: 1},
		{Name: "审批人", Type: 1},
		{Name: "目标仓库", Type: 1},
		{Name: "目标分支", Type: 1},
		{Name: "工作分支", Type: 1},
		{Name: "风险等级", Type: 1},
		{Name: "测试结果", Type: 1},
		{Name: "MR 链接", Type: 1},
		{Name: "Token 成本", Type: 2},
		{Name: "最近失败原因", Type: 1},
		{Name: "审批链接", Type: 1},
		{Name: "交付文档", Type: 1},
		{Name: "需求ID", Type: 1},
		{Name: "需求标题", Type: 1},
		{Name: "任务ID", Type: 1},
		{Name: "任务标题", Type: 1},
		{Name: "任务类型", Type: 1},
		{Name: "负责人", Type: 1},
		{Name: "负责人ID", Type: 1},
		{Name: "优先级", Type: 1},
		{Name: "预计工期", Type: 2},
		{Name: "计划开始", Type: 5},
		{Name: "计划结束", Type: 5},
		{Name: "状态", Type: 1},
		{Name: "通知文案", Type: 1},
		{Name: "文档链接", Type: 1},
	} {
		if _, ok := existing[field.Name]; ok {
			continue
		}

		var createResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := c.doJSONWithToken(ctx, http.MethodPost, fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/fields", appToken, tableID), appAccessToken, map[string]any{
			"field_name": field.Name,
			"type":       field.Type,
		}, &createResp); err != nil {
			return err
		}
		if createResp.Code != 0 {
			return fmt.Errorf("create bitable field %s failed: %s", field.Name, createResp.Msg)
		}
	}

	return nil
}

func (c *Client) buildTaskRecordFields(task model.Task) map[string]any {
	fields := map[string]any{
		"需求ID":  task.SessionID,
		"需求标题":  task.Title,
		"任务ID":  task.ID,
		"任务标题":  task.Title,
		"任务类型":  string(task.Type),
		"负责人":   task.AssigneeName,
		"负责人ID": task.AssigneeID,
		"优先级":   string(task.Priority),
		"预计工期":  task.EstimateDays,
		"状态":    string(task.Status),
		"通知文案":  task.NotifyContent,
		"文档链接":  task.DocURL,
	}
	if task.PlannedStartAt != nil {
		fields["计划开始"] = task.PlannedStartAt.UnixMilli()
	}
	if task.PlannedEndAt != nil {
		fields["计划结束"] = task.PlannedEndAt.UnixMilli()
	}
	return fields
}

func (c *Client) buildPipelineRunRecordFields(payload PipelineRunSyncPayload) map[string]any {
	run := payload.Run
	fields := map[string]any{
		"PipelineRunID": run.ID,
		"需求ID":          utils.Coalesce(run.SourceSessionID, run.ID),
		"需求标题":          run.Title,
		"来源":            pipelineSource(run),
		"状态":            string(run.Status),
		"当前阶段":          run.CurrentStageKey,
		"发起人":           run.CreatedBy,
		"审批人":           latestPendingApprover(payload.Checkpoints, run.CreatedBy),
		"目标仓库":          run.TargetRepo,
		"目标分支":          run.TargetBranch,
		"工作分支":          run.WorkBranch,
		"风险等级":          latestRiskLevel(payload.Artifacts),
		"测试结果":          latestTestResult(payload.Artifacts),
		"MR 链接":         latestPRMRURL(payload.Deliveries),
		"Token 成本":      totalTokenUsage(payload.AgentRuns),
		"最近失败原因":        latestFailureCause(payload.Stages, payload.FailureCause),
		"审批链接":          fmt.Sprintf("%s/approvals/%s", strings.TrimRight(c.cfg.BaseURL, "/"), run.ID),
		"交付文档":          run.FeishuDocURL,
	}
	if run.StartedAt != nil {
		fields["计划开始"] = run.StartedAt.UnixMilli()
	}
	if run.FinishedAt != nil {
		fields["计划结束"] = run.FinishedAt.UnixMilli()
	}
	return fields
}

func pipelineSource(run model.PipelineRun) string {
	if strings.TrimSpace(run.SourceSessionID) != "" {
		return "bot_message"
	}
	if strings.TrimSpace(run.SelectedDocUrls) != "" && run.SelectedDocUrls != "[]" {
		return "feishu_doc"
	}
	return "manual"
}

func latestPendingApprover(checkpoints []model.Checkpoint, fallback string) string {
	for idx := len(checkpoints) - 1; idx >= 0; idx-- {
		if checkpoints[idx].Status == model.CheckpointPending {
			return fallback
		}
		if checkpoints[idx].ApproverID != "" {
			return checkpoints[idx].ApproverID
		}
	}
	return fallback
}

func latestRiskLevel(artifacts []model.Artifact) string {
	for idx := len(artifacts) - 1; idx >= 0; idx-- {
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(artifacts[idx].ContentJSON), &payload)
		if value, ok := payload["riskLevel"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
		if issues, ok := payload["issues"].([]any); ok && len(issues) > 3 {
			return "high"
		}
	}
	return "medium"
}

func latestTestResult(artifacts []model.Artifact) string {
	for idx := len(artifacts) - 1; idx >= 0; idx-- {
		if artifacts[idx].ArtifactType != model.ArtifactTestExecution && artifacts[idx].ArtifactType != model.ArtifactTestReport {
			continue
		}
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(artifacts[idx].ContentJSON), &payload)
		if status, ok := payload["status"].(string); ok && strings.TrimSpace(status) != "" {
			return status
		}
		if conclusion, ok := payload["conclusion"].(string); ok && strings.TrimSpace(conclusion) != "" {
			return conclusion
		}
		return "passed"
	}
	return "skipped"
}

func latestPRMRURL(deliveries []model.GitDelivery) string {
	for idx := len(deliveries) - 1; idx >= 0; idx-- {
		if strings.TrimSpace(deliveries[idx].PRMRURL) != "" {
			return deliveries[idx].PRMRURL
		}
	}
	return ""
}

func latestFailureCause(stages []model.StageRun, fallback string) string {
	for idx := len(stages) - 1; idx >= 0; idx-- {
		if strings.TrimSpace(stages[idx].ErrorMessage) != "" {
			return stages[idx].ErrorMessage
		}
	}
	return fallback
}

func totalTokenUsage(agentRuns []model.AgentRun) int64 {
	var total int64
	for _, agentRun := range agentRuns {
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(agentRun.TokenUsageJSON), &payload); err != nil {
			continue
		}
		if value, ok := numericTokenValue(payload["total"]); ok {
			total += value
			continue
		}
		if value, ok := numericTokenValue(payload["total_tokens"]); ok {
			total += value
			continue
		}
		if value, ok := numericTokenValue(payload["totalTokens"]); ok {
			total += value
			continue
		}
		for _, key := range []string{"prompt", "completion"} {
			if value, ok := numericTokenValue(payload[key]); ok {
				total += value
			}
		}
	}
	return total
}

func numericTokenValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}

func (c *Client) openAPIURL(path string) string {
	base := strings.TrimRight(utils.Coalesce(c.cfg.OpenBaseURL, "https://open.feishu.cn"), "/")
	return base + path
}

type BitableRecord struct {
	Fields map[string]any `json:"fields"`
}

func (c *Client) CopyBitableTemplate(ctx context.Context, templateAppToken string) (string, error) {
	token, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}

	body, _ := json.Marshal(map[string]any{
		"name":            "AI 生成任务表格",
		"without_content": false,
		"time_zone":       "Asia/Shanghai",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.openAPIURL(fmt.Sprintf("/open-apis/bitable/v1/apps/%s/copy", templateAppToken)),
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			App struct {
				AppToken string `json:"app_token"`
			} `json:"app"`
		} `json:"data"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("copy bitable template failed: %s", resp.Msg)
	}
	return resp.Data.App.AppToken, nil
}

func (c *Client) GetBitableTableID(ctx context.Context, appToken string) (string, error) {
	token, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.openAPIURL(fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables", appToken)),
		nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				TableID string `json:"table_id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("get bitable tables failed: %s", resp.Msg)
	}
	if len(resp.Data.Items) == 0 {
		return "", fmt.Errorf("no tables found in bitable app %s", appToken)
	}
	return resp.Data.Items[0].TableID, nil
}

func (c *Client) BatchCreateBitableRecords(ctx context.Context, appToken, tableID string, records []BitableRecord) ([]string, error) {
	token, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]any{"records": records})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.openAPIURL(fmt.Sprintf("/open-apis/bitable/v1/apps/%s/tables/%s/records/batch_create", appToken, tableID)),
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Records []struct {
				RecordID string `json:"record_id"`
			} `json:"records"`
		} `json:"data"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("batch create bitable records failed: %s", resp.Msg)
	}

	ids := make([]string, len(resp.Data.Records))
	for i, r := range resp.Data.Records {
		ids[i] = r.RecordID
	}
	return ids, nil
}

func (c *Client) BitableTemplateToken() string {
	return c.cfg.BitableTemplateToken
}

// BaseURL returns the configured base URL of the application
func (c *Client) BaseURL() string {
	return c.cfg.BaseURL
}

func (c *Client) doJSON(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("feishu api request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode feishu api response: %w", err)
	}
	return nil
}

func (c *Client) doJSONWithToken(ctx context.Context, method string, path string, token string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.openAPIURL(path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	return c.doJSON(req, target)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func chunkStrings(items []string, size int) [][]string {
	if size <= 0 {
		return [][]string{items}
	}
	if len(items) == 0 {
		return nil
	}

	chunks := make([][]string, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func formatTaskDate(value *time.Time) string {
	if value == nil {
		return "待排期"
	}
	return value.Format("2006-01-02")
}
