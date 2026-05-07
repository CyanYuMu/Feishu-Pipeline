package pipeline

import "encoding/json"

const (
	DefaultTemplateID  = "feature-delivery"
	BugFixTemplateID   = "bug-fix"
	RefactorTemplateID = "refactor"
)

type TemplateDefinition struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind,omitempty"`
	Description string            `json:"description,omitempty"`
	UseCases    []string          `json:"useCases,omitempty"`
	Defaults    map[string]any    `json:"defaults,omitempty"`
	Stages      []StageDefinition `json:"stages"`
}

func DefaultTemplateDefinitionJSON() string {
	return TemplateDefinitionJSON(DefaultTemplateID)
}

func TemplateDefinitionJSON(templateID string) string {
	definition := TemplateDefinitionFor(templateID)
	bytes, err := json.Marshal(definition)
	if err != nil {
		return `{"id":"feature-delivery","name":"Feature Delivery"}`
	}
	return string(bytes)
}

func TemplateDefinitionFor(templateID string) TemplateDefinition {
	definition := TemplateDefinition{
		ID:          DefaultTemplateID,
		Name:        "Feature Delivery",
		Kind:        "feature",
		Description: "从自然语言需求到可交付代码变更的新功能研发流水线。",
		UseCases:    []string{"新功能开发", "API / 页面能力新增", "端到端交付演示"},
		Defaults: map[string]any{
			"riskLevel":          "medium",
			"checkpointPolicy":   "design_and_review",
			"codegenFocus":       []string{"correctness", "tests", "delivery"},
			"recommendedBranch":  "devflow/feature",
			"requirementMinimum": []string{"目标", "范围", "验收标准"},
		},
		Stages: DefaultStageDefinitions,
	}
	switch templateID {
	case BugFixTemplateID:
		definition.ID = BugFixTemplateID
		definition.Name = "Bug Fix Delivery"
		definition.Kind = "bugfix"
		definition.Description = "面向缺陷复现、根因分析、修复验证和回归交付的流水线。"
		definition.UseCases = []string{"线上缺陷修复", "测试回归问题", "稳定性问题定位"}
		definition.Defaults = map[string]any{
			"riskLevel":          "high",
			"checkpointPolicy":   "design_and_review",
			"codegenFocus":       []string{"root_cause", "minimal_patch", "regression_tests"},
			"recommendedBranch":  "devflow/bugfix",
			"requirementMinimum": []string{"现象", "复现步骤", "期望结果", "实际结果"},
		}
	case RefactorTemplateID:
		definition.ID = RefactorTemplateID
		definition.Name = "Refactor Delivery"
		definition.Kind = "refactor"
		definition.Description = "面向结构优化、技术债治理和低风险行为保持的重构流水线。"
		definition.UseCases = []string{"模块重构", "技术债治理", "代码结构优化"}
		definition.Defaults = map[string]any{
			"riskLevel":          "medium_high",
			"checkpointPolicy":   "design_and_review",
			"codegenFocus":       []string{"behavior_preservation", "incremental_changes", "test_safety"},
			"recommendedBranch":  "devflow/refactor",
			"requirementMinimum": []string{"重构目标", "保持不变的行为", "影响范围", "回滚策略"},
		}
	}
	return definition
}

func ParseTemplateDefinition(raw string) (TemplateDefinition, error) {
	if raw == "" {
		return TemplateDefinition{ID: DefaultTemplateID, Name: "Feature Delivery", Stages: DefaultStageDefinitions}, nil
	}
	var definition TemplateDefinition
	if err := json.Unmarshal([]byte(raw), &definition); err != nil {
		return TemplateDefinition{}, err
	}
	if len(definition.Stages) == 0 {
		definition.Stages = DefaultStageDefinitions
	}
	return definition, nil
}
