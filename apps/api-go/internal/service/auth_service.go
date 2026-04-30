package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/repo"
	"feishu-pipeline/apps/api-go/internal/utils"

	"gorm.io/gorm"
)

// 创建带代理支持的 HTTP Client
func newGitHubHTTPClient() *http.Client {
	transport := &http.Transport{}

	// 优先使用 GitHub 专用代理
	proxyStr := os.Getenv("GITHUB_HTTPS_PROXY")
	if proxyStr == "" {
		proxyStr = os.Getenv("GITHUB_HTTP_PROXY")
	}
	// 自动检测系统代理环境变量
	if proxyStr == "" {
		proxyStr = os.Getenv("HTTPS_PROXY")
	}
	if proxyStr == "" {
		proxyStr = os.Getenv("https_proxy")
	}
	if proxyStr == "" {
		proxyStr = os.Getenv("HTTP_PROXY")
	}
	if proxyStr == "" {
		proxyStr = os.Getenv("http_proxy")
	}

	if proxyStr != "" {
		proxyURL, err := url.Parse(proxyStr)
		if err == nil && proxyURL.Host != "" && proxyURL.Port() != "" && proxyURL.Port() != "0" {
			transport.Proxy = http.ProxyURL(proxyURL)
			log.Printf("[GitHub OAuth] Using proxy: %s", proxyURL.String())
		} else {
			log.Printf("[GitHub OAuth] Invalid proxy config '%s', connecting directly", proxyStr)
			transport.Proxy = nil
		}
	} else {
		log.Printf("[GitHub OAuth] No proxy configured, connecting directly to GitHub")
		transport.Proxy = nil
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

var ErrAuthenticationRequired = errors.New("authentication required")

// stringValue 安全获取字符串指针的值，如果为 nil 则返回默认值
func stringValue(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

type AuthService struct {
	repository    *repo.Repository
	feishuClient  *feishu.Client
	sessionTTL    time.Duration
	githubClientID     string
	githubClientSecret string
	githubRedirectURI  string
}

func NewAuthService(repository *repo.Repository, feishuClient *feishu.Client, sessionTTL time.Duration) *AuthService {
	return &AuthService{
		repository:   repository,
		feishuClient: feishuClient,
		sessionTTL:   sessionTTL,
	}
}

func NewAuthServiceWithGitHub(repository *repo.Repository, feishuClient *feishu.Client, sessionTTL time.Duration, githubClientID, githubClientSecret, githubRedirectURI string) *AuthService {
	return &AuthService{
		repository:        repository,
		feishuClient:      feishuClient,
		sessionTTL:        sessionTTL,
		githubClientID:    githubClientID,
		githubClientSecret: githubClientSecret,
		githubRedirectURI: githubRedirectURI,
	}
}

func (s *AuthService) FeishuAppID() string {
	return s.feishuClient.AppID()
}

func (s *AuthService) FeishuEnabled() bool {
	return s.feishuClient.Enabled()
}

func (s *AuthService) FeishuOAuthScope() string {
	return s.feishuClient.OAuthScope()
}

func (s *AuthService) GitHubClientID() string {
	return s.githubClientID
}

// 直接通过 GitHub 用户信息创建会话（TS后端验证后调用）
func (s *AuthService) LoginByGitHubUserID(ctx context.Context, userID, name, email, avatar string) (model.User, model.LoginSession, error) {
	user := model.User{
		ID:        userID,
		Name:      name,
		Email:     email,
		AvatarURL: avatar,
		Role:      model.RoleOther,
	}
	if user.Name == "" {
		user.Name = "GitHub用户"
	}
	if email != "" {
		user.Departments = inferDepartmentsFromEmail(email)
	} else {
		user.Departments = []string{"其他"}
	}

	// 如果用户已存在，保留原角色
	if existing, err := s.repository.FindUserByID(ctx, user.ID); err == nil {
		if existing.Role == model.RoleAdmin {
			user.Role = model.RoleAdmin
		}
	}

	// 保存用户
	if err := s.repository.UpsertUser(ctx, &user); err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	// 创建登录会话
	now := time.Now().UTC()
	loginSession := model.LoginSession{
		ID:        utils.NewID("login"),
		UserID:    user.ID,
		ExpiresAt: now.Add(s.sessionTTL),
	}

	if err := s.repository.CreateLoginSession(ctx, &loginSession); err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	return user, loginSession, nil
}

// GitHub OAuth 登录（独立登录模式）
func (s *AuthService) LoginByGitHubCode(ctx context.Context, code string) (model.User, model.LoginSession, error) {
	if s.githubClientID == "" || s.githubClientSecret == "" {
		return model.User{}, model.LoginSession{}, errors.New("github oauth not configured")
	}

	// 1. 用 code 换取 access_token
	tokenRes, err := s.exchangeGitHubCode(code)
	if err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	// 2. 用 access_token 获取用户信息
	profile, err := s.getGitHubUserInfo(tokenRes.AccessToken)
	if err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	// 3. 创建/更新用户
	user := model.User{
		ID:        "gh_" + strconv.FormatInt(profile.ID, 10),
		Name:      stringValue(profile.Name, profile.Login),
		Email:     stringValue(profile.Email, ""),
		AvatarURL: profile.AvatarURL,
		Role:      model.RoleOther,
	}
	if user.Email == "" {
		user.Departments = []string{"其他"}
	} else {
		// 尝试从邮箱推断部门
		user.Departments = inferDepartmentsFromEmail(user.Email)
	}

	// 如果用户已存在，保留原角色
	if existing, err := s.repository.FindUserByID(ctx, user.ID); err == nil {
		if existing.Role == model.RoleAdmin {
			user.Role = model.RoleAdmin
		}
	}

	// 保存用户
	if err := s.repository.UpsertUser(ctx, &user); err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	// 4. 创建登录会话
	now := time.Now().UTC()
	loginSession := model.LoginSession{
		ID:        utils.NewID("login"),
		UserID:    user.ID,
		ExpiresAt: now.Add(s.sessionTTL),
	}

	if err := s.repository.CreateLoginSession(ctx, &loginSession); err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	return user, loginSession, nil
}

// BindGitHubToUser 为当前已登录用户绑定 GitHub 账号
func (s *AuthService) BindGitHubToUser(ctx context.Context, userID string, code string) (model.User, error) {
	if s.githubClientID == "" || s.githubClientSecret == "" {
		return model.User{}, errors.New("github oauth not configured")
	}

	// 1. 用 code 换取 access_token
	tokenRes, err := s.exchangeGitHubCode(code)
	if err != nil {
		return model.User{}, err
	}

	// 2. 用 access_token 获取用户信息
	profile, err := s.getGitHubUserInfo(tokenRes.AccessToken)
	if err != nil {
		return model.User{}, err
	}

	// 3. 查找当前用户
	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		return model.User{}, errors.New("user not found")
	}

	// 4. 绑定 GitHub 信息到当前用户
	user.GitHubID = strconv.FormatInt(profile.ID, 10)
	user.GitHubLogin = profile.Login
	user.GitHubAvatar = profile.AvatarURL
	user.GitHubAccessToken = tokenRes.AccessToken // 保存 access_token 用于后续 API 调用

	// 保存用户
	if err := s.repository.UpsertUser(ctx, &user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

// UnbindGitHubFromUser 解绑当前用户的 GitHub 账号
func (s *AuthService) UnbindGitHubFromUser(ctx context.Context, userID string) (model.User, error) {
	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		return model.User{}, errors.New("user not found")
	}

	user.GitHubID = ""
	user.GitHubLogin = ""
	user.GitHubAvatar = ""
	user.GitHubAccessToken = ""

	if err := s.repository.UpsertUser(ctx, &user); err != nil {
		return model.User{}, err
	}

	return user, nil
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type GitHubRepo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
	Description string `json:"description"`
}

// ListGitHubRepos 获取当前用户的 GitHub 仓库列表
func (s *AuthService) ListGitHubRepos(ctx context.Context, userID string) ([]GitHubRepo, error) {
	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		log.Printf("[GitHub Repos] User not found: %s", userID)
		return nil, errors.New("user not found")
	}

	log.Printf("[GitHub Repos] User %s GitHubAccessToken length: %d", userID, len(user.GitHubAccessToken))
	if user.GitHubAccessToken == "" {
		log.Printf("[GitHub Repos] GitHub not bound for user: %s", userID)
		return nil, errors.New("github not bound")
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/repos?sort=updated&per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+user.GitHubAccessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[GitHub Repos] Request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("[GitHub Repos] Response status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[GitHub Repos] Error response: %s", string(body))
		return nil, fmt.Errorf("github api error: status=%d", resp.StatusCode)
	}

	var repos []GitHubRepo
	if err := json.Unmarshal(body, &repos); err != nil {
		log.Printf("[GitHub Repos] Parse error: %v, body: %s", err, string(body))
		return nil, fmt.Errorf("failed to parse github repos response: %v", err)
	}

	log.Printf("[GitHub Repos] Found %d repos", len(repos))
	return repos, nil
}

func (s *AuthService) exchangeGitHubCode(code string) (*githubTokenResponse, error) {
	log.Printf("[GitHub OAuth] Exchanging code: client_id=%s, redirect_uri=%s", s.githubClientID, s.githubRedirectURI)
	
	data := url.Values{}
	data.Set("client_id", s.githubClientID)
	data.Set("client_secret", s.githubClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.githubRedirectURI)

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("[GitHub OAuth] Failed to create request: %v", err)
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[GitHub OAuth] Request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	
	log.Printf("[GitHub OAuth] Response status: %d", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[GitHub OAuth] Failed to read response body: %v", err)
		return nil, err
	}
	
	log.Printf("[GitHub OAuth] Response body: %s", string(body))

	var result githubTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse github token response: %v, body: %s", err, string(body))
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("github oauth failed: no access_token returned, body: %s", string(body))
	}

	return &result, nil
}

type githubUserResponse struct {
	ID        int64   `json:"id"`
	Login     string  `json:"login"`
	Name      *string `json:"name"`
	Email     *string `json:"email"`
	AvatarURL string  `json:"avatar_url"`
}

func (s *AuthService) getGitHubUserInfo(accessToken string) (*githubUserResponse, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := newGitHubHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user githubUserResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to parse github user response: %v", err)
	}

	return &user, nil
}

