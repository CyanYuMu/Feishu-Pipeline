package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
	"feishu-pipeline/apps/api-go/internal/service"
	authtype "feishu-pipeline/apps/api-go/internal/type/auth"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService    *service.AuthService
	cookieName     string
	cookieTTL      time.Duration
	cookieSecure   bool
	cookieSameSite http.SameSite
}

func NewAuthController(authService *service.AuthService, cookieName string, cookieTTL time.Duration, cookieSecure bool, cookieSameSite string) *AuthController {
	return &AuthController{
		authService:    authService,
		cookieName:     cookieName,
		cookieTTL:      cookieTTL,
		cookieSecure:   cookieSecure,
		cookieSameSite: parseSameSite(cookieSameSite),
	}
}

// GitHubConfig
// @tags 认证
// @summary 获取 GitHub OAuth 前端配置
// @router /api/auth/github/config [GET]
// @produce application/json
func (c *AuthController) GitHubConfig(ctx *gin.Context) {
	clientID := c.authService.GitHubClientID()
	writeSuccess(ctx, http.StatusOK, map[string]interface{}{
		"enabled":    clientID != "",
		"clientId":   clientID,
		"authorizeUrl": "https://github.com/login/oauth/authorize",
		"callbackUrl": "/api/auth/github/callback",
	})
}

// GitHubLogin
// @tags 认证
// @summary GitHub OAuth 登录
// @router /api/auth/github/login [POST]
// @accept application/json
// @produce application/json
// @param req body map[string]string true "json入参: code, user_id, name, email, avatar"
// @success 200 {object} authtype.LoginResponse
// @failure 400 {object} map[string]string
func (c *AuthController) GitHubLogin(ctx *gin.Context) {
	var request map[string]string
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	code := request["code"]
	userID := request["user_id"]
	name := request["name"]
	email := request["email"]
	avatar := request["avatar"]

	if code == "" && userID == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("code or user_id is required"))
		return
	}

	var user model.User
	var session model.LoginSession
	var err error

	if code != "" {
		// 通过 code 登录（TS后端已验证过 GitHub）
		user, session, err = c.authService.LoginByGitHubCode(ctx.Request.Context(), code)
	} else {
		// 直接通过 user_id 登录（TS后端传递的用户信息）
		user, session, err = c.authService.LoginByGitHubUserID(ctx.Request.Context(), userID, name, email, avatar)
	}

	if err != nil {
		writeError(ctx, http.StatusUnauthorized, err)
		return
	}

	c.writeSessionCookie(ctx, session)
	writeSuccess(ctx, http.StatusOK, authtype.NewLoginResponse(user))
}

// GitHubAuthorize
// @tags 认证
// @summary GitHub OAuth 授权跳转
// @description 构建 GitHub OAuth 授权 URL 并重定向
// @router /api/auth/github/authorize [GET]
// @param state query string true "状态参数，用于 CSRF 防护"
// @param redirect query string false "授权成功后的重定向 URL"
// @success 302 {string} string "重定向到 GitHub 授权页面"
func (c *AuthController) GitHubAuthorize(ctx *gin.Context) {
	clientID := c.authService.GitHubClientID()
	if clientID == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("github oauth not configured"))
		return
	}

	state := ctx.Query("state")
	redirectURI := ctx.Query("redirect")
	callbackURL := c.GitHubCallbackURL()

	// 构建回调 URL，如果有最终重定向目标则传递
	finalCallbackURL := callbackURL
	if redirectURI != "" {
		finalCallbackURL = callbackURL + "?redirect=" + url.QueryEscape(redirectURI)
	}

	// 构建 GitHub OAuth 授权 URL
	authURL := "https://github.com/login/oauth/authorize?" +
		"client_id=" + clientID +
		"&redirect_uri=" + url.QueryEscape(finalCallbackURL) +
		"&scope=read:user,user:email,repo" +
		"&state=" + state

	ctx.Redirect(http.StatusFound, authURL)
}

// GitHubCallbackURL returns the GitHub OAuth callback URL
func (c *AuthController) GitHubCallbackURL() string {
	return "http://localhost:5173/auth/callback"
}

