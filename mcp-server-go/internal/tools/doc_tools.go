package tools

import (
	"context"
	"fmt"
	"mcp-server-go/internal/services"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// WikiWriterArgs Wiki 写作参数（简化版）
type WikiWriterArgs struct {
	OutputFile string `json:"output_file,omitempty" jsonschema:"default=wiki_outline.md,description=输出文件名"`
	Style      string `json:"style,omitempty" jsonschema:"description=书写风格（technical/tutorial/reference/blog 或自定义要求）"`
}

// RegisterDocTools 注册文档工具
func RegisterDocTools(s *server.MCPServer, sm *SessionManager, ai *services.ASTIndexer) {
	s.AddTool(mcp.NewTool("wiki_writer",
		mcp.WithDescription(`wiki_writer - Wiki 大纲生成工具

用途：
  为项目生成 Wiki 文档大纲和章节规划，支持自定义书写风格。

参数：
  output_file (可选)
    输出文件名，默认 wiki_outline.md

  style (可选)
    书写风格：
    - technical: 技术文档风格（简洁专业）
    - tutorial: 教程指南风格（循序渐进）
    - reference: 参考资料风格（详细完整）
    - blog: 博客风格（轻松活泼）
    - 或直接输入自定义要求

工作流程：
  1. 获取项目地图作为参考资料
  2. LLM 自主探索代码生成大纲
  3. 询问用户选择书写风格
  4. 生成书写指南附加到文档末尾

示例：
  wiki_writer()
  wiki_writer(output_file="MPM_Wiki.md", style="technical")
  wiki_writer(style="面向新手，多用 emoji，代码要详细注释")

触发词：
  "mpm wiki", "mpm 文档"`),
		mcp.WithInputSchema[WikiWriterArgs](),
	), wrapWikiWriter(sm, ai))
}

func wrapWikiWriter(sm *SessionManager, ai *services.ASTIndexer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args WikiWriterArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		// 设置默认输出文件
		outputFile := args.OutputFile
		if outputFile == "" {
			outputFile = "wiki_outline.md"
		}

		// 1. 调用 project_map 生成地图作为参考资料
		mapResult, err := ai.MapProjectWithScope(sm.ProjectRoot, "symbols", "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("生成项目地图失败: %v", err)), nil
		}

		// 2. 读取生成的 map 文件内容（完整内容）
		mapFile := filepath.Join(sm.ProjectRoot, ".mcp-data", "project_map_symbols.md")
		mapContent, _ := os.ReadFile(mapFile)
		mapContentStr := string(mapContent)

		// 3. 返回生成指引
		var sb strings.Builder
		sb.WriteString("══════════════════════════════════════════════════════════════\n")
		sb.WriteString("                    【Wiki 大纲生成】\n")
		sb.WriteString("══════════════════════════════════════════════════════════════\n\n")

		sb.WriteString("## 📋 参考资料（项目地图）\n\n")
		if len(mapContentStr) > 10000 {
			// 内容太长，显示摘要和文件路径
			sb.WriteString(fmt.Sprintf("> 📄 完整地图：`.mcp-data/project_map_symbols.md` (%d 字符)\n\n", len(mapContentStr)))
			sb.WriteString("**摘要**：\n\n")
			sb.WriteString(formatMapResult(mapResult))
			sb.WriteString("\n\n")
		} else {
			// 内容适中，直接显示
			sb.WriteString(mapContentStr)
			sb.WriteString("\n\n")
		}
		sb.WriteString("---\n\n")

		sb.WriteString("## ✍️ 你的任务\n\n")
		sb.WriteString("基于上述项目地图，生成一套完整的 Wiki 大纲和章节规划文档。\n\n")
		sb.WriteString("**要求**：\n")
		sb.WriteString("- 用最流行的方式组织章节\n")
		sb.WriteString("- 可以自主使用 code_search 查找符号\n")
		sb.WriteString("- 可以使用 Read 阅读具体实现\n")
		sb.WriteString("- 输出完整的大纲文档\n\n")

		sb.WriteString("## 🎨 书写风格选择\n\n")
		sb.WriteString("**预置模板**（输入数字或名称）：\n")
		sb.WriteString("- `1` 或 `technical` → 技术文档风格（简洁专业）\n")
		sb.WriteString("- `2` 或 `tutorial` → 教程指南风格（循序渐进）\n")
		sb.WriteString("- `3` 或 `reference` → 参考资料风格（详细完整）\n")
		sb.WriteString("- `4` 或 `blog` → 博客风格（轻松活泼）\n\n")
		sb.WriteString("**自定义要求**：\n")
		sb.WriteString("- 直接输入你的风格要求\n")
		sb.WriteString("- 例如：\"面向新手，多用 emoji，代码要详细注释\"\n\n")

		if args.Style != "" {
			sb.WriteString(fmt.Sprintf("---\n\n**当前选择**：%s\n\n", args.Style))
			sb.WriteString("**生成的书写指南**：\n\n")
			sb.WriteString(generateStyleGuide(args.Style))
		}

		sb.WriteString(fmt.Sprintf("\n---\n\n💾 **保存到**：`%s`\n", outputFile))

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// formatMapResult 格式化地图结果
func formatMapResult(mapResult *services.MapResult) string {
	if mapResult == nil {
		return "项目地图获取失败"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**📊 统计**: %d 文件 | %d 符号\n\n",
		mapResult.Statistics.TotalFiles, mapResult.Statistics.TotalSymbols))
	sb.WriteString("**📁 目录结构**:\n\n")

	// 按目录组织显示
	for dir, nodes := range mapResult.Structure {
		sb.WriteString(fmt.Sprintf("- `%s/` (%d 符号)\n", dir, len(nodes)))
	}

	return sb.String()
}

// generateStyleGuide 生成书写指南
func generateStyleGuide(style string) string {
	// 预置模板
	templates := map[string]string{
		"technical": `
# Wiki 书写指南 - 技术文档风格

## 语言风格
- 简洁准确，避免冗余
- 使用专业术语
- 保持客观中立

## 写作手法
- 章节结构清晰，层次分明
- 代码示例带注释
- 重点内容加粗强调

## 格式要求
- Markdown 格式
- 代码块指定语言
- 标题层级规范
`,
		"tutorial": `
# Wiki 书写指南 - 教程风格

## 语言风格
- 循序渐进，从简单到复杂
- 语言通俗，适合新手
- 多用示例和类比

## 写作手法
- 每个概念配示例
- 使用图示辅助说明
- 分步骤详细讲解

## 格式要求
- 代码块完整可运行
- 多用 Mermaid 流程图
- 步骤编号清晰
`,
		"reference": `
# Wiki 书写指南 - 参考资料风格

## 语言风格
- 详细完整，全面覆盖
- 准确描述每个细节
- 保持结构一致

## 写作手法
- 按功能模块组织
- 每个函数/接口独立说明
- 提供参数详解

## 格式要求
- 表格展示参数列表
- 代码签名完整
- 交叉引用相关内容
`,
		"blog": `
# Wiki 书写指南 - 博客风格

## 语言风格
- 轻松活泼，富有感染力
- 使用 emoji 增强可读性
- 讲故事的方式

## 写作手法
- 多用实例和生活类比
- 使用引用突出金句
- 图文并茂

## 格式要求
- 适当使用 emoji
- 引用块强调重点
- 图片配说明
`,
	}

	// 检查是否是预置模板
	if tpl, ok := templates[strings.ToLower(style)]; ok {
		return tpl
	}

	// 自定义要求，与默认模板融合
	return fmt.Sprintf(`
# Wiki 书写指南（个性化定制）

## 用户要求
%s

---

## 基础规范（与默认模板融合）
- 简洁准确，避免冗余
- 代码示例带注释
- 章节结构清晰

**注**：以上要求已与默认写作规范融合，确保文档质量与个性化需求的平衡。
`, style)
}
