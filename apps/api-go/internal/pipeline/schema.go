package pipeline

const (
	SchemaFieldSummary              = "summary"
	SchemaFieldGoals                = "goals"
	SchemaFieldNonGoals             = "nonGoals"
	SchemaFieldAcceptanceCriteria   = "acceptanceCriteria"
	SchemaFieldQuestions            = "questions"
	SchemaFieldContextSources       = "contextSources"
	SchemaFieldFeishuDocs           = "feishuDocs"
	SchemaFieldBitableFields        = "bitableFields"
	SchemaFieldConversationSignals  = "conversationSignals"
	SchemaFieldImpactFiles          = "impactFiles"
	SchemaFieldAPIChanges           = "apiChanges"
	SchemaFieldDataModelChanges     = "dataModelChanges"
	SchemaFieldImplementationPlan   = "implementationPlan"
	SchemaFieldChangedFiles         = "changedFiles"
	SchemaFieldPatches              = "patches"
	SchemaFieldDiffSummary          = "diffSummary"
	SchemaFieldManualSteps          = "manualSteps"
	SchemaFieldTestPlan             = "testPlan"
	SchemaFieldCommands             = "commands"
	SchemaFieldCommandResults       = "commandResults"
	SchemaFieldStatus               = "status"
	SchemaFieldConclusion           = "conclusion"
	SchemaFieldIssues               = "issues"
	SchemaFieldSecurityNotes        = "securityNotes"
	SchemaFieldMaintainabilityNotes = "maintainabilityNotes"
	SchemaFieldTestCoverageNotes    = "testCoverageNotes"
	SchemaFieldValidation           = "validation"
	SchemaFieldPRTitle              = "prTitle"
	SchemaFieldPRBody               = "prBody"
	SchemaFieldManualReleaseNotes   = "manualReleaseNotes"
)

// MergeStrategy 合并策略类型
type MergeStrategy string

const (
	// MergeStrategyVoting 投票策略：多个Agent结果投票选择最优
	MergeStrategyVoting MergeStrategy = "voting"
	// MergeStrategyBestQuality 择优策略：选择质量最高的结果
	MergeStrategyBestQuality MergeStrategy = "best_quality"
	// MergeStrategySummarize 汇总策略：使用汇总Agent合并多个结果
	MergeStrategySummarize MergeStrategy = "summarize"
	// MergeStrategyFirstSuccess 优先策略：选择第一个成功的结果
	MergeStrategyFirstSuccess MergeStrategy = "first_success"
)

// AgentInstanceConfig 单个Agent实例配置
type AgentInstanceConfig struct {
	AgentKey     string            `json:"agentKey"`     // Agent类型标识
	Role         string            `json:"role"`         // 自定义角色描述（可选，覆盖默认）
	SystemPrompt string            `json:"systemPrompt"` // 自定义系统提示词（可选，覆盖默认）
	UserPrompt   string            `json:"userPrompt"`   // 自定义用户提示词（可选，覆盖默认）
	Model        string            `json:"model"`        // 模型名称（可选，覆盖默认）
	Temperature  float32           `json:"temperature"`  // 温度参数（可选）
	MaxTokens    int               `json:"maxTokens"`    // 最大token数（可选）
	Metadata     map[string]string `json:"metadata"`     // 元数据（可选）
}

// MultiAgentConfig 多Agent协作配置
type MultiAgentConfig struct {
	Enabled           bool                  `json:"enabled"`           // 是否启用多Agent
	Agents            []AgentInstanceConfig `json:"agents"`            // Agent实例列表
	MergeStrategy     MergeStrategy         `json:"mergeStrategy"`     // 合并策略
	MaxConcurrency    int                   `json:"maxConcurrency"`    // 最大并发数（默认2）
	FailFast          bool                  `json:"failFast"`          // 是否快速失败（只要有一个Agent失败就终止）
	RequireMinSuccess int                   `json:"requireMinSuccess"` // 要求最少成功的Agent数量（默认1）
}
