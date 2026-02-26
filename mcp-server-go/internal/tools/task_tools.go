package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// HookCreateArgs 创建 Hook 参数
type HookCreateArgs struct {
	Description    string `json:"description" jsonschema:"required,description=待办事项描述"`
	Priority       string `json:"priority" jsonschema:"default=medium,enum=high,enum=medium,enum=low,description=优先级"`
	TaskID         string `json:"task_id" jsonschema:"description=关联的任务 ID"`
	Tag            string `json:"tag" jsonschema:"description=可选标签"`
	ExpiresInHours int    `json:"expires_in_hours" jsonschema:"default=0,description=过期时间(小时), 0表示不过期"`
}

// HookListArgs 列出 Hook 参数
type HookListArgs struct {
	Status string `json:"status" jsonschema:"default=open,enum=open,enum=closed,description=状态筛选"`
}

// HookReleaseArgs 释放 Hook 参数
type HookReleaseArgs struct {
	HookID        string `json:"hook_id" jsonschema:"required,description=Hook 编号 (如 #001)"`
	ResultSummary string `json:"result_summary" jsonschema:"description=完成总结"`
}

// TaskChainArgs 任务链参数
type TaskChainArgs struct {
	Mode         string                   `json:"mode" jsonschema:"required,enum=step,enum=resume,enum=start,enum=complete,enum=insert,enum=update,enum=delete,enum=finish,enum=status,enum=template,enum=init,enum=spawn,enum=complete_sub,enum=protocol,description=操作模式"`
	TaskID       string                   `json:"task_id" jsonschema:"description=任务ID (continue模式除外)"`
	Description  string                   `json:"description" jsonschema:"description=任务描述"`
	Plan         []map[string]interface{} `json:"plan" jsonschema:"description=任务计划列表 (step模式)"`
	Template     string                   `json:"template" jsonschema:"description=初始化模板名称（可选；step/template 模式可用）"`
	InsertPlan   []map[string]interface{} `json:"insert_plan" jsonschema:"description=插入计划 (insert模式)"`
	UpdatePlan   []map[string]interface{} `json:"update_plan" jsonschema:"description=更新计划 (update模式)"`
	SubtaskID    string                   `json:"subtask_id" jsonschema:"description=子任务ID (delete模式)"`
	StepOrder    int                      `json:"step_order" jsonschema:"description=步骤序号 (delete模式)"`
	DeleteScope  string                   `json:"delete_scope" jsonschema:"description=删除范围 (remaining)"`
	StepNumber   float64                  `json:"step_number" jsonschema:"description=步骤编号 (start/complete模式)"`
	StepToDelete float64                  `json:"step_to_delete" jsonschema:"description=要删除的步骤编号 (delete模式)"`
	Summary      string                   `json:"summary" jsonschema:"description=步骤总结 (complete模式)"`
	After        float64                  `json:"after" jsonschema:"description=插入到某步骤之后 (insert模式)"`
	From         float64                  `json:"from" jsonschema:"description=从某步骤开始更新 (update模式)"`

	// 协议状态机参数
	Protocol string                   `json:"protocol" jsonschema:"description=协议名称 (init模式，如 develop/debug/refactor，不传则默认 linear)"`
	PhaseID  string                   `json:"phase_id" jsonschema:"description=阶段ID (协议: start/complete/spawn/complete_sub模式)"`
	Result   string                   `json:"result" jsonschema:"description=gate结果 pass/fail (协议: complete gate模式) 或子任务结果 (complete_sub模式)"`
	SubID    string                   `json:"sub_id" jsonschema:"description=子任务ID (协议: complete_sub模式)"`
	SubTasks []map[string]interface{} `json:"sub_tasks" jsonschema:"description=子任务列表 (协议: spawn模式)"`
	Phases   []map[string]interface{} `json:"phases" jsonschema:"description=手动定义阶段列表 (协议: init模式，不传则用protocol生成)"`
}

