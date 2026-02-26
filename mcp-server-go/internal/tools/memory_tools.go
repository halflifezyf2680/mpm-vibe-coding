package tools

import (
	"context"
	"fmt"
	"mcp-server-go/internal/core"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MemoItem 定义了录入事项的结构
type MemoItem struct {
	Category string `json:"category" jsonschema:"description=分类 (如：修改、开发、决策)，必须使用用户对话语言"`
	Entity   string `json:"entity" jsonschema:"description=改动的实体，必须使用用户对话语言"`
	Act      string `json:"act" jsonschema:"description=具体的行动，必须使用用户对话语言"`
	Path     string `json:"path" jsonschema:"description=文件路径"`
	Content  string `json:"content" jsonschema:"description=详细内容，必须使用用户对话语言"`
	Key      string `json:"key,omitempty" jsonschema:"description=兼容字段：键"`
	Value    string `json:"value,omitempty" jsonschema:"description=兼容字段：值"`
}

// MemoArgs 备忘录参数
type MemoArgs struct {
	Items []MemoItem `json:"items" jsonschema:"required,description=录入事项列表"`
	Lang  string     `json:"lang" jsonschema:"enum=zh,enum=en,default=zh,description=当前用户对话的语言 (zh=中文, en=英文)"`
}

// RegisterMemoryTools 注册备忘与检索工具
func RegisterMemoryTools(s *server.MCPServer, sm *SessionManager) {
	s.AddTool(mcp.NewTool("memo",
		mcp.WithDescription(`memo - 项目的"黑匣子" (如果不记，等于没做)

用途：
  【修改后必选】任何代码/文档修改后，严禁不留记录直接结束。
  这不仅是给用户看的，更是为了你自己以后能检索到 "当时为什么这么改"。它是项目演进的唯一真理源 (SSOT)。

参数：
  items (必填 - JSON 数组):
    ⚠️ 注意：items 本身就是一个数组，即使只记录一条也要用 [{...}] 包裹
    
    每个数组元素包含以下字段（全部必填）：
    - category: 分类，如 "修改"、"开发"、"决策"、"重构"、"避坑"
    - entity: 改动的实体（文件名、函数名、模块名）
    - act: 简要行为描述，如 "修复Bug"、"新增功能"、"技术选型"
    - path: 文件路径
    - content: 详细说明，解释"为什么这么改"而非只说"改了什么"
  
  lang (可选，默认 zh): 
    记录语言，建议始终使用中文

完整调用示例（JSON格式）：
  {
    "items": [
      {
        "category": "修改",
        "entity": "SessionManager",
        "act": "修复空指针异常",
        "path": "core/session.go",
        "content": "添加 nil 检查，防止未初始化的配置导致 panic"
      }
    ],
    "lang": "zh"
  }

触发词：
  "mpm memo", "mpm 记录", "mpm 存档"`),
		mcp.WithInputSchema[MemoArgs](),
	), wrapMemo(sm))

	// 注：known_facts 已在 RegisterIntelligenceTools 中注册,此处删除重复注册
}

func wrapMemo(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if sm.Memory == nil {
			return mcp.NewToolResultError("记忆层尚未初始化，请先执行 initialize_project 任务。"), nil
		}
		var args MemoArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数格式错误： %v", err)), nil
		}

		// 根据语种判定本地化术语
		txtSystem := "System"
		txtInfo := "Info"
		txtManual := "Manual Entry"

		if args.Lang == "zh" {
			txtSystem = "系统"
			txtInfo = "信息"
			txtManual = "手动录入"
		}

		var memos []core.Memo
		for _, item := range args.Items {
			memo := core.Memo{
				Category: fallback(item.Category, "开发"),
				Path:     fallback(item.Path, "-"),
				Content:  item.Content,
			}

			// 智取实体名
			ent := item.Entity
			if ent == "" || ent == "-" {
				ent = item.Key
			}
			if ent == "" || ent == "-" {
				c := fallback(item.Content, item.Value)
				lines := strings.Split(c, "\n")
				if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
					ent = strings.TrimSpace(lines[0])
				} else {
					ent = txtSystem
				}
			}
			memo.Entity = ent

			// 智取行动名
			act := item.Act
			if act == "" || act == "-" {
				if item.Key != "" {
					act = txtInfo
				} else {
					act = txtManual
				}
			}
			memo.Act = act

			memos = append(memos, memo)
		}

		ids, err := sm.Memory.AddMemos(ctx, memos)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("保存备忘录失败： %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("已成功录入 %d 条记录 (IDs: %v)。", len(ids), ids)), nil
	}
}

func fallback(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
