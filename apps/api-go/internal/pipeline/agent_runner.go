package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"feishu-pipeline/apps/api-go/internal/model"
)

type AgentRunner struct {
	provider AgentProvider
	registry *PromptRegistry
}

func NewAgentRunner(provider AgentProvider, registry *PromptRegistry) *AgentRunner {
	if registry == nil {
		registry = DefaultPromptRegistry()
	}
	return &AgentRunner{provider: provider, registry: registry}
}

// ExecuteSingle 执行单个Agent（原Execute方法重命名）
func (r *AgentRunner) ExecuteSingle(ctx context.Context, spec AgentPromptSpec, stageContext StageContext, fallback StageHandler) (StageExecutionResult, error) {
	inputJSON := marshalJSONString(stageContext.Input)
	if r.provider == nil {
		return r.executeFallback(ctx, stageContext, fallback, spec, inputJSON, "provider_unavailable", nil)
	}

	startedAt := time.Now()
	response, err := r.provider.Generate(ctx, AgentProviderRequest{
		AgentKey:     spec.AgentKey,
		StageKey:     spec.StageKey,
		SystemPrompt: spec.SystemPrompt,
		UserPrompt:   spec.UserPrompt,
		InputJSON:    inputJSON,
		Metadata:     map[string]string{"runId": stageContext.Run.ID, "stageRunId": stageContext.Stage.ID},
	})
	latencyMS := response.LatencyMS
	if latencyMS <= 0 {
		latencyMS = time.Since(startedAt).Milliseconds()
	}
	if err != nil {
		return r.executeFallback(ctx, stageContext, fallback, spec, inputJSON, "provider_error: "+err.Error(), &response)
	}

	payload, parseErr := decodeAgentJSON(response.Content)
	if parseErr != nil {
		return r.executeFallback(ctx, stageContext, fallback, spec, inputJSON, "json_parse_error: "+parseErr.Error(), &response)
	}
	if validationErr := validateAgentPayload(payload, spec.RequiredFields, spec.FieldTypes); validationErr != nil {
		return r.executeFallback(ctx, stageContext, fallback, spec, inputJSON, "schema_validation_error: "+validationErr.Error(), &response)
	}

	enriched, handlerErr := r.enrichProviderPayload(ctx, stageContext, fallback, spec, payload)
	if handlerErr != nil {
		return r.executeFallback(ctx, stageContext, fallback, spec, inputJSON, "handler_post_process_error: "+handlerErr.Error(), &response)
	}
	outputJSON := marshalJSONString(enriched)
	result := StageExecutionResult{
		ArtifactType: spec.ArtifactType,
		Title:        spec.ArtifactTitle,
		ContentText:  contentTextFromPayload(spec.ArtifactTitle, enriched),
		ContentJSON:  outputJSON,
		OutputJSON:   outputJSON,
	}
	result.AgentRun = &AgentObservation{
		AgentKey:       spec.AgentKey,
		Provider:       r.provider.Name(),
		Model:          r.provider.Model(),
		PromptSnapshot: promptSnapshot(spec),
		InputJSON:      inputJSON,
		OutputJSON:     result.OutputJSON,
		TokenUsageJSON: tokenUsageJSON(response, ""),
		LatencyMS:      latencyMS,
		Status:         model.AgentRunSucceeded,
	}
	return result, nil
}

// Execute 执行Agent，自动检测是否需要多Agent模式
func (r *AgentRunner) Execute(ctx context.Context, stageContext StageContext, fallback StageHandler) (StageExecutionResult, error) {
	spec, ok := r.registry.Build(stageContext)
	if !ok {
		return fallback.Execute(ctx, stageContext)
	}

	// 如果是多Agent配置，执行多Agent流程
	if spec.MultiAgent != nil && spec.MultiAgent.Enabled && len(spec.MultiAgent.Agents) > 0 {
		return r.ExecuteMulti(ctx, stageContext, fallback, spec.MultiAgent)
	}

	// 否则执行单Agent流程
	return r.ExecuteSingle(ctx, spec, stageContext, fallback)
}

