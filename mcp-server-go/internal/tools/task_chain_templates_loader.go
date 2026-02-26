package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type taskChainTemplateFile struct {
	Templates []taskChainTemplate `json:"templates" yaml:"templates"`
}

func getTaskChainTemplateFileCandidates(projectRoot string) []string {
	if strings.TrimSpace(projectRoot) == "" {
		return nil
	}
	base := filepath.Join(projectRoot, ".mcp-config")
	return []string{
		filepath.Join(base, "task_chain_templates.yaml"),
		filepath.Join(base, "task_chain_templates.yml"),
		filepath.Join(base, "task_chain_templates.json"),
	}
}

func resolveTaskChainTemplateFile(projectRoot string) (string, bool) {
	for _, p := range getTaskChainTemplateFileCandidates(projectRoot) {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func parseTaskChainTemplates(data []byte) ([]taskChainTemplate, error) {
	var wrapper taskChainTemplateFile
	if err := yaml.Unmarshal(data, &wrapper); err == nil {
		if len(wrapper.Templates) > 0 {
			return wrapper.Templates, nil
		}
	}

	var list []taskChainTemplate
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("模板文件为空")
	}
	return list, nil
}

func validateTaskChainTemplates(templates []taskChainTemplate) error {
	for i, t := range templates {
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("template[%d].name 不能为空", i)
		}
		if len(t.Steps) == 0 {
			return fmt.Errorf("template[%s].steps 不能为空", t.Name)
		}
		for j, s := range t.Steps {
			if strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("template[%s].steps[%d].name 不能为空", t.Name, j)
			}
		}
	}
	return nil
}

func loadTaskChainTemplatesFromFile(filePath string) ([]taskChainTemplate, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	templates, err := parseTaskChainTemplates(data)
	if err != nil {
		return nil, err
	}

	if err := validateTaskChainTemplates(templates); err != nil {
		return nil, err
	}

	return templates, nil
}

// getTaskChainTemplatesForSession returns merged templates (built-in + optional project overrides).
// It loads from disk on every call so editing the file takes effect immediately without restarting MCP.
func getTaskChainTemplatesForSession(sm *SessionManager) ([]taskChainTemplate, string, error) {
	builtins := getBuiltInTaskChainTemplates()
	root := ""
	if sm != nil {
		root = strings.TrimSpace(sm.ProjectRoot)
	}
	if root == "" {
		return builtins, "", nil
	}

	defaultHint := fmt.Sprintf("可选自定义模板文件: %s (保存后立即生效，无需重启)", filepath.Join(root, ".mcp-config", "task_chain_templates.yaml"))

	filePath, ok := resolveTaskChainTemplateFile(root)
	if !ok {
		return builtins, defaultHint, nil
	}

	custom, err := loadTaskChainTemplatesFromFile(filePath)
	if err != nil {
		return builtins, defaultHint, fmt.Errorf("%s: %w", filePath, err)
	}

	merged := make(map[string]taskChainTemplate, len(builtins)+len(custom))
	for _, t := range builtins {
		merged[normalizeTemplateName(t.Name)] = t
	}
	for _, t := range custom {
		merged[normalizeTemplateName(t.Name)] = t
	}

	out := make([]taskChainTemplate, 0, len(merged))
	for _, t := range merged {
		out = append(out, t)
	}

	note := fmt.Sprintf("自定义模板已加载: %s (编辑保存后立即生效，无需重启)", filePath)
	return out, note, nil
}
