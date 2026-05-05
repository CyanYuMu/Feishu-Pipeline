package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
)

// ResultMerger 结果合并器接口
type ResultMerger interface {
	Merge(ctx context.Context, stageContext StageContext, results []StageExecutionResult, fallback StageHandler) (StageExecutionResult, error)
}

// NewResultMerger 创建对应策略的合并器
func NewResultMerger(strategy MergeStrategy, runner *AgentRunner) ResultMerger {
	switch strategy {
	case MergeStrategyVoting:
		return &VotingMerger{}
	case MergeStrategyBestQuality:
		return &BestQualityMerger{}
	case MergeStrategySummarize:
		return &SummarizingMerger{runner: runner}
	case MergeStrategyFirstSuccess:
		return &FirstSuccessMerger{}
	default:
		// 默认使用择优策略
		return &BestQualityMerger{}
	}
}

// VotingMerger 投票策略合并器
// 对于数值类型取多数值，对于字符串类型取最长内容，对于数组类型取并集
type VotingMerger struct{}

func (m *VotingMerger) Merge(ctx context.Context, stageContext StageContext, results []StageExecutionResult, fallback StageHandler) (StageExecutionResult, error) {
	if len(results) == 0 {
		return StageExecutionResult{}, fmt.Errorf("no results to merge")
	}
	if len(results) == 1 {
		return results[0], nil
	}

	// 收集所有结果的payload
	var payloads []map[string]any
	for _, result := range results {
		var payload map[string]any
		if err := json.Unmarshal([]byte(result.OutputJSON), &payload); err == nil {
			payloads = append(payloads, payload)
		}
	}

	if len(payloads) == 0 {
		return results[0], nil
	}

	// 合并payload
	mergedPayload := make(map[string]any)
	fields := getCommonFields(payloads)

	for _, field := range fields {
		values := getFieldValues(payloads, field)
		if len(values) == 0 {
			continue
		}

		// 根据字段类型选择合并策略
		switch values[0].(type) {
		case string:
			// 字符串类型取最长内容或最频繁出现的
			mergedPayload[field] = mergeStringField(values)
		case float64, int, int64:
			// 数值类型取出现次数最多的
			mergedPayload[field] = mergeNumberField(values)
		case []any:
			// 数组类型取并集
			mergedPayload[field] = mergeArrayField(values)
		case bool:
			// 布尔类型取多数值
			mergedPayload[field] = mergeBoolField(values)
		default:
			// 其他类型取第一个非空值
			mergedPayload[field] = values[0]
		}
	}

	// 生成合并后的结果
	baseResult := results[0]
	mergedJSON, _ := json.Marshal(mergedPayload)
	mergedResult := StageExecutionResult{
		ArtifactType: baseResult.ArtifactType,
		Title:        baseResult.Title,
		ContentText:  mergeContentText(results),
		ContentJSON:  string(mergedJSON),
		OutputJSON:   string(mergedJSON),
	}

	return mergedResult, nil
}

// BestQualityMerger 择优策略合并器
// 选择质量最高的结果（基于必填字段完整性、内容长度等）
type BestQualityMerger struct{}

func (m *BestQualityMerger) Merge(ctx context.Context, stageContext StageContext, results []StageExecutionResult, fallback StageHandler) (StageExecutionResult, error) {
	if len(results) == 0 {
		return StageExecutionResult{}, fmt.Errorf("no results to merge")
	}
	if len(results) == 1 {
		return results[0], nil
	}

	// 计算每个结果的质量分数
	var bestResult StageExecutionResult
	maxScore := -1

	for _, result := range results {
		score := calculateQualityScore(result)
		if score > maxScore {
			maxScore = score
			bestResult = result
		}
	}

	return bestResult, nil
}

// SummarizingMerger 汇总策略合并器
// 使用专门的汇总Agent将多个结果合并为一个
type SummarizingMerger struct {
	runner *AgentRunner
}

func (m *SummarizingMerger) Merge(ctx context.Context, stageContext StageContext, results []StageExecutionResult, fallback StageHandler) (StageExecutionResult, error) {
	if len(results) == 0 {
		return StageExecutionResult{}, fmt.Errorf("no results to merge")
	}
	if len(results) == 1 {
		return results[0], nil
	}

	// 获取基础spec
	spec, ok := m.runner.registry.Build(stageContext)
	if !ok {
		return fallback.Execute(ctx, stageContext)
	}

	// 构建汇总prompt
	resultsJSON, _ := json.Marshal(results)
	systemPrompt := fmt.Sprintf(`你是多Agent结果汇总专家，负责将多个Agent的执行结果合并为一个高质量的最终结果。
输入是多个Agent对同一任务的输出，你需要：
1. 综合所有结果的优点，去除重复内容
2. 保持输出格式与原输出结构一致：%s
3. 不要遗漏任何重要信息
4. 只输出JSON，不要其他解释`, spec.OutputContract)

	userPrompt := fmt.Sprintf(`多个Agent的执行结果：
%s

请将这些结果合并为一个完整、准确的JSON输出。`, string(resultsJSON))

	// 调用汇总Agent
	mergedSpec := AgentPromptSpec{
		StageKey:       spec.StageKey,
		AgentKey:       fmt.Sprintf("%s_summarizer", spec.AgentKey),
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		RequiredFields: spec.RequiredFields,
		FieldTypes:     spec.FieldTypes,
		ArtifactType:   spec.ArtifactType,
		ArtifactTitle:  spec.ArtifactTitle,
	}

	// 执行汇总
	return m.runner.ExecuteSingle(ctx, mergedSpec, stageContext, fallback)
}

