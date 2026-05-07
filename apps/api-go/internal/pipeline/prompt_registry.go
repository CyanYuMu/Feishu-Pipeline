package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"feishu-pipeline/apps/api-go/internal/model"
)

const (
	AgentRequirementAnalyst = "requirement_analyst"
	AgentSolutionDesigner   = "solution_designer"
	AgentCodeGenerator      = "code_generator"
	AgentTestGenerator      = "test_generator"
	AgentCodeReviewer       = "code_reviewer"
	AgentDeliveryIntegrator = "delivery_integrator"
)

type AgentPromptSpec struct {
	StageKey       string
	AgentKey       string
	SystemPrompt   string
	UserPrompt     string
	OutputContract string // 输出契约JSON结构
	RequiredFields []string
	FieldTypes     map[string]AgentFieldType
	ArtifactType   model.ArtifactType
	ArtifactTitle  string
	// 多Agent配置
	MultiAgent *MultiAgentConfig `json:"multiAgent,omitempty"`
}

type AgentFieldType string

const (
	AgentFieldString AgentFieldType = "string"
	AgentFieldArray  AgentFieldType = "array"
)

type StagePromptDefinition struct {
	StageKey       string
	AgentKey       string
	Role           string
	OutputContract string
	RequiredFields []string
	FieldTypes     map[string]AgentFieldType
	ArtifactType   model.ArtifactType
	ArtifactTitle  string
	// 多Agent配置
	MultiAgent *MultiAgentConfig `json:"multiAgent,omitempty"`
}

type PromptRegistry struct {
	definitions map[string]StagePromptDefinition
}

