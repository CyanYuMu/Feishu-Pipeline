package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"feishu-pipeline/apps/api-go/internal/external"
	"feishu-pipeline/apps/api-go/internal/model"
)

type CodeGenerationHandler struct {
	githubService *external.GitHubService
}

// NewCodeGenerationHandler 创建代码生成处理器
func NewCodeGenerationHandler(githubService *external.GitHubService) *CodeGenerationHandler {
	return &CodeGenerationHandler{githubService: githubService}
}

func (h *CodeGenerationHandler) Execute(_ context.Context, ctx StageContext) (StageExecutionResult, error) {
	impactFiles := nestedStringSlice(ctx.Input, "latestArtifacts", model.ArtifactSolutionDesign, SchemaFieldImpactFiles)

	// 如果有 GitHub 服务，尝试读取仓库上下文
	var repoContext []map[string]any
	if h.githubService != nil && ctx.Run.TargetRepo != "" && ctx.Run.TargetRepo != "self" {
		repoContext = h.buildRepoContext(ctx)
	}

	if len(impactFiles) == 0 {
		impactFiles = []string{"apps/api-go/internal/pipeline/executor.go", "apps/api-go/internal/service/pipeline_service.go"}
	}

	changeSet := h.buildChangeSetWithContext(impactFiles, repoContext, ctx.Run.RequirementText)
	patches := make([]map[string]any, 0, len(changeSet))
	for _, item := range changeSet {
		patches = append(patches, map[string]any{
			"filePath":         item["filePath"],
			"changeType":       item["changeType"],
			"patch":            item["proposedPatch"],
			"reason":           item["reason"],
			"contextIncluded":  item["contextIncluded"],
			"originalContent":  item["originalContent"],
			"proposedDiff":     item["proposedDiff"],
		})
	}

	payload := baseStagePayload(ctx)
	payload[SchemaFieldSummary] = "生成受控代码变更计划，基于仓库真实上下文。"
	payload[SchemaFieldChangedFiles] = impactFiles
	payload[SchemaFieldPatches] = patches
	payload[SchemaFieldDiffSummary] = []string{"基于仓库上下文生成变更计划", "变更已结构化待审批"}
	payload[SchemaFieldManualSteps] = []string{"用户预览变更计划", "审批后系统写入文件并推送"}
	payload["inputs"] = []string{"技术方案", "影响文件列表", "仓库上下文"}
	payload["outputs"] = []string{"changeSet", "diff 摘要", "待验证文件列表", "codePlan"}
	payload["risks"] = []string{"等待用户审批后才会写入文件"}
	payload["nextActions"] = []string{"进入评审阶段", "用户预览并确认变更计划"}
	payload["workBranch"] = ctx.Run.WorkBranch
	payload["changeSet"] = changeSet
	payload["repoContextFiles"] = len(repoContext)

	return newStageResult(model.ArtifactCodeDiff, "代码变更计划", payload, formatChangeSet(ctx.Run.WorkBranch, changeSet)), nil
}

// buildRepoContext 构建仓库上下文
func (h *CodeGenerationHandler) buildRepoContext(ctx StageContext) []map[string]any {
	var contextFiles []map[string]any

	owner, repo, ok := external.ParseRepoPath(ctx.Run.TargetRepo)
	if !ok {
		return contextFiles
	}

	branch := ctx.Run.TargetBranch
	if branch == "" {
		branch = "main"
	}

	keywords := extractKeywords(ctx.Run.RequirementText)
	allFiles, err := h.githubService.ListFilesRecursive(context.Background(), "", owner, repo, branch, 5)
	if err != nil {
		return contextFiles
	}

	for _, file := range allFiles {
		if !isCodeFile(file.Name) {
			continue
		}

		shouldInclude := len(keywords) == 0 // 无关键词时包含所有文件
		if !shouldInclude {
			for _, kw := range keywords {
				if strings.Contains(strings.ToLower(file.Path), strings.ToLower(kw)) ||
					strings.Contains(strings.ToLower(file.Name), strings.ToLower(kw)) {
					shouldInclude = true
					break
				}
			}
		}

		if shouldInclude {
			content, sha, err := h.githubService.GetFileContent(context.Background(), "", owner, repo, file.Path, branch)
			if err != nil {
				continue
			}

			contextFiles = append(contextFiles, map[string]any{
				"path":    file.Path,
				"name":    file.Name,
				"content": truncateContent(content, 500),
				"sha":     sha,
			})

			if len(contextFiles) >= 10 {
				break
			}
		}
	}

	return contextFiles
}

func (h *CodeGenerationHandler) buildChangeSetWithContext(impactFiles []string, repoContext []map[string]any, requirement string) []map[string]any {
	changeSet := make([]map[string]any, 0, len(impactFiles))

	ctxMap := make(map[string]string)
	for _, ctx := range repoContext {
		if path, ok := ctx["path"].(string); ok {
			if content, ok := ctx["content"].(string); ok {
				ctxMap[path] = content
			}
		}
	}

	for _, file := range impactFiles {
		changeType := changeTypeForPath(file)
		proposedPatch := generatePatchSummary(file, ctxMap[file], requirement)

		item := map[string]any{
			"filePath":         file,
			"changeType":       changeType,
			"reason":           proposedPatchSummary(file),
			"proposedPatch":    proposedPatch,
			"contextIncluded":  ctxMap[file] != "",
		}

		if originalContent, ok := ctxMap[file]; ok {
			item["originalContent"] = originalContent
			item["proposedDiff"] = generateDiff(originalContent, proposedPatch)
		}

		changeSet = append(changeSet, item)
	}

	return changeSet
}

