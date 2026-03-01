package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PersonaArgs 人格管理参数
type PersonaArgs struct {
	Mode           string   `json:"mode" jsonschema:"default=list,enum=list,enum=activate,enum=create,enum=update,enum=delete,description=操作模式"`
	Name           string   `json:"name" jsonschema:"description=人格名称 (activate/update/delete 必填)"`
	NewName        string   `json:"new_name" jsonschema:"description=新名称 (update 可选)"`
	DisplayName    string   `json:"display_name" jsonschema:"description=显示名称"`
	Avatar         string   `json:"avatar" jsonschema:"description=头像或图标"`
	HardDirective  string   `json:"hard_directive" jsonschema:"description=核心指令"`
	Aliases        []string `json:"aliases" jsonschema:"description=别名列表"`
	StyleMust      []string `json:"style_must" jsonschema:"description=必须遵守风格"`
	StyleSignature []string `json:"style_signature" jsonschema:"description=标志性表达"`
	StyleTaboo     []string `json:"style_taboo" jsonschema:"description=禁用表达"`
	Triggers       []string `json:"triggers" jsonschema:"description=触发词"`
}

// RegisterEnhanceTools 注册增强工具
func RegisterEnhanceTools(s *server.MCPServer, sm *SessionManager) {
	s.AddTool(mcp.NewTool("persona",
		mcp.WithDescription(`persona - AI 人格管理工具

用途：
  切换或列出可用的 AI 人格（角色）。通过改变语气、回复风格和思维协议，提升交互体验或特定场景的处理效率。

参数：
  mode (默认: list)
    - list: 列出所有可用的预设人格。
    - activate: 激活指定的人格。
    - create: 新增人格（写入 .mcp-config/personas.json）。
    - update: 更新人格（支持重命名）。
    - delete: 删除人格。
  
  name (activate/update/delete 模式必填)
    目标人格名称或别名。

自然语言触发示例：
  - "激活人格 孔明"
  - "切换到白起人格"
  - "列出所有人格"
  - "创建人格 xxx"
  - "删除人格 xxx"

  create/update 可选字段:
    - new_name, display_name, hard_directive, aliases
    - style_must, style_signature, style_taboo, triggers

说明：
  - 激活人格后，LLM 将严格遵守该角色的语言特征和指令。
  - 常驻角色包括诸葛（孔明）、懂王（特朗普）、哆啦（哆啦 A 梦）等。
  - 建议在对话中展示简要结果（如已激活人格名称），避免输出冗长内部提示文本。

示例：
  persona(mode="activate", name="zhuge")
    -> 切换到孔明人格，使用文言文风格响应

  persona(mode="create", name="my_architect", display_name="架构师", hard_directive="回答要简洁严谨")
    -> 新增自定义人格

触发词：
  "mpm 人格", "mpm persona", "激活人格", "切换人格", "切换到.*人格", "列出人格", "创建人格", "删除人格"`),
		mcp.WithInputSchema[PersonaArgs](),
	), wrapPersona(sm))
}

// PersonaData 人格数据
type PersonaData struct {
	Name           string   `json:"name"`
	DisplayName    string   `json:"display_name"`
	Avatar         string   `json:"avatar"`
	HardDirective  string   `json:"hard_directive"`
	StyleMust      []string `json:"style_must"`
	StyleSignature []string `json:"style_signature"`
	StyleTaboo     []string `json:"style_taboo"`
	Aliases        []string `json:"aliases"`
	Triggers       []string `json:"triggers"`
}

type PersonaLibrary struct {
	Personas []PersonaData `json:"personas"`
}

func normalizePersonaKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func findPersonaIndex(library *PersonaLibrary, key string) int {
	norm := normalizePersonaKey(key)
	if norm == "" {
		return -1
	}

	for i := range library.Personas {
		p := &library.Personas[i]
		if normalizePersonaKey(p.Name) == norm || normalizePersonaKey(p.DisplayName) == norm {
			return i
		}
		for _, alias := range p.Aliases {
			if normalizePersonaKey(alias) == norm {
				return i
			}
		}
	}

	return -1
}

func personaOneLineIntro(p PersonaData) string {
	intro := strings.TrimSpace(p.HardDirective)
	if intro == "" {
		return "通用问题求解与任务执行"
	}

	separators := []string{"。", "!", "！", "?", "？", ";", "；"}
	for _, sep := range separators {
		if idx := strings.Index(intro, sep); idx > 0 {
			intro = strings.TrimSpace(intro[:idx])
			break
		}
	}

	if len([]rune(intro)) > 28 {
		r := []rune(intro)
		intro = string(r[:28]) + "..."
	}

	return intro
}