func inferDepartmentsFromEmail(email string) []string {
	email = strings.ToLower(email)
	domain := ""
	if idx := strings.Index(email, "@"); idx > 0 {
		domain = email[idx+1:]
	}
	switch {
	case strings.Contains(domain, "product"):
		return []string{"产品"}
	case strings.Contains(domain, "frontend") || strings.Contains(domain, "front-end"):
		return []string{"前端"}
	case strings.Contains(domain, "backend") || strings.Contains(domain, "back-end") || strings.Contains(domain, "server"):
		return []string{"后端"}
	default:
		return []string{"其他"}
	}
}

func (s *AuthService) LoginByCode(ctx context.Context, code string) (model.User, model.LoginSession, error) {
	token, err := s.feishuClient.ExchangeCodeForUserToken(ctx, code)
	if err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	profile, err := s.feishuClient.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return model.User{}, model.LoginSession{}, err
	}

	user := mapProfileToUser(profile)
	departments, classifyErr := s.resolveDepartments(ctx, token.AccessToken, profile)
	if classifyErr == nil {
		user.Departments = departments
		user.Role = classifyRoleByDepartments(departments)
	}

	if existing, err := s.repository.FindUserByID(ctx, user.ID); err == nil {
		// 管理员角色由后台显式维护，不被登录时的部门同步覆盖。
		if existing.Role == model.RoleAdmin {
			user.Role = model.RoleAdmin
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, model.LoginSession{}, err
	}

	now := time.Now().UTC()
	credential := model.FeishuCredential{
		ID:                    "cred_" + user.ID,
		UserID:                user.ID,
		OpenID:                profile.OpenID,
		UnionID:               profile.UnionID,
		FeishuUserID:          profile.FeishuUserID,
		AccessToken:           token.AccessToken,
		RefreshToken:          token.RefreshToken,
		AccessTokenExpiresAt:  token.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: token.RefreshTokenExpiresAt,
		LastLoginAt:           now,
		LastRefreshAt:         now,
	}
	loginSession := model.LoginSession{
		ID:        utils.NewID("login"),
		UserID:    user.ID,
		ExpiresAt: now.Add(s.sessionTTL),
	}

	if err := s.repository.SaveFeishuLoginState(ctx, &repo.FeishuLoginState{
		User:       user,
		Credential: credential,
		Session:    loginSession,
	}); err != nil {
		return model.User{}, model.LoginSession{}, err
	}
	return user, loginSession, nil
}

