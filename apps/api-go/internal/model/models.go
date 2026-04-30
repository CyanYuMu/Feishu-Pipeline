package model

import "time"

type Role string

const (
	RoleProduct  Role = "product"
	RoleFrontend Role = "frontend"
	RoleBackend  Role = "backend"
	RoleOther    Role = "other"
	RoleAdmin    Role = "admin"
)

type SessionStatus string

const (
	SessionDraft      SessionStatus = "draft"
	SessionPublished  SessionStatus = "published"
	SessionInDelivery SessionStatus = "in_delivery"
	SessionTesting    SessionStatus = "testing"
	SessionDone       SessionStatus = "done"
	SessionArchived   SessionStatus = "archived"
)

type MessageRole string

const (
	MessageUser      MessageRole = "user"
	MessageAssistant MessageRole = "assistant"
	MessageSystem    MessageRole = "system"
)

type TaskType string

const (
	TaskFrontend TaskType = "frontend"
	TaskBackend  TaskType = "backend"
	TaskShared   TaskType = "shared"
)

type TaskPriority string

const (
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityLow    TaskPriority = "low"
)

type TaskStatus string

const (
	TaskTodo       TaskStatus = "todo"
	TaskInProgress TaskStatus = "in_progress"
	TaskTesting    TaskStatus = "testing"
	TaskDone       TaskStatus = "done"
)

type PipelineRunStatus string

const (
	PipelineRunDraft           PipelineRunStatus = "draft"
	PipelineRunQueued          PipelineRunStatus = "queued"
	PipelineRunRunning         PipelineRunStatus = "running"
	PipelineRunWaitingApproval PipelineRunStatus = "waiting_approval"
	PipelineRunPaused          PipelineRunStatus = "paused"
	PipelineRunFailed          PipelineRunStatus = "failed"
	PipelineRunCompleted       PipelineRunStatus = "completed"
	PipelineRunTerminated      PipelineRunStatus = "terminated"
)

type StageType string

const (
	StageTypeAnalysis   StageType = "analysis"
	StageTypeDesign     StageType = "design"
	StageTypeCheckpoint StageType = "checkpoint"
	StageTypeCodegen    StageType = "codegen"
	StageTypeTest       StageType = "test"
	StageTypeReview     StageType = "review"
	StageTypeDelivery   StageType = "delivery"
)

type StageRunStatus string

const (
	StageRunPending         StageRunStatus = "pending"
	StageRunQueued          StageRunStatus = "queued"
	StageRunRunning         StageRunStatus = "running"
	StageRunWaitingApproval StageRunStatus = "waiting_approval"
	StageRunSucceeded       StageRunStatus = "succeeded"
	StageRunFailed          StageRunStatus = "failed"
	StageRunSkipped         StageRunStatus = "skipped"
)

type ArtifactType string

const (
	ArtifactStructuredRequirement ArtifactType = "structured_requirement"
	ArtifactSolutionDesign        ArtifactType = "solution_design"
	ArtifactCodeDiff              ArtifactType = "code_diff"
	ArtifactTestReport            ArtifactType = "test_report"
	ArtifactReviewReport          ArtifactType = "review_report"
	ArtifactDeliverySummary       ArtifactType = "delivery_summary"
)

type CheckpointType string

const (
	CheckpointDesignReview CheckpointType = "design_review"
	CheckpointCodeReview   CheckpointType = "code_review"
)

type CheckpointStatus string

const (
	CheckpointPending  CheckpointStatus = "pending"
	CheckpointApproved CheckpointStatus = "approved"
	CheckpointRejected CheckpointStatus = "rejected"
)

type AgentRunStatus string

const (
	AgentRunPending   AgentRunStatus = "pending"
	AgentRunRunning   AgentRunStatus = "running"
	AgentRunSucceeded AgentRunStatus = "succeeded"
	AgentRunFailed    AgentRunStatus = "failed"
)

type GitDeliveryStatus string

const (
	GitDeliveryPending   GitDeliveryStatus = "pending"
	GitDeliveryDraft     GitDeliveryStatus = "draft"
	GitDeliveryReady     GitDeliveryStatus = "ready"
	GitDeliveryCompleted GitDeliveryStatus = "completed"
	GitDeliveryFailed    GitDeliveryStatus = "failed"
)