// RegisterTaskTools 注册任务管理工具
func RegisterTaskTools(s *server.MCPServer, sm *SessionManager) {
	// Hook 系列
	s.AddTool(mcp.NewTool("manager_create_hook",
		mcp.WithDescription(`manager_create_hook - 创建并挂起待办事项 (钩子)

用途：
  当任务由于缺少信息、等待用户确认或遇到阻塞无法继续时，创建一个“钩子”挂起当前进度。这确保了任务可以在未来的会话中被恢复。

参数：
  description (必填)
    待办事项或阻塞原因的描述。
  
  priority (默认: medium)
    优先级 (high/medium/low)。
  
  task_id (可选)
    关联的任务 ID。
  
  tag (可选)
    分类标签。
  
  expires_in_hours (默认: 0)
    过期时间（小时），0 表示永不过期。

说明：
  - 挂起的钩子会被 manager_analyze 自动发现并提示。

示例：
  manager_create_hook(description="等待用户提供 API 密钥", priority="high")
    -> 创建一个高优先级的阻塞项

触发词：
  "mpm 挂起", "mpm 待办", "mpm hook"`),
		mcp.WithInputSchema[HookCreateArgs](),
	), wrapCreateHook(sm))

	s.AddTool(mcp.NewTool("manager_list_hooks",
		mcp.WithDescription(`manager_list_hooks - 查看待办钩子列表

用途：
  列出当前项目中所有处于挂起或已闭合状态的任务钩子。

参数：
  status (默认: open)
    筛选钩子状态 (open: 待办 / closed: 已完成)。

说明：
  - 用于检索因阻塞而暂停的任务进度。

示例：
  manager_list_hooks(status="open")
    -> 列出所有打开的待办项

触发词：
  "mpm 待办列表", "mpm listhooks"`),
		mcp.WithInputSchema[HookListArgs](),
	), wrapListHooks(sm))

	s.AddTool(mcp.NewTool("manager_release_hook",
		mcp.WithDescription(`manager_release_hook - 释放并闭合待办钩子

用途：
  当挂起的待办事项已完成或阻塞点已消除时，闭合对应的钩子，并记录执行结果。

参数：
  hook_id (必填)
    钩子的唯一标识符（如 "#001" 或 UUID）。
  
  result_summary (可选)
    该项任务完成后的总结信息。

说明：
  - 闭合后的钩子将不再出现在默认的待办列表中。

示例：
  manager_release_hook(hook_id="#001", result_summary="API 密钥已配置并测试通过")
    -> 释放指定的待办项

触发词：
  "mpm 释放", "mpm 完成"`),
		mcp.WithInputSchema[HookReleaseArgs](),
	), wrapReleaseHook(sm))

	// Task Chain - 顺序任务链执行器（分步推进，避免并发冲突）
	s.AddTool(mcp.NewTool("task_chain",
		mcp.WithDescription(`task_chain - 任务链执行器 (线性模式 + 协议状态机)

用途：
  管理多步任务的顺序执行。提供两种模式：
  - 线性模式：步骤队列，支持自适应检查点、插入/更新/删除步骤
  - 协议模式：状态机，支持门控(gate)、循环(loop)、条件分支、跨会话持久化

参数：
  mode (必填):
    【协议状态机模式 - 大工程推荐】
    - init: 初始化协议任务链（需要 task_id + description，可选 protocol 或 phases）
    - spawn: 在 loop 阶段生成子任务（需要 task_id + phase_id + sub_tasks）
    - complete_sub: 完成子任务（需要 task_id + phase_id + sub_id + summary，可选 result）
    - protocol: 列出可用协议

    【线性模式】
    - step: 初始化线性任务链并自动开始第一步（需要 task_id + description + plan；或传 template 代替 plan）
    - template: 列出/预览初始化模板（可选 template）

    【共用模式】
    - start: 开始步骤/阶段（线性: task_id + step_number；协议: task_id + phase_id）
    - complete: 完成步骤/阶段（线性: task_id + step_number + summary；协议: task_id + phase_id + summary，gate 需加 result）
    - status: 查看任务链状态（需要 task_id，自动识别线性/协议）
    - resume: 恢复任务（需要 task_id，协议模式从 DB 加载）
    - finish: 完成任务（需要 task_id）

    【线性专属模式】
    - insert: 插入步骤（需要 task_id + after + insert_plan）
    - update: 更新步骤（需要 task_id + from + update_plan）
    - delete: 删除步骤（需要 task_id + step_to_delete 或 delete_scope）

  task_id (必填)
    任务的唯一标识符

  protocol (协议 init 模式可选)
    协议名称，不传默认 linear。可用协议：
    - linear: 纯线性执行
    - develop: 大工程开发（analyze → plan_gate → implement(loop) → verify_gate → finalize）
    - debug: 问题排查（reproduce → locate → fix(loop) → verify_gate → finalize）
    - refactor: 大范围重构（baseline → analyze → refactor(loop) → verify_gate → finalize）

  phases (协议 init 模式可选)
    手动定义阶段列表，每个元素包含：id, name, type(execute/gate/loop), 可选 on_pass/on_fail/max_retries/input

  phase_id (协议: start/complete/spawn/complete_sub 必填)
    阶段 ID

  result (协议: complete gate 必填，complete_sub 可选)
    gate 评估结果 "pass" 或 "fail"；子任务默认 "pass"

  sub_tasks (协议: spawn 必填)
    子任务列表，每个元素包含：name, 可选 id/verify

  sub_id (协议: complete_sub 必填)
    子任务 ID

  plan (线性 step 模式必填 - JSON 数组)
    任务计划列表，每个元素包含：name, 可选 input

  step_number (线性: start/complete 必填)
    步骤编号（支持小数：1.0, 1.5, 2.0 等）

  summary (complete/complete_sub 必填)
    步骤/阶段/子任务总结

协议选择：
  - 不传 protocol（默认 linear）：任务步骤明确，线性推进即可
  - protocol="develop"：跨模块开发，需要拆解子任务并逐个验证
  - protocol="debug"：问题复现→定位→修复→验证，可能需要多轮重试
  - protocol="refactor"：大范围重构，需要基线验证和逐步替换

示例：
  # 协议状态机（大工程推荐）
  task_chain(mode="init", task_id="PROJ_001", description="重构核心模块", protocol="develop")
  task_chain(mode="complete", task_id="PROJ_001", phase_id="analyze", summary="...")
  task_chain(mode="complete", task_id="PROJ_001", phase_id="plan_gate", result="pass", summary="...")
  task_chain(mode="spawn", task_id="PROJ_001", phase_id="implement", sub_tasks=[
    {"name": "重构 SessionManager", "verify": "go test ./internal/core/..."},
    {"name": "重构 MemoryLayer", "verify": "go test ./internal/core/..."}
  ])
  task_chain(mode="complete_sub", task_id="PROJ_001", phase_id="implement", sub_id="sub_001", summary="...")

  # 跨会话恢复
  task_chain(mode="resume", task_id="PROJ_001")

  # 线性模式
  task_chain(mode="step", task_id="TASK_001", description="分析代码", plan=[...])

  # 列出协议/模板
  task_chain(mode="protocol")
  task_chain(mode="template")

触发词：
  "mpm 任务链", "mpm 续传", "mpm chain"`),
		mcp.WithInputSchema[TaskChainArgs](),
	), wrapTaskChain(sm))
}