// ExecuteMulti 并行执行多个Agent并合并结果
func (r *AgentRunner) ExecuteMulti(ctx context.Context, stageContext StageContext, fallback StageHandler, config *MultiAgentConfig) (StageExecutionResult, error) {
	// 构建所有Agent的prompt spec
	specs, ok := r.registry.BuildMulti(stageContext)
	if !ok {
		return fallback.Execute(ctx, stageContext)
	}

	// 控制并发数
	maxConcurrency := config.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 2 // 默认并发数
	}
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// 收集所有Agent的执行结果
	results := make([]StageExecutionResult, len(specs))
	errors := make([]error, len(specs))

	// 并行执行所有Agent
	for i, spec := range specs {
		wg.Add(1)
		go func(idx int, agentSpec AgentPromptSpec) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行单个Agent
			result, err := r.ExecuteSingle(ctx, agentSpec, stageContext, fallback)
			results[idx] = result
			errors[idx] = err
		}(i, spec)
	}

	// 等待所有Agent执行完成
	wg.Wait()

	// 收集成功的结果
	var successfulResults []StageExecutionResult
	var successfulObservations []*AgentObservation
	var failedErrors []error

	for i, result := range results {
		if errors[i] == nil && result.AgentRun != nil && result.AgentRun.Status == model.AgentRunSucceeded {
			successfulResults = append(successfulResults, result)
			successfulObservations = append(successfulObservations, result.AgentRun)
		} else if errors[i] != nil {
			failedErrors = append(failedErrors, errors[i])
		}
	}

	// 检查是否满足最少成功数量要求
	minSuccess := config.RequireMinSuccess
	if minSuccess <= 0 {
		minSuccess = 1
	}
	if len(successfulResults) < minSuccess {
		// 成功数量不足，返回fallback
		inputJSON := marshalJSONString(stageContext.Input)
		fallbackSpec, _ := r.registry.Build(stageContext)
		return r.executeFallback(ctx, stageContext, fallback, fallbackSpec, inputJSON,
			fmt.Sprintf("multi_agent_failed: only %d/%d agents succeeded, required min %d",
				len(successfulResults), len(specs), minSuccess), nil)
	}

	// 合并结果
	merger := NewResultMerger(config.MergeStrategy, r)
	mergedResult, mergeErr := merger.Merge(ctx, stageContext, successfulResults, fallback)
	if mergeErr != nil {
		inputJSON := marshalJSONString(stageContext.Input)
		fallbackSpec, _ := r.registry.Build(stageContext)
		return r.executeFallback(ctx, stageContext, fallback, fallbackSpec, inputJSON,
			"merge_failed: "+mergeErr.Error(), nil)
	}

	// 构建多Agent的Observation，包含所有子Agent的结果
	mergedObservation := &AgentObservation{
		AgentKey:       fmt.Sprintf("%s_multi", stageContext.Stage.StageKey),
		Provider:       "multi_agent",
		Model:          "composite",
		PromptSnapshot: "", // 多Agent模式下prompt由各个子Agent自行记录
		InputJSON:      marshalJSONString(stageContext.Input),
		OutputJSON:     mergedResult.OutputJSON,
		TokenUsageJSON: aggregateTokenUsage(successfulObservations),
		LatencyMS:      aggregateLatency(successfulObservations),
		Status:         model.AgentRunSucceeded,
		ChildAgentRuns: successfulObservations,
		MergeStrategy:  config.MergeStrategy,
		MergeMetadata: map[string]any{
			"totalAgents":   len(specs),
			"successAgents": len(successfulResults),
			"failedAgents":  len(failedErrors),
			"minSuccess":    minSuccess,
		},
	}

	mergedResult.AgentRun = mergedObservation
	return mergedResult, nil
}

// aggregateTokenUsage 汇总多个Agent的Token使用情况
func aggregateTokenUsage(observations []*AgentObservation) string {
	var totalInput, totalOutput, totalTotal int
	var rawList []map[string]any

	for _, obs := range observations {
		var usage map[string]any
		if err := json.Unmarshal([]byte(obs.TokenUsageJSON), &usage); err == nil {
			if input, ok := usage["inputTokens"].(float64); ok {
				totalInput += int(input)
			}
			if output, ok := usage["outputTokens"].(float64); ok {
				totalOutput += int(output)
			}
			if total, ok := usage["totalTokens"].(float64); ok {
				totalTotal += int(total)
			}
			rawList = append(rawList, usage)
		}
	}

	aggregated := map[string]any{
		"inputTokens":  totalInput,
		"outputTokens": totalOutput,
		"totalTokens":  totalTotal,
		"agentUsages":  rawList,
	}
	data, _ := json.Marshal(aggregated)
	return string(data)
}

// aggregateLatency 汇总多个Agent的延迟（取最大值）
func aggregateLatency(observations []*AgentObservation) int64 {
	var maxLatency int64 = 0
	for _, obs := range observations {
		if obs.LatencyMS > maxLatency {
			maxLatency = obs.LatencyMS
		}
	}
	return maxLatency
}

