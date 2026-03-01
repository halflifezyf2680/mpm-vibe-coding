package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ============================================================================
// 数据结构 - 与 Rust ast_indexer 输出格式匹配
// ============================================================================

// Node 符号节点
type Node struct {
	ID            string   `json:"id"`
	NodeType      string   `json:"type"`
	Name          string   `json:"name"`
	QualifiedName string   `json:"qualified_name"`
	ScopePath     string   `json:"scope_path,omitempty"`
	FilePath      string   `json:"file_path"`
	LineStart     int      `json:"line_start"`
	LineEnd       int      `json:"line_end"`
	Signature     string   `json:"signature,omitempty"`
	Calls         []string `json:"calls,omitempty"`
}

// Stats 统计信息
type Stats struct {
	TotalFiles   int `json:"total_files"`
	TotalSymbols int `json:"total_symbols"`
}

// MapResult 项目地图结果 (--mode map)
type MapResult struct {
	Statistics    Stats              `json:"statistics"`
	Structure     map[string][]Node  `json:"structure"`
	Elapsed       string             `json:"elapsed"`
	ComplexityMap map[string]float64 `json:"complexity_map,omitempty"` // 符号名 -> 复杂度分数
}

// StructureDirInfo 目录结构信息（--mode structure）
type StructureDirInfo struct {
	FileCount int      `json:"file_count"`
	Files     []string `json:"files"`
}

// StructureResult 目录结构结果（--mode structure）
type StructureResult struct {
	Status     string                      `json:"status"`
	TotalFiles int                         `json:"total_files"`
	Structure  map[string]StructureDirInfo `json:"structure"`
}

// CandidateMatch 候选匹配
type CandidateMatch struct {
	Node      Node    `json:"node"`
	MatchType string  `json:"match_type"`
	Score     float32 `json:"score"`
}

// CallerInfo 调用者信息
type CallerInfo struct {
	Node     Node   `json:"node"`
	CallType string `json:"call_type"`
}

// QueryResult 查询结果 (--mode query)
type QueryResult struct {
	Status       string           `json:"status"`
	Query        string           `json:"query"`
	FoundSymbol  *Node            `json:"found_symbol"`
	MatchType    string           `json:"match_type,omitempty"`
	Candidates   []CandidateMatch `json:"candidates"`
	RelatedNodes []CallerInfo     `json:"related_nodes"`
}

// ImpactResult 影响分析结果 (--mode analyze)
type ImpactResult struct {
	Status                string       `json:"status"`
	NodeID                string       `json:"node_id"`
	ComplexityScore       float64      `json:"complexity_score"`
	ComplexityLevel       string       `json:"complexity_level"`
	RiskLevel             string       `json:"risk_level"`
	AffectedNodes         int          `json:"affected_nodes"`
	DirectCallers         []CallerInfo `json:"direct_callers"`
	IndirectCallers       []CallerInfo `json:"indirect_callers"`
	ModificationChecklist []string     `json:"modification_checklist"`
	Message               string       `json:"message,omitempty"`
}

// IndexResult 索引结果 (--mode index)
type IndexResult struct {
	Status       string `json:"status"`
	TotalFiles   int    `json:"total_files"`
	ParsedFiles  int    `json:"parsed_files,omitempty"`
	MetaFiles    int    `json:"meta_files,omitempty"`
	SkippedFiles int    `json:"skipped_files,omitempty"`
	Strategy     string `json:"strategy,omitempty"`
	ElapsedMs    int64  `json:"elapsed_ms"`
}

// NamingAnalysis 命名风格分析结果
type NamingAnalysis struct {
	FileCount      int      `json:"file_count"`
	SymbolCount    int      `json:"symbol_count"`
	DominantStyle  string   `json:"dominant_style"` // snake_case / camelCase / mixed
	SnakeCasePct   string   `json:"snake_case_pct"`
	CamelCasePct   string   `json:"camel_case_pct"`
	ClassStyle     string   `json:"class_style"` // PascalCase
	CommonPrefixes []string `json:"common_prefixes"`
	SampleNames    []string `json:"sample_names"` // 样例 函数名
	IsNewProject   bool     `json:"is_new_project"`
}

