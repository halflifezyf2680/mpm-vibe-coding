package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func callTaskChainTool(t *testing.T, sm *SessionManager, args map[string]any) string {
	t.Helper()
	handler := wrapTaskChain(sm)
	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "task_chain",
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("task_chain call failed: %v", err)
	}
	return getTextResult(t, result)
}

func TestTaskChainTemplate_ListAndPreview(t *testing.T) {
	sm := &SessionManager{}

	listText := callTaskChainTool(t, sm, map[string]any{
		"mode": "template",
	})
	if !strings.Contains(listText, "develop") {
		t.Fatalf("template list should include develop: %s", listText)
	}
	if !strings.Contains(listText, "debug") {
		t.Fatalf("template list should include debug: %s", listText)
	}

	previewText := callTaskChainTool(t, sm, map[string]any{
		"mode":     "template",
		"template": "develop",
	})
	if !strings.Contains(previewText, "template 预览") {
		t.Fatalf("unexpected template preview output: %s", previewText)
	}
}

func TestTaskChainTemplate_StepInitAndStatus(t *testing.T) {
	sm := &SessionManager{}

	stepText := callTaskChainTool(t, sm, map[string]any{
		"mode":        "step",
		"task_id":     "TASK_TPL_001",
		"description": "测试模板初始化",
		"template":    "develop",
	})
	if !strings.Contains(stepText, "【Step 1 开始】") {
		t.Fatalf("expected step 1 start output, got: %s", stepText)
	}

	statusText := callTaskChainTool(t, sm, map[string]any{
		"mode":    "status",
		"task_id": "TASK_TPL_001",
	})
	if !strings.Contains(statusText, "in_progress") {
		t.Fatalf("status should contain in_progress: %s", statusText)
	}
}

func TestTaskChainTemplate_CustomFile_HotReloadAndOverride(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".mcp-config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	filePath := filepath.Join(configDir, "task_chain_templates.yaml")

	sm := &SessionManager{ProjectRoot: root}

	// 1) write custom template and override built-in develop
	first := `templates:
  - name: develop
    description: OVERRIDE develop
    steps:
      - name: D1
        input: "noop"
  - name: custom1
    description: Custom One
    steps:
      - name: C1
        input: "noop"
`
	if err := os.WriteFile(filePath, []byte(first), 0644); err != nil {
		t.Fatalf("write template file failed: %v", err)
	}

	list1 := callTaskChainTool(t, sm, map[string]any{"mode": "template"})
	if !strings.Contains(list1, "custom1") {
		t.Fatalf("list should include custom1: %s", list1)
	}
	if !strings.Contains(list1, "OVERRIDE develop") {
		t.Fatalf("list should reflect overridden develop: %s", list1)
	}

	// 2) hot reload: replace custom1 with custom2
	second := `templates:
  - name: custom2
    description: Custom Two
    steps:
      - name: X1
        input: "noop"
`
	if err := os.WriteFile(filePath, []byte(second), 0644); err != nil {
		t.Fatalf("rewrite template file failed: %v", err)
	}

	list2 := callTaskChainTool(t, sm, map[string]any{"mode": "template"})
	if !strings.Contains(list2, "custom2") {
		t.Fatalf("list should include custom2 after reload: %s", list2)
	}
	if strings.Contains(list2, "custom1") {
		t.Fatalf("list should not include custom1 after reload: %s", list2)
	}

	stepText := callTaskChainTool(t, sm, map[string]any{
		"mode":        "step",
		"task_id":     "TASK_CUSTOM_001",
		"description": "使用 custom2 初始化",
		"template":    "custom2",
	})
	if !strings.Contains(stepText, "【Step 1 开始】") || !strings.Contains(stepText, "X1") {
		t.Fatalf("expected custom template step start output, got: %s", stepText)
	}
}