func (s *AuthService) CurrentUser(ctx context.Context, userID string) (model.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return model.User{}, ErrAuthenticationRequired
	}

	user, err := s.repository.FindUserByID(ctx, userID)
	if err == nil {
		credential, refreshErr := s.EnsureFreshCredential(ctx, userID)
		if refreshErr != nil && !errors.Is(refreshErr, gorm.ErrRecordNotFound) {
			return model.User{}, refreshErr
		}
		if user.Role == model.RoleOther && refreshErr == nil {
			if updated, syncErr := s.syncDepartments(ctx, &user, credential.AccessToken, credential.FeishuUserID, credential.OpenID); syncErr == nil && updated {
				return user, nil
			}
		}
		return user, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, ErrAuthenticationRequired
	}
	return user, err
}

func (s *AuthService) syncDepartments(ctx context.Context, user *model.User, accessToken string, feishuUserID string, openID string) (bool, error) {
	userIdentifier := strings.TrimSpace(openID)
	userIDType := "open_id"
	if userIdentifier == "" {
		userIdentifier = strings.TrimSpace(feishuUserID)
		userIDType = "user_id"
	}
	if userIdentifier == "" {
		return false, nil
	}

	departments, err := s.feishuClient.ListUserDepartments(ctx, accessToken, userIdentifier, userIDType)
	if err != nil {
		log.Printf("[sync] department sync failed for user %s: %v", user.ID, err)
		return false, err
	}

	names := make([]string, 0, len(departments))
	for _, d := range departments {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = strings.TrimSpace(d.NameEN)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return false, nil
	}

	user.Departments = names
	newRole := classifyRoleByDepartments(names)
	if newRole != model.RoleOther {
		user.Role = newRole
	}
	if err := s.repository.UpsertUser(ctx, user); err != nil {
		return false, err
	}
	log.Printf("[sync] department synced for user %s: departments=%v role=%s", user.ID, names, user.Role)
	return true, nil
}