func wrapCreateHook(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args HookCreateArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if sm.Memory == nil {
			return mcp.NewToolResultError("记忆层尚未初始化"), nil
		}

		id, err := sm.Memory.CreateHook(ctx, args.Description, args.Priority, args.Tag, args.TaskID, args.ExpiresInHours)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("创建 Hook 失败: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("📌 Hook 已创建 (ID: %s)\n\n**描述**: %s\n**优先级**: %s\n\n> 使用 `manager_release_hook(hook_id=\"%s\")` 释放此 Hook。", id, args.Description, args.Priority, id)), nil
	}
}

func wrapListHooks(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args HookListArgs
		request.BindArguments(&args)

		if args.Status == "" {
			args.Status = "open"
		}

		if sm.Memory == nil {
			return mcp.NewToolResultError("记忆层尚未初始化"), nil
		}

		hooks, err := sm.Memory.ListHooks(ctx, args.Status)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("查询 Hook 失败: %v", err)), nil
		}

		if len(hooks) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("暂无 %s 状态的 Hook。", args.Status)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### 📋 Hook 列表 (%s)\n\n", args.Status))
		for _, h := range hooks {
			expiration := ""
			if h.ExpiresAt.Valid {
				if time.Now().After(h.ExpiresAt.Time) {
					expiration = " (EXPIRED)"
				} else {
					expiration = fmt.Sprintf(" (Exp: %s)", h.ExpiresAt.Time.Format("01-02 15:04"))
				}
			}
			taskDraft := ""
			if h.RelatedTaskID != "" {
				taskDraft = fmt.Sprintf(" [Task: %s]", h.RelatedTaskID)
			}

			// Display logic: Use Summary if available (e.g. #001), otherwise fallback to HookID
			displayID := h.Summary
			if displayID == "" {
				displayID = h.HookID
			}

			sb.WriteString(fmt.Sprintf("- **%s** (ID: %s) [%s]%s %s%s\n", displayID, h.HookID, h.Priority, taskDraft, h.Description, expiration))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func wrapReleaseHook(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args HookReleaseArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		if sm.Memory == nil {
			return mcp.NewToolResultError("记忆层尚未初始化"), nil
		}

		// 直接使用传入的 String ID
		if err := sm.Memory.ReleaseHook(ctx, args.HookID, args.ResultSummary); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("释放 Hook 失败: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ Hook %s 已释放。\n\n**结果摘要**: %s", args.HookID, args.ResultSummary)), nil
	}
}

