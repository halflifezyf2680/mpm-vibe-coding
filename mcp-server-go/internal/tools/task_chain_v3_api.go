package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mcp-server-go/internal/core"

	"github.com/mark3labs/mcp-go/mcp"
)

// ========== 协议状态机 API Handler ==========

func convertToMapSlice(v interface{}) ([]map[string]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	switch val := v.(type) {
	case string:
		var result []map[string]interface{}
		if err := json.Unmarshal([]byte(val), &result); err != nil {
			return nil, err
		}
		return result, nil
	case []interface{}:
		var result []map[string]interface{}
		for _, item := range val {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result, nil
	case []map[string]interface{}:
		return val, nil
	default:
		return nil, fmt.Errorf("未经支持的参数格式: %T", v)
	}
}

// ensureV3Map 确保 TaskChainsV3 map 已初始化
func ensureV3Map(sm *SessionManager) {
	if sm.TaskChainsV3 == nil {
		sm.TaskChainsV3 = make(map[string]*TaskChainV3)
	}
}

// persistV3Chain 持久化协议任务链到 DB 并追加事件
func persistV3Chain(ctx context.Context, sm *SessionManager, chain *TaskChainV3, eventType, phaseID, subID, payload string) error {
	if sm.Memory == nil {
		return nil // 无记忆层时跳过持久化
	}

	phasesJSON, err := chain.MarshalPhases()
	if err != nil {
		return err
	}

	rec := &core.TaskChainRecord{
		TaskID:       chain.TaskID,
		Description:  chain.Description,
		Protocol:     chain.Protocol,
		Status:       chain.Status,
		PhasesJSON:   phasesJSON,
		CurrentPhase: chain.CurrentPhase,
		ReinitCount:  chain.ReinitCount,
	}
	if err := sm.Memory.SaveTaskChain(ctx, rec); err != nil {
		return err
	}

	if eventType != "" {
		evt := &core.TaskChainEvent{
			TaskID:    chain.TaskID,
			PhaseID:   phaseID,
			SubID:     subID,
			EventType: eventType,
			Payload:   payload,
		}
		if _, err := sm.Memory.AppendTaskChainEvent(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}

// getOrLoadV3Chain 从内存获取协议链，不存在则从 DB 加载
func getOrLoadV3Chain(ctx context.Context, sm *SessionManager, taskID string) (*TaskChainV3, error) {
	ensureV3Map(sm)

	if chain, ok := sm.TaskChainsV3[taskID]; ok {
		return chain, nil
	}

	// 尝试从 DB 加载
	if sm.Memory == nil {
		return nil, fmt.Errorf("任务 %s 不存在（内存中无记录，记忆层未初始化）", taskID)
	}

	rec, err := sm.Memory.LoadTaskChain(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("加载任务 %s 失败: %w", taskID, err)
	}
	if rec == nil {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	phases, err := UnmarshalPhases(rec.PhasesJSON)
	if err != nil {
		return nil, fmt.Errorf("反序列化 phases 失败: %w", err)
	}

	chain := &TaskChainV3{
		TaskID:       rec.TaskID,
		Description:  rec.Description,
		Protocol:     rec.Protocol,
		Status:       rec.Status,
		Phases:       phases,
		CurrentPhase: rec.CurrentPhase,
		ReinitCount:  rec.ReinitCount,
	}
	sm.TaskChainsV3[taskID] = chain
	return chain, nil
}

// parsePhasesFromArgs 从 map 参数解析 Phase 列表
func parsePhasesFromArgs(phaseMaps []map[string]interface{}) ([]Phase, error) {
	if len(phaseMaps) == 0 {
		return nil, fmt.Errorf("phases 不能为空")
	}

	phases := make([]Phase, 0, len(phaseMaps))
	for _, pm := range phaseMaps {
		p := Phase{
			Status: PhasePending,
		}

		if v, ok := pm["id"]; ok {
			p.ID = fmt.Sprintf("%v", v)
		} else {
			return nil, fmt.Errorf("phase 缺少 id 字段")
		}
		if v, ok := pm["name"]; ok {
			p.Name = fmt.Sprintf("%v", v)
		} else {
			p.Name = p.ID
		}
		if v, ok := pm["type"]; ok {
			p.Type = PhaseType(fmt.Sprintf("%v", v))
		} else {
			p.Type = PhaseExecute
		}
		if v, ok := pm["input"]; ok {
			p.Input = fmt.Sprintf("%v", v)
		}
		if v, ok := pm["on_pass"]; ok {
			p.OnPass = fmt.Sprintf("%v", v)
		}
		if v, ok := pm["on_fail"]; ok {
			p.OnFail = fmt.Sprintf("%v", v)
		}
		if v, ok := pm["max_retries"]; ok {
			if n, ok := v.(float64); ok {
				p.MaxRetries = int(n)
			}
		}

		phases = append(phases, p)
	}
	return phases, nil
}

// parseSubTasksFromArgs 从 map 参数解析 SubTask 列表
func parseSubTasksFromArgs(subMaps []map[string]interface{}) ([]SubTask, error) {
	if len(subMaps) == 0 {
		return nil, fmt.Errorf("sub_tasks 不能为空")
	}

	subs := make([]SubTask, 0, len(subMaps))
	for i, sm := range subMaps {
		st := SubTask{
			Status: SubTaskPending,
		}
		if v, ok := sm["name"]; ok {
			st.Name = fmt.Sprintf("%v", v)
		} else {
			return nil, fmt.Errorf("sub_task[%d] 缺少 name 字段", i)
		}
		if v, ok := sm["id"]; ok {
			st.ID = fmt.Sprintf("%v", v)
		} else {
			st.ID = fmt.Sprintf("sub_%03d", i+1)
		}
		if v, ok := sm["verify"]; ok {
			st.Verify = fmt.Sprintf("%v", v)
		}
		subs = append(subs, st)
	}
	return subs, nil
}

// ========== Mode Handlers ==========

// initTaskChainV3 初始化协议任务链
func initTaskChainV3(ctx context.Context, sm *SessionManager, args TaskChainArgs) (*mcp.CallToolResult, error) {
	if args.TaskID == "" {
		return mcp.NewToolResultError("init 模式需要 task_id 参数"), nil
	}

	ensureV3Map(sm)

	// 解析 phases
	var phases []Phase
	var err error
	protocol := strings.TrimSpace(args.Protocol)

	if args.Phases != nil {
		phaseMaps, convErr := convertToMapSlice(args.Phases)
		if convErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("处理 phases 参数失败: %v", convErr)), nil
		}
		// 手动定义 phases
		phases, err = parsePhasesFromArgs(phaseMaps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("解析 phases 失败: %v", err)), nil
		}
		if protocol == "" {
			protocol = "custom"
		}
	} else {
		// 从协议生成
		if protocol == "" {
			protocol = "linear"
		}
		phases, err = buildPhasesFromProtocol(protocol, args.Description)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	// 检测是否为 re-init（任务链已存在）
	reinitCount := 0
	if existing, ok := sm.TaskChainsV3[args.TaskID]; ok {
		reinitCount = existing.ReinitCount + 1
		if reinitCount > 1 {
			return mcp.NewToolResultError(fmt.Sprintf(
				"任务 '%s' 已 re-init %d 次，自审升级：请停下来向用户说明当前问题并询问如何继续。",
				args.TaskID, existing.ReinitCount,
			)), nil
		}
	}

	chain := &TaskChainV3{
		TaskID:      args.TaskID,
		Description: args.Description,
		Protocol:    protocol,
		Status:      "running",
		Phases:      phases,
		ReinitCount: reinitCount,
	}

	sm.TaskChainsV3[args.TaskID] = chain

	// 持久化
	if err := persistV3Chain(ctx, sm, chain, "init", "", "", args.Description); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("持久化失败: %v", err)), nil
	}

	// 自动开始第一个阶段
	if len(phases) > 0 {
		firstPhase := phases[0].ID
		if err := chain.StartPhase(firstPhase); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("启动首阶段失败: %v", err)), nil
		}
		_ = persistV3Chain(ctx, sm, chain, "start", firstPhase, "", "")
	}

	return mcp.NewToolResultText(renderV3InitResult(chain)), nil
}

