package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type taskChainTemplateStep struct {
	Name  string `json:"name" yaml:"name"`
	Input string `json:"input" yaml:"input"`
}

type taskChainTemplate struct {
	Name        string                  `json:"name" yaml:"name"`
	Description string                  `json:"description" yaml:"description"`
	Steps       []taskChainTemplateStep `json:"steps" yaml:"steps"`
}

func normalizeTemplateName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func getBuiltInTaskChainTemplates() []taskChainTemplate {
	return []taskChainTemplate{
		{
			Name:        "develop",
			Description: "功能开发/增强（定位→影响→实现→验证→memo）",
			Steps: []taskChainTemplateStep{
				{Name: "项目初始化与范围确认", Input: "initialize_project(project_root='<ABS_PROJECT_ROOT>')"},
				{Name: "定位核心符号/入口", Input: "code_search(query='<symbol>', scope='<scope>', search_type='any')"},
				{Name: "流程追踪（可选，快速理解主链路）", Input: "flow_trace(symbol_name='<symbol>', scope='<scope>', direction='both', mode='brief')"},
				{Name: "影响分析（改动前必做）", Input: "code_impact(symbol_name='<symbol>', direction='both')"},
				{Name: "实施改动（按约束最小化）", Input: ""},
				{Name: "验证与回归（跑测试/构建）", Input: "go test ./... / npm test / pytest"},
				{Name: "沉淀记录（SSOT）", Input: "memo(items=[...])"},
			},
		},
		{
			Name:        "debug",
			Description: "问题排查（复现→定位→缩小→修复→回归→memo）",
			Steps: []taskChainTemplateStep{
				{Name: "复现与收集证据（日志/堆栈/最小复现）", Input: ""},
				{Name: "定位相关符号/入口", Input: "code_search(query='<symbol_or_file>', scope='<scope>', search_type='any')"},
				{Name: "流程追踪/调用链（找关键分支）", Input: "flow_trace(symbol_name='<symbol>', scope='<scope>', direction='both', mode='standard')"},
				{Name: "影响分析（修复点外溢评估）", Input: "code_impact(symbol_name='<symbol>', direction='both')"},
				{Name: "修复并加回归测试", Input: ""},
				{Name: "验证（复现用例 + 全量/相关测试）", Input: "go test ./... / npm test / pytest"},
				{Name: "沉淀记录（SSOT）", Input: "memo(items=[...])"},
			},
		},
		{
			Name:        "refactor",
			Description: "重构（基线→锚点→影响→小步替换→验证→memo）",
			Steps: []taskChainTemplateStep{
				{Name: "项目初始化与范围确认", Input: "initialize_project(project_root='<ABS_PROJECT_ROOT>')"},
				{Name: "锚点定位（当前实现在哪里）", Input: "code_search(query='<symbol>', scope='<scope>', search_type='any')"},
				{Name: "影响分析（上游/下游）", Input: "code_impact(symbol_name='<symbol>', direction='both')"},
				{Name: "建立安全网（补测试/最小验证脚本）", Input: ""},
				{Name: "小步重构（每步可回退）", Input: ""},
				{Name: "验证与回归（跑测试/构建）", Input: "go test ./... / npm test / pytest"},
				{Name: "沉淀记录（SSOT）", Input: "memo(items=[...])"},
			},
		},
	}
}

func findTaskChainTemplate(templates []taskChainTemplate, name string) (taskChainTemplate, bool) {
	needle := normalizeTemplateName(name)
	for _, t := range templates {
		if normalizeTemplateName(t.Name) == needle {
			return t, true
		}
	}
	return taskChainTemplate{}, false
}

func buildPlanFromTemplate(sm *SessionManager, name string) ([]map[string]interface{}, error) {
	templates, _, err := getTaskChainTemplatesForSession(sm)
	if err != nil {
		return nil, err
	}

	tmpl, ok := findTaskChainTemplate(templates, name)
	if !ok {
		return nil, fmt.Errorf("未知 template: %s", name)
	}
	plan := make([]map[string]interface{}, 0, len(tmpl.Steps))
	for _, s := range tmpl.Steps {
		step := map[string]interface{}{"name": s.Name}
		if strings.TrimSpace(s.Input) != "" {
			step["input"] = s.Input
		}
		plan = append(plan, step)
	}
	return plan, nil
}

func renderTemplateList(sm *SessionManager) string {
	templates, sourceNote, err := getTaskChainTemplatesForSession(sm)
	var warn string
	if err != nil {
		warn = err.Error()
		templates = getBuiltInTaskChainTemplates()
		sourceNote = ""
	}
	// stable output
	sort.Slice(templates, func(i, j int) bool {
		return normalizeTemplateName(templates[i].Name) < normalizeTemplateName(templates[j].Name)
	})

	var sb strings.Builder
	sb.WriteString("### 🧩 可用 task_chain templates\n\n")
	for _, t := range templates {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}
	if strings.TrimSpace(sourceNote) != "" {
		sb.WriteString("\n" + sourceNote + "\n")
	}
	if strings.TrimSpace(warn) != "" {
		sb.WriteString("\n⚠️ 自定义模板加载失败：" + warn + "\n")
	}
	sb.WriteString("\n用法示例：\n\n")
	sb.WriteString("task_chain(mode=\"template\", template=\"develop\")\n")
	sb.WriteString("task_chain(mode=\"step\", task_id=\"TASK_001\", description=\"...\", template=\"develop\")\n")
	return sb.String()
}

func renderTemplatePreview(sm *SessionManager, name string) (*mcp.CallToolResult, error) {
	plan, err := buildPlanFromTemplate(sm, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(plan)
	b := bytes.TrimSpace(buf.Bytes())

	canon := normalizeTemplateName(name)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 🧩 template 预览: %s\n\n", canon))
	sb.WriteString("可直接复制作为 plan：\n\n")
	sb.WriteString(string(b))
	sb.WriteString("\n\n或直接用 template 初始化：\n\n")
	sb.WriteString("task_chain(mode=\"step\", task_id=\"TASK_001\", description=\"...\", template=\"" + canon + "\")\n")
	return mcp.NewToolResultText(sb.String()), nil
}