// ============================================================================
// ASTIndexer 核心服务
// ============================================================================

// ASTIndexer AST 索引器服务
type ASTIndexer struct {
	BinaryPath  string
	indexMu     sync.Mutex
	lastIndexAt map[string]time.Time
}

const defaultIndexFreshness = 5 * time.Minute
const defaultIndexCommandTimeout = 30 * time.Minute

// NewASTIndexer 创建 AST 索引器
func NewASTIndexer() *ASTIndexer {
	newIndexer := func(path string) *ASTIndexer {
		return &ASTIndexer{
			BinaryPath:  path,
			lastIndexAt: make(map[string]time.Time),
		}
	}

	exeName := "ast_indexer.exe"
	if runtime.GOOS != "windows" {
		exeName = "ast_indexer"
	}

	// 获取当前可执行文件所在目录
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// 尝试在同级 bin 目录查找
		binPath := filepath.Join(execDir, "bin", exeName)
		if fileExists(binPath) {
			return newIndexer(binPath)
		}
		// 尝试同级目录
		sameDirPath := filepath.Join(execDir, exeName)
		if fileExists(sameDirPath) {
			return newIndexer(sameDirPath)
		}
	}

	// 兜底：尝试相对路径
	paths := []string{
		filepath.Join("bin", exeName),
		filepath.Join("mcp-server-go", "bin", exeName),
	}

	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		if fileExists(abs) {
			return newIndexer(abs)
		}
	}

	return newIndexer(exeName)
}

func normalizeProjectRoot(projectRoot string) string {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return projectRoot
	}
	return absRoot
}

func getIndexCommandTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("MPM_AST_INDEX_TIMEOUT_SECONDS"))
	if raw == "" {
		return defaultIndexCommandTimeout
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return defaultIndexCommandTimeout
	}
	return time.Duration(sec) * time.Second
}

func (ai *ASTIndexer) markIndexFresh(projectRoot string) {
	root := normalizeProjectRoot(projectRoot)
	ai.indexMu.Lock()
	ai.lastIndexAt[root] = time.Now()
	ai.indexMu.Unlock()
}

func (ai *ASTIndexer) shouldSkipIndex(projectRoot string, maxAge time.Duration) bool {
	root := normalizeProjectRoot(projectRoot)

	ai.indexMu.Lock()
	if ts, ok := ai.lastIndexAt[root]; ok && time.Since(ts) < maxAge {
		ai.indexMu.Unlock()
		return true
	}
	ai.indexMu.Unlock()

	info, err := os.Stat(getDBPath(root))
	if err != nil {
		return false
	}
	if time.Since(info.ModTime()) >= maxAge {
		return false
	}

	if !hasUsableIndex(getDBPath(root)) {
		return false
	}

	ai.indexMu.Lock()
	ai.lastIndexAt[root] = time.Now()
	ai.indexMu.Unlock()
	return true
}

func (ai *ASTIndexer) EnsureFreshIndex(projectRoot string) (*IndexResult, error) {
	if ai.shouldSkipIndex(projectRoot, defaultIndexFreshness) {
		return &IndexResult{Status: "cached"}, nil
	}
	return ai.Index(projectRoot)
}

func hasUsableIndex(dbPath string) bool {
	info, err := os.Stat(dbPath)
	if err != nil || info.Size() <= 0 {
		return false
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()

	var filesTableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='files'").Scan(&filesTableCount); err != nil {
		return false
	}
	if filesTableCount == 0 {
		return false
	}

	var fileCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&fileCount); err != nil {
		return false
	}

	return fileCount > 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getDBPath 获取数据库路径
func getDBPath(projectRoot string) string {
	// 【修复】确保返回绝对路径,防止Rust引擎将文件写到错误位置
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		// 如果转换失败,使用原路径(但可能有风险)
		absRoot = projectRoot
	}
	return filepath.Join(absRoot, ".mcp-data", "symbols.db")
}