// GitHubCallback
// @tags 认证
// @summary GitHub OAuth 回调
// @description 接收 GitHub OAuth 授权码，交换 access_token，创建登录会话，然后重定向到前端
// @router /api/auth/github/callback [GET]
// @produce application/json
// @param code query string true "GitHub 授权码"
// @param state query string false "状态参数"
// @param redirect query string false "授权成功后的重定向 URL"
// @success 302 {string} string "重定向到前端页面"
// @failure 400 {object} map[string]string
// @failure 401 {object} map[string]string
func (c *AuthController) GitHubCallback(ctx *gin.Context) {
	code := ctx.Query("code")
	if code == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("missing code parameter"))
		return
	}

	redirectURI := ctx.Query("redirect")

	_, session, err := c.authService.LoginByGitHubCode(ctx.Request.Context(), code)
	if err != nil {
		writeError(ctx, http.StatusUnauthorized, err)
		return
	}

	// 设置 Session Cookie
	c.writeSessionCookie(ctx, session)

	// 优先使用 redirect 参数，否则默认跳转到前端首页
	targetURL := redirectURI
	if targetURL == "" {
		targetURL = "http://localhost:5173/"
	} else {
		// 清理 URL 中的 provider 参数，避免重复
		redirectURL, _ := url.Parse(targetURL)
		if redirectURL != nil {
			q := redirectURL.Query()
			q.Del("provider")
			redirectURL.RawQuery = q.Encode()
			targetURL = redirectURL.String()
		}
	}

	ctx.Redirect(http.StatusFound, targetURL)
}

// GitHubBind
// @tags 认证
// @summary 为当前用户绑定 GitHub 账号
// @router /api/auth/github/bind [POST]
// @accept application/json
// @produce application/json
// @param req body map[string]string true "json入参: code"
// @success 200 {object} authtype.UserResponse
func (c *AuthController) GitHubBind(ctx *gin.Context) {
	var request map[string]string
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	code := request["code"]
	if code == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("code is required"))
		return
	}

	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	user, err := c.authService.BindGitHubToUser(ctx.Request.Context(), userID, code)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, authtype.NewUserResponse(user))
}

// GitHubUnbind
// @tags 认证
// @summary 解绑当前用户的 GitHub 账号
// @router /api/auth/github/unbind [POST]
// @produce application/json
// @success 200 {object} map[string]string
func (c *AuthController) GitHubUnbind(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	_, err := c.authService.UnbindGitHubFromUser(ctx.Request.Context(), userID)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, map[string]string{"status": "unbound"})
}

// GitHubRepos
// @tags 认证
// @summary 获取当前用户绑定的 GitHub 仓库列表
// @router /api/github/repos [GET]
// @produce application/json
// @success 200 {object} []service.GitHubRepo
func (c *AuthController) GitHubRepos(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	repos, err := c.authService.ListGitHubRepos(ctx.Request.Context(), userID)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, repos)
}

// GitHubBranches
// @tags 认证
// @summary 获取 GitHub 仓库分支列表
// @router /api/github/repos/{owner}/{repo}/branches [GET]
// @param owner path string true "仓库所有者"
// @param repo path string true "仓库名称"
// @produce application/json
func (c *AuthController) GitHubBranches(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	owner := ctx.Param("owner")
	repo := ctx.Param("repo")
	if owner == "" || repo == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("owner and repo are required"))
		return
	}

	branches, err := c.authService.ListGitHubBranches(ctx.Request.Context(), userID, owner, repo)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, branches)
}

// CreateGitHubRepo
// @tags 认证
// @summary 创建 GitHub 仓库
// @router /api/github/repos [POST]
// @accept application/json
// @produce application/json
// @param req body service.CreateGitHubRepoInput true "仓库信息"
func (c *AuthController) CreateGitHubRepo(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	var input service.CreateGitHubRepoInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeError(ctx, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	if input.Name == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("repo name is required"))
		return
	}

	result, err := c.authService.CreateGitHubRepo(ctx.Request.Context(), userID, input)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusCreated, result)
}