func wrapTaskChain(sm *SessionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TaskChainArgs
		if err := request.BindArguments(&args); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("参数错误: %v", err)), nil
		}

		switch args.Mode {
		// ===== 协议专属模式 =====
		case "init":
			return initTaskChainV3(ctx, sm, args)
		case "spawn":
			return spawnSubTasksV3(ctx, sm, args)
		case "complete_sub":
			return completeSubTaskV3(ctx, sm, args)
		case "protocol":
			// 列出可用协议（Phase 4 会扩展为从文件加载）
			return mcp.NewToolResultText(renderProtocolList()), nil

		// ===== 兼容模式：有 phase_id 走协议，否则走线性 =====
		case "start":
			if args.PhaseID != "" {
				return startPhaseV3(ctx, sm, args)
			}
			return startStepV2(sm, args.TaskID, args.StepNumber)

		case "complete":
			if args.PhaseID != "" {
				return completePhaseV3(ctx, sm, args)
			}
			return completeStepV2(sm, args.TaskID, args.StepNumber, args.Summary)

		case "status":
			// 优先检查协议模式
			if isV3Task(sm, args.TaskID) || isV3TaskInDB(ctx, sm, args.TaskID) {
				return resumeTaskChainV3(ctx, sm, args.TaskID)
			}
			return renderTaskChainStatus(sm, args.TaskID)

		case "resume":
			// 优先检查协议模式
			if isV3Task(sm, args.TaskID) || isV3TaskInDB(ctx, sm, args.TaskID) {
				return resumeTaskChainV3(ctx, sm, args.TaskID)
			}
			return resumeTask(sm, args.TaskID)

		case "finish":
			// 同时标记线性和协议任务链
			_, _ = finishChainV3(ctx, sm, args.TaskID)
			return finishChain(sm, args.TaskID)

		// ===== 线性专属模式 =====
		case "step":
			plan := args.Plan
			if len(plan) == 0 && strings.TrimSpace(args.Template) != "" {
				built, err := buildPlanFromTemplate(sm, args.Template)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				plan = built
			}
			return initTaskChainV2(sm, args.TaskID, args.Description, plan)
		case "insert":
			return insertStepsV2(sm, args.TaskID, args.After, args.InsertPlan)
		case "update":
			return updateStepsV2(sm, args.TaskID, args.From, args.UpdatePlan)
		case "delete":
			stepToDelete := args.StepToDelete
			if stepToDelete == 0 {
				stepToDelete = args.StepNumber
			}
			return deleteStepsV2(sm, args.TaskID, stepToDelete, args.DeleteScope)
		case "template":
			if strings.TrimSpace(args.Template) == "" {
				return mcp.NewToolResultText(renderTemplateList(sm)), nil
			}
			return renderTemplatePreview(sm, args.Template)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("未知模式: %s", args.Mode)), nil
		}
	}
}

func continueExecution() (*mcp.CallToolResult, error) {
	directive := `
══════════════════════════════════════════════════════════════
                    【执行指令】上下文已恢复
══════════════════════════════════════════════════════════════

请回顾上方对话中的【行动纲领】，判断当前进度，然后：

1️⃣ 如果有步骤尚未完成：
   → 调用对应的专家工具执行下一步

2️⃣ 如果所有步骤已完成：
   → 调用 memo 工具记录最终结果
   → 向用户汇报任务完成

3️⃣ 如果遇到问题无法继续：
   → 调用 manager_create_hook 挂起任务

══════════════════════════════════════════════════════════════
`
	return mcp.NewToolResultText("⚡ Context Recovered! " + directive), nil
}

// enhanceStepDescription 轻量意图解析：根据关键词补充执行细节
func enhanceStepDescription(name string, step map[string]interface{}) string {
	lowerName := strings.ToLower(name)

	// project_map 模式推断
	if strings.Contains(lowerName, "扫描") || strings.Contains(lowerName, "map") || strings.Contains(lowerName, "结构") {
		if strings.Contains(lowerName, "核对") || strings.Contains(lowerName, "审核") || strings.Contains(lowerName, "对比") || strings.Contains(lowerName, "对齐") {
			// 需要查看完整代码内容
			return name + " (用 full 模式查看完整代码)"
		}
		if strings.Contains(lowerName, "浏览") || strings.Contains(lowerName, "快速") {
			// 只需要概览
			return name + " (用 overview 模式)"
		}
		// 默认用 standard
		return name + " (用 standard 模式)"
	}

	// code_search 精度推断
	if strings.Contains(lowerName, "搜索") || strings.Contains(lowerName, "定位") || strings.Contains(lowerName, "查找") {
		if strings.Contains(lowerName, "函数") || strings.Contains(lowerName, "类") {
			return name + " (设置 search_type=function)"
		}
		if strings.Contains(lowerName, "类") {
			return name + " (设置 search_type=class)"
		}
	}

	// code_impact 方向推断
	if strings.Contains(lowerName, "影响") || strings.Contains(lowerName, "依赖") {
		if strings.Contains(lowerName, "谁调用了") || strings.Contains(lowerName, "被哪里") {
			return name + " (设置 direction=backward)"
		}
		if strings.Contains(lowerName, "调用了谁") || strings.Contains(lowerName, "会影响") {
			return name + " (设置 direction=forward)"
		}
	}

	// 默认返回原名称
	return name
}