// getOutputPath 获取临时输出路径
func getOutputPath(projectRoot string, mode string) string {
	// 【修复】确保返回绝对路径,防止缓存文件跑到C盘
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		// 如果转换失败,使用原路径(但可能有风险)
		absRoot = projectRoot
	}
	mcpData := filepath.Join(absRoot, ".mcp-data")
	_ = os.MkdirAll(mcpData, 0755)
	return filepath.Join(mcpData, fmt.Sprintf(".ast_result_%s.json", mode))
}

// ============================================================================
// 技术栈检测与过滤配置 (移植自 ast_indexer_helper.py)
// ============================================================================

// detectTechStackAndConfig 智能检测技术栈，返回(允许的扩展名, 忽略的目录)
func detectTechStackAndConfig(projectRoot string) (extensions string, ignoreDirs string) {
	var stackDetected []string
	var exts []string

	// 基础忽略目录
	ignores := []string{
		".git", "__pycache__", "node_modules", ".venv", "venv",
		"dist", "build", ".idea", ".vscode",
		"release", "releases", "archive", "backup", "old",
	}

	// 从 .gitignore 解析额外的忽略目录
	gitignoreDirs := parseGitignoreDirs(projectRoot)
	ignores = append(ignores, gitignoreDirs...)

	// 一次性递归扫描文件扩展名，避免只看根目录导致误判
	extSet := scanProjectExtensions(projectRoot, ignores, 8)
	hasExt := func(ext string) bool {
		ext = strings.TrimPrefix(strings.ToLower(ext), ".")
		return extSet[ext]
	}

	// 1. 检测 Python
	if fileExists(filepath.Join(projectRoot, "requirements.txt")) ||
		fileExists(filepath.Join(projectRoot, "pyproject.toml")) ||
		hasExt(".py") {
		stackDetected = append(stackDetected, "python")
		exts = append(exts, ".py")
		ignores = append(ignores, "site-packages", "htmlcov", ".pytest_cache")
	}

	// 2. 检测 Frontend (Node/React/Vue)
	if fileExists(filepath.Join(projectRoot, "package.json")) ||
		hasExt(".js") || hasExt(".jsx") || hasExt(".ts") || hasExt(".tsx") || hasExt(".vue") || hasExt(".svelte") {
		stackDetected = append(stackDetected, "frontend")
		exts = append(exts, ".js", ".jsx", ".ts", ".tsx", ".vue", ".svelte", ".css", ".html")
		ignores = append(ignores, "coverage", ".next", ".nuxt", "out")
	}

	// 3. 检测 Go
	if fileExists(filepath.Join(projectRoot, "go.mod")) || hasExt(".go") {
		stackDetected = append(stackDetected, "go")
		exts = append(exts, ".go")
		ignores = append(ignores, "vendor", "bin")
	}

	// 4. 检测 Rust (递归搜索)
	if hasRustProject(projectRoot) || hasExt(".rs") {
		stackDetected = append(stackDetected, "rust")
		exts = append(exts, ".rs")
		ignores = append(ignores, "target")
	}

	// 5. 检测 C/C++
	if hasExt(".c") || hasExt(".cpp") || hasExt(".h") || hasExt(".hpp") || hasExt(".cc") ||
		fileExists(filepath.Join(projectRoot, "CMakeLists.txt")) {
		stackDetected = append(stackDetected, "cpp")
		exts = append(exts, ".c", ".h", ".cpp", ".hpp", ".cc")
		ignores = append(ignores, "cmake-build-debug", "obj")
	}

	// 6. 检测 Java
	if hasExt(".java") || fileExists(filepath.Join(projectRoot, "pom.xml")) ||
		fileExists(filepath.Join(projectRoot, "build.gradle")) {
		stackDetected = append(stackDetected, "java")
		exts = append(exts, ".java")
		ignores = append(ignores, ".gradle")
	}

	// 如果没有检测到特定栈，不限制扩展名
	if len(stackDetected) == 0 {
		return "", uniqueJoin(ignores)
	}

	return uniqueJoin(exts), uniqueJoin(ignores)
}