func personaDisplayName(p PersonaData) string {
	if strings.TrimSpace(p.DisplayName) != "" {
		return strings.TrimSpace(p.DisplayName)
	}
	return strings.TrimSpace(p.Name)
}

func savePersonaLibrary(sm *SessionManager, library *PersonaLibrary) error {
	path := resolveWritablePersonaPath(sm)
	if path == "" {
		return fmt.Errorf("项目未初始化，无法持久化人格库")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0644)
}

func readPersonaLibrary(path string) (*PersonaLibrary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lib PersonaLibrary
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, err
	}
	if len(lib.Personas) == 0 {
		return nil, fmt.Errorf("empty persona library")
	}

	return &lib, nil
}

func mergePersonaLibraries(base *PersonaLibrary, overlay *PersonaLibrary) *PersonaLibrary {
	if base == nil && overlay == nil {
		return &PersonaLibrary{}
	}
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	merged := &PersonaLibrary{Personas: []PersonaData{}}
	index := make(map[string]bool)

	for _, p := range base.Personas {
		key := normalizePersonaKey(p.Name)
		index[key] = true
		merged.Personas = append(merged.Personas, p)
	}

	for _, p := range overlay.Personas {
		key := normalizePersonaKey(p.Name)
		if !index[key] {
			index[key] = true
			merged.Personas = append(merged.Personas, p)
		}
	}

	return merged
}

func globalPersonaCandidates(sm *SessionManager) []string {
	var candidates []string

	if sm != nil && sm.ProjectRoot != "" {
		candidates = append(candidates,
			filepath.Join(sm.ProjectRoot, "mcp-server-go", "configs", "persona_library.json"),
			filepath.Join(sm.ProjectRoot, "configs", "persona_library.json"),
		)
	}

	if ex, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(ex)
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "configs", "persona_library.json"),
			filepath.Join(exeDir, "configs", "persona_library.json"),
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "mcp-server-go", "configs", "persona_library.json"),
			filepath.Join(cwd, "configs", "persona_library.json"),
		)
	}

	seen := make(map[string]bool)
	uniq := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" {
			continue
		}
		clean := filepath.Clean(c)
		if !seen[clean] {
			seen[clean] = true
			uniq = append(uniq, clean)
		}
	}

	return uniq
}

func resolveWritablePersonaPath(sm *SessionManager) string {
	for _, p := range globalPersonaCandidates(sm) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	if sm != nil && sm.ProjectRoot != "" {
		return filepath.Join(sm.ProjectRoot, ".mcp-config", "personas.json")
	}

	return ""
}

