package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RipgrepEngine 封装 ripgrep 命令行工具
type RipgrepEngine struct {
	BinPath string
}

// NewRipgrepEngine 创建新的搜索引擎实例
func NewRipgrepEngine() *RipgrepEngine {
	// 默认假设 rg 在 PATH 中
	// 也可以后续扩展为查找 bundled binary
	return &RipgrepEngine{BinPath: "rg"}
}

// SearchOptions 搜索选项
type SearchOptions struct {
	Query          string   // 搜索关键词
	RootPath       string   // 搜索根目录
	IsRegex        bool     // 是否正则
	CaseSensitive  bool     // 是否区分大小写
	WordMatch      bool     // 是否全词匹配
	Extensions     []string // 文件扩展名过滤 (e.g. "go", "py")
	IncludePattern []string // 包含的文件 glob (e.g. "*.go")
	IgnorePattern  []string // 忽略的文件 glob
	ContextLines   int      // 上下文行数
	MaxCount       int      // 最大结果数
}

// TextMatch 代表一个文本匹配项
type TextMatch struct {
	FilePath      string `json:"file_path"`
	LineNumber    int    `json:"line_number"`
	Content       string `json:"content"`        // 匹配行的内容
	ContextBefore string `json:"context_before"` // 上文
	ContextAfter  string `json:"context_after"`  // 下文
	Submatches    []int  `json:"submatches"`     // 匹配字符的起止偏移量 [start, end, start, end...]
}

// RipgrepRawMatch rg --json 输出的原始结构 (部分字段)
type RgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type RgMatchData struct {
	Path       RgPathData       `json:"path"`
	Lines      RgLineData       `json:"lines"`
	LineNumber int              `json:"line_number"`
	Absolute   int              `json:"absolute_offset"`
	Submatches []RgSubmatchData `json:"submatches"`
}

type RgPathData struct {
	Text string `json:"text"`
}

type RgLineData struct {
	Text string `json:"text"`
}

type RgSubmatchData struct {
	Match RgMatchText `json:"match"`
	Start int         `json:"start"`
	End   int         `json:"end"`
}

type RgMatchText struct {
	Text string `json:"text"`
}

// Search 执行搜索
func (e *RipgrepEngine) Search(ctx context.Context, opts SearchOptions) ([]TextMatch, error) {
	if opts.RootPath == "" {
		return nil, fmt.Errorf("root path is required")
	}

	args := []string{"--json"} // 强制 JSON 输出

	if !opts.CaseSensitive {
		args = append(args, "-i")
	}
	if !opts.IsRegex {
		args = append(args, "-F") // Fixed string
	}
	if opts.WordMatch {
		args = append(args, "-w")
	}
	if opts.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", opts.ContextLines))
	}
	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-m%d", opts.MaxCount))
	}

	// 排除常见干扰项
	// 默认排除 .git, node_modules 等 (rg 默认会处理 .gitignore)
	// 这里添加额外的强制排除
	defaultIgnores := []string{
		".git", ".svn", ".hg", "node_modules", "dist", "build", "target", "vendor",
		".idea", ".vscode", "__pycache__", ".venv", "venv",
		".mcp-data", // 🆕 排除 MPM 缓存目录，避免搜索到临时文件
		"*.lock", "*.log", "*.map", "*.min.js", "*.min.css",
	}
	for _, ignore := range defaultIgnores {
		args = append(args, "-g", "!"+ignore)
	}

	// 用户自定义忽略
	for _, ignore := range opts.IgnorePattern {
		args = append(args, "-g", "!"+ignore)
	}

	// 包含模式
	for _, include := range opts.IncludePattern {
		args = append(args, "-g", include)
	}

	// 扩展名过滤
	// rg -t type 需要预定义类型，较麻烦。直接用 glob 模拟
	for _, ext := range opts.Extensions {
		ext = strings.TrimPrefix(ext, ".")
		args = append(args, "-g", "*."+ext)
	}

	// 目标
	args = append(args, opts.Query)
	args = append(args, opts.RootPath)

	cmd := exec.CommandContext(ctx, e.BinPath, args...)

	// 不设 Dir，直接对 RootPath 搜索。但如果 RootPath 是相对路径，可能需要
	// cmd.Dir = opts.RootPath // 暂不设置，假设 RootPath 是绝对路径

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 设置超时
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, e.BinPath, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	if err != nil {
		// rg 返回 1 表示没找到，不是错误
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []TextMatch{}, nil
		}
		// 如果是命令找不到，执行 Native Fallback
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "无法将") {
			return e.nativeSearch(ctx, opts)
		}
		return nil, fmt.Errorf("ripgrep failed: %v, stderr: %s", err, stderr.String())
	}

	return e.parseOutput(stdout.Bytes())
}

// nativeSearch 使用 Go 原生 遍历进行简单搜索 (兜底方案)
func (e *RipgrepEngine) nativeSearch(ctx context.Context, opts SearchOptions) ([]TextMatch, error) {
	var results []TextMatch
	root := opts.RootPath
	query := opts.Query
	if !opts.CaseSensitive {
		query = strings.ToLower(query)
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}
		if info.IsDir() {
			// 简单忽略常见目录
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "target" || name == "build" || name == ".mcp-data" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查扩展名
		if len(opts.Extensions) > 0 {
			ext := filepath.Ext(path)
			matched := false
			for _, e := range opts.Extensions {
				if strings.EqualFold(ext, "."+strings.TrimPrefix(e, ".")) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// 读取文件内容进行简单搜索
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			displayLine := line
			if !opts.CaseSensitive {
				line = strings.ToLower(line)
			}

			if strings.Contains(line, query) {
				results = append(results, TextMatch{
					FilePath:   filepath.ToSlash(path),
					LineNumber: i + 1,
					Content:    strings.TrimSpace(displayLine),
				})
				if opts.MaxCount > 0 && len(results) >= opts.MaxCount {
					return fmt.Errorf("limit reached")
				}
			}

			// 检查 Context 超时
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return nil, err
	}

	return results, nil
}

// parseOutput 解析 JSON 输出
func (e *RipgrepEngine) parseOutput(output []byte) ([]TextMatch, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var results []TextMatch

	// 暂存 context，rg json 的 context 是分开的消息
	// 目前简化处理，只提取 match 类型的行
	// TODO: 完整支持 context (rg 输出顺序是 Context -> Match -> Context)

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg RgMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // 忽略解析错误行
		}

		if msg.Type == "match" {
			var matchData RgMatchData
			if err := json.Unmarshal(msg.Data, &matchData); err != nil {
				continue
			}

			// 提取 submatches
			var subs []int
			for _, sm := range matchData.Submatches {
				subs = append(subs, sm.Start, sm.End)
			}

			// 修正 windows 路径分割符
			cleanPath := strings.ReplaceAll(matchData.Path.Text, "\\", "/")

			// 简单的内容去空白 (display friendly)
			// 实际上 rg --json 返回的是包含换行符的完整行
			content := strings.TrimRight(matchData.Lines.Text, "\r\n")

			results = append(results, TextMatch{
				FilePath:   cleanPath,
				LineNumber: matchData.LineNumber,
				Content:    content,
				Submatches: subs,
			})
		}
	}

	return results, nil
}
