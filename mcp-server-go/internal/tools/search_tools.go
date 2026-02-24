package tools

import (
	"context"
	"fmt"
	"mcp-server-go/internal/services"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SearchArgs 搜索参数
type SearchArgs struct {
	Query      string `json:"query" jsonschema:"required,description=搜索关键词"`
	Scope      string `json:"scope" jsonschema:"description=限定范围"`
	SearchType string `json:"search_type" jsonschema:"default=any,enum=any,enum=function,enum=class,description=符号类型过滤"`
}

// RegisterSearchTools 注册搜索工具
func RegisterSearchTools(s *server.MCPServer, sm *SessionManager, ai *services.ASTIndexer) {

	s.AddTool(mcp.NewTool("code_search",
		mcp.WithDescription(`code_search - 代码符号定位 (比 grep 更懂代码)

用途：
  【精确定位】当你只知道名字（函数名/类名），但不知道它在哪个文件时，别用 grep，用我。
  我也支持搜索特定范围内的符号定义，是阅读代码的导航员。

参数策略：
  query (必填)
    不要写自然语言！直接写代码符号名（如 "SessionManager" 或 "HandleRequest"）。
  
  scope (可选)
    知道大概在哪个目录？填进来（如 "internal/core"），能大幅提高准确率。
  
  search_type (可选)
    - 找函数实现？ -> "function"
    - 找数据结构？ -> "class"
    - 只要是代码？ -> "any" (默认)

返回：
  告诉代码符号定义所在的精确文件路径和行号。

触发词：
  "mpm 搜索", "mpm 定位", "mpm 符号", "mpm find"`),
		mcp.WithInputSchema[SearchArgs](),
	), wrapSearch(sm, ai))
}

func wrapSearch(sm *SessionManager, ai *services.ASTIndexer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if sm.ProjectRoot == "" {
			return mcp.NewToolResultError("项目尚未初始化，请先执行 initialize_project。"), nil
		}

		var args SearchArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数格式错误: %v", err)), nil
		}

		// 优先按范围补录（热点目录），否则按新鲜度检查全量索引
		if strings.TrimSpace(args.Scope) != "" {
			_, _ = ai.IndexScope(sm.ProjectRoot, args.Scope)
		} else {
			_, _ = ai.EnsureFreshIndex(sm.ProjectRoot)
		}

		// 1. AST Search (Core Strategy)
		astResult, err := ai.SearchSymbolWithScope(sm.ProjectRoot, args.Query, args.Scope)
		if err != nil {
			// Log error but continue to grep if possible
		}

		// 1.1 Scope Filtering (Client-side enforcement)
		if astResult != nil && astResult.FoundSymbol != nil && args.Scope != "" {
			path := strings.ReplaceAll(astResult.FoundSymbol.FilePath, "\\", "/")
			scope := strings.ReplaceAll(args.Scope, "\\", "/")
			if !strings.Contains(path, scope) {
				astResult.FoundSymbol = nil
			}
		}

		// 1.5 Type Filtering
		if astResult != nil && args.SearchType != "" && args.SearchType != "any" {
			wantType := args.SearchType // function or class

			// Filter FoundSymbol
			if astResult.FoundSymbol != nil {
				t := astResult.FoundSymbol.NodeType
				match := false
				if wantType == "function" && (t == "function" || t == "method") {
					match = true
				} else if wantType == "class" && (t == "class" || t == "struct" || t == "interface") {
					match = true
				}
				if !match {
					astResult.FoundSymbol = nil
				}
			}

			// Filter Candidates
			var kept []services.CandidateMatch
			for _, c := range astResult.Candidates {
				t := c.Node.NodeType
				match := false
				if wantType == "function" && (t == "function" || t == "method") {
					match = true
				} else if wantType == "class" && (t == "class" || t == "struct" || t == "interface") {
					match = true
				}
				if match {
					kept = append(kept, c)
				}
			}
			astResult.Candidates = kept
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### 关于「%s」的搜索结果\n\n", args.Query))

		// 2. Decide if Grep is needed
		// Fallback trigger: No Exact Match found in AST (after filtering)
		useGrep := astResult == nil || astResult.FoundSymbol == nil

		// 如果 AST 找到了精确匹配，直接展示，不进行 grep (避免噪音)
		if astResult != nil && astResult.FoundSymbol != nil {
			sb.WriteString(fmt.Sprintf("✅ **精确定义** (%s):\n", astResult.MatchType))
			node := astResult.FoundSymbol
			sb.WriteString(fmt.Sprintf("- **%s** `%s` @ `%s` L%d-%d\n",
				node.NodeType, node.Name, node.FilePath, node.LineStart, node.LineEnd))

			if node.Signature != "" {
				sb.WriteString(fmt.Sprintf("  Config: `%s`\n", node.Signature))
			}
			sb.WriteString("\n")
		} else if astResult != nil && len(astResult.Candidates) > 0 {
			// 展示 AST 候选
			sb.WriteString("🔍 **相似符号** (AST):\n")
			for i, c := range astResult.Candidates {
				if i >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("- [%s] `%s` @ `%s` (score: %.2f)\n",
					c.Node.NodeType, c.Node.Name, c.Node.FilePath, c.Score))
			}
			sb.WriteString("\n")
		}

		// 3. Ripgrep Fallback (Text Search & Deep Context)
		if useGrep {
			rg := services.NewRipgrepEngine()

			// 智能检测是否包含路径分隔符，如果有，只搜那个文件或目录
			searchRoot := sm.ProjectRoot
			if args.Scope != "" {
				searchRoot = filepath.Join(sm.ProjectRoot, args.Scope)
			}

			matches, err := rg.Search(ctx, services.SearchOptions{
				Query:         args.Query,
				RootPath:      searchRoot,
				CaseSensitive: false, // 默认不区分大小写
				WordMatch:     false,
				MaxCount:      20, // 限制数量以防爆炸
				ContextLines:  0,
			})

			if err == nil && len(matches) > 0 {
				sb.WriteString(fmt.Sprintf("🕵️ **文本搜索结果** (Ripgrep found %d matches):\n", len(matches)))

				// Group by File
				grouped := make(map[string][]services.TextMatch)
				for _, m := range matches {
					grouped[m.FilePath] = append(grouped[m.FilePath], m)
				}

				// Deep Context Analysis (Limited to top 10 unique files to save time)
				filesProcessed := 0
				for path, fileMatches := range grouped {
					if filesProcessed >= 10 {
						sb.WriteString(fmt.Sprintf("... (剩余 %d 个文件的匹配已省略)\n", len(grouped)-filesProcessed))
						break
					}

					sb.WriteString(fmt.Sprintf("📄 **%s**\n", path))

					for i, m := range fileMatches {
						if i >= 3 {
							sb.WriteString(fmt.Sprintf("  ... (本文件还有 %d 处匹配)\n", len(fileMatches)-i))
							break
						}

						// 🧠 Deep Context: 反查所属符号
						// 性能优化：只查第一个匹配的Context，或者每行都查？
						// 查每行有助于定位 "Where is it used?"
						// 但 exec 开销大。仅对前几行反查。
						contextInfo := ""
						if i < 3 {
							owner, _ := ai.GetSymbolAtLine(sm.ProjectRoot, path, m.LineNumber)
							if owner != nil {
								contextInfo = fmt.Sprintf("in `%s` (%s)", owner.Name, owner.NodeType)
							} else {
								contextInfo = "(global)"
							}
						}

						cleanContent := strings.TrimSpace(m.Content)
						if len(cleanContent) > 80 {
							cleanContent = cleanContent[:80] + "..."
						}

						sb.WriteString(fmt.Sprintf("  L%d: `%s` %s\n", m.LineNumber, cleanContent, contextInfo))
					}
					filesProcessed++
				}
				sb.WriteString("\n")
			} else {
				if len(matches) == 0 && (astResult == nil || (astResult.FoundSymbol == nil && len(astResult.Candidates) == 0)) {
					sb.WriteString(fmt.Sprintf("⚠️ **未找到「%s」** → 换词重试（同义词/缩写/驼峰变体），或用 `project_map` 先看结构\n", args.Query))
				}
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
