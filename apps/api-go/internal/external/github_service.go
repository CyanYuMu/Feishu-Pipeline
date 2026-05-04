package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubService 提供 GitHub 仓库内容读取能力
type GitHubService struct {
	httpClient *http.Client
}

// NewGitHubService 创建 GitHub 服务
func NewGitHubService() *GitHubService {
	return &GitHubService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// RepoFile 仓库文件信息
type RepoFile struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	SHA     string `json:"sha,omitempty"`
	Size    int    `json:"size,omitempty"`
}

// GetRepoContents 获取仓库目录内容
func (s *GitHubService) GetRepoContents(ctx context.Context, token, owner, repo, path, branch string) ([]RepoFile, error) {
	apiPath := path
	if apiPath == "" {
		apiPath = "/"
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, apiPath, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req, token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(body))
	}

	var contents []RepoFile
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		var single RepoFile
		if err := json.NewDecoder(resp.Body).Decode(&single); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}
		return []RepoFile{single}, nil
	}

	return contents, nil
}

// GetFileContent 获取文件内容
func (s *GitHubService) GetFileContent(ctx context.Context, token, owner, repo, path, ref string) (content, sha string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	s.setAuthHeader(req, token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("GitHub API 错误: %d", resp.StatusCode)
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		SHA      string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	sha = result.SHA
	if result.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(result.Content, "\n", ""))
		if err != nil {
			return "", sha, fmt.Errorf("解码失败: %w", err)
		}
		return string(decoded), sha, nil
	}
	return result.Content, sha, nil
}

// CreateFile 创建或更新文件
func (s *GitHubService) CreateFile(ctx context.Context, token, owner, repo, path, branch, content, message, sha string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	body := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  branch,
	}
	if sha != "" {
		body["sha"] = sha
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	s.setAuthHeader(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ListFilesRecursive 递归列出文件
func (s *GitHubService) ListFilesRecursive(ctx context.Context, token, owner, repo, branch string, maxDepth int) ([]RepoFile, error) {
	var allFiles []RepoFile
	s.walkDir(ctx, token, owner, repo, "", branch, maxDepth, 0, &allFiles)
	return allFiles, nil
}

func (s *GitHubService) walkDir(ctx context.Context, token, owner, repo, path, branch string, maxDepth, depth int, results *[]RepoFile) error {
	if maxDepth > 0 && depth >= maxDepth {
		return nil
	}
	contents, err := s.GetRepoContents(ctx, token, owner, repo, path, branch)
	if err != nil {
		return nil
	}
	for _, item := range contents {
		if item.Type == "dir" {
			s.walkDir(ctx, token, owner, repo, item.Path, branch, maxDepth, depth+1, results)
		} else {
			*results = append(*results, item)
		}
	}
	return nil
}

// setAuthHeader 设置认证头
func (s *GitHubService) setAuthHeader(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Feishu-Pipeline/1.0")
}

// CreatePullRequest 创建 Pull Request
func (s *GitHubService) CreatePullRequest(ctx context.Context, token, owner, repo, head, base, title, body string) (prNumber int, prURL string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)

	reqBody := map[string]any{
		"head": head,
		"base": base,
		"title": title,
	}
	if body != "" {
		reqBody["body"] = body
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return 0, "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return 0, "", err
	}
	s.setAuthHeader(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("GitHub API 错误: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Number int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", err
	}

	return result.Number, result.HTMLURL, nil
}

// ParseRepoPath 解析仓库路径，返回 owner 和 repo
func ParseRepoPath(targetRepo string) (owner, repo string, ok bool) {
	parts := strings.Split(targetRepo, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