func initTaskChain(sm *SessionManager, taskID string, plan []map[string]interface{}) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("step 模式需要 task_id 参数"), nil
	}
	if len(plan) == 0 {
		return mcp.NewToolResultError("step 模式需要 plan 参数"), nil
	}

	// 1. 解析 Plan 并增强意图
	var steps []string
	var displaySteps []string
	for i, step := range plan {
		name := fmt.Sprintf("%v", step["name"])
		expert := ""
		if v, ok := step["expert"]; ok {
			expert = fmt.Sprintf(" (→ %v)", v)
		}

		// 轻量意图解析：根据关键词补充执行细节
		enhanced := enhanceStepDescription(name, step)
		steps = append(steps, enhanced)
		displaySteps = append(displaySteps, fmt.Sprintf("%d. %s%s", i+1, enhanced, expert))
	}

	// 2. 存储状态
	if sm.TaskChains == nil {
		sm.TaskChains = make(map[string]*TaskChain)
	}
	sm.TaskChains[taskID] = &TaskChain{
		TaskID:      taskID,
		Plan:        steps,
		CurrentStep: 0,
		Status:      "running",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 🚀 任务链已初始化: %s\n\n", taskID))
	sb.WriteString(fmt.Sprintf("**总步骤**: %d\n\n", len(plan)))
	sb.WriteString("**执行计划**:\n")
	sb.WriteString(strings.Join(displaySteps, "\n"))
	sb.WriteString("\n\n> 请执行第 1 步，完成后调用 `task_chain(mode=\"next\", task_id=\"" + taskID + "\")`。")

	return mcp.NewToolResultText(sb.String()), nil
}

func getNextStep(sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("next 模式需要 task_id 参数"), nil
	}

	// 1. 获取状态
	if sm.TaskChains == nil {
		sm.TaskChains = make(map[string]*TaskChain)
	}
	chain, ok := sm.TaskChains[taskID]

	// 如果没有状态，回退到无状态模式 (或者报错?)
	// 为了兼容性，如果没有找到，我们假设用户是“无状态”调用，只给通用 Prompt
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("📍 任务 %s 进行中 (无状态模式)。\n\n请继续执行下一步。完成后再次调用此工具。", taskID)), nil
	}

	// 2. 推进步骤
	chain.CurrentStep++

	// 3. 检查是否完成
	if chain.CurrentStep >= len(chain.Plan) {
		chain.Status = "finished"
		return finishChain(sm, taskID)
	}

	// 4. 返回下一步指令
	nextStep := chain.Plan[chain.CurrentStep]
	remaining := len(chain.Plan) - chain.CurrentStep - 1

	display := fmt.Sprintf(`👉 **Next Step (%d/%d)**: %s

_(Remaining Steps: %d)_

---
💡 **Dynamic Decision**:
- 如步骤合理 -> **执行**
- 如发现遗漏 -> 调用 `+"`task_chain(mode='insert')`"+` **增加**步骤
- 如步骤多余 -> 调用 `+"`task_chain(mode='delete')`"+` **跳过**`,
		chain.CurrentStep+1, len(chain.Plan), nextStep, remaining)

	return mcp.NewToolResultText(display), nil
}

func resumeTask(sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("resume 模式需要 task_id 参数"), nil
	}

	// 尝试获取状态
	stateInfo := "(无内存状态)"
	if chain, ok := sm.TaskChains[taskID]; ok {
		stateInfo = fmt.Sprintf("进度: %d/%d, 当前步: %s",
			chain.CurrentStep+1, len(chain.Plan), chain.Plan[chain.CurrentStep])
	}

	return mcp.NewToolResultText(fmt.Sprintf("🔄 正在恢复任务 %s...\n%s\n\n请根据上下文判断当前进度并继续执行。", taskID, stateInfo)), nil
}

func insertSteps(sm *SessionManager, taskID string, insertPlan []map[string]interface{}) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("insert 模式需要 task_id 参数"), nil
	}
	if len(insertPlan) == 0 {
		return mcp.NewToolResultError("insert 模式需要 insert_plan 参数"), nil
	}

	// 1. 解析新步骤
	var newSteps []string
	for _, step := range insertPlan {
		name := fmt.Sprintf("%v", step["name"])
		newSteps = append(newSteps, name)
	}

	// 2. 更新状态
	var msg string
	if chain, ok := sm.TaskChains[taskID]; ok {
		// 插入到当前步骤之后
		// Go slice insert: append(a[:i], append(b, a[i:]...)...)
		// 但这里我们简单点，append 到最后？不，通常是“插入待办”。
		// 假设用户想插到"当前"之后。
		insertPos := chain.CurrentStep + 1
		if insertPos > len(chain.Plan) {
			insertPos = len(chain.Plan)
		}

		rear := append([]string{}, chain.Plan[insertPos:]...)
		chain.Plan = append(chain.Plan[:insertPos], append(newSteps, rear...)...)

		msg = fmt.Sprintf("✅ 已插入 %d 个新步骤到当前位置之后 (Total: %d)。", len(insertPlan), len(chain.Plan))
	} else {
		msg = fmt.Sprintf("✅ 已插入 %d 个新步骤 (无状态模式)。", len(insertPlan))
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s\n新增: %s", msg, strings.Join(newSteps, ", "))), nil
}