type InPageEditStatus string

const (
	InPageEditPending  InPageEditStatus = "pending"
	InPageEditPreview  InPageEditStatus = "preview"
	InPageEditApplied  InPageEditStatus = "applied"
	InPageEditReverted InPageEditStatus = "reverted"
)

type BaseModel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PipelineTemplate struct {
	ID             string `gorm:"primaryKey;size:64"`
	Name           string `gorm:"size:128;not null"`
	Description    string `gorm:"type:text;not null"`
	Version        string `gorm:"size:32;not null"`
	DefinitionJSON string `gorm:"type:text;not null"`
	IsActive       bool   `gorm:"not null;default:true"`
	BaseModel
}

type PipelineRun struct {
	ID              string            `gorm:"primaryKey;size:64"`
	TemplateID      string            `gorm:"size:64;not null;index"`
	Template        PipelineTemplate  `gorm:"foreignKey:TemplateID;references:ID"`
	Title           string            `gorm:"size:255;not null"`
	RequirementText string            `gorm:"type:text;not null"`
	SourceSessionID string            `gorm:"size:64;index"`
	TargetRepo      string            `gorm:"size:255;not null"`
	TargetBranch    string            `gorm:"size:255;not null"`
	WorkBranch      string            `gorm:"size:255;not null"`
	Status          PipelineRunStatus `gorm:"size:32;not null;index"`
	CurrentStageKey string            `gorm:"size:128;not null"`
	CreatedBy       string            `gorm:"size:64;not null;index"`
	StartedAt       *time.Time
	FinishedAt      *time.Time
	BaseModel
}

type StageRun struct {
	ID            string         `gorm:"primaryKey;size:64"`
	PipelineRunID string         `gorm:"size:64;not null;index"`
	StageKey      string         `gorm:"size:128;not null"`
	StageType     StageType      `gorm:"size:32;not null"`
	Status        StageRunStatus `gorm:"size:32;not null;index"`
	Attempt       int            `gorm:"not null;default:1"`
	InputJSON     string         `gorm:"type:text"`
	OutputJSON    string         `gorm:"type:text"`
	ErrorMessage  string         `gorm:"type:text"`
	StartedAt     *time.Time
	FinishedAt    *time.Time
	BaseModel
}

type Artifact struct {
	ID            string       `gorm:"primaryKey;size:64"`
	PipelineRunID string       `gorm:"size:64;not null;index"`
	StageRunID    string       `gorm:"size:64;index"`
	ArtifactType  ArtifactType `gorm:"size:64;not null;index"`
	Title         string       `gorm:"size:255;not null"`
	ContentText   string       `gorm:"type:text"`
	ContentJSON   string       `gorm:"type:text"`
	FilePath      string       `gorm:"size:512"`
	MetaJSON      string       `gorm:"type:text"`
	BaseModel
}

type Checkpoint struct {
	ID             string           `gorm:"primaryKey;size:64"`
	PipelineRunID  string           `gorm:"size:64;not null;index"`
	StageRunID     string           `gorm:"size:64;index"`
	CheckpointType CheckpointType   `gorm:"size:64;not null"`
	Status         CheckpointStatus `gorm:"size:32;not null;index"`
	ApproverID     string           `gorm:"size:64"`
	Decision       string           `gorm:"size:32"`
	Comment        string           `gorm:"type:text"`
	DecidedAt      *time.Time
	BaseModel
}

type AgentRun struct {
	ID             string         `gorm:"primaryKey;size:64"`
	PipelineRunID  string         `gorm:"size:64;not null;index"`
	StageRunID     string         `gorm:"size:64;index"`
	AgentKey       string         `gorm:"size:128;not null"`
	Provider       string         `gorm:"size:64"`
	Model          string         `gorm:"size:128"`
	PromptSnapshot string         `gorm:"type:text"`
	InputJSON      string         `gorm:"type:text"`
	OutputJSON     string         `gorm:"type:text"`
	TokenUsageJSON string         `gorm:"type:text"`
	LatencyMS      int64          `gorm:"not null;default:0"`
	Status         AgentRunStatus `gorm:"size:32;not null;index"`
	ErrorMessage   string         `gorm:"type:text"`
	BaseModel
}