// scanProjectExtensions 递归扫描项目内出现过的扩展名
func scanProjectExtensions(projectRoot string, ignoreDirs []string, maxDepth int) map[string]bool {
	result := make(map[string]bool)
	ignoreSet := make(map[string]bool)

	for _, dir := range ignoreDirs {
		d := strings.TrimSpace(strings.ToLower(strings.Trim(dir, "/\\")))
		if d != "" {
			ignoreSet[d] = true
		}
	}

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maxDepth {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			name := e.Name()
			nameLower := strings.ToLower(name)

			if e.IsDir() {
				if shouldSkipDetectDir(nameLower, ignoreSet) {
					continue
				}
				walk(filepath.Join(dir, name), depth+1)
				continue
			}

			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
			if ext != "" {
				result[ext] = true
			}
		}
	}

	walk(projectRoot, 0)
	return result
}

func shouldSkipDetectDir(name string, ignoreSet map[string]bool) bool {
	if ignoreSet[name] {
		return true
	}

	switch name {
	case ".git", "node_modules", "vendor", "target", "dist", "build", "coverage", ".next", ".nuxt", "out",
		"__pycache__", ".pytest_cache", ".venv", "venv", "site-packages", ".idea", ".vscode", ".mcp-data",
		"release", "releases", "archive", "backup", "old":
		return true
	default:
		return false
	}
}

// parseGitignoreDirs 解析 .gitignore 文件，提取目录忽略规则
func parseGitignoreDirs(projectRoot string) []string {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil
	}

	var ignoredDirs []string
	fileExtensions := map[string]bool{
		"txt": true, "md": true, "json": true, "yml": true, "yaml": true,
		"toml": true, "lock": true, "log": true, "py": true, "js": true,
		"ts": true, "rs": true, "go": true, "java": true, "c": true,
		"cpp": true, "h": true, "hpp": true, "sql": true, "db": true,
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 跳过否定规则
		if strings.HasPrefix(line, "!") {
			continue
		}

		// 优先级 1: 以 / 结尾的明确目录
		if strings.HasSuffix(line, "/") {
			dirName := strings.TrimSuffix(line, "/")
			dirName = strings.TrimPrefix(dirName, "/")
			dirName = strings.ReplaceAll(dirName, "**/", "")
			if dirName != "" && !strings.HasPrefix(dirName, "**") {
				ignoredDirs = append(ignoredDirs, dirName)
			}
			continue
		}

		// 优先级 2: 包含 / 的路径模式
		if strings.Contains(line, "/") {
			parts := strings.Split(line, "/")
			pathPart := strings.ReplaceAll(parts[len(parts)-1], "*", "")
			if pathPart != "" && !strings.HasPrefix(pathPart, "**") {
				// 检查是否是纯文件名（包含扩展名）
				if strings.Contains(pathPart, ".") {
					extParts := strings.Split(pathPart, ".")
					ext := strings.ToLower(extParts[len(extParts)-1])
					if fileExtensions[ext] {
						continue
					}
				}
				ignoredDirs = append(ignoredDirs, pathPart)
			}
		}
	}

	return ignoredDirs
}

// hasFilesWithExt 检查目录下是否有指定扩展名的文件
func hasFilesWithExt(dir string, ext string) bool {
	extSet := scanProjectExtensions(dir, nil, 8)
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	return extSet[ext]
}

// hasRustProject 递归检查是否有 Rust 项目
func hasRustProject(projectRoot string) bool {
	if fileExists(filepath.Join(projectRoot, "Cargo.toml")) {
		return true
	}
	// 递归搜索子目录（最多6层）
	return hasCargoTomlRecursive(projectRoot, 0, 6)
}