func extractKeywords(requirement string) []string {
	if requirement == "" {
		return []string{}
	}

	words := strings.Fields(requirement)
	var keywords []string
	for _, w := range words {
		if len(w) >= 4 && !isStopWord(w) {
			keywords = append(keywords, strings.Trim(w, ".,;:!:?"))
			if len(keywords) >= 5 {
				break
			}
		}
	}
	return keywords
}

func isStopWord(word string) bool {
	stopWords := []string{"the", "and", "for", "with", "this", "that", "from", "have", "been", "will", "shall", "could", "would", "should", "需要", "实现", "功能", "系统", "用户", "我们"}
	lowerWord := strings.ToLower(word)
	for _, sw := range stopWords {
		if lowerWord == sw {
			return true
		}
	}
	return false
}

func isCodeFile(filename string) bool {
	codeExtensions := []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".c", ".cpp", ".cs", ".rb", ".rs", ".swift", ".kt"}
	for _, ext := range codeExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}
	return false
}

func truncateContent(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (内容已截断)"
}

func generatePatchSummary(filePath, originalContent, requirement string) string {
	if strings.Contains(filePath, "executor") {
		return "重构阶段执行逻辑，添加受控 patch apply 能力"
	}
	if strings.Contains(filePath, "pipeline_service") {
		return "补充 ExecuteChanges 方法支持变更计划落地"
	}
	if strings.Contains(filePath, "controller") {
		return "暴露 execute-changes API 供前端调用"
	}
	if strings.Contains(filePath, "docs") {
		return "更新技术设计文档与赛题要求一致"
	}
	summaryLen := 50
	if len(requirement) < summaryLen {
		summaryLen = len(requirement)
	}
	return fmt.Sprintf("根据需求生成代码变更：%s", requirement[:summaryLen])
}

func generateDiff(original, proposed string) string {
	return fmt.Sprintf("- 原始代码 (共 %d 行)\n+ 新代码 (%d 字符)\n\n变更概要: %s",
		strings.Count(original, "\n")+1, len(proposed), proposed)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func changeTypeForPath(path string) string {
	if strings.HasSuffix(path, "context.go") || strings.HasSuffix(path, "executor_test.go") {
		return "create"
	}
	return "modify"
}

func proposedPatchSummary(path string) string {
	switch {
	case strings.Contains(path, "executor"):
		return "拆分阶段处理逻辑并补齐结构化阶段产物。"
	case strings.Contains(path, "pipeline_service"):
		return "复用现有生命周期与 checkpoint 回退能力，补充可观测数据入口。"
	case strings.Contains(path, "controller") || strings.Contains(path, "types"):
		return "为前端工作台暴露阶段执行记录与产物字段。"
	case strings.Contains(path, "docs"):
		return "保持实现与产品开发设计、赛题要求一致。"
	default:
		return "按技术方案补充 DevFlow Pipeline 闭环能力。"
	}
}

func formatChangeSet(workBranch string, changeSet []map[string]any) string {
	lines := []string{fmt.Sprintf("工作分支：%s", workBranch), "变更计划："}
	for _, item := range changeSet {
		lines = append(lines, fmt.Sprintf("- %s [%s]：%s", item["filePath"], item["changeType"], item["reason"]))
	}
	return strings.Join(lines, "\n")
}

// ExecuteChangesParams 变更执行参数
type ExecuteChangesParams struct {
	RunID       string
	ChangeSet   []ChangeItem
	CommitterID string
	Token       string // GitHub token
}

// ChangeItem 单个变更项
type ChangeItem struct {
	FilePath   string `json:"filePath"`
	NewContent string `json:"newContent"`
	SHA        string `json:"sha,omitempty"` // 文件 SHA，更新时需要
}

// ExecutionResult 变更执行结果
type ExecutionResult struct {
	AppliedFiles []string          `json:"appliedFiles"`
	FailedFiles  []map[string]any  `json:"failedFiles"`
	Summary      string            `json:"summary"`
}

// ExecuteChanges 执行变更计划
func ExecuteChanges(ctx context.Context, gh *external.GitHubService, params ExecuteChangesParams, run model.PipelineRun) (*ExecutionResult, error) {
	result := &ExecutionResult{
		AppliedFiles: []string{},
		FailedFiles:  []map[string]any{},
	}

	owner, repo, ok := external.ParseRepoPath(run.TargetRepo)
	if !ok {
		return nil, fmt.Errorf("无效的仓库路径: %s", run.TargetRepo)
	}

	branch := run.WorkBranch
	if branch == "" {
		branch = "main"
	}

	for _, item := range params.ChangeSet {
		err := gh.CreateFile(ctx, params.Token, owner, repo, item.FilePath, branch, item.NewContent,
			fmt.Sprintf("Feishu Pipeline: %s", run.Title), item.SHA)

		if err != nil {
			result.FailedFiles = append(result.FailedFiles, map[string]any{
				"filePath": item.FilePath,
				"error":    err.Error(),
			})
		} else {
			result.AppliedFiles = append(result.AppliedFiles, item.FilePath)
		}
	}

	if len(result.AppliedFiles) > 0 {
		result.Summary = fmt.Sprintf("成功应用 %d/%d 个文件变更", len(result.AppliedFiles), len(params.ChangeSet))
	} else {
		result.Summary = "变更执行失败，未成功应用任何文件"
	}

	return result, nil
}

// MarshalChangeSet 将 changeSet 序列化为 JSON
func MarshalChangeSet(changeSet []map[string]any) string {
	data, _ := json.Marshal(changeSet)
	return string(data)
}

// ParseChangeSet 从 JSON 解析 changeSet
func ParseChangeSet(jsonStr string) ([]map[string]any, error) {
	var result []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}
	return result, nil
}
