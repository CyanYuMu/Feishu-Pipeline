package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// 读取仓库上下文
	var repoContext []map[string]any
	if ctx.Run.TargetRepo == "self" || ctx.Run.TargetRepo == "" {
		// 本地代码库上下文
		repoContext = h.buildLocalRepoContext(ctx)
	} else if h.githubService != nil {
		// 远程GitHub仓库上下文
		repoContext = h.buildRepoContext(ctx)
	}

	// 如果没有指定影响文件，从上下文提取或使用默认值
	if len(impactFiles) == 0 {
		if len(repoContext) > 0 {
			// 从上下文提取前几个文件作为影响文件
			for _, f := range repoContext {
				if path, ok := f["path"].(string); ok {
					impactFiles = append(impactFiles, path)
				}
				if len(impactFiles) >= 3 {
					break
				}
			}
		} else {
			impactFiles = []string{"apps/api-go/internal/pipeline/executor.go", "apps/api-go/internal/service/pipeline_service.go"}
		}
	}

	changeSet := h.buildChangeSetWithContext(impactFiles, repoContext, ctx.Run.RequirementText)
	patches := make([]map[string]any, 0, len(changeSet))
	for _, item := range changeSet {
		patches = append(patches, map[string]any{
			"filePath":        item["filePath"],
			"changeType":      item["changeType"],
			"patch":           item["proposedPatch"],
			"reason":          item["reason"],
			"contextIncluded": item["contextIncluded"],
			"originalContent": item["originalContent"],
			"proposedDiff":    item["proposedDiff"],
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

// buildLocalRepoContext 构建本地代码库上下文
func (h *CodeGenerationHandler) buildLocalRepoContext(ctx StageContext) []map[string]any {
	var contextFiles []map[string]any
	rootDir, err := os.Getwd() // 获取当前工作目录（项目根目录）
	if err != nil {
		return contextFiles
	}

	keywords := extractKeywords(ctx.Run.RequirementText)
	// 遍历项目目录，最多递归5层
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 跳过目录、非代码文件、隐藏目录和依赖目录
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			// 限制递归深度
			rel, _ := filepath.Rel(rootDir, path)
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) >= 5 {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCodeFile(info.Name()) {
			return nil
		}

		// 匹配关键词
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		shouldInclude := len(keywords) == 0 // 无关键词时包含所有文件
		if !shouldInclude {
			for _, kw := range keywords {
				if strings.Contains(strings.ToLower(relPath), strings.ToLower(kw)) ||
					strings.Contains(strings.ToLower(info.Name()), strings.ToLower(kw)) {
					shouldInclude = true
					break
				}
			}
		}

		if shouldInclude {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			contextFiles = append(contextFiles, map[string]any{
				"path":    relPath,
				"name":    info.Name(),
				"content": truncateContent(string(content), 500),
				"local":   true,
			})

			if len(contextFiles) >= 10 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("buildLocalRepoContext error: %v\n", err)
	}
	return contextFiles
}

// buildRepoContext 构建远程GitHub仓库上下文
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
				"local":   false,
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

	// 读取本地文件的完整内容（如果存在）
	for _, file := range impactFiles {
		// 尝试读取本地完整文件内容
		fullContent, err := os.ReadFile(file)
		var originalContent string
		if err == nil {
			originalContent = string(fullContent)
		} else if ctxMap[file] != "" {
			originalContent = ctxMap[file]
		} else {
			originalContent = "// 无法获取文件内容"
		}

		changeType := changeTypeForPath(file)
		proposedPatch := generateProposedPatch(file, originalContent, requirement)
		diff := generateGitDiff(file, originalContent, proposedPatch)

		item := map[string]any{
			"filePath":        file,
			"changeType":      changeType,
			"reason":          proposedPatchSummary(file),
			"proposedPatch":   proposedPatch,
			"originalContent": originalContent,
			"proposedDiff":    diff,
			"contextIncluded": originalContent != "// 无法获取文件内容",
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

// generateProposedPatch 基于原始内容和需求生成模拟的代码变更
func generateProposedPatch(filePath, originalContent, requirement string) string {
	// 简单的模拟逻辑：根据文件类型和关键词生成合理的变更
	lines := strings.Split(originalContent, "\n")
	var newLines []string
	modified := false

	// 对于 Go 文件，在合适的位置添加日志或注释
	if strings.HasSuffix(filePath, ".go") {
		for i, line := range lines {
			newLines = append(newLines, line)
			// 在函数开头添加日志
			if !modified && strings.Contains(line, "func ") && strings.Contains(line, "(") && strings.Contains(line, ")") {
				// 查找函数体开始的 {
				for j := i; j < len(lines); j++ {
					if strings.Contains(lines[j], "{") {
						// 在 { 后面添加日志
						newLines = append(newLines, "\t// TODO: 自动生成的变更 - 添加入口日志")
						newLines = append(newLines, "\tlog.Printf(\"进入函数: %s\", \" "+extractFunctionName(line)+"\")")
						modified = true
						break
					}
				}
			}
			// 添加需求相关的注释
			if !modified && i == 0 && strings.HasPrefix(line, "package ") {
				newLines = append(newLines, "")
				newLines = append(newLines, "// 自动生成的代码变更")
				newLines = append(newLines, fmt.Sprintf("// 需求: %s", truncateContent(requirement, 100)))
				modified = true
			}
		}
	} else if strings.HasSuffix(filePath, ".ts") || strings.HasSuffix(filePath, ".tsx") {
		// 对于 TypeScript 文件，添加 console.log
		for _, line := range lines {
			newLines = append(newLines, line)
			if !modified && strings.Contains(line, "function ") || strings.Contains(line, "const ") && strings.Contains(line, "=>") {
				newLines = append(newLines, "  // 自动生成的变更 - 添加调试日志")
				newLines = append(newLines, "  console.log('执行功能:', "+fmt.Sprintf("'%s'", truncateContent(requirement, 50))+")")
				modified = true
				break
			}
		}
	} else {
		// 其他文件类型，在开头添加注释
		newLines = append(newLines, fmt.Sprintf("# 自动生成的代码变更 - %s", time.Now().Format(time.RFC3339)))
		newLines = append(newLines, fmt.Sprintf("# 需求: %s", truncateContent(requirement, 200)))
		newLines = append(newLines, "")
		newLines = append(newLines, lines...)
		modified = true
	}

	if !modified {
		// 如果没有修改，添加一个注释到末尾
		newLines = append(lines, "")
		newLines = append(newLines, "// 自动生成的变更")
		newLines = append(newLines, fmt.Sprintf("// 实现需求: %s", truncateContent(requirement, 100)))
	}

	return strings.Join(newLines, "\n")
}

// extractFunctionName 从函数定义行提取函数名
func extractFunctionName(line string) string {
	parts := strings.Split(line, "func ")
	if len(parts) < 2 {
		return "unknown"
	}
	funcPart := parts[1]
	nameEnd := strings.Index(funcPart, "(")
	if nameEnd == -1 {
		return "unknown"
	}
	return strings.TrimSpace(funcPart[:nameEnd])
}

// generateGitDiff 生成标准 git 格式的 diff
func generateGitDiff(filePath, original, proposed string) string {
	originalLines := strings.Split(original, "\n")
	proposedLines := strings.Split(proposed, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	diff.WriteString("--- a/" + filePath + "\n")
	diff.WriteString("+++ b/" + filePath + "\n")

	// 简单的行级 diff 实现
	maxLines := maxInt(len(originalLines), len(proposedLines))
	var hunkStart int
	var hunk []string
	inHunk := false

	for i := 0; i < maxLines; i++ {
		var origLine, propLine string
		if i < len(originalLines) {
			origLine = originalLines[i]
		}
		if i < len(proposedLines) {
			propLine = proposedLines[i]
		}

		if origLine == propLine {
			if inHunk {
				// 结束当前 hunk
				hunk = append(hunk, " "+origLine)
				// 最多保持3行上下文
				if len(hunk) >= 6 && i >= len(originalLines)-1 || i >= len(proposedLines)-1 {
					writeHunk(&diff, hunkStart, hunk)
					inHunk = false
					hunk = nil
				}
			}
		} else {
			if !inHunk {
				// 开始新的 hunk，包含前面3行上下文
				hunkStart = maxInt(0, i-3)
				hunk = nil
				for j := hunkStart; j < i; j++ {
					if j < len(originalLines) {
						hunk = append(hunk, " "+originalLines[j])
					}
				}
				inHunk = true
			}
			// 添加变更行
			if i < len(originalLines) {
				hunk = append(hunk, "-"+origLine)
			}
			if i < len(proposedLines) {
				hunk = append(hunk, "+"+propLine)
			}
		}
	}

	// 处理剩余的 hunk
	if inHunk {
		writeHunk(&diff, hunkStart, hunk)
	}

	return diff.String()
}

// writeHunk 写入 diff hunk 头部
func writeHunk(diff *strings.Builder, start int, hunk []string) {
	// 计算 hunk 的行数
	oldCount := 0
	newCount := 0
	for _, line := range hunk {
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") {
			oldCount++
		}
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, " ") {
			newCount++
		}
	}
	diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", start+1, oldCount, start+1, newCount))
	for _, line := range hunk {
		diff.WriteString(line + "\n")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
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
	AppliedFiles []string         `json:"appliedFiles"`
	FailedFiles  []map[string]any `json:"failedFiles"`
	Summary      string           `json:"summary"`
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