func hasCargoTomlRecursive(dir string, depth, maxDepth int) bool {
	if depth >= maxDepth {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			subdir := filepath.Join(dir, e.Name())
			if fileExists(filepath.Join(subdir, "Cargo.toml")) {
				return true
			}
			if hasCargoTomlRecursive(subdir, depth+1, maxDepth) {
				return true
			}
		}
	}
	return false
}

// uniqueJoin 去重并用逗号连接
func uniqueJoin(items []string) string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		item = strings.TrimPrefix(item, ".")
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return strings.Join(result, ",")
}

// ============================================================================
// 核心方法

// ============================================================================

// MapProject 绘制项目地图 (--mode map)
func (ai *ASTIndexer) MapProject(projectRoot string, detail string) (*MapResult, error) {
	return ai.MapProjectWithScope(projectRoot, detail, "")
}

// StructureProjectWithScope 快速目录结构扫描（--mode structure，不依赖符号索引）
func (ai *ASTIndexer) StructureProjectWithScope(projectRoot string, scope string) (*StructureResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "structure")
	_, ignoreDirs := detectTechStackAndConfig(projectRoot)

	_ = os.Remove(outputPath)

	if scope == "." || scope == "./" {
		scope = ""
	}

	args := []string{
		"--mode", "structure",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--detail", "standard",
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	if ignoreDirs != "" {
		args = append(args, "--ignore-dirs", ignoreDirs)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("目录结构扫描失败: %v: %s", err, msg)
		}
		return nil, fmt.Errorf("目录结构扫描失败: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录结构结果失败: %v", err)
	}

	var result StructureResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析目录结构结果失败: %v", err)
	}

	return &result, nil
}

// MapProjectWithScope 带范围的项目地图
func (ai *ASTIndexer) MapProjectWithScope(projectRoot string, detail string, scope string) (*MapResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "map")

	// 清理旧文件
	_ = os.Remove(outputPath)

	// 智能技术栈检测
	_, ignoreDirs := detectTechStackAndConfig(projectRoot)

	// 如果 scope 是 "." 或 "./"，清理掉，让 Rust 引擎执行全量扫描
	if scope == "." || scope == "./" {
		scope = ""
	}

	args := []string{
		"--mode", "map",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--detail", detail,
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	// 允许 Rust 引擎自动探测所有语言，除非明确指定（暂不自动限定）
	// if exts != "" {
	// 	args = append(args, "--extensions", exts)
	// }
	if ignoreDirs != "" {
		args = append(args, "--ignore-dirs", ignoreDirs)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot // 设置工作目录

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("项目地图生成失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取地图结果失败: %v", err)
	}

	var result MapResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析地图结果失败: %v", err)
	}

	return &result, nil
}

// SearchSymbol 搜索符号 (--mode query)
func (ai *ASTIndexer) SearchSymbol(projectRoot string, query string) (*QueryResult, error) {
	return ai.SearchSymbolWithScope(projectRoot, query, "")
}

// SearchSymbolWithScope 带范围的符号搜索
func (ai *ASTIndexer) SearchSymbolWithScope(projectRoot string, query string, scope string) (*QueryResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "query")

	// 清理旧文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "query",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--query", query,
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("符号搜索失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取搜索结果失败: %v", err)
	}

	var result QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %v", err)
	}

	return &result, nil
}

// GetSymbolAtLine 获取指定文件行号处的符号信息 (--mode query --file --line)
func (ai *ASTIndexer) GetSymbolAtLine(projectRoot string, filePath string, line int) (*Node, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, fmt.Sprintf("line_%d", line))

	// 清理所有旧的 line_*.json 临时文件（避免泄漏）
	mcpData := filepath.Join(projectRoot, ".mcp-data")
	if entries, err := os.ReadDir(mcpData); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), ".ast_result_line_") && strings.HasSuffix(e.Name(), ".json") {
				_ = os.Remove(filepath.Join(mcpData, e.Name()))
			}
		}
	}

	// 清理当前文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "query",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--file", filePath,
		"--line", fmt.Sprintf("%d", line),
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("定位符号失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取定位结果失败: %v", err)
	}

	var result QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析定位结果失败: %v", err)
	}

	return result.FoundSymbol, nil
}

