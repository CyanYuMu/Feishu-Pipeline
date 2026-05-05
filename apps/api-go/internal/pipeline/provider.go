package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"feishu-pipeline/apps/api-go/internal/external/ai"
	"feishu-pipeline/apps/api-go/internal/model"
)

const (
	AgentProviderDeterministic = "deterministic"
	AgentModelDeterministic    = "fallback"
)

type AgentProvider interface {
	Name() string
	Model() string
	Generate(context.Context, AgentProviderRequest) (AgentProviderResponse, error)
}

type AgentProviderRequest struct {
	AgentKey     string
	StageKey     string
	SystemPrompt string
	UserPrompt   string
	InputJSON    string
	Temperature  float32
	MaxTokens    int
	Metadata     map[string]string
}

type AgentProviderResponse struct {
	Content      string
	RawJSON      string
	TokenUsage   TokenUsage
	LatencyMS    int64
	FinishReason string
	Metadata     map[string]any
}

type TokenUsage struct {
	InputTokens  int            `json:"inputTokens,omitempty"`
	OutputTokens int            `json:"outputTokens,omitempty"`
	TotalTokens  int            `json:"totalTokens,omitempty"`
	Raw          map[string]any `json:"raw,omitempty"`
}

type AgentObservation struct {
	AgentKey       string
	Provider       string
	Model          string
	PromptSnapshot string
	InputJSON      string
	OutputJSON     string
	TokenUsageJSON string
	LatencyMS      int64
	Status         model.AgentRunStatus
	ErrorMessage   string
	// 多Agent扩展字段
	ParentAgentRunID string              `json:"parentAgentRunID,omitempty"` // 父AgentRun ID（子Agent）
	ChildAgentRuns   []*AgentObservation `json:"childAgentRuns,omitempty"`   // 子Agent执行结果（父Agent）
	MergeStrategy    MergeStrategy       `json:"mergeStrategy,omitempty"`    // 使用的合并策略
	MergeMetadata    map[string]any      `json:"mergeMetadata,omitempty"`    // 合并过程元数据
}

type TextGenerator interface {
	Generate(ctx context.Context, systemPrompt string, userPrompt string) (content string, usage ai.TokenUsage, err error)
}

type TextGenerationProvider struct {
	name      string
	model     string
	generator TextGenerator
}

func NewTextGenerationProvider(name string, model string, generator TextGenerator) *TextGenerationProvider {
	if generator == nil {
		return nil
	}
	if name == "" {
		name = "llm"
	}
	return &TextGenerationProvider{name: name, model: model, generator: generator}
}

func (p *TextGenerationProvider) Name() string {
	return p.name
}

func (p *TextGenerationProvider) Model() string {
	return p.model
}

func (p *TextGenerationProvider) Generate(ctx context.Context, req AgentProviderRequest) (AgentProviderResponse, error) {
	if p == nil || p.generator == nil {
		return AgentProviderResponse{}, fmt.Errorf("agent provider is not configured")
	}
	startedAt := time.Now()
	content, usage, err := p.generator.Generate(ctx, req.SystemPrompt, req.UserPrompt)
	latency := time.Since(startedAt).Milliseconds()
	if err != nil {
		return AgentProviderResponse{LatencyMS: latency}, err
	}
	raw, _ := json.Marshal(map[string]any{"content": content})

	// 转换ai.TokenUsage到pipeline.TokenUsage
	tokenUsage := TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		Raw:          usage.Raw,
	}

	return AgentProviderResponse{
		Content:    content,
		RawJSON:    string(raw),
		LatencyMS:  latency,
		TokenUsage: tokenUsage,
	}, nil
}