func (s *AuthService) ResolveSessionUserID(ctx context.Context, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", ErrAuthenticationRequired
	}

	if err := s.repository.DeleteExpiredLoginSessions(ctx, time.Now().UTC()); err != nil {
		return "", err
	}

	session, err := s.repository.FindLoginSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrAuthenticationRequired
		}
		return "", err
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.repository.DeleteLoginSessionByID(ctx, sessionID)
		return "", ErrAuthenticationRequired
	}
	return session.UserID, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return s.repository.DeleteLoginSessionByID(ctx, sessionID)
}

func (s *AuthService) EnsureFreshCredential(ctx context.Context, userID string) (model.FeishuCredential, error) {
	credential, err := s.repository.FindCredentialByUserID(ctx, userID)
	if err != nil {
		return model.FeishuCredential{}, err
	}

	now := time.Now().UTC()
	if credential.AccessTokenExpiresAt.After(now.Add(1 * time.Minute)) {
		return credential, nil
	}
	if credential.RefreshTokenExpiresAt.Before(now) {
		return model.FeishuCredential{}, ErrAuthenticationRequired
	}

	token, err := s.feishuClient.RefreshUserToken(ctx, credential.RefreshToken)
	if err != nil {
		return model.FeishuCredential{}, err
	}

	credential.AccessToken = token.AccessToken
	credential.RefreshToken = token.RefreshToken
	credential.AccessTokenExpiresAt = token.AccessTokenExpiresAt
	credential.RefreshTokenExpiresAt = token.RefreshTokenExpiresAt
	credential.LastRefreshAt = now
	if err := s.repository.SaveCredential(ctx, &credential); err != nil {
		return model.FeishuCredential{}, err
	}
	return credential, nil
}

func mapProfileToUser(profile feishu.UserProfile) model.User {
	return model.User{
		ID:           "fs_" + profile.OpenID,
		FeishuOpenID: profile.OpenID,
		Name:         utils.Coalesce(profile.Name, profile.EnName, "飞书用户"),
		Email:        utils.Coalesce(profile.EnterpriseEmail, profile.Email),
		AvatarURL:    profile.AvatarURL,
		Role:         model.RoleOther,
		Departments:  []string{"其他"},
	}
}

func (s *AuthService) resolveDepartments(ctx context.Context, userAccessToken string, profile feishu.UserProfile) ([]string, error) {
	userIdentifier := strings.TrimSpace(profile.OpenID)
	userIDType := "open_id"
	if userIdentifier == "" {
		userIdentifier = strings.TrimSpace(profile.FeishuUserID)
		userIDType = "user_id"
	}
	if userIdentifier == "" {
		return []string{"其他"}, errors.New("feishu user identifier is empty")
	}

	items, err := s.feishuClient.ListUserDepartments(ctx, userAccessToken, userIdentifier, userIDType)
	if err != nil {
		return []string{"其他"}, err
	}
	if len(items) == 0 {
		return []string{"其他"}, nil
	}

	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.NameEN)
		}
		if isLikelyDepartmentCode(name) {
			continue
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		return []string{"其他"}, nil
	}
	return names, nil
}

func classifyRoleByDepartments(departments []string) model.Role {
	hasKeyword := func(keywords ...string) bool {
		for _, department := range departments {
			name := normalizeDepartmentName(department)
			for _, keyword := range keywords {
				if strings.Contains(name, keyword) {
					return true
				}
			}
		}
		return false
	}

	switch {
	case hasKeyword("产品", "product", "pm"):
		return model.RoleProduct
	case hasKeyword("前端", "frontend", "front-end", "fe"):
		return model.RoleFrontend
	case hasKeyword("后端", "backend", "back-end", "be", "服务端", "server"):
		return model.RoleBackend
	default:
		return model.RoleOther
	}
}

func normalizeDepartmentName(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func isLikelyDepartmentCode(value string) bool {
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