type GitDelivery struct {
	ID               string            `gorm:"primaryKey;size:64"`
	PipelineRunID    string            `gorm:"size:64;not null;index"`
	Provider         string            `gorm:"size:64;not null"`
	Repo             string            `gorm:"size:255;not null"`
	BaseBranch       string            `gorm:"size:255;not null"`
	HeadBranch       string            `gorm:"size:255;not null"`
	CommitSHA        string            `gorm:"size:128"`
	PRMRURL          string            `gorm:"size:512"`
	PRMRTitle        string            `gorm:"size:255"`
	PRMRBody         string            `gorm:"type:text"`
	ChangedFilesJSON string            `gorm:"type:text"`
	ValidationJSON   string            `gorm:"type:text"`
	SummaryMarkdown  string            `gorm:"type:text"`
	Status           GitDeliveryStatus `gorm:"size:32;not null;index"`
	BaseModel
}

type InPageEditSession struct {
	ID                string           `gorm:"primaryKey;size:64"`
	PageURL           string           `gorm:"size:512;not null"`
	PipelineRunID     string           `gorm:"size:64;index"`
	SelectionJSON     string           `gorm:"type:text"`
	InstructionText   string           `gorm:"type:text;not null"`
	LocatorResultJSON string           `gorm:"type:text"`
	PreviewStatus     InPageEditStatus `gorm:"size:32;not null;index"`
	CreatedBy         string           `gorm:"size:64;not null;index"`
	BaseModel
}

type User struct {
	ID           string   `gorm:"primaryKey;size:64"`
	FeishuOpenID string   `gorm:"size:128;uniqueIndex"`
	Name         string   `gorm:"size:128;not null"`
	Email        string   `gorm:"size:128"`
	AvatarURL    string   `gorm:"size:512"`
	Role         Role     `gorm:"size:32;not null"`
	Departments  []string `gorm:"serializer:json"`
	// GitHub 绑定信息
	GitHubID          string `gorm:"size:64"`
	GitHubLogin       string `gorm:"size:128"`
	GitHubAvatar      string `gorm:"size:512"`
	GitHubAccessToken string `gorm:"type:text"` // 用于调用 GitHub API
	BaseModel
}

type FeishuCredential struct {
	ID                    string    `gorm:"primaryKey;size:64"`
	UserID                string    `gorm:"size:64;not null;uniqueIndex"`
	User                  User      `gorm:"foreignKey:UserID;references:ID"`
	OpenID                string    `gorm:"size:128;not null;uniqueIndex"`
	UnionID               string    `gorm:"size:128"`
	FeishuUserID          string    `gorm:"size:128"`
	AccessToken           string    `gorm:"type:text;not null"`
	RefreshToken          string    `gorm:"type:text;not null"`
	AccessTokenExpiresAt  time.Time `gorm:"not null"`
	RefreshTokenExpiresAt time.Time `gorm:"not null"`
	LastLoginAt           time.Time `gorm:"not null"`
	LastRefreshAt         time.Time
	BaseModel
}

type LoginSession struct {
	ID        string    `gorm:"primaryKey;size:128"`
	UserID    string    `gorm:"size:64;not null;index"`
	User      User      `gorm:"foreignKey:UserID;references:ID"`
	ExpiresAt time.Time `gorm:"not null;index"`
	BaseModel
}

type Session struct {
	ID      string        `gorm:"primaryKey;size:64"`
	Title   string        `gorm:"size:255;not null"`
	Summary string        `gorm:"type:text;not null"`
	Status  SessionStatus `gorm:"size:32;not null;index"`
	OwnerID string        `gorm:"size:64;not null;index"`
	Owner   User          `gorm:"foreignKey:OwnerID;references:ID"`
	BaseModel
}

type Message struct {
	ID        string      `gorm:"primaryKey;size:64"`
	SessionID string      `gorm:"size:64;not null;index"`
	Role      MessageRole `gorm:"size:32;not null"`
	Content   string      `gorm:"type:text;not null"`
	CreatedAt time.Time   `gorm:"autoCreateTime"`
}

