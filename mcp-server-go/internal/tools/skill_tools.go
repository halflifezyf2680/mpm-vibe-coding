package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"
)

// SkillMetadata 技能元数据
type SkillMetadata struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Category    string   `yaml:"category" json:"category"`
	Trigger     []string `yaml:"trigger" json:"trigger"`
	Version     string   `yaml:"version" json:"version"`
}

// SkillEntry 技能分目
type SkillEntry struct {
	Metadata  SkillMetadata       `json:"metadata"`
	FilePath  string              `json:"file_path"`
	Resources map[string][]string `json:"resources"`
}

// SkillLoadArgs 加载技能参数
type SkillLoadArgs struct {
	Name     string `json:"name" jsonschema:"required,description=Skill 名称 (文件夹名或元数据中的 name)"`
	Level    string `json:"level" jsonschema:"default=standard,enum=standard,enum=full,description=加载级别"`
	Resource string `json:"resource" jsonschema:"description=可选。指定要加载的子资源路径 (如 references/guide.md)"`
	Refresh  bool   `json:"refresh" jsonschema:"description=是否强制刷新缓存"`
}

// RegisterSkillTools 注册技能库工具
func RegisterSkillTools(s *server.MCPServer, sm *SessionManager) {
	s.AddTool(mcp.NewTool("skill_list",
		mcp.WithDescription(`skill_list - 列出可用技能库 (领域知识)

用途：
  扫描并列出所有可用的技能（Skill）。技能是针对特定领域或任务的专家级指导文档。

参数：
  无

说明：
  - 会同时扫描项目本地和全局的技能目录。
  - 项目本地的技能会优先覆盖同名的全局技能。

示例：
  skill_list()
    -> 查看所有可用技能及其简要描述

触发词：
  "mpm 技能列表", "mpm skills"`),
	), wrapSkillList(sm))

	s.AddTool(mcp.NewTool("skill_load",
		mcp.WithDescription(`skill_load - 加载并阅读技能文档 (专家指导)

用途：
  【必备】在处理不熟悉的技术或领域时，加载对应的技能文档以获取专家的详细指导和最佳实践。

参数：
  name (必填)
    技能的名称或所在文件夹名。
  
  level (默认: standard)
    - standard: 加载标准摘要。
    - full: 加载完整元数据和详细内容。
  
  resource (可选)
    指定加载该技能下的特定子资源文件（如 references/manual.md）。
  
  refresh (默认: false)
    设为 true 以强制刷新技能缓存。

说明：
  - 加载技能后，LLM 必须仔细阅读其内容，严禁在阅读前采取任何实质行动。
  - 支持加载技能包内自带的脚本、参考文档或资源清单。

示例：
  skill_load(name="Refactoring", level="full")
    -> 获取重构专家的详细指导

触发词：
  "mpm 加载技能", "mpm skill", "mpm loadskill"`),
		mcp.WithInputSchema[SkillLoadArgs](),
	), wrapSkillLoad(sm))
}

var (
	skillCache []SkillEntry
	skillMap   map[string]*SkillEntry
)

func scanSkills(sm *SessionManager) error {
	if sm.ProjectRoot == "" {
		return fmt.Errorf("项目尚未初始化")
	}

	// 定义扫描源 (优先级从低到高：MPM安装目录 < ~/.mpm < 项目本地)
	// 先扫描优先级低的，后扫描的会覆盖 map 中的同名 Key
	paths := []string{}

	// 1. MPM 安装目录下的 skills/ (随 MPM 分发的官方 Skills)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// 可执行文件在 bin/ 下，skills 在 mcp-server-go/ 下
		mpmSkills := filepath.Join(exeDir, "..", "skills")
		paths = append(paths, mpmSkills)
	}

	// 2. 用户全局 Skills 目录 (~/.mpm/skills/)
	if home, err := os.UserHomeDir(); err == nil {
		globalSkills := filepath.Join(home, ".mpm", "skills")
		paths = append(paths, globalSkills)
	}

	// 3. 项目本地 Skills (优先级最高)
	projectLocal := filepath.Join(sm.ProjectRoot, "skills")
	paths = append(paths, projectLocal)

	// 临时 Map 用于去重和覆盖
	tempMap := make(map[string]SkillEntry)

	for _, root := range paths {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillDir := filepath.Join(root, entry.Name())
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				skillFile = filepath.Join(skillDir, "skill.md")
				if _, err := os.Stat(skillFile); os.IsNotExist(err) {
					continue
				}
			}

			content, err := os.ReadFile(skillFile)
			if err != nil {
				continue
			}

			// 移除 BOM
			if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
				content = content[3:]
			}

			meta := parseFrontmatter(string(content))
			if meta.Name == "" {
				meta.Name = entry.Name()
			}

			// 如果是 Global 路径，标记 Category
			if strings.Contains(root, "mcp-expert-server") && meta.Category == "global" {
				meta.Category = "global (shared)"
			}

			resources := scanResources(skillDir)

			skillEntry := SkillEntry{
				Metadata:  meta,
				FilePath:  skillFile,
				Resources: resources,
			}

			// 存入 Map (后扫描的 Local 会覆盖 Global)
			tempMap[meta.Name] = skillEntry
			// 同时支持目录名作为 Key (如果与 Name 不同)
			if entry.Name() != meta.Name {
				// 注意：如果目录名冲突，也会覆盖。这是预期的。
				// 但主要 Key 应该是 Name。
				// 这里为了保持 skill_load 能用目录名索引，我们稍微做点妥协
				// 只在 Name 不存在时才用目录名? 或者都存?
			}
		}
	}

	// 重建 Cache 和 最终 Map
	newCache := make([]SkillEntry, 0, len(tempMap))
	for _, v := range tempMap {
		newCache = append(newCache, v)
	}

	// 排序
	sort.Slice(newCache, func(i, j int) bool {
		return newCache[i].Metadata.Name < newCache[j].Metadata.Name
	})

	// 重建索引 Map (指向 Cache 中的稳定地址)
	newMap := make(map[string]*SkillEntry)
	for i := range newCache {
		entry := &newCache[i]
		newMap[entry.Metadata.Name] = entry

		// 尝试从路径获取目录名，也作为索引
		dirName := filepath.Base(filepath.Dir(entry.FilePath))
		if dirName != entry.Metadata.Name {
			newMap[dirName] = entry
		}
	}

	skillCache = newCache
	skillMap = newMap
	return nil
}