// startPhaseV3 开始协议阶段
func startPhaseV3(ctx context.Context, sm *SessionManager, args TaskChainArgs) (*mcp.CallToolResult, error) {
	if args.TaskID == "" {
		return mcp.NewToolResultError("start 模式需要 task_id 参数"), nil
	}
	if args.PhaseID == "" {
		return mcp.NewToolResultError("协议 start 模式需要 phase_id 参数"), nil
	}

	chain, err := getOrLoadV3Chain(ctx, sm, args.TaskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := chain.StartPhase(args.PhaseID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	_ = persistV3Chain(ctx, sm, chain, "start", args.PhaseID, "", "")

	p := chain.findPhase(args.PhaseID)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【Phase '%s' 开始】%s\n", p.ID, p.Name))
	sb.WriteString(fmt.Sprintf("类型: %s\n", p.Type))
	if p.Input != "" {
		sb.WriteString(fmt.Sprintf("建议调用: %s\n", p.Input))
	}
	sb.WriteString(fmt.Sprintf("\n完成后调用:\n"))
	switch p.Type {
	case PhaseGate:
		sb.WriteString(fmt.Sprintf("  task_chain(mode=\"complete\", task_id=\"%s\", phase_id=\"%s\", result=\"pass|fail\", summary=\"...\")\n", args.TaskID, args.PhaseID))
	case PhaseLoop:
		sb.WriteString(fmt.Sprintf("  先 spawn 子任务:\n  task_chain(mode=\"spawn\", task_id=\"%s\", phase_id=\"%s\", sub_tasks=[...])\n", args.TaskID, args.PhaseID))
	default:
		sb.WriteString(fmt.Sprintf("  task_chain(mode=\"complete\", task_id=\"%s\", phase_id=\"%s\", summary=\"...\")\n", args.TaskID, args.PhaseID))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// completePhaseV3 完成协议阶段（dispatch execute/gate）
func completePhaseV3(ctx context.Context, sm *SessionManager, args TaskChainArgs) (*mcp.CallToolResult, error) {
	if args.TaskID == "" {
		return mcp.NewToolResultError("complete 模式需要 task_id 参数"), nil
	}
	if args.PhaseID == "" {
		return mcp.NewToolResultError("协议 complete 模式需要 phase_id 参数"), nil
	}
	if args.Summary == "" {
		return mcp.NewToolResultError("complete 模式必须提供 summary"), nil
	}

	chain, err := getOrLoadV3Chain(ctx, sm, args.TaskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	p := chain.findPhase(args.PhaseID)
	if p == nil {
		return mcp.NewToolResultError(fmt.Sprintf("phase '%s' not found", args.PhaseID)), nil
	}

	var sb strings.Builder

	switch p.Type {
	case PhaseGate:
		if args.Result == "" {
			return mcp.NewToolResultError("gate 阶段必须提供 result (pass/fail)"), nil
		}
		nextID, retryInfo, err := chain.CompleteGate(args.PhaseID, args.Result, args.Summary)
		if err != nil {
			_ = persistV3Chain(ctx, sm, chain, "fail", args.PhaseID, "", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		payload, _ := json.Marshal(map[string]string{"result": args.Result, "summary": args.Summary})
		_ = persistV3Chain(ctx, sm, chain, "complete", args.PhaseID, "", string(payload))

		sb.WriteString(fmt.Sprintf("【Gate '%s' 完成】结果: %s\n", args.PhaseID, args.Result))
		sb.WriteString(fmt.Sprintf("Summary: %s\n\n", args.Summary))
		if retryInfo != "" {
			sb.WriteString(fmt.Sprintf("⚠️ %s\n", retryInfo))
		}
		if nextID != "" {
			sb.WriteString(renderV3NextPhaseHint(chain, args.TaskID, nextID))
		} else if chain.IsFinished() {
			chain.Status = "finished"
			_ = persistV3Chain(ctx, sm, chain, "finish", "", "", "")
			sb.WriteString("✅ 所有阶段已完成。\n")
			sb.WriteString(fmt.Sprintf("  task_chain(mode=\"finish\", task_id=\"%s\")\n", args.TaskID))
		}

	case PhaseExecute:
		nextID, err := chain.CompleteExecute(args.PhaseID, args.Summary)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		payload, _ := json.Marshal(map[string]string{"summary": args.Summary})
		_ = persistV3Chain(ctx, sm, chain, "complete", args.PhaseID, "", string(payload))

		sb.WriteString(fmt.Sprintf("【Phase '%s' 完成】%s\n", args.PhaseID, p.Name))
		sb.WriteString(fmt.Sprintf("Summary: %s\n\n", args.Summary))
		if nextID != "" {
			sb.WriteString(renderV3NextPhaseHint(chain, args.TaskID, nextID))
		} else if chain.IsFinished() {
			chain.Status = "finished"
			_ = persistV3Chain(ctx, sm, chain, "finish", "", "", "")
			sb.WriteString("✅ 所有阶段已完成。\n")
			sb.WriteString(fmt.Sprintf("  task_chain(mode=\"finish\", task_id=\"%s\")\n", args.TaskID))
		}

	case PhaseLoop:
		// loop 阶段的 complete 由子任务全部完成后自动触发，这里处理手动 complete
		p.Status = PhasePassed
		p.Summary = args.Summary
		payload, _ := json.Marshal(map[string]string{"summary": args.Summary})
		_ = persistV3Chain(ctx, sm, chain, "complete", args.PhaseID, "", string(payload))

		sb.WriteString(fmt.Sprintf("【Loop '%s' 完成】%s\n", args.PhaseID, p.Name))
		sb.WriteString(fmt.Sprintf("Summary: %s\n\n", args.Summary))
		next := chain.nextPhaseAfter(args.PhaseID)
		if next != nil {
			sb.WriteString(renderV3NextPhaseHint(chain, args.TaskID, next.ID))
		} else if chain.IsFinished() {
			chain.Status = "finished"
			_ = persistV3Chain(ctx, sm, chain, "finish", "", "", "")
			sb.WriteString("✅ 所有阶段已完成。\n")
		}

	default:
		return mcp.NewToolResultError(fmt.Sprintf("未知阶段类型: %s", p.Type)), nil
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// spawnSubTasksV3 在 loop 阶段生成子任务
func spawnSubTasksV3(ctx context.Context, sm *SessionManager, args TaskChainArgs) (*mcp.CallToolResult, error) {
	if args.TaskID == "" {
		return mcp.NewToolResultError("spawn 模式需要 task_id 参数"), nil
	}
	if args.PhaseID == "" {
		return mcp.NewToolResultError("spawn 模式需要 phase_id 参数"), nil
	}
	if args.SubTasks == nil {
		return mcp.NewToolResultError("spawn 模式需要 sub_tasks 参数"), nil
	}

	chain, err := getOrLoadV3Chain(ctx, sm, args.TaskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subMaps, convErr := convertToMapSlice(args.SubTasks)
	if convErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("处理 sub_tasks 参数失败: %v", convErr)), nil
	}

	subs, err := parseSubTasksFromArgs(subMaps)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("解析 sub_tasks 失败: %v", err)), nil
	}

	if err := chain.SpawnSubTasks(args.PhaseID, subs); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	payload, _ := json.Marshal(subs)
	_ = persistV3Chain(ctx, sm, chain, "spawn", args.PhaseID, "", string(payload))

	// 自动开始第一个子任务
	firstSub := chain.NextPendingSubTask(args.PhaseID)
	if firstSub != nil {
		_ = chain.StartSubTask(args.PhaseID, firstSub.ID)
		_ = persistV3Chain(ctx, sm, chain, "start_sub", args.PhaseID, firstSub.ID, "")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("已创建 %d 个子任务:\n", len(subs)))
	for _, s := range subs {
		status := string(s.Status)
		if firstSub != nil && s.ID == firstSub.ID {
			status = "active"
		}
		sb.WriteString(fmt.Sprintf("  • %s: %s [%s]\n", s.ID, s.Name, status))
	}
	if firstSub != nil {
		sb.WriteString(fmt.Sprintf("\n→ 开始执行: %s「%s」\n", firstSub.ID, firstSub.Name))
		if firstSub.Verify != "" {
			sb.WriteString(fmt.Sprintf("  验证命令: %s\n", firstSub.Verify))
		}
		sb.WriteString(fmt.Sprintf("\n完成后调用:\n  task_chain(mode=\"complete_sub\", task_id=\"%s\", phase_id=\"%s\", sub_id=\"%s\", result=\"pass|fail\", summary=\"...\")\n",
			args.TaskID, args.PhaseID, firstSub.ID))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// completeSubTaskV3 完成子任务
func completeSubTaskV3(ctx context.Context, sm *SessionManager, args TaskChainArgs) (*mcp.CallToolResult, error) {
	if args.TaskID == "" {
		return mcp.NewToolResultError("complete_sub 模式需要 task_id 参数"), nil
	}
	if args.PhaseID == "" {
		return mcp.NewToolResultError("complete_sub 模式需要 phase_id 参数"), nil
	}
	if args.SubID == "" {
		return mcp.NewToolResultError("complete_sub 模式需要 sub_id 参数"), nil
	}
	if args.Summary == "" {
		return mcp.NewToolResultError("complete_sub 模式必须提供 summary"), nil
	}

	result := args.Result
	if result == "" {
		result = "pass"
	}

	chain, err := getOrLoadV3Chain(ctx, sm, args.TaskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	allDone, err := chain.CompleteSubTask(args.PhaseID, args.SubID, result, args.Summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	payload, _ := json.Marshal(map[string]string{"result": result, "summary": args.Summary})
	_ = persistV3Chain(ctx, sm, chain, "complete_sub", args.PhaseID, args.SubID, string(payload))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【子任务 %s 完成】结果: %s\n", args.SubID, result))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", args.Summary))

	if allDone {
		sb.WriteString(fmt.Sprintf("✅ Loop '%s' 所有子任务已完成\n", args.PhaseID))
		next := chain.nextPhaseAfter(args.PhaseID)
		if next != nil {
			sb.WriteString(renderV3NextPhaseHint(chain, args.TaskID, next.ID))
		} else if chain.IsFinished() {
			chain.Status = "finished"
			_ = persistV3Chain(ctx, sm, chain, "finish", "", "", "")
			sb.WriteString("✅ 所有阶段已完成。\n")
		}
	} else {
		// 自动开始下一个子任务
		nextSub := chain.NextPendingSubTask(args.PhaseID)
		if nextSub != nil {
			_ = chain.StartSubTask(args.PhaseID, nextSub.ID)
			_ = persistV3Chain(ctx, sm, chain, "start_sub", args.PhaseID, nextSub.ID, "")
			sb.WriteString(fmt.Sprintf("→ 下一个子任务: %s「%s」\n", nextSub.ID, nextSub.Name))
			if nextSub.Verify != "" {
				sb.WriteString(fmt.Sprintf("  验证命令: %s\n", nextSub.Verify))
			}
			sb.WriteString(fmt.Sprintf("\n  task_chain(mode=\"complete_sub\", task_id=\"%s\", phase_id=\"%s\", sub_id=\"%s\", result=\"pass|fail\", summary=\"...\")\n",
				args.TaskID, args.PhaseID, nextSub.ID))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// resumeTaskChainV3 从 DB 恢复协议任务链
func resumeTaskChainV3(ctx context.Context, sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	if taskID == "" {
		return mcp.NewToolResultError("resume 模式需要 task_id 参数"), nil
	}

	chain, err := getOrLoadV3Chain(ctx, sm, taskID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(renderV3StatusJSON(chain)), nil
}

// finishChainV3 完成协议任务链
func finishChainV3(ctx context.Context, sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	chain, err := getOrLoadV3Chain(ctx, sm, taskID)
	if err != nil {
		return nil, nil // 协议链不存在，不处理
	}

	chain.Status = "finished"
	_ = persistV3Chain(ctx, sm, chain, "finish", "", "", "")
	return nil, nil // 由调用方统一输出
}

// ========== 渲染辅助 ==========

func renderV3InitResult(chain *TaskChainV3) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("协议任务链已初始化: %s\n", chain.TaskID))
	sb.WriteString(fmt.Sprintf("协议: %s\n", chain.Protocol))
	sb.WriteString(fmt.Sprintf("阶段数: %d\n\n", len(chain.Phases)))

	for _, p := range chain.Phases {
		marker := "○"
		if p.Status == PhaseActive {
			marker = "▶"
		}
		typeTag := ""
		if p.Type == PhaseGate {
			typeTag = " [gate]"
		} else if p.Type == PhaseLoop {
			typeTag = " [loop]"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s%s\n", marker, p.ID, p.Name, typeTag))
	}

	if chain.CurrentPhase != "" {
		p := chain.findPhase(chain.CurrentPhase)
		if p != nil {
			sb.WriteString(fmt.Sprintf("\n→ 当前阶段: %s「%s」\n", p.ID, p.Name))
			if p.Input != "" {
				sb.WriteString(fmt.Sprintf("  建议调用: %s\n", p.Input))
			}
			sb.WriteString(fmt.Sprintf("\n完成后调用:\n  task_chain(mode=\"complete\", task_id=\"%s\", phase_id=\"%s\", summary=\"...\")\n",
				chain.TaskID, p.ID))
		}
	}

	return sb.String()
}

func renderV3NextPhaseHint(chain *TaskChainV3, taskID, nextID string) string {
	p := chain.findPhase(nextID)
	if p == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("→ 下一阶段: %s「%s」(%s)\n", p.ID, p.Name, p.Type))
	sb.WriteString(fmt.Sprintf("  task_chain(mode=\"start\", task_id=\"%s\", phase_id=\"%s\")\n", taskID, nextID))

	// 自审提示
	sb.WriteString("\n🔍 自审：当前发现是否与初始目标一致？\n")
	sb.WriteString("  • 一切正常 → 继续执行上方 start 指令\n")
	sb.WriteString("  • 发现重大偏差，信息足够 → 重新 init（覆盖当前链）\n")
	sb.WriteString("  • 发现重大偏差，信息不足 → 先调工具补充信息，再决定是否 re-init\n")
	if chain.ReinitCount > 0 {
		sb.WriteString(fmt.Sprintf("  ⚠️  已 re-init %d 次，若仍有问题请停下询问用户\n", chain.ReinitCount))
	}

	return sb.String()
}

func renderV3StatusJSON(chain *TaskChainV3) string {
	type subTaskView struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Status  string `json:"status"`
		Summary string `json:"summary,omitempty"`
	}
	type phaseView struct {
		ID         string        `json:"id"`
		Name       string        `json:"name"`
		Type       string        `json:"type"`
		Status     string        `json:"status"`
		Summary    string        `json:"summary,omitempty"`
		RetryCount int           `json:"retry_count,omitempty"`
		SubTotal   int           `json:"sub_total,omitempty"`
		SubDone    int           `json:"sub_done,omitempty"`
		SubTasks   []subTaskView `json:"sub_tasks,omitempty"`
	}
	type statusView struct {
		TaskID       string      `json:"task_id"`
		Description  string      `json:"description"`
		Protocol     string      `json:"protocol"`
		Status       string      `json:"status"`
		CurrentPhase string      `json:"current_phase"`
		Phases       []phaseView `json:"phases"`
	}

	sv := statusView{
		TaskID:       chain.TaskID,
		Description:  chain.Description,
		Protocol:     chain.Protocol,
		Status:       chain.Status,
		CurrentPhase: chain.CurrentPhase,
	}

	for _, p := range chain.Phases {
		pv := phaseView{
			ID:     p.ID,
			Name:   p.Name,
			Type:   string(p.Type),
			Status: string(p.Status),
		}
		if p.Summary != "" {
			pv.Summary = p.Summary
		}
		if p.Type == PhaseGate && p.RetryCount > 0 {
			pv.RetryCount = p.RetryCount
		}
		if p.Type == PhaseLoop && len(p.SubTasks) > 0 {
			pv.SubTotal = len(p.SubTasks)
			var stViews []subTaskView
			for _, s := range p.SubTasks {
				if s.Status == SubTaskPassed || s.Status == SubTaskFailed {
					pv.SubDone++
				}
				stv := subTaskView{
					ID:     s.ID,
					Name:   s.Name,
					Status: string(s.Status),
				}
				if s.Summary != "" {
					stv.Summary = s.Summary
				}
				stViews = append(stViews, stv)
			}
			pv.SubTasks = stViews
		}
		sv.Phases = append(sv.Phases, pv)
	}

	data, _ := json.MarshalIndent(sv, "", "  ")
	return string(data)
}

// buildPhasesFromProtocol 根据协议名称生成 Phase 列表（Phase 4 会扩展完整协议）
func buildPhasesFromProtocol(protocol, description string) ([]Phase, error) {
	switch protocol {
	case "linear":
		// linear 协议：单个 execute 阶段
		return []Phase{
			{ID: "main", Name: "执行", Type: PhaseExecute, Status: PhasePending, Input: description},
		}, nil

	case "develop":
		return []Phase{
			{ID: "analyze", Name: "需求分析与拆解", Type: PhaseExecute, Status: PhasePending},
			{ID: "plan_gate", Name: "拆解是否充分？", Type: PhaseGate, Status: PhasePending, OnPass: "implement", OnFail: "analyze", MaxRetries: 2},
			{ID: "implement", Name: "逐个实现子任务", Type: PhaseLoop, Status: PhasePending},
			{ID: "verify_gate", Name: "集成验证", Type: PhaseGate, Status: PhasePending, OnPass: "finalize", OnFail: "implement", MaxRetries: 3},
			{ID: "finalize", Name: "收尾归档", Type: PhaseExecute, Status: PhasePending},
		}, nil

	case "debug":
		return []Phase{
			{ID: "reproduce", Name: "复现问题", Type: PhaseExecute, Status: PhasePending},
			{ID: "locate", Name: "定位根因", Type: PhaseExecute, Status: PhasePending},
			{ID: "fix", Name: "逐个修复", Type: PhaseLoop, Status: PhasePending},
			{ID: "verify_gate", Name: "验证修复", Type: PhaseGate, Status: PhasePending, OnPass: "finalize", OnFail: "fix", MaxRetries: 3},
			{ID: "finalize", Name: "收尾归档", Type: PhaseExecute, Status: PhasePending},
		}, nil

	case "refactor":
		return []Phase{
			{ID: "baseline", Name: "基线验证", Type: PhaseExecute, Status: PhasePending},
			{ID: "analyze", Name: "分析重构范围", Type: PhaseExecute, Status: PhasePending},
			{ID: "refactor", Name: "逐步重构", Type: PhaseLoop, Status: PhasePending},
			{ID: "verify_gate", Name: "回归验证", Type: PhaseGate, Status: PhasePending, OnPass: "finalize", OnFail: "refactor", MaxRetries: 3},
			{ID: "finalize", Name: "收尾归档", Type: PhaseExecute, Status: PhasePending},
		}, nil

	default:
		return nil, fmt.Errorf("未知协议: %s（可用: linear, develop, debug, refactor）", protocol)
	}
}

// isV3Task 判断任务是否为协议任务链
func isV3Task(sm *SessionManager, taskID string) bool {
	ensureV3Map(sm)
	_, ok := sm.TaskChainsV3[taskID]
	return ok
}

// isV3TaskInDB 检查 DB 中是否存在协议任务链
func isV3TaskInDB(ctx context.Context, sm *SessionManager, taskID string) bool {
	if sm.Memory == nil {
		return false
	}
	rec, err := sm.Memory.LoadTaskChain(ctx, taskID)
	return err == nil && rec != nil
}

// renderProtocolList 列出可用协议
func renderProtocolList() string {
	protocols := []struct {
		Name string
		Desc string
		Flow string
	}{
		{"linear", "纯线性执行（默认）", "main"},
		{"develop", "大工程开发协议", "analyze → plan_gate → implement(loop) → verify_gate → finalize"},
		{"debug", "问题排查协议", "reproduce → locate → fix(loop) → verify_gate → finalize"},
		{"refactor", "大范围重构协议", "baseline → analyze → refactor(loop) → verify_gate → finalize"},
	}

	var sb strings.Builder
	sb.WriteString("可用协议:\n\n")
	for _, p := range protocols {
		sb.WriteString(fmt.Sprintf("  %s - %s\n    %s\n\n", p.Name, p.Desc, p.Flow))
	}
	sb.WriteString("使用方式:\n")
	sb.WriteString("  task_chain(mode=\"init\", task_id=\"...\", protocol=\"develop\", description=\"...\")\n")
	sb.WriteString("\n协议选择:\n")
	sb.WriteString("  - 不传 protocol（默认 linear）：任务步骤明确，线性推进即可\n")
	sb.WriteString("  - protocol=\"develop\"：跨模块开发，需要拆解子任务并逐个验证\n")
	sb.WriteString("  - protocol=\"debug\"：问题复现→定位→修复→验证，可能需要多轮重试\n")
	sb.WriteString("  - protocol=\"refactor\"：大范围重构，需要基线验证和逐步替换\n")
	return sb.String()
}
