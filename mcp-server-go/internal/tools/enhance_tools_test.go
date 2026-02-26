package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func getTextResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatalf("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatalf("result content is empty")
	}
	text, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("result content is not text")
	}
	return text.Text
}

func callPersonaTool(t *testing.T, sm *SessionManager, args map[string]any) string {
	t.Helper()
	handler := wrapPersona(sm)
	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "persona",
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("persona call failed: %v", err)
	}
	return getTextResult(t, result)
}

func TestPersonaLifecycle_CreateActivateUpdateDelete(t *testing.T) {
	root := t.TempDir()
	sm := &SessionManager{ProjectRoot: root}

	createText := callPersonaTool(t, sm, map[string]any{
		"mode":           "create",
		"name":           "arch_guard",
		"display_name":   "架构守卫",
		"hard_directive": "回答时先给风险评估再给方案",
		"aliases":        []string{"guard", "守卫"},
	})
	if !strings.Contains(createText, "已创建人格") {
		t.Fatalf("unexpected create output: %s", createText)
	}

	configPath := filepath.Join(root, ".mcp-config", "personas.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("persona config should exist after create: %v", err)
	}

	activateText := callPersonaTool(t, sm, map[string]any{
		"mode": "activate",
		"name": "arch_guard",
	})
	if !strings.Contains(activateText, "人格已激活") {
		t.Fatalf("unexpected activate output: %s", activateText)
	}

	updateText := callPersonaTool(t, sm, map[string]any{
		"mode":           "update",
		"name":           "arch_guard",
		"new_name":       "arch_keeper",
		"display_name":   "架构卫士",
		"hard_directive": "先列假设，再给结论",
	})
	if !strings.Contains(updateText, "已更新人格") {
		t.Fatalf("unexpected update output: %s", updateText)
	}

	activateByAliasText := callPersonaTool(t, sm, map[string]any{
		"mode": "activate",
		"name": "guard",
	})
	if !strings.Contains(activateByAliasText, "arch_keeper") {
		t.Fatalf("activate by alias should hit renamed persona: %s", activateByAliasText)
	}

	deleteText := callPersonaTool(t, sm, map[string]any{
		"mode": "delete",
		"name": "arch_keeper",
	})
	if !strings.Contains(deleteText, "已删除人格") {
		t.Fatalf("unexpected delete output: %s", deleteText)
	}

	listText := callPersonaTool(t, sm, map[string]any{
		"mode": "list",
	})
	if strings.Contains(listText, "arch_keeper") {
		t.Fatalf("deleted persona should not appear in list: %s", listText)
	}
}