func DefaultPromptRegistry() *PromptRegistry {
	definitions := []StagePromptDefinition{
		{
			StageKey:       StageRequirementAnalysis,
			AgentKey:       AgentRequirementAnalyst,
			Role:           "你是需求分析 Agent，负责把自然语言需求转成稳定、可审批、可被后续阶段消费的结构化需求。",
			OutputContract: `{"summary":"","goals":[],"nonGoals":[],"acceptanceCriteria":[],"questions":[],"risks":[]}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldGoals, SchemaFieldAcceptanceCriteria},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:            AgentFieldString,
				SchemaFieldGoals:              AgentFieldArray,
				SchemaFieldNonGoals:           AgentFieldArray,
				SchemaFieldAcceptanceCriteria: AgentFieldArray,
				SchemaFieldQuestions:          AgentFieldArray,
				"risks":                       AgentFieldArray,
			},
			ArtifactType:  model.ArtifactStructuredRequirement,
			ArtifactTitle: "结构化需求",
		},
		{
			StageKey:       StageSolutionDesign,
			AgentKey:       AgentSolutionDesigner,
			Role:           "你是方案设计 Agent，负责结合需求、仓库上下文和历史审批意见，输出技术方案和影响范围。",
			OutputContract: `{"summary":"","impactFiles":[],"apiChanges":[],"dataModelChanges":[],"implementationPlan":[],"risks":[]}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldImpactFiles, SchemaFieldImplementationPlan},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:            AgentFieldString,
				SchemaFieldImpactFiles:        AgentFieldArray,
				SchemaFieldAPIChanges:         AgentFieldArray,
				SchemaFieldDataModelChanges:   AgentFieldArray,
				SchemaFieldImplementationPlan: AgentFieldArray,
				"risks":                       AgentFieldArray,
			},
			ArtifactType:  model.ArtifactSolutionDesign,
			ArtifactTitle: "技术方案",
		},
		{
			StageKey:       StageCodeGeneration,
			AgentKey:       AgentCodeGenerator,
			Role:           "你是代码生成 Agent，负责输出受控代码变更计划。当前阶段只能生成结构化 patch 计划，不允许要求直接写文件、删除文件或推送远程仓库。",
			OutputContract: `{"summary":"","changedFiles":[],"patches":[],"diffSummary":[],"manualSteps":[]}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldChangedFiles, SchemaFieldDiffSummary},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:      AgentFieldString,
				SchemaFieldChangedFiles: AgentFieldArray,
				SchemaFieldPatches:      AgentFieldArray,
				SchemaFieldDiffSummary:  AgentFieldArray,
				SchemaFieldManualSteps:  AgentFieldArray,
			},
			ArtifactType:  model.ArtifactCodeDiff,
			ArtifactTitle: "代码变更计划",
			// 多Agent配置：三个不同侧重点的代码生成Agent
			MultiAgent: &MultiAgentConfig{
				Enabled:           true,
				MergeStrategy:     MergeStrategySummarize, // 使用汇总策略合并结果
				MaxConcurrency:    2,
				FailFast:          false,
				RequireMinSuccess: 2,
				Agents: []AgentInstanceConfig{
					{
						AgentKey: "code_generator_performance",
						Role:     "你是性能优化专家级代码生成 Agent，负责输出高性能、低资源消耗的代码变更计划。优先考虑执行效率、内存使用优化、算法复杂度优化。当前阶段只能生成结构化 patch 计划，不允许要求直接写文件、删除文件或推送远程仓库。",
					},
					{
						AgentKey: "code_generator_readable",
						Role:     "你是可读性和可维护性专家级代码生成 Agent，负责输出结构清晰、易于理解和维护的代码变更计划。优先考虑代码风格一致性、命名规范、注释完整性、模块化设计。当前阶段只能生成结构化 patch 计划，不允许要求直接写文件、删除文件或推送远程仓库。",
					},
					{
						AgentKey: "code_generator_security",
						Role:     "你是安全专家级代码生成 Agent，负责输出安全可靠、无漏洞的代码变更计划。优先考虑输入校验、错误处理、边界条件、安全最佳实践。当前阶段只能生成结构化 patch 计划，不允许要求直接写文件、删除文件或推送远程仓库。",
					},
				},
			},
		},
		{
			StageKey:       StageTestGeneration,
			AgentKey:       AgentTestGenerator,
			Role:           "你是测试生成 Agent，负责根据需求和代码变更计划输出测试计划。命令执行由后端白名单控制，你只输出建议和结构化结果。",
			OutputContract: `{"summary":"","testPlan":[],"commands":[],"commandResults":[],"status":""}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldTestPlan, SchemaFieldStatus},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:        AgentFieldString,
				SchemaFieldTestPlan:       AgentFieldArray,
				SchemaFieldCommands:       AgentFieldArray,
				SchemaFieldCommandResults: AgentFieldArray,
				SchemaFieldStatus:         AgentFieldString,
			},
			ArtifactType:  model.ArtifactTestReport,
			ArtifactTitle: "测试报告",
		},
		{
			StageKey:       StageCodeReview,
			AgentKey:       AgentCodeReviewer,
			Role:           "你是代码评审 Agent，负责从正确性、安全性、可维护性、测试充分性维度审查变更。",
			OutputContract: `{"summary":"","conclusion":"","issues":[],"securityNotes":[],"maintainabilityNotes":[],"testCoverageNotes":[]}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldConclusion, SchemaFieldIssues},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:              AgentFieldString,
				SchemaFieldConclusion:           AgentFieldString,
				SchemaFieldIssues:               AgentFieldArray,
				SchemaFieldSecurityNotes:        AgentFieldArray,
				SchemaFieldMaintainabilityNotes: AgentFieldArray,
				SchemaFieldTestCoverageNotes:    AgentFieldArray,
			},
			ArtifactType:  model.ArtifactReviewReport,
			ArtifactTitle: "评审报告",
			// 多Agent配置：三个不同侧重点的代码评审Agent
			MultiAgent: &MultiAgentConfig{
				Enabled:           true,
				MergeStrategy:     MergeStrategyBestQuality, // 使用择优策略合并结果
				MaxConcurrency:    2,
				FailFast:          false,
				RequireMinSuccess: 2,
				Agents: []AgentInstanceConfig{
					{
						AgentKey: "code_reviewer_security",
						Role:     "你是安全专家级代码评审 Agent，专门负责审查代码中的安全漏洞、注入风险、权限问题、数据泄露隐患等安全相关问题。你需要重点关注：输入校验、身份认证、授权机制、敏感数据处理、错误处理、边界条件、安全最佳实践等方面。输出必须符合指定的JSON结构。",
					},
					{
						AgentKey: "code_reviewer_performance",
						Role:     "你是性能优化专家级代码评审 Agent，专门负责审查代码的性能问题，包括算法复杂度、内存使用、资源泄漏、数据库查询优化、缓存策略、并发性能等方面。你需要识别性能瓶颈并提出优化建议。输出必须符合指定的JSON结构。",
					},
					{
						AgentKey: "code_reviewer_maintainability",
						Role:     "你是可维护性专家级代码评审 Agent，专门负责审查代码的可读性、可维护性、架构设计合理性、代码规范一致性、命名规范、注释完整性、模块化设计、重复代码等方面。你需要提出改进代码质量的建议。输出必须符合指定的JSON结构。",
					},
				},
			},
		},
		{
			StageKey:       StageDelivery,
			AgentKey:       AgentDeliveryIntegrator,
			Role:           "你是交付集成 Agent，负责汇总需求、方案、测试和评审产物，生成安全的 PR/MR 草稿和发布说明。审批确认后，系统可通过 execute-changes 将变更提交到 GitHub 并创建远程 PR。",
			OutputContract: `{"summary":"","changedFiles":[],"validation":[],"prTitle":"","prBody":"","manualReleaseNotes":[]}`,
			RequiredFields: []string{SchemaFieldSummary, SchemaFieldPRTitle, SchemaFieldPRBody},
			FieldTypes: map[string]AgentFieldType{
				SchemaFieldSummary:            AgentFieldString,
				SchemaFieldChangedFiles:       AgentFieldArray,
				SchemaFieldValidation:         AgentFieldArray,
				SchemaFieldPRTitle:            AgentFieldString,
				SchemaFieldPRBody:             AgentFieldString,
				SchemaFieldManualReleaseNotes: AgentFieldArray,
			},
			ArtifactType:  model.ArtifactDeliverySummary,
			ArtifactTitle: "交付摘要",
		},
	}
	registry := &PromptRegistry{definitions: map[string]StagePromptDefinition{}}
	for _, definition := range definitions {
		registry.definitions[definition.StageKey] = definition
	}
	return registry
}

// Build 构建单Agent提示词（保持向后兼容）
func (r *PromptRegistry) Build(stageContext StageContext) (AgentPromptSpec, bool) {
	if r == nil {
		return AgentPromptSpec{}, false
	}
	definition, ok := r.definitions[stageContext.Stage.StageKey]
	if !ok {
		return AgentPromptSpec{}, false
	}
	inputJSON, _ := json.MarshalIndent(stageContext.Input, "", "  ")
	systemPrompt := strings.Join([]string{
		definition.Role,
		"你在 DevFlow Engine 的一个 Pipeline Stage 中工作。",
		"你必须只输出合法 JSON，不要输出 Markdown 代码块、解释、前后缀说明。",
		"输出 JSON 必须满足以下结构，字段名必须保持一致：",
		definition.OutputContract,
		"字符串字段必须是 JSON string，数组字段必须是 JSON array；不要用逗号分隔字符串代替数组。",
		"如果信息不足，在 risks 或 questions 字段中说明，不要编造不存在的文件或外部状态。",
	}, "\n")
	userPrompt := fmt.Sprintf("当前 Pipeline 阶段：%s\nRun 标题：%s\n目标仓库：%s\n目标分支：%s\n工作分支：%s\n\n阶段输入 JSON：\n%s\n\n请按系统要求输出本阶段 JSON。",
		stageContext.Stage.StageKey,
		stageContext.Run.Title,
		stageContext.Run.TargetRepo,
		stageContext.Run.TargetBranch,
		stageContext.Run.WorkBranch,
		string(inputJSON),
	)

	spec := AgentPromptSpec{
		StageKey:       definition.StageKey,
		AgentKey:       definition.AgentKey,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		OutputContract: definition.OutputContract,
		RequiredFields: definition.RequiredFields,
		FieldTypes:     definition.FieldTypes,
		ArtifactType:   definition.ArtifactType,
		ArtifactTitle:  definition.ArtifactTitle,
		MultiAgent:     definition.MultiAgent,
	}

	// 如果配置了多Agent，为每个Agent构建提示词
	if definition.MultiAgent != nil && definition.MultiAgent.Enabled && len(definition.MultiAgent.Agents) > 0 {
		for i, agentConfig := range definition.MultiAgent.Agents {
			// 如果Agent有自定义Role，使用它构建系统提示词
			agentSystemPrompt := systemPrompt
			if agentConfig.Role != "" {
				agentSystemPrompt = strings.Join([]string{
					agentConfig.Role,
					"你在 DevFlow Engine 的一个 Pipeline Stage 中工作。",
					"你必须只输出合法 JSON，不要输出 Markdown 代码块、解释、前后缀说明。",
					"输出 JSON 必须满足以下结构，字段名必须保持一致：",
					definition.OutputContract,
					"字符串字段必须是 JSON string，数组字段必须是 JSON array；不要用逗号分隔字符串代替数组。",
					"如果信息不足，在 risks 或 questions 字段中说明，不要编造不存在的文件或外部状态。",
				}, "\n")
			} else if agentConfig.SystemPrompt != "" {
				agentSystemPrompt = agentConfig.SystemPrompt
			}

			agentUserPrompt := userPrompt
			if agentConfig.UserPrompt != "" {
				agentUserPrompt = agentConfig.UserPrompt
			}

			agentKey := agentConfig.AgentKey
			if agentKey == "" {
				agentKey = fmt.Sprintf("%s_%d", definition.AgentKey, i)
			}

			// 更新配置
			agentConfig.SystemPrompt = agentSystemPrompt
			agentConfig.UserPrompt = agentUserPrompt
			agentConfig.AgentKey = agentKey
			definition.MultiAgent.Agents[i] = agentConfig
		}
	}

	return spec, true
}

// BuildMulti 构建多Agent提示词列表
func (r *PromptRegistry) BuildMulti(stageContext StageContext) ([]AgentPromptSpec, bool) {
	spec, ok := r.Build(stageContext)
	if !ok {
		return nil, false
	}

	// 如果没有配置多Agent，返回单Agent列表
	if spec.MultiAgent == nil || !spec.MultiAgent.Enabled || len(spec.MultiAgent.Agents) == 0 {
		return []AgentPromptSpec{spec}, true
	}

	// 为每个Agent构建独立的spec
	var specs []AgentPromptSpec
	for _, agentConfig := range spec.MultiAgent.Agents {
		agentSpec := AgentPromptSpec{
			StageKey:       spec.StageKey,
			AgentKey:       agentConfig.AgentKey,
			SystemPrompt:   agentConfig.SystemPrompt,
			UserPrompt:     agentConfig.UserPrompt,
			RequiredFields: spec.RequiredFields,
			FieldTypes:     spec.FieldTypes,
			ArtifactType:   spec.ArtifactType,
			ArtifactTitle:  spec.ArtifactTitle,
		}
		specs = append(specs, agentSpec)
	}

	return specs, true
}