func parseFrontmatter(content string) SkillMetadata {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	match := re.FindStringSubmatch(content)
	var meta SkillMetadata
	if len(match) > 1 {
		yaml.Unmarshal([]byte(match[1]), &meta)
	}

	// 如果没有描述，尝试从正文提取第一段
	if meta.Description == "" {
		body := re.ReplaceAllString(content, "")
		paras := strings.Split(strings.TrimSpace(body), "\n\n")
		if len(paras) > 0 {
			desc := paras[0]
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			meta.Description = desc
		}
	}

	if meta.Category == "" {
		meta.Category = "global"
	}
	return meta
}

func scanResources(dir string) map[string][]string {
	res := make(map[string][]string)
	subDirs := []string{"references", "scripts", "assets"}
	for _, sub := range subDirs {
		p := filepath.Join(dir, sub)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			files, _ := os.ReadDir(p)
			for _, f := range files {
				if !f.IsDir() {
					res[sub] = append(res[sub], f.Name())
				}
			}
		}
	}
	return res
}

func wrapSkillList(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if skillCache == nil {
			if err := scanSkills(sm); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("扫描技能库失败: %v", err)), nil
			}
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("#### 发现 %d 个可用技能\n\n", len(skillCache)))
		for _, s := range skillCache {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Metadata.Name, s.Metadata.Description))
		}
		sb.WriteString("\n> 使用 `skill_load(name=\"...\")` 加载完整内容。")

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func wrapSkillLoad(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args SkillLoadArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if args.Refresh || skillCache == nil {
			if err := scanSkills(sm); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("扫描技能库失败: %v", err)), nil
			}
		}

		entry, ok := skillMap[args.Name]
		if !ok {
			// 简单的模糊匹配建议
			var suggestions []string
			for k := range skillMap {
				if strings.Contains(strings.ToLower(k), strings.ToLower(args.Name)) {
					suggestions = append(suggestions, k)
				}
			}
			msg := fmt.Sprintf("未匹配到技能 \"%s\"。", args.Name)
			if len(suggestions) > 0 {
				msg += fmt.Sprintf(" 你是不是想找: %s?", strings.Join(suggestions, ", "))
			}
			return mcp.NewToolResultText(msg), nil
		}

		skillDir := filepath.Dir(entry.FilePath)

		// 情况 1: 加载子资源
		if args.Resource != "" {
			// 安全检查
			targetPath := filepath.Join(skillDir, args.Resource)
			absTarget, _ := filepath.Abs(targetPath)
			absSkillDir, _ := filepath.Abs(skillDir)

			if !strings.HasPrefix(absTarget, absSkillDir) {
				return mcp.NewToolResultError("禁止访问技能目录外的资源"), nil
			}

			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				// 智能回退
				for _, sub := range []string{"references", "scripts", "assets"} {
					p := filepath.Join(skillDir, sub, filepath.Base(args.Resource))
					if _, err := os.Stat(p); err == nil {
						targetPath = p
						break
					}
				}
			}

			content, err := os.ReadFile(targetPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("无法加载资源: %s", args.Resource)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("### Resource: %s\n\n%s", args.Resource, string(content))), nil
		}

		// 情况 2: 加载主文档
		content, err := os.ReadFile(entry.FilePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("读取技能文件失败: %v", err)), nil
		}

		body := regexp.MustCompile(`(?s)^---\s*\n.*?\n---\s*\n`).ReplaceAllString(string(content), "")

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Skill: %s\n\n", entry.Metadata.Name))

		if args.Level == "full" {
			fm, _ := json.MarshalIndent(entry.Metadata, "", "  ")
			sb.WriteString("```json\n")
			sb.WriteString(string(fm))
			sb.WriteString("\n```\n\n")
		}

		sb.WriteString(body)

		if len(entry.Resources) > 0 {
			sb.WriteString("\n\n## 可用资源 (Bundled Resources)\n")
			keys := make([]string, 0, len(entry.Resources))
			for k := range entry.Resources {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				v := entry.Resources[k]
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", k, strings.Join(v, ", ")))
			}
			sb.WriteString("\n> 若需加载子资源，请使用 `skill_load(name=\"...\", resource=\"references/xxx.md\")`。")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func (sm *SessionManager) GetSkillContent(name string) (string, error) {
	if skillMap == nil {
		scanSkills(sm)
	}
	entry, ok := skillMap[name]
	if !ok {
		return "", fmt.Errorf("skill not found")
	}
	content, err := os.ReadFile(entry.FilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
