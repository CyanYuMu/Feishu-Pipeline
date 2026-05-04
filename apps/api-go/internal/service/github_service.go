package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitHubContentService 提供 GitHub 仓库内容读取能力
type GitHubContentService struct {
	authService *AuthService
	httpClient  *http.Client
}

// NewGitHubContentService 创建 GitHub 内容服务
func NewGitHubContentService(authService *AuthService) *GitHubContentService {
	return &GitHubContentService{
		authService: authService,
		httpClient:  newGitHubHTTPClient(),
	}
}

// RepoContent 仓库内容项
type RepoContent struct {
	Type        string `json:"type"`        // "file" or "dir"
	Name        string `json:"name"`
	Path        string `json:"path"`
	Content     string `json:"content,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	DownloadURL string `json:"download_url,omitempty"`
}

// GetRepoContents 获取仓库目录内容
func (s *GitHubContentService) GetRepoContents(ctx context.Context, userID, owner, repo, path, branch string) ([]RepoContent, error) {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return nil, err
	}

	apiPath := path
	if apiPath == "" {
		apiPath = "/"
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, apiPath, branch)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("仓库路径不存在: %s", path)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("没有权限访问仓库")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 判断是文件还是目录
	var contents []RepoContent
	if err := json.Unmarshal(body, &contents); err != nil {
		// 可能是单个文件响应
		var file RepoContent
		if err := json.Unmarshal(body, &file); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}
		return []RepoContent{file}, nil
	}

	return contents, nil
}

// GetFileContent 获取单个文件内容
func (s *GitHubContentService) GetFileContent(ctx context.Context, userID, owner, repo, path, ref string) (content string, sha string, err error) {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return "", "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("文件不存在: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	// GitHub 返回 Base64 编码的内容
	if result.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(result.Content)
		if err != nil {
			return "", result.SHA, fmt.Errorf("解码文件内容失败: %w", err)
		}
		return string(decoded), result.SHA, nil
	}

	return result.Content, result.SHA, nil
}

// GetFileContentRaw 直接获取文件内容（不处理 base64）
func (s *GitHubContentService) GetFileContentRaw(ctx context.Context, userID, owner, repo, path, ref string) (string, string, error) {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return "", "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("文件不存在: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	sha := result.SHA
	
	// GitHub 返回 Base64 编码的内容
	if result.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(result.Content, "\n", ""))
		if err != nil {
			return "", sha, fmt.Errorf("解码文件内容失败: %w", err)
		}
		return string(decoded), sha, nil
	}

	return result.Content, sha, nil
}

// CreateOrUpdateFile 创建或更新文件
func (s *GitHubContentService) CreateOrUpdateFile(ctx context.Context, userID, owner, repo, path, branch, content, message, sha string) error {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)
	
	// 构建请求体
	body := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  branch,
	}
	if sha != "" {
		body["sha"] = sha // 更新文件需要提供 SHA
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnprocessableEntity && strings.Contains(resp.Header.Get("Status"), "422") {
		// 文件已存在但没有提供 SHA
		return fmt.Errorf("文件已存在，需要提供 SHA 进行更新")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteFile 删除文件
func (s *GitHubContentService) DeleteFile(ctx context.Context, userID, owner, repo, path, branch, sha, message string) error {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	// GitHub 删除 API 需要 SHA
	if sha == "" {
		return fmt.Errorf("删除文件需要提供 SHA")
	}

	body := map[string]string{
		"message": message,
		"sha":     sha,
		"branch":  branch,
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateBranch 创建新分支
func (s *GitHubContentService) CreateBranch(ctx context.Context, userID, owner, repo, branch, fromBranch string) error {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", owner, repo)

	body := map[string]string{
		"ref": fmt.Sprintf("refs/heads/%s", branch),
		"sha": fromBranch, // 需要提供源分支的 SHA
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetBranchSHA 获取分支的最新 SHA
func (s *GitHubContentService) GetBranchSHA(ctx context.Context, userID, owner, repo, branch string) (string, error) {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s", owner, repo, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("分支不存在: %s", branch)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Commit.SHA, nil
}

// ListFilesRecursive 递归列出仓库中的所有文件
func (s *GitHubContentService) ListFilesRecursive(ctx context.Context, userID, owner, repo, branch string, maxDepth int) ([]RepoContent, error) {
	var allFiles []RepoContent
	
	err := s.walkDirectory(ctx, userID, owner, repo, "", branch, maxDepth, 0, &allFiles)
	if err != nil {
		return nil, err
	}

	return allFiles, nil
}

// walkDirectory 递归遍历目录
func (s *GitHubContentService) walkDirectory(ctx context.Context, userID, owner, repo, path, branch string, maxDepth, currentDepth int, results *[]RepoContent) error {
	if maxDepth > 0 && currentDepth >= maxDepth {
		return nil
	}

	contents, err := s.GetRepoContents(ctx, userID, owner, repo, path, branch)
	if err != nil {
		// 如果是文件（不是目录），直接添加
		if path != "" {
			*results = append(*results, RepoContent{Path: path, Type: "file"})
		}
		return nil
	}

	for _, item := range contents {
		if item.Type == "dir" {
			// 递归处理子目录
			s.walkDirectory(ctx, userID, owner, repo, item.Path, branch, maxDepth, currentDepth+1, results)
		} else {
			*results = append(*results, item)
		}
	}

	return nil
}

// SearchFiles 搜索仓库中的文件
func (s *GitHubContentService) SearchFiles(ctx context.Context, userID, owner, repo, query, branch string) ([]RepoContent, error) {
	// 先获取所有文件，然后过滤
	allFiles, err := s.ListFilesRecursive(ctx, userID, owner, repo, branch, 5)
	if err != nil {
		return nil, err
	}

	var matchingFiles []RepoContent
	queryLower := strings.ToLower(query)

	for _, file := range allFiles {
		// 检查文件名和路径是否包含查询词
		if strings.Contains(strings.ToLower(file.Name), queryLower) ||
			strings.Contains(strings.ToLower(file.Path), queryLower) {
			matchingFiles = append(matchingFiles, file)
		}
	}

	return matchingFiles, nil
}

// GetUserGitHubToken 获取用户的 GitHub 访问令牌
func (s *GitHubContentService) getUserGitHubToken(ctx context.Context, userID string) (string, error) {
	user, err := s.authService.CurrentUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("获取用户失败: %w", err)
	}

	if user.GitHubAccessToken == "" {
		return "", fmt.Errorf("用户未绑定 GitHub 账号")
	}

	return user.GitHubAccessToken, nil
}

// CheckRepoAccess 检查用户是否有仓库访问权限
func (s *GitHubContentService) CheckRepoAccess(ctx context.Context, userID, owner, repo string) (bool, error) {
	token, err := s.getUserGitHubToken(ctx, userID)
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}