// Analyze 执行影响分析 (--mode analyze)
func (ai *ASTIndexer) Analyze(projectRoot string, symbol string, direction string) (*ImpactResult, error) {
	// 先确保索引是最新的
	_, _ = ai.EnsureFreshIndex(projectRoot)

	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "analyze")

	// 清理旧文件
	_ = os.Remove(outputPath)

	args := []string{
		"--mode", "analyze",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
		"--query", symbol,
	}
	if direction != "" {
		args = append(args, "--direction", direction)
	}

	cmd := exec.Command(ai.BinaryPath, args...)
	cmd.Dir = projectRoot

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("影响分析执行失败: %v", err)
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("读取分析结果失败: %v", err)
	}

	var result ImpactResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析分析结果失败: %v", err)
	}

	return &result, nil
}

func (ai *ASTIndexer) runIndexCommand(projectRoot string, args []string) error {
	timeout := getIndexCommandTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ai.BinaryPath, args...)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return fmt.Errorf("索引命令超时(%s): %s", timeout, msg)
		}
		return fmt.Errorf("索引命令超时(%s)", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return fmt.Errorf("%v: %s", err, msg)
		}
		return err
	}
	return nil
}

func buildIndexArgs(projectRoot, dbPath, outputPath, ignoreDirs, extensions, scope string, useExtensions bool, forceFull bool) []string {
	args := []string{
		"--mode", "index",
		"--project", projectRoot,
		"--db", dbPath,
		"--output", outputPath,
	}
	if forceFull {
		args = append(args, "--force-full")
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	if ignoreDirs != "" {
		args = append(args, "--ignore-dirs", ignoreDirs)
	}
	if useExtensions && extensions != "" {
		args = append(args, "--extensions", extensions)
	}
	return args
}

// Index 刷新索引 (--mode index)
func (ai *ASTIndexer) Index(projectRoot string) (*IndexResult, error) {
	return ai.indexWithOptions(projectRoot, "", false)
}

// IndexFull 强制全量索引（禁用 bootstrap）
func (ai *ASTIndexer) IndexFull(projectRoot string) (*IndexResult, error) {
	return ai.indexWithOptions(projectRoot, "", true)
}

func (ai *ASTIndexer) indexWithOptions(projectRoot string, scope string, forceFull bool) (*IndexResult, error) {
	dbPath := getDBPath(projectRoot)
	outputPath := getOutputPath(projectRoot, "index")

	// 确保 .mcp-data 目录存在
	mcpData := filepath.Join(projectRoot, ".mcp-data")
	_ = os.MkdirAll(mcpData, 0755)
	// 清理旧文件
	_ = os.Remove(outputPath)

	// 技术栈检测仅用于忽略目录与失败兜底，不再默认启用扩展白名单
	extensions, ignoreDirs := detectTechStackAndConfig(projectRoot)

	// 第一阶段：默认全量扫描（不传 --extensions），让 Rust 端按真实文件扩展自适应
	args := buildIndexArgs(projectRoot, dbPath, outputPath, ignoreDirs, extensions, scope, false, forceFull)
	if err := ai.runIndexCommand(projectRoot, args); err != nil {
		// 第二阶段：仅在全量扫描失败时，退回到扩展白名单模式
		if extensions != "" {
			_ = os.Remove(outputPath)
			retryArgs := buildIndexArgs(projectRoot, dbPath, outputPath, ignoreDirs, extensions, scope, true, forceFull)
			if retryErr := ai.runIndexCommand(projectRoot, retryArgs); retryErr != nil {
				return nil, fmt.Errorf("索引刷新失败: 全量扫描失败(%v); 扩展模式重试失败(%v)", err, retryErr)
			}
		} else {
			return nil, fmt.Errorf("索引刷新失败: %v", err)
		}
	}

	// 读取输出文件
	data, err := os.ReadFile(outputPath)
	if err != nil {
		// 索引可能不输出文件，返回默认结果
		result := &IndexResult{Status: "success"}
		ai.markIndexFresh(projectRoot)
		return result, nil
	}

	var result IndexResult
	if err := json.Unmarshal(data, &result); err != nil {
		fallback := &IndexResult{Status: "success"}
		ai.markIndexFresh(projectRoot)
		return fallback, nil
	}

	ai.markIndexFresh(projectRoot)
	return &result, nil
}

// IndexScope 按目录范围增量刷新索引（用于热点补录）
func (ai *ASTIndexer) IndexScope(projectRoot string, scope string) (*IndexResult, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "." || scope == "./" {
		return ai.Index(projectRoot)
	}
	return ai.indexWithOptions(projectRoot, scope, false)
}

// AnalyzeNamingStyle 分析项目命名风格
func (ai *ASTIndexer) AnalyzeNamingStyle(projectRoot string) (*NamingAnalysis, error) {
	// 1. 确保索引存在 (且尝试刷新)
	if _, err := ai.EnsureFreshIndex(projectRoot); err != nil {
		// 如果索引失败，尝试直接读取现有数据库
		// 什么也不做
	}

	dbPath := getDBPath(projectRoot)
	if !fileExists(dbPath) {
		return &NamingAnalysis{IsNewProject: true}, nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 2. 统计文件数
	var fileCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&fileCount); err != nil {
		// 可能表不存在
		return &NamingAnalysis{IsNewProject: true}, nil
	}

	if fileCount < 3 {
		return &NamingAnalysis{IsNewProject: true, FileCount: fileCount}, nil
	}

	// 3. 提取所有函数名
	rows, err := db.Query("SELECT name FROM symbols WHERE symbol_type IN ('function', 'method') LIMIT 1000")
	if err != nil {
		return nil, fmt.Errorf("查询符号失败: %v", err)
	}
	defer rows.Close()

	var funcNames []string
	var snakeCount, camelCount int
	// reSnake := regexp.MustCompile(`^[a-z0-9_]+$`) // Unused
	reCamel := regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)

	prefixCounts := make(map[string]int)

	for rows.Next() {
		var name string
		rows.Scan(&name)
		funcNames = append(funcNames, name)

		// 风格判定
		if strings.Contains(name, "_") && strings.ToLower(name) == name {
			snakeCount++
		} else if reCamel.MatchString(name) && !strings.Contains(name, "_") {
			camelCount++
		}

		// 前缀提取 (如 get_, set_, on_)
		parts := strings.Split(name, "_")
		if len(parts) > 1 {
			prefixCounts[parts[0]+"_"]++
		} else if strings.HasPrefix(name, "get") && len(name) > 3 && name[3] >= 'A' && name[3] <= 'Z' {
			prefixCounts["get"]++ // camelCase get
		}
	}

	// 4. 计算结果
	totalFuncs := len(funcNames)
	if totalFuncs == 0 {
		return &NamingAnalysis{IsNewProject: true, FileCount: fileCount}, nil
	}

	snakePct := float64(snakeCount) / float64(totalFuncs) * 100
	camelPct := float64(camelCount) / float64(totalFuncs) * 100

	style := "snake_case"
	if camelCount > snakeCount {
		style = "camelCase"
	} else if snakeCount == 0 && camelCount == 0 {
		style = "mixed"
	}

	// 提取Top前缀
	var prefixes []string
	for p, c := range prefixCounts {
		if c > max(2, totalFuncs/20) { // 至少出现2次且占比>5%
			prefixes = append(prefixes, p)
		}
	}
	// 简单取前5个作为展示
	if len(prefixes) > 5 {
		prefixes = prefixes[:5]
	}

	// 样例数据 (取前10个)
	var samples []string
	if totalFuncs > 10 {
		samples = funcNames[:10]
	} else {
		samples = funcNames
	}

	return &NamingAnalysis{
		FileCount:      fileCount,
		SymbolCount:    totalFuncs,
		DominantStyle:  style,
		SnakeCasePct:   fmt.Sprintf("%.1f%%", snakePct),
		CamelCasePct:   fmt.Sprintf("%.1f%%", camelPct),
		ClassStyle:     "PascalCase", // 默认假设
		CommonPrefixes: prefixes,
		SampleNames:    samples,
		IsNewProject:   false,
	}, nil
}