func (r *AgentRunner) enrichProviderPayload(ctx context.Context, stageContext StageContext, fallback StageHandler, spec AgentPromptSpec, providerPayload map[string]any) (map[string]any, error) {
	enriched := baseStagePayload(stageContext)
	if spec.StageKey == StageTestGeneration || spec.StageKey == StageTestExecution {
		fallbackResult, err := fallback.Execute(ctx, stageContext)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(fallbackResult.OutputJSON), &enriched)
	}
	for key, value := range providerPayload {
		if (spec.StageKey == StageTestGeneration || spec.StageKey == StageTestExecution) && isBackendControlledTestField(key) {
			continue
		}
		enriched[key] = value
	}
	return enriched, nil
}

func (r *AgentRunner) executeFallback(ctx context.Context, stageContext StageContext, fallback StageHandler, spec AgentPromptSpec, inputJSON string, reason string, providerResponse *AgentProviderResponse) (StageExecutionResult, error) {
	startedAt := time.Now()
	result, err := fallback.Execute(ctx, stageContext)
	status := model.AgentRunSucceeded
	errorMessage := ""
	if err != nil {
		status = model.AgentRunFailed
		errorMessage = err.Error()
	}
	usage := AgentProviderResponse{}
	if providerResponse != nil {
		usage = *providerResponse
	}
	result.AgentRun = &AgentObservation{
		AgentKey:       spec.AgentKey,
		Provider:       AgentProviderDeterministic,
		Model:          AgentModelDeterministic,
		PromptSnapshot: promptSnapshot(spec),
		InputJSON:      inputJSON,
		OutputJSON:     result.OutputJSON,
		TokenUsageJSON: tokenUsageJSON(usage, reason),
		LatencyMS:      time.Since(startedAt).Milliseconds(),
		Status:         status,
		ErrorMessage:   errorMessage,
	}
	return result, err
}

func decodeAgentJSON(content string) (map[string]any, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("empty provider content")
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func validateAgentPayload(payload map[string]any, requiredFields []string, fieldTypes map[string]AgentFieldType) error {
	for _, field := range requiredFields {
		value, ok := payload[field]
		if !ok {
			return fmt.Errorf("missing required field %q", field)
		}
		if value == nil {
			return fmt.Errorf("required field %q is null", field)
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf("required field %q is empty", field)
		}
		if items, ok := value.([]any); ok && len(items) == 0 {
			return fmt.Errorf("required field %q is empty", field)
		}
	}
	for field, fieldType := range fieldTypes {
		value, ok := payload[field]
		if !ok || value == nil {
			continue
		}
		switch fieldType {
		case AgentFieldString:
			if _, ok := value.(string); !ok {
				return fmt.Errorf("field %q must be string", field)
			}
		case AgentFieldArray:
			if _, ok := value.([]any); !ok {
				return fmt.Errorf("field %q must be array", field)
			}
		}
	}
	return nil
}

func isBackendControlledTestField(key string) bool {
	switch key {
	case SchemaFieldCommands, SchemaFieldCommandResults, SchemaFieldStatus:
		return true
	default:
		return false
	}
}

func contentTextFromPayload(title string, payload map[string]any) string {
	summary, _ := payload[SchemaFieldSummary].(string)
	if strings.TrimSpace(summary) == "" {
		return title
	}
	return fmt.Sprintf("%s：%s", title, summary)
}

func promptSnapshot(spec AgentPromptSpec) string {
	return marshalJSONString(map[string]any{
		"agentKey":     spec.AgentKey,
		"stageKey":     spec.StageKey,
		"systemPrompt": spec.SystemPrompt,
		"userPrompt":   spec.UserPrompt,
	})
}

func tokenUsageJSON(response AgentProviderResponse, fallbackReason string) string {
	payload := map[string]any{
		"inputTokens":  response.TokenUsage.InputTokens,
		"outputTokens": response.TokenUsage.OutputTokens,
		"totalTokens":  response.TokenUsage.TotalTokens,
		"finishReason": response.FinishReason,
		"raw":          response.TokenUsage.Raw,
		"metadata":     response.Metadata,
	}
	if strings.TrimSpace(response.RawJSON) != "" {
		payload["rawResponse"] = response.RawJSON
	}
	if strings.TrimSpace(fallbackReason) != "" {
		payload["fallbackReason"] = fallbackReason
	}
	return marshalJSONString(payload)
}

func marshalJSONString(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