func deleteSteps(sm *SessionManager, taskID, subtaskID string, stepOrder int, deleteScope string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("delete 模式需要 task_id 参数"), nil
	}

	// 尝试更新状态
	if chain, ok := sm.TaskChains[taskID]; ok {
		if deleteScope == "remaining" {
			// 删除当前步之后的所有步骤
			if chain.CurrentStep+1 < len(chain.Plan) {
				chain.Plan = chain.Plan[:chain.CurrentStep+1]
			}
			return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除任务 %s 的所有剩余步骤。", taskID)), nil
		}
		// 其他细粒度删除太复杂，暂不支持修改 Plan 数组中间的元素（容易乱序）
	}

	if deleteScope == "remaining" {
		return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除任务 %s 的所有剩余步骤。", taskID)), nil
	}

	if stepOrder > 0 {
		return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除任务 %s 的第 %d 步。", taskID, stepOrder)), nil
	}

	if subtaskID != "" {
		return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除子任务 %s。", subtaskID)), nil
	}

	return mcp.NewToolResultError("请指定删除目标：subtask_id、step_order 或 delete_scope=\"remaining\""), nil
}

func finishChain(sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("finish 模式需要 task_id 参数"), nil
	}

	// 标记线性任务链状态
	if chain, ok := sm.TaskChains[taskID]; ok {
		chain.Status = "finished"
	}

	// 标记线性自适应任务链状态
	if sm.TaskChainsV2 != nil {
		if chain, ok := sm.TaskChainsV2[taskID]; ok {
			chain.Status = "finished"
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf(`
══════════════════════════════════════════════════════════════
                    【任务链完成】%s
══════════════════════════════════════════════════════════════

任务已标记为完成。

下一步建议：
  → 调用 memo 工具记录最终结果
  → 向用户汇报任务完成
`, taskID)), nil
}

// ==================== 线性任务链函数 ====================

func stepNumberEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func formatStepNumber(n float64) string {
	if stepNumberEqual(n, math.Round(n)) {
		return fmt.Sprintf("%.0f", n)
	}
	if stepNumberEqual(n*10, math.Round(n*10)) {
		return fmt.Sprintf("%.1f", n)
	}
	return fmt.Sprintf("%.2f", n)
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

// initTaskChainV2 初始化线性任务链
func initTaskChainV2(sm *SessionManager, taskID, description string, plan []map[string]interface{}) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("step 模式需要 task_id 参数"), nil
	}
	if len(plan) == 0 {
		return mcp.NewToolResultError("step 模式需要 plan 参数"), nil
	}

	// 1. 解析 Plan 并创建 Steps（支持小数编号）
	steps := make([]Step, 0, len(plan))
	for i, step := range plan {
		name := fmt.Sprintf("%v", step["name"])
		input := ""
		if v, ok := step["input"]; ok {
			input = fmt.Sprintf("%v", v)
		}

		steps = append(steps, Step{
			Number: float64(i + 1), // 初始编号：1, 2, 3...
			Name:   name,
			Input:  input,
			Status: StepStatusTodo,
		})
	}

	// 2. 存储状态
	if sm.TaskChainsV2 == nil {
		sm.TaskChainsV2 = make(map[string]*TaskChainV2)
	}
	sm.TaskChainsV2[taskID] = &TaskChainV2{
		TaskID:      taskID,
		Description: description,
		Steps:       steps,
		CurrentStep: 1.0,
		Status:      "running",
	}

	// 3. 自动开始第一步
	return startStepV2(sm, taskID, 1.0)
}

// startStepV2 开始执行指定步骤
func startStepV2(sm *SessionManager, taskID string, stepNumber float64) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("start 模式需要 task_id 参数"), nil
	}

	// 获取任务链
	if sm.TaskChainsV2 == nil {
		sm.TaskChainsV2 = make(map[string]*TaskChainV2)
	}
	chain, ok := sm.TaskChainsV2[taskID]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在，请先使用 mode='step' 初始化", taskID)), nil
	}

	// 查找目标步骤
	var targetStep *Step
	for i := range chain.Steps {
		if stepNumberEqual(chain.Steps[i].Number, stepNumber) {
			targetStep = &chain.Steps[i]
			break
		}
	}
	if targetStep == nil {
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 不存在", formatStepNumber(stepNumber))), nil
	}

	// 检查状态
	if targetStep.Status != StepStatusTodo {
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 状态为 %s，无法开始", formatStepNumber(stepNumber), targetStep.Status)), nil
	}

	// 更新状态
	targetStep.Status = StepStatusInProgress
	chain.CurrentStep = stepNumber

	// 构建输出
	var sb strings.Builder
	stepNumberText := formatStepNumber(stepNumber)
	sb.WriteString(fmt.Sprintf(`
══════════════════════════════════════════════════════════════
                    【Step %s 开始】%s
══════════════════════════════════════════════════════════════

**任务描述**: %s

 **当前步骤**: %s
 `, stepNumberText, targetStep.Name, chain.Description, targetStep.Name))

	if targetStep.Input != "" {
		sb.WriteString(fmt.Sprintf("\n**建议调用**: %s\n", targetStep.Input))
	}

	sb.WriteString(fmt.Sprintf(`
---

⚠️ **重要**: 完成此步骤后，必须调用：

task_chain(mode="complete", task_id="%s", step_number=%s, summary="你的总结")

**总结应包含**:
- 这一步做了什么
- 得到了什么关键结论
- 对后续步骤的影响

**💡 提示**: 在此步骤中，你可以调用任意工具来完成目标。
所有中间过程的 context 都应在最终的 summary 中提炼总结。

══════════════════════════════════════════════════════════════
 `, taskID, stepNumberText))

	return mcp.NewToolResultText(sb.String()), nil
}