// RiskInfo 风险信息
type RiskInfo struct {
	SymbolName string  `json:"symbol_name"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
}

// ComplexityReport 复杂度报告
type ComplexityReport struct {
	HighRiskSymbols []RiskInfo `json:"high_risk_symbols"`
	TotalAnalyzed   int        `json:"total_analyzed"`
}

// AnalyzeComplexity 分析符号复杂度 (基于调用关系)
// 简单的中心度分析：Fan-out (出度) 高代表依赖复杂，Fan-in (入度) 高代表影响范围广/责任重
func (ai *ASTIndexer) AnalyzeComplexity(projectRoot string, symbolNames []string) (*ComplexityReport, error) {
	if len(symbolNames) == 0 {
		return &ComplexityReport{}, nil
	}

	dbPath := getDBPath(projectRoot)
	if !fileExists(dbPath) {
		return nil, nil // No DB, no analysis
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var report ComplexityReport
	report.TotalAnalyzed = len(symbolNames)

	hasCalleeID := hasColumn(db, "calls", "callee_id")

	for _, name := range symbolNames {
		// 1. 获取 Symbol 信息（ID + canonical_id）
		rows, err := db.Query("SELECT symbol_id, symbol_type, canonical_id FROM symbols WHERE name = ?", name)
		if err != nil {
			continue
		}

		type symbolRef struct {
			id          int
			canonicalID string
		}
		var symbols []symbolRef
		for rows.Next() {
			var s symbolRef
			var sType string
			if err := rows.Scan(&s.id, &sType, &s.canonicalID); err != nil {
				continue
			}
			if sType == "function" || sType == "method" || sType == "class" {
				symbols = append(symbols, s)
			}
		}
		rows.Close()

		if len(symbols) == 0 {
			continue
		}

		// 聚合所有同名符号的指标
		var maxFanIn, maxFanOut int

		for _, sym := range symbols {
			// Fan-out: 我调用了谁 (caller_id = symbol_id)
			var fanOut int
			db.QueryRow("SELECT COUNT(*) FROM calls WHERE caller_id = ?", sym.id).Scan(&fanOut)
			if fanOut > maxFanOut {
				maxFanOut = fanOut
			}

			// Fan-in: 优先 callee_id，回退 callee_name
			var fanIn int
			if hasCalleeID {
				db.QueryRow(
					"SELECT COUNT(*) FROM calls WHERE callee_id = ? OR (callee_id IS NULL AND callee_name = ?)",
					sym.canonicalID, name,
				).Scan(&fanIn)
			} else {
				db.QueryRow("SELECT COUNT(*) FROM calls WHERE callee_name = ?", name).Scan(&fanIn)
			}
			if fanIn > maxFanIn {
				maxFanIn = fanIn
			}
		}

		// 简单的评分模型
		// FanOut > 10 -> Complex Logic
		// FanIn > 20 -> High Impact Core
		score := float64(maxFanOut)*1.0 + float64(maxFanIn)*0.5

		var reasons []string
		if maxFanOut > 10 {
			reasons = append(reasons, fmt.Sprintf("High Coupling (Calls: %d)", maxFanOut))
		}
		if maxFanIn > 20 {
			reasons = append(reasons, fmt.Sprintf("Core Module (Ref by: %d)", maxFanIn))
		}

		// 🆕 始终添加到报告，即使复杂度很低
		report.HighRiskSymbols = append(report.HighRiskSymbols, RiskInfo{
			SymbolName: name,
			Score:      score,
			Reason:     strings.Join(reasons, ", "),
		})
	}

	return &report, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hasColumn(db *sql.DB, table string, column string) bool {
	q := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	var n int
	if err := db.QueryRow(q, column).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