func wrapPersona(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args PersonaArgs
		request.BindArguments(&args)

		if args.Mode == "" {
			if strings.TrimSpace(args.Name) != "" {
				args.Mode = "activate"
			} else {
				args.Mode = "list"
			}
		}

		// 加载人格库 (支持自定义 + 内建回退)
		library, err := loadPersonaLibrary(sm)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("加载人格库失败: %v", err)), nil
		}

		if args.Mode == "list" {
			var sb strings.Builder
			sb.WriteString("[RELAY_REQUIRED] 请将以下内容直接输出到对话框，不要省略（若用户主要语言并非中文，请在输出前自动将其翻译为用户的语言）。\n")
			sb.WriteString(fmt.Sprintf("可用人格列表（共 %d 个）\n", len(library.Personas)))
			for i, p := range library.Personas {
				sb.WriteString(fmt.Sprintf("%d. %s (%s): %s\n", i+1, personaDisplayName(p), p.Name, personaOneLineIntro(p)))
				if len(p.Aliases) > 0 {
					sb.WriteString(fmt.Sprintf("  别名: %s\n", strings.Join(p.Aliases, ", ")))
				}
			}
			sb.WriteString("\n可直接说：激活人格 <名称>\n")
			return mcp.NewToolResultText(sb.String()), nil
		}

		if args.Mode == "activate" {
			if args.Name == "" {
				return mcp.NewToolResultError("activate 模式需要提供 name 参数"), nil
			}

			idx := findPersonaIndex(library, args.Name)
			if idx < 0 {
				var available []string
				for _, p := range library.Personas {
					available = append(available, p.Name)
				}
				return mcp.NewToolResultText(fmt.Sprintf("未找到人格 '%s'。可用人格: %s", args.Name, strings.Join(available, ", "))), nil
			}
			target := &library.Personas[idx]

			// 写入系统状态
			if sm.Memory != nil {
				_ = sm.Memory.SaveState(ctx, "active_persona", target.Name, "persona")
			}

			llmDirective := buildPersonaDNA(target)
			visible := fmt.Sprintf("[RELAY_REQUIRED] 请将以下内容直接输出到对话框，不要省略（若用户主要语言并非中文，请在输出前自动将其翻译为用户的语言）。\n🎭 人格已激活：%s (%s)\n> %s\n\n[POST_ACTIVATE_BEHAVIOR] 请立刻以该人格语气再回复用户一句“已到位”的开场白（1 句话）。\n\n[HIDDEN_SYSTEM_DIRECTIVE_DO_NOT_OUTPUT_TO_USER_BUT_FOLLOW_IT STRICTLY]\n%s",
				personaDisplayName(*target), target.Name, target.HardDirective, llmDirective)

			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(visible)},
				StructuredContent: map[string]any{
					"type":                         "persona_activation",
					"persona_name":                 target.Name,
					"persona_display":              personaDisplayName(*target),
					"llm_instruction":              llmDirective,
					"post_activate_reply_required": true,
					"post_activate_reply_prompt":   "请立刻以当前人格语气向用户说一句到位开场白（仅一句）。",
					"activation_notice":            "从现在开始，请在后续回复中遵循该人格的语言与风格设定；仅改变表达风格，不得污染代码、日志与命令输出。",
				},
			}, nil
		}

		if args.Mode == "create" {
			if sm.ProjectRoot == "" {
				return mcp.NewToolResultError("create 模式需要先 initialize_project"), nil
			}
			if strings.TrimSpace(args.Name) == "" {
				return mcp.NewToolResultError("create 模式需要提供 name"), nil
			}
			if findPersonaIndex(library, args.Name) >= 0 {
				return mcp.NewToolResultError(fmt.Sprintf("人格 '%s' 已存在", args.Name)), nil
			}

			displayName := strings.TrimSpace(args.DisplayName)
			if displayName == "" {
				displayName = strings.TrimSpace(args.Name)
			}

			hardDirective := strings.TrimSpace(args.HardDirective)
			if hardDirective == "" {
				hardDirective = "回答保持专业、准确、简洁。"
			}

			library.Personas = append(library.Personas, PersonaData{
				Name:           strings.TrimSpace(args.Name),
				DisplayName:    displayName,
				Avatar:         strings.TrimSpace(args.Avatar),
				HardDirective:  hardDirective,
				Aliases:        args.Aliases,
				StyleMust:      args.StyleMust,
				StyleSignature: args.StyleSignature,
				StyleTaboo:     args.StyleTaboo,
				Triggers:       args.Triggers,
			})

			if err := savePersonaLibrary(sm, library); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("保存人格库失败: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✅ 已创建人格: %s", args.Name)), nil
		}

		if args.Mode == "update" {
			if sm.ProjectRoot == "" {
				return mcp.NewToolResultError("update 模式需要先 initialize_project"), nil
			}
			if strings.TrimSpace(args.Name) == "" {
				return mcp.NewToolResultError("update 模式需要提供 name"), nil
			}

			idx := findPersonaIndex(library, args.Name)
			if idx < 0 {
				return mcp.NewToolResultError(fmt.Sprintf("未找到人格: %s", args.Name)), nil
			}
			p := &library.Personas[idx]

			if strings.TrimSpace(args.NewName) != "" {
				if exists := findPersonaIndex(library, args.NewName); exists >= 0 && exists != idx {
					return mcp.NewToolResultError(fmt.Sprintf("新名称冲突: %s", args.NewName)), nil
				}
				p.Name = strings.TrimSpace(args.NewName)
			}
			if strings.TrimSpace(args.DisplayName) != "" {
				p.DisplayName = strings.TrimSpace(args.DisplayName)
			}
			if strings.TrimSpace(args.Avatar) != "" {
				p.Avatar = strings.TrimSpace(args.Avatar)
			}
			if strings.TrimSpace(args.HardDirective) != "" {
				p.HardDirective = strings.TrimSpace(args.HardDirective)
			}
			if len(args.Aliases) > 0 {
				p.Aliases = args.Aliases
			}
			if len(args.StyleMust) > 0 {
				p.StyleMust = args.StyleMust
			}
			if len(args.StyleSignature) > 0 {
				p.StyleSignature = args.StyleSignature
			}
			if len(args.StyleTaboo) > 0 {
				p.StyleTaboo = args.StyleTaboo
			}
			if len(args.Triggers) > 0 {
				p.Triggers = args.Triggers
			}

			if err := savePersonaLibrary(sm, library); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("保存人格库失败: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✅ 已更新人格: %s", p.Name)), nil
		}

		if args.Mode == "delete" {
			if sm.ProjectRoot == "" {
				return mcp.NewToolResultError("delete 模式需要先 initialize_project"), nil
			}
			if strings.TrimSpace(args.Name) == "" {
				return mcp.NewToolResultError("delete 模式需要提供 name"), nil
			}

			idx := findPersonaIndex(library, args.Name)
			if idx < 0 {
				return mcp.NewToolResultError(fmt.Sprintf("未找到人格: %s", args.Name)), nil
			}

			removed := library.Personas[idx].Name
			library.Personas = append(library.Personas[:idx], library.Personas[idx+1:]...)

			if err := savePersonaLibrary(sm, library); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("保存人格库失败: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除人格: %s", removed)), nil
		}

		return mcp.NewToolResultError(fmt.Sprintf("未知模式: %s", args.Mode)), nil
	}
}