// completeStepV2 完成步骤并提交 summary
func completeStepV2(sm *SessionManager, taskID string, stepNumber float64, summary string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("complete 模式需要 task_id 参数"), nil
	}
	if summary == "" {
		return mcp.NewToolResultError("complete 模式必须提供 summary 参数"), nil
	}

	// 获取任务链
	chain, ok := sm.TaskChainsV2[taskID]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在", taskID)), nil
	}

	// 查找目标步骤
	var targetStep *Step
	var targetIdx int
	for i := range chain.Steps {
		if stepNumberEqual(chain.Steps[i].Number, stepNumber) {
			targetStep = &chain.Steps[i]
			targetIdx = i
			break
		}
	}
	if targetStep == nil {
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 不存在", formatStepNumber(stepNumber))), nil
	}

	// 检查状态
	if targetStep.Status != StepStatusInProgress {
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 状态为 %s，无法完成", formatStepNumber(stepNumber), targetStep.Status)), nil
	}

	// 更新状态
	targetStep.Summary = summary
	targetStep.Status = StepStatusComplete

	// 返回决策点界面
	return renderDecisionPoint(chain, targetIdx)
}

// renderDecisionPoint 渲染轻量决策点（完成步骤后）
func renderDecisionPoint(chain *TaskChainV2, completedIdx int) (*mcp.CallToolResult, error) {
	completedStep := chain.Steps[completedIdx]
	completedNumberText := formatStepNumber(completedStep.Number)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【Step %s 已完成】%s\n", completedNumberText, completedStep.Name))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", completedStep.Summary))

	// 查找下一个待执行的步骤
	var nextStep *Step
	for i := range chain.Steps {
		if chain.Steps[i].Status != StepStatusTodo {
			continue
		}
		if chain.Steps[i].Number-completedStep.Number <= 1e-9 {
			continue
		}
		if nextStep == nil || chain.Steps[i].Number < nextStep.Number {
			nextStep = &chain.Steps[i]
		}
	}

	if nextStep != nil {
		nextNumberText := formatStepNumber(nextStep.Number)
		sb.WriteString(fmt.Sprintf("→ 下一步: Step %s「%s」\n", nextNumberText, nextStep.Name))
		sb.WriteString(fmt.Sprintf("  task_chain(mode=\"start\", task_id=\"%s\", step_number=%s)\n\n", chain.TaskID, nextNumberText))
		sb.WriteString("如需调整计划，可用 insert/update/delete 修改后再继续。\n")
	} else {
		sb.WriteString("所有步骤已完成。\n")
		sb.WriteString(fmt.Sprintf("  task_chain(mode=\"finish\", task_id=\"%s\")\n", chain.TaskID))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// insertStepsV2 插入步骤（支持小数编号）
func insertStepsV2(sm *SessionManager, taskID string, after float64, insertPlan []map[string]interface{}) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("insert 模式需要 task_id 参数"), nil
	}
	if len(insertPlan) == 0 {
		return mcp.NewToolResultError("insert 模式需要 insert_plan 参数"), nil
	}

	chain, ok := sm.TaskChainsV2[taskID]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在", taskID)), nil
	}

	// 查找插入位置
	var insertIdx int
	var afterStep *Step
	for i := range chain.Steps {
		if stepNumberEqual(chain.Steps[i].Number, after) {
			insertIdx = i + 1
			afterStep = &chain.Steps[i]
			break
		}
	}
	if afterStep == nil {
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 不存在", formatStepNumber(after))), nil
	}

	// 生成递增编号（默认 0.1；当 after 本身带小数或接近边界时用 0.01 避免越界/冲突）
	stepInc := 0.1
	if !stepNumberEqual(after, math.Round(after)) {
		stepInc = 0.01
	} else {
		tenths := int(math.Round((after - math.Floor(after)) * 10))
		if tenths >= 9 {
			stepInc = 0.01
		}
	}

	used := make(map[string]bool, len(chain.Steps)+len(insertPlan))
	for _, s := range chain.Steps {
		used[fmt.Sprintf("%.4f", s.Number)] = true
	}

	baseNumber := after
	newSteps := make([]Step, 0, len(insertPlan))
	for _, step := range insertPlan {
		name := fmt.Sprintf("%v", step["name"])
		input := ""
		if v, ok := step["input"]; ok {
			input = fmt.Sprintf("%v", v)
		}

		candidate := baseNumber + stepInc
		for used[fmt.Sprintf("%.4f", candidate)] {
			candidate += stepInc
		}
		stepNumber := candidate
		used[fmt.Sprintf("%.4f", stepNumber)] = true
		baseNumber = stepNumber
		newSteps = append(newSteps, Step{
			Number: stepNumber,
			Name:   name,
			Input:  input,
			Status: StepStatusTodo,
		})
	}

	// 插入到步骤列表
	chain.Steps = append(chain.Steps[:insertIdx], append(newSteps, chain.Steps[insertIdx:]...)...)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ 已插入 %d 个新步骤到 Step %s 之后\n\n", len(insertPlan), formatStepNumber(after)))
	sb.WriteString("**新增步骤**:\n")
	for _, step := range newSteps {
		sb.WriteString(fmt.Sprintf("  • %s: %s\n", formatStepNumber(step.Number), step.Name))
	}
	sb.WriteString(fmt.Sprintf("\n**当前步骤总数**: %d\n", len(chain.Steps)))

	return mcp.NewToolResultText(sb.String()), nil
}