// FeishuDocuments
// @tags 认证
// @summary 获取飞书文档列表
// @router /api/feishu/documents [GET]
// @param folder_token query string false "文件夹token"
// @produce application/json
func (c *AuthController) FeishuDocuments(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	folderToken := ctx.Query("folder_token")
	docs, err := c.authService.GetFeishuDocuments(ctx.Request.Context(), userID, folderToken)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, docs)
}

// FeishuDocumentContent
// @tags 认证
// @summary 获取飞书文档内容
// @router /api/feishu/documents/:documentId/content [GET]
// @param documentId path string true "文档ID"
// @produce text/plain
func (c *AuthController) FeishuDocumentContent(ctx *gin.Context) {
	userID := currentUserID(ctx)
	if userID == "" {
		writeError(ctx, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}

	documentID := ctx.Param("documentId")
	if documentID == "" {
		writeError(ctx, http.StatusBadRequest, errors.New("document_id is required"))
		return
	}

	content, err := c.authService.GetFeishuDocumentContent(ctx.Request.Context(), userID, documentID)
	if err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// FeishuConfig
// @tags 认证
// @summary 获取飞书 SSO 前端配置
// @router /api/auth/feishu/config [GET]
// @produce application/json
func (c *AuthController) FeishuConfig(ctx *gin.Context) {
	writeSuccess(ctx, http.StatusOK, authtype.FeishuSSOConfigResponse{
		Enabled: c.authService.FeishuEnabled(),
		AppID:   c.authService.FeishuAppID(),
		Scope:   c.authService.FeishuOAuthScope(),
	})
}

// SSOLogin
// @tags 认证
// @summary 飞书 SSO 登录
// @router /api/auth/feishu/sso/login [POST]
// @accept application/json
// @produce application/json
// @param req body authtype.FeishuSSOLoginRequest true "json入参"
func (c *AuthController) SSOLogin(ctx *gin.Context) {
	var request authtype.FeishuSSOLoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	user, session, err := c.authService.LoginByCode(ctx.Request.Context(), request.Code)
	if err != nil {
		writeError(ctx, http.StatusBadGateway, err)
		return
	}

	c.writeSessionCookie(ctx, session)
	writeSuccess(ctx, http.StatusOK, authtype.NewLoginResponse(user))
}

// Logout
// @tags 认证
// @summary 登出
// @router /api/auth/logout [POST]
// @produce application/json
func (c *AuthController) Logout(ctx *gin.Context) {
	sessionID, _ := ctx.Cookie(c.cookieName)
	if err := c.authService.Logout(ctx.Request.Context(), sessionID); err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}
	c.clearSessionCookie(ctx)
	writeSuccess(ctx, http.StatusOK, map[string]string{"status": "logged_out"})
}

// Me
// @tags 认证
// @summary 当前登录用户信息
// @router /api/me [GET]
// @produce application/json
func (c *AuthController) Me(ctx *gin.Context) {
	user, err := c.authService.CurrentUser(ctx.Request.Context(), currentUserID(ctx))
	if err != nil {
		writeError(ctx, http.StatusUnauthorized, err)
		return
	}
	writeSuccess(ctx, http.StatusOK, authtype.NewUserResponse(user))
}

func (c *AuthController) writeSessionCookie(ctx *gin.Context, session model.LoginSession) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     c.cookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: c.cookieSameSite,
		MaxAge:   int(c.cookieTTL.Seconds()),
		Expires:  session.ExpiresAt,
	})
}

func (c *AuthController) clearSessionCookie(ctx *gin.Context) {
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     c.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.cookieSecure,
		SameSite: c.cookieSameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// HandleFeishuCardCallback 处理飞书交互卡片按钮回调
// @tags 飞书
// @summary 飞书卡片回调
// @description 接收飞书交互卡片按钮点击事件，处理 approve/reject 操作
// @router /public/feishu/card/callback [POST]
// @accept application/json
// @produce application/json
func (c *AuthController) HandleFeishuCardCallback(ctx *gin.Context) {
	var request service.FeishuCardCallbackRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeError(ctx, http.StatusBadRequest, err)
		return
	}

	result, err := c.authService.HandleFeishuCardCallback(ctx.Request.Context(), request)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, err)
		return
	}

	writeSuccess(ctx, http.StatusOK, result)
}