// FirstSuccessMerger 优先策略合并器
// 选择第一个成功的结果
type FirstSuccessMerger struct{}

func (m *FirstSuccessMerger) Merge(ctx context.Context, stageContext StageContext, results []StageExecutionResult, fallback StageHandler) (StageExecutionResult, error) {
	if len(results) == 0 {
		return StageExecutionResult{}, fmt.Errorf("no results to merge")
	}
	// 返回第一个成功的结果
	for _, result := range results {
		if result.AgentRun != nil && result.AgentRun.Status == model.AgentRunSucceeded {
			return result, nil
		}
	}
	// 如果没有成功的结果，返回第一个
	return results[0], nil
}

// 辅助函数

// getCommonFields 获取所有payload的公共字段
func getCommonFields(payloads []map[string]any) []string {
	if len(payloads) == 0 {
		return nil
	}

	fieldCounts := make(map[string]int)
	for _, payload := range payloads {
		for field := range payload {
			fieldCounts[field]++
		}
	}

	var commonFields []string
	for field, count := range fieldCounts {
		if count == len(payloads) {
			commonFields = append(commonFields, field)
		}
	}

	return commonFields
}

// getFieldValues 获取所有payload中指定字段的值
func getFieldValues(payloads []map[string]any, field string) []any {
	var values []any
	for _, payload := range payloads {
		if value, ok := payload[field]; ok {
			values = append(values, value)
		}
	}
	return values
}

// mergeStringField 合并字符串字段：取最频繁出现的，长度相近则取最长的
func mergeStringField(values []any) string {
	if len(values) == 0 {
		return ""
	}

	// 统计出现频率
	freq := make(map[string]int)
	maxFreq := 0
	var mostFreq string
	maxLen := 0
	var longest string

	for _, v := range values {
		if s, ok := v.(string); ok {
			freq[s]++
			if freq[s] > maxFreq {
				maxFreq = freq[s]
				mostFreq = s
			}
			if len(s) > maxLen {
				maxLen = len(s)
				longest = s
			}
		}
	}

	// 如果最高频率大于1，返回最频繁的
	if maxFreq > 1 {
		return mostFreq
	}

	// 否则返回最长的
	return longest
}

// mergeNumberField 合并数字字段：取出现次数最多的
func mergeNumberField(values []any) any {
	if len(values) == 0 {
		return 0
	}

	freq := make(map[float64]int)
	maxFreq := 0
	var mostFreq float64

	for _, v := range values {
		var num float64
		switch val := v.(type) {
		case float64:
			num = val
		case int:
			num = float64(val)
		case int64:
			num = float64(val)
		default:
			continue
		}
		freq[num]++
		if freq[num] > maxFreq {
			maxFreq = freq[num]
			mostFreq = num
		}
	}

	return mostFreq
}

// mergeArrayField 合并数组字段：取并集，去重
func mergeArrayField(values []any) []any {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var result []any

	for _, v := range values {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				// 转换为字符串作为键去重
				key := fmt.Sprintf("%v", item)
				if !seen[key] {
					seen[key] = true
					result = append(result, item)
				}
			}
		}
	}

	return result
}

// mergeBoolField 合并布尔字段：取多数值
func mergeBoolField(values []any) bool {
	if len(values) == 0 {
		return false
	}

	trueCount := 0
	falseCount := 0

	for _, v := range values {
		if b, ok := v.(bool); ok {
			if b {
				trueCount++
			} else {
				falseCount++
			}
		}
	}

	return trueCount >= falseCount
}

// calculateQualityScore 计算结果的质量分数
func calculateQualityScore(result StageExecutionResult) int {
	score := 0

	// 成功的结果基础分
	if result.AgentRun != nil && result.AgentRun.Status == model.AgentRunSucceeded {
		score += 100
	}

	// 解析payload
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.OutputJSON), &payload); err != nil {
		return score
	}

	// 内容长度加分
	score += len(result.OutputJSON) / 100

	// 必填字段完整性加分
	for _, value := range payload {
		if value != nil {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					score += 10
				}
			case []any:
				if len(v) > 0 {
					score += len(v) * 5
				}
			default:
				score += 5
			}
		}
	}

	// 延迟越低分数越高
	if result.AgentRun != nil {
		score -= int(result.AgentRun.LatencyMS) / 1000
	}

	return score
}

// mergeContentText 合并内容文本
func mergeContentText(results []StageExecutionResult) string {
	var texts []string
	for _, result := range results {
		if result.ContentText != "" {
			texts = append(texts, result.ContentText)
		}
	}
	if len(texts) == 0 {
		return ""
	}
	if len(texts) == 1 {
		return texts[0]
	}

	// 返回最长的内容文本
	maxLen := 0
	longest := ""
	for _, text := range texts {
		if len(text) > maxLen {
			maxLen = len(text)
			longest = text
		}
	}
	return longest
}