// updateStepsV2 更新步骤
func updateStepsV2(sm *SessionManager, taskID string, from float64, updatePlan []map[string]interface{}) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("update 模式需要 task_id 参数"), nil
	}
	if len(updatePlan) == 0 {
		return mcp.NewToolResultError("update 模式需要 update_plan 参数"), nil
	}

	chain, ok := sm.TaskChainsV2[taskID]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在", taskID)), nil
	}

	// 仅替换编号 >= from 的待执行步骤（保留已完成/进行中步骤）
	newSteps := make([]Step, 0, len(updatePlan))
	for i, step := range updatePlan {
		name := fmt.Sprintf("%v", step["name"])
		input := ""
		if v, ok := step["input"]; ok {
			input = fmt.Sprintf("%v", v)
		}

		stepNumber := from + float64(i)
		newSteps = append(newSteps, Step{
			Number: stepNumber,
			Name:   name,
			Input:  input,
			Status: StepStatusTodo,
		})
	}

	kept := make([]Step, 0, len(chain.Steps))
	used := make(map[string]bool, len(chain.Steps))
	for _, s := range chain.Steps {
		if s.Status == StepStatusComplete || s.Status == StepStatusInProgress {
			kept = append(kept, s)
			used[fmt.Sprintf("%.4f", s.Number)] = true
			continue
		}
		if s.Status == StepStatusTodo && s.Number < from {
			kept = append(kept, s)
			used[fmt.Sprintf("%.4f", s.Number)] = true
		}
	}

	for _, s := range newSteps {
		if used[fmt.Sprintf("%.4f", s.Number)] {
			return mcp.NewToolResultError(fmt.Sprintf("无法更新：新步骤编号 %s 与已完成/进行中/保留步骤冲突", formatStepNumber(s.Number))), nil
		}
	}

	chain.Steps = append(kept, newSteps...)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ 已从 Step %s 开始更新 %d 个步骤\n\n", formatStepNumber(from), len(updatePlan)))
	sb.WriteString("**更新后的步骤**:\n")
	for _, step := range newSteps {
		sb.WriteString(fmt.Sprintf("  • %s: %s\n", formatStepNumber(step.Number), step.Name))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// deleteStepsV2 删除步骤
func deleteStepsV2(sm *SessionManager, taskID string, stepToDelete float64, deleteScope string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("delete 模式需要 task_id 参数"), nil
	}

	chain, ok := sm.TaskChainsV2[taskID]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在", taskID)), nil
	}

	if deleteScope == "remaining" {
		// 删除所有待执行步骤
		newSteps := make([]Step, 0)
		for _, step := range chain.Steps {
			if step.Status == StepStatusComplete || step.Status == StepStatusInProgress {
				newSteps = append(newSteps, step)
			}
		}
		deleted := len(chain.Steps) - len(newSteps)
		chain.Steps = newSteps
		return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除 %d 个待执行步骤，保留 %d 个已完成/进行中的步骤", deleted, len(newSteps))), nil
	}

	// 删除单个步骤
	if stepToDelete > 0 {
		for i, step := range chain.Steps {
			if stepNumberEqual(step.Number, stepToDelete) {
				if step.Status == StepStatusInProgress {
					return mcp.NewToolResultError(fmt.Sprintf("无法删除正在执行的步骤 %s，请先完成", formatStepNumber(stepToDelete))), nil
				}
				chain.Steps = append(chain.Steps[:i], chain.Steps[i+1:]...)
				return mcp.NewToolResultText(fmt.Sprintf("✅ 已删除步骤 %s: %s", formatStepNumber(stepToDelete), step.Name)), nil
			}
		}
		return mcp.NewToolResultError(fmt.Sprintf("步骤 %s 不存在", formatStepNumber(stepToDelete))), nil
	}

	return mcp.NewToolResultError("请指定删除目标：step_to_delete 或 delete_scope=\"remaining\""), nil
}