type Requirement struct {
	ID                  string        `gorm:"primaryKey;size:64"`
	SessionID           string        `gorm:"size:64;not null;uniqueIndex"`
	Title               string        `gorm:"size:255;not null"`
	Summary             string        `gorm:"type:text;not null"`
	Status              SessionStatus `gorm:"size:32;not null"`
	DeliverySummary     string        `gorm:"type:text;not null"`
	ReferencedKnowledge []string      `gorm:"serializer:json"`
	PublishedAt         time.Time
	BaseModel
}

type Task struct {
	ID                 string       `gorm:"primaryKey;size:64"`
	SessionID          string       `gorm:"size:64;not null;index"`
	Title              string       `gorm:"size:255;not null"`
	Description        string       `gorm:"type:text;not null"`
	Type               TaskType     `gorm:"size:32;not null"`
	Status             TaskStatus   `gorm:"size:32;not null"`
	AssigneeName       string       `gorm:"size:128;not null"`
	AssigneeRole       Role         `gorm:"size:32;not null"`
	AssigneeID         string       `gorm:"size:128"`
	AssigneeIDType     string       `gorm:"size:32"`
	Priority           TaskPriority `gorm:"size:32;not null;default:medium"`
	EstimateDays       int          `gorm:"not null;default:1"`
	PlannedStartAt     *time.Time
	PlannedEndAt       *time.Time
	NotifyContent      string   `gorm:"type:text"`
	AcceptanceCriteria []string `gorm:"serializer:json"`
	Risks              []string `gorm:"serializer:json"`
	DocURL             string   `gorm:"size:512"`
	BitableAppToken    string   `gorm:"size:128"`
	BitableTableID     string   `gorm:"size:128"`
	BitableRecordID    string   `gorm:"size:128"`
	BitableRecordURL   string   `gorm:"size:512"`
	BaseModel
}

type RoleMapping struct {
	ID          string   `gorm:"primaryKey;size:64"`
	Name        string   `gorm:"size:128;not null"`
	Keyword     string   `gorm:"size:128;not null"`
	Role        Role     `gorm:"size:32;not null"`
	Departments []string `gorm:"serializer:json"`
}

type RoleOwner struct {
	ID           string `gorm:"primaryKey;size:64"`
	Role         Role   `gorm:"size:32;not null;uniqueIndex"`
	OwnerName    string `gorm:"size:128;not null"`
	FeishuID     string `gorm:"size:128"`
	FeishuIDType string `gorm:"size:32"`
	Enabled      bool   `gorm:"not null;default:false"`
	BaseModel
}

type KnowledgeSource struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:255;not null"`
	Content   string `gorm:"type:text;not null"`
	UpdatedAt time.Time
}

type MessageDelivery struct {
	ID         string    `gorm:"primaryKey;size:64"`
	TaskID     string    `gorm:"size:64;not null;index"`
	Channel    string    `gorm:"size:64;not null"`
	Receiver   string    `gorm:"size:128;not null"`
	Status     string    `gorm:"size:64;not null"`
	RemoteID   string    `gorm:"size:128"`
	RawPayload string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// Project 项目 - 汇总多个 API 文档到一个 Swagger UI
type Project struct {
	ID          string `gorm:"primaryKey;size:128"`
	Title       string `gorm:"size:255;not null"`
	Description string `gorm:"size:1024"`
	SwaggerURL  string `gorm:"size:512"` // 汇总后的 Swagger UI URL
	DocUrls     string `gorm:"type:text"` // JSON array of document URLs
	GitHubRepo  string `gorm:"size:512"` // 关联的 GitHub 仓库，多个用逗号分隔，格式：owner/repo1,owner/repo2
	BaseModel
}

// OpenAPISpec API 文档 - 属于某个项目
type OpenAPISpec struct {
	ID         string `gorm:"primaryKey;size:128"`
	ProjectID  string `gorm:"size:128;index"` // 所属项目
	Title      string `gorm:"size:255"`
	Description string `gorm:"size:1024"`
	SpecJSON   string `gorm:"type:text;not null"`
	SwaggerURL string `gorm:"size:512"`
	DocUrls    string `gorm:"type:text"` // JSON array of document URLs
	BaseModel
}
