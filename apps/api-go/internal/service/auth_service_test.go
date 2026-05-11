package service

import (
	"testing"

	"feishu-pipeline/apps/api-go/internal/model"
)

func TestMergeFeishuLoginUserPreservesGitHubBinding(t *testing.T) {
	existing := model.User{
		ID:                "gh_123",
		Name:              "octo",
		Role:              model.RoleOther,
		GitHubID:          "123",
		GitHubLogin:       "octo",
		GitHubAvatar:      "https://avatars.githubusercontent.com/u/123",
		GitHubAccessToken: "github-token",
	}
	feishuUser := model.User{
		ID:           "fs_ou_abc",
		FeishuOpenID: "ou_abc",
		Name:         "飞书用户",
		Role:         model.RoleProduct,
		Departments:  []string{"产品部"},
	}

	merged := mergeFeishuLoginUser(existing, feishuUser)

	if merged.ID != existing.ID {
		t.Fatalf("expected existing user id to be preserved, got %s", merged.ID)
	}
	if merged.GitHubID != existing.GitHubID || merged.GitHubAccessToken != existing.GitHubAccessToken {
		t.Fatalf("expected github binding to be preserved, got id=%q token=%q", merged.GitHubID, merged.GitHubAccessToken)
	}
	if merged.FeishuOpenID != feishuUser.FeishuOpenID {
		t.Fatalf("expected feishu open id to be set, got %q", merged.FeishuOpenID)
	}
}

func TestMergeGitHubLoginUserPreservesFeishuBinding(t *testing.T) {
	existing := model.User{
		ID:           "fs_ou_abc",
		FeishuOpenID: "ou_abc",
		Name:         "飞书用户",
		Role:         model.RoleProduct,
		Departments:  []string{"产品部"},
	}
	githubUser := model.User{
		ID:                "gh_123",
		Name:              "octo",
		Role:              model.RoleOther,
		GitHubID:          "123",
		GitHubLogin:       "octo",
		GitHubAvatar:      "https://avatars.githubusercontent.com/u/123",
		GitHubAccessToken: "github-token",
	}

	merged := mergeGitHubLoginUser(existing, githubUser)

	if merged.ID != existing.ID {
		t.Fatalf("expected existing user id to be preserved, got %s", merged.ID)
	}
	if merged.FeishuOpenID != existing.FeishuOpenID {
		t.Fatalf("expected feishu binding to be preserved, got %q", merged.FeishuOpenID)
	}
	if merged.GitHubID != githubUser.GitHubID || merged.GitHubAccessToken != githubUser.GitHubAccessToken {
		t.Fatalf("expected github binding to be refreshed, got id=%q token=%q", merged.GitHubID, merged.GitHubAccessToken)
	}
}