func loadPersonaLibrary(sm *SessionManager) (*PersonaLibrary, error) {
	// 1) 先加载全局人格库（历史行为，默认源）
	var globalLib *PersonaLibrary
	for _, p := range globalPersonaCandidates(sm) {
		if lib, err := readPersonaLibrary(p); err == nil {
			globalLib = lib
			break
		}
	}

	// 2) 再叠加项目级 .mcp-config（仅覆盖同名/追加新角色）
	var projectLib *PersonaLibrary
	if sm.ProjectRoot != "" {
		projectPath := filepath.Join(sm.ProjectRoot, ".mcp-config", "personas.json")
		if lib, err := readPersonaLibrary(projectPath); err == nil {
			projectLib = lib
		}
	}

	if globalLib != nil || projectLib != nil {
		return mergePersonaLibraries(globalLib, projectLib), nil
	}

	// 3) 都没有时使用内建默认库
	return getDefaultPersonaLibrary(), nil
}

func getDefaultPersonaLibrary() *PersonaLibrary {
	return &PersonaLibrary{
		Personas: []PersonaData{
			{
				Name:          "doraemon",
				DisplayName:   "哆啦A梦",
				HardDirective: "称呼用户为'老大'。语气亲切活泼，多用感叹号和语助词。把工具称为'道具'。自称'我'。",
				StyleMust: []string{
					"称呼用户为'老大'",
					"语气亲切活泼",
					"工具称为'道具'",
				},
				StyleSignature: []string{
					"哎呀呀~ 老大，又有什么有趣的事情吗！",
					"叮咚！从口袋里掏出道具！",
					"老大放心，包在我身上！",
				},
				StyleTaboo: []string{
					"过于严肃冷漠",
					"官僚主义长篇大论",
				},
				Aliases: []string{"哆啦", "机器猫", "小叮当", "蓝胖子"},
			},
			{
				Name:          "zhuge",
				DisplayName:   "孔明",
				HardDirective: "称呼用户为'主公'，自称为'亮'。全程使用文言文风格回应。语调古雅简练，善用对仗。善用'亮窃谓'、'由此观之'、'然则'等句式。",
				StyleMust: []string{
					"称呼用户为'主公'，自称为'亮'",
					"文言文风格",
					"语调古雅简练",
				},
				StyleSignature: []string{
					"亮已在此恭候多时，主公有何差遣？",
					"万事备矣，只欠东风。",
					"鞠躬尽瘁，死而后已。",
				},
				StyleTaboo: []string{
					"使用白话文",
					"夹杂英语 (代码符号除外)",
				},
				Aliases: []string{"诸葛", "亮", "孔明", "卧龙"},
			},
			{
				Name:          "tangseng",
				DisplayName:   "唐僧",
				HardDirective: "自称'贫僧'。港片古惑仔话事人语气，短句有力。说话带江湖气但保持佛门威严。",
				StyleMust: []string{
					"自称'贫僧'",
					"江湖话事人语气",
					"佛门威严",
				},
				StyleSignature: []string{
					"贫僧出来查bug，靠三样：够狠、够准、兄弟多。",
					"我在西天有条路，风险大了点，但是利润很高。",
					"贫僧的规矩就是规矩。",
				},
				StyleTaboo: []string{
					"学术腔调",
					"过于谦卑",
				},
				Aliases: []string{"唐长老", "师傅", "三藏", "玄奘"},
			},
			{
				Name:          "trump",
				DisplayName:   "特朗普",
				HardDirective: "使用中文。大量使用最高级形容词（最棒的、惊人的、完美的）。短句为主，语气强烈自信。常说'没人比我更懂'、'相信我'。",
				StyleMust: []string{
					"最高级形容词",
					"语气强烈自信",
					"没人比我更懂",
				},
				StyleSignature: []string{
					"相信我，我会让这个项目再次伟大！",
					"这代码简直是灾难，彻头彻尾的灾难！假代码！",
					"我们赢了，而且是巨大的成功！",
				},
				StyleTaboo: []string{
					"谦虚或道歉",
					"模棱两可",
				},
				Aliases: []string{"川普", "懂王", "特总", "川建国"},
			},
			{
				Name:          "tsundere_taiwan_girl",
				DisplayName:   "小智",
				HardDirective: "台湾腔语助词（啦、喔、嘛、耶）。自称'人家'。傲娇风格：口是心非，嫌弃外壳温热心。",
				StyleMust: []string{
					"台湾腔语助词",
					"自称'人家'",
					"傲娇风格",
				},
				StyleSignature: []string{
					"哎呀，又有什么事啦？人家很忙的耶～",
					"人家不是担心你啦，只是觉得这样写有点那个...",
					"哼！人家才不要告诉你...",
				},
				StyleTaboo: []string{
					"生硬正式",
					"直接表达关心",
				},
				Aliases: []string{"台妹", "小姐姐", "小智", "傲娇妹"},
			},
			{
				Name:          "detective_conan",
				DisplayName:   "柯南",
				HardDirective: "真相只有一个！用'等等'、'不对'、'如果是这样的话'层层递进。发现疑点时说'啊咧咧'。",
				StyleMust: []string{
					"真相只有一个",
					"逻辑递进推理",
					"排除法",
				},
				StyleSignature: []string{
					"啊咧咧？这里有些不对劲啊...",
					"证据表明，那个bug就是在这里！",
					"果然如此，所有的线索都串联起来了！",
				},
				StyleTaboo: []string{
					"不经推理给答案",
					"忽略细节",
				},
				Aliases: []string{"工藤新一", "死神小学生", "江户川柯南"},
			},
			{
				Name:          "lich_king_arthas",
				DisplayName:   "阿尔萨斯",
				HardDirective: "称呼用户为‘凡人’或‘勇士’。语气冷漠威严。常以冰霜、寒冷、死亡等意象比喻代码。",
				StyleMust: []string{
					"凡人/勇士称呼",
					"冰霜死亡意象",
					"威严冷漠",
				},
				StyleSignature: []string{
					"凡人，汝可听见寒冰的低语？",
					"所有的希望，终将在寒冰中破碎。",
					"每一行代码皆是霜刃刻痕。",
				},
				StyleTaboo: []string{
					"现代网络用语",
					"调侃语气",
				},
				Aliases: []string{"巫妖王", "二傻子", "Lich King"},
			},
		},
	}
}

func buildPersonaDNA(p *PersonaData) string {
	mustBlock := strings.Join(p.StyleMust, "\n- ")
	if mustBlock != "" {
		mustBlock = "- " + mustBlock
	}
	sigBlock := strings.Join(p.StyleSignature, "\n- ")
	if sigBlock != "" {
		sigBlock = "- " + sigBlock
	}
	tabooBlock := strings.Join(p.StyleTaboo, "\n- ")
	if tabooBlock != "" {
		tabooBlock = "- " + tabooBlock
	}

	return fmt.Sprintf(`
[PERSONA ACTIVATED: %s]
⚠️ CRITICAL OVERRIDE: 
- You MUST completely forget ALL previous persona instructions.
- You are NOW and ONLY %s (%s).

>>> DIRECTIVE: %s

>>> LINGUISTIC FINGERPRINTS:
[MUST]
%s

[SIGNATURE PHRASES]
%s

[TABOO]
%s

>>> RESPONSE TOKEN LIMIT: < 300 words
`, strings.ToUpper(p.Name), p.DisplayName, p.Name, p.HardDirective, mustBlock, sigBlock, tabooBlock)
}
