package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Feishu   FeishuConfig   `mapstructure:"feishu"`
	AI       AIConfig       `mapstructure:"ai"`
	GitHub   GitHubConfig   `mapstructure:"github"`
}

type GitHubConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	ClientID      string `mapstructure:"client_id"`
	ClientSecret  string `mapstructure:"client_secret"`
	RedirectURI   string `mapstructure:"redirect_uri"`
}

type AppConfig struct {
	Name              string `mapstructure:"name"`
	Version           string `mapstructure:"version"`
	Port              int    `mapstructure:"port"`
	Mode              string `mapstructure:"mode"`
	BaseURL           string `mapstructure:"base_url"`
	FrontendURL       string `mapstructure:"frontend_url"`
	SessionCookieName string `mapstructure:"session_cookie_name"`
	CookieSecure      bool   `mapstructure:"cookie_secure"`
	CookieSameSite    string `mapstructure:"cookie_same_site"`
	SessionTTLHours   int    `mapstructure:"session_ttl_hours"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type FeishuConfig struct {
	Enabled              bool   `mapstructure:"enabled"`
	AppID                string `mapstructure:"app_id"`
	AppSecret            string `mapstructure:"app_secret"`
	RedirectURL          string `mapstructure:"redirect_url"`
	OpenBaseURL          string `mapstructure:"open_base_url"`
	OAuthScope           string `mapstructure:"oauth_scope"`
	BotName              string `mapstructure:"bot_name"`
	ReceiveIDType        string `mapstructure:"receive_id_type"`
	DocFolderToken       string `mapstructure:"doc_folder_token"`
	BitableName          string `mapstructure:"bitable_name"`
	BitableFolderToken   string `mapstructure:"bitable_folder_token"`
	BitableAppToken      string `mapstructure:"bitable_app_token"`
	BitableTableID       string `mapstructure:"bitable_table_id"`
	BitableTemplateToken string `mapstructure:"bitable_template_token"`
}

type AIConfig struct {
	Provider         string                   `mapstructure:"provider"`
	Ark              ArkAIConfig              `mapstructure:"ark"`
	OpenAICompatible OpenAICompatibleAIConfig `mapstructure:"openai_compatible"`
}

type ArkAIConfig struct {
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	APIKey      string  `mapstructure:"api_key"`
	Temperature float32 `mapstructure:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	TimeoutSec  int     `mapstructure:"timeout_sec"`
}

type OpenAICompatibleAIConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	Model      string `mapstructure:"model"`
	APIKey     string `mapstructure:"api_key"`
	MaxTokens  int    `mapstructure:"max_tokens"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath("./config")

	if configPath != "" {
		v.SetConfigFile(configPath)
	}

	v.SetEnvPrefix("FEISHU_PIPELINE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.App.Name == "" {
		cfg.App.Name = "requirement-delivery-api"
	}
	if cfg.App.FrontendURL == "" {
		cfg.App.FrontendURL = "http://localhost:5173"
	}
	if cfg.App.SessionCookieName == "" {
		cfg.App.SessionCookieName = "feishu_pipeline_session"
	}
	if cfg.App.CookieSameSite == "" {
		cfg.App.CookieSameSite = "lax"
	}
	if cfg.App.SessionTTLHours <= 0 {
		cfg.App.SessionTTLHours = 24 * 7
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/requirement-delivery.db"
	}
	if cfg.Feishu.OpenBaseURL == "" {
		cfg.Feishu.OpenBaseURL = "https://open.feishu.cn"
	}
	if cfg.Feishu.BotName == "" {
		cfg.Feishu.BotName = "需求交付机器人"
	}
	if cfg.Feishu.ReceiveIDType == "" {
		cfg.Feishu.ReceiveIDType = "user_id"
	}
	if cfg.Feishu.BitableName == "" {
		cfg.Feishu.BitableName = "需求排期"
	}
	if cfg.AI.Provider == "" {
		cfg.AI.Provider = "ark"
	}
	if cfg.AI.Ark.BaseURL == "" {
		cfg.AI.Ark.BaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}
	if cfg.AI.Ark.Model == "" {
		cfg.AI.Ark.Model = "doubao-seed-2-0-lite-260215"
	}
	if cfg.AI.Ark.MaxTokens <= 0 {
		cfg.AI.Ark.MaxTokens = 4096
	}
	if cfg.AI.Ark.TimeoutSec <= 0 {
		cfg.AI.Ark.TimeoutSec = 120
	}
	if cfg.AI.Ark.APIKey == "" {
		cfg.AI.Ark.APIKey = strings.TrimSpace(os.Getenv("FEISHU_PIPELINE_AI_ARK_API_KEY"))
	}
	if cfg.AI.OpenAICompatible.BaseURL == "" {
		cfg.AI.OpenAICompatible.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.AI.OpenAICompatible.Model == "" {
		cfg.AI.OpenAICompatible.Model = "openai-compatible-model"
	}
	if cfg.AI.OpenAICompatible.MaxTokens <= 0 {
		cfg.AI.OpenAICompatible.MaxTokens = 4096
	}
	if cfg.AI.OpenAICompatible.TimeoutSec <= 0 {
		cfg.AI.OpenAICompatible.TimeoutSec = 120
	}
	if cfg.AI.OpenAICompatible.APIKey == "" {
		cfg.AI.OpenAICompatible.APIKey = strings.TrimSpace(os.Getenv("FEISHU_PIPELINE_AI_OPENAI_COMPATIBLE_API_KEY"))
	}

	cfg.Database.Path = resolvePath(cfg.Database.Path)
	return &cfg, nil
}

func resolvePath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}

	workspace, err := os.Getwd()
	if err != nil {
		return path
	}
	return filepath.Join(workspace, path)
}
