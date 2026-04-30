package authtype

import "feishu-pipeline/apps/api-go/internal/model"

type UserResponse struct {
	ID           string     `json:"id"`
	FeishuOpenID string     `json:"feishuOpenID,omitempty"`
	Name         string     `json:"name"`
	Email        string     `json:"email,omitempty"`
	AvatarUrl    string     `json:"avatarUrl,omitempty"`
	Role         model.Role `json:"role"`
	Departments  []string   `json:"departments"`
	// GitHub 绑定信息
	GitHubID     string `json:"githubId,omitempty"`
	GitHubLogin  string `json:"githubLogin,omitempty"`
	GitHubAvatar string `json:"githubAvatar,omitempty"`
}

type FeishuSSOLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

type FeishuSSOConfigResponse struct {
	Enabled bool   `json:"enabled"`
	AppID   string `json:"appId,omitempty"`
	Scope   string `json:"scope,omitempty"`
}

type LoginResponse struct {
	User UserResponse `json:"user"`
}

func NewUserResponse(user model.User) UserResponse {
	return UserResponse{
		ID:           user.ID,
		FeishuOpenID: user.FeishuOpenID,
		Name:         user.Name,
		Email:        user.Email,
		AvatarUrl:    user.AvatarURL,
		Role:         user.Role,
		Departments:  user.Departments,
		GitHubID:     user.GitHubID,
		GitHubLogin:  user.GitHubLogin,
		GitHubAvatar: user.GitHubAvatar,
	}
}

func NewLoginResponse(user model.User) LoginResponse {
	return LoginResponse{
		User: NewUserResponse(user),
	}
}
