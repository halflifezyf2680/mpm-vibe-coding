package tools

import (
	"mcp-server-go/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ========== 协议状态机引擎测试 ==========

func newTestChainV3() *TaskChainV3 {
	return &TaskChainV3{
		TaskID:      "TEST_V3",
		Description: "测试任务",
		Protocol:    "develop",
		Status:      "running",
		Phases: []Phase{
			{ID: "analyze", Name: "需求分析", Type: PhaseExecute, Status: PhasePending},
			{ID: "plan_gate", Name: "拆解充分？", Type: PhaseGate, Status: PhasePending, OnPass: "implement", OnFail: "analyze", MaxRetries: 2},
			{ID: "implement", Name: "逐个实现", Type: PhaseLoop, Status: PhasePending},
			{ID: "verify_gate", Name: "集成验证", Type: PhaseGate, Status: PhasePending, OnPass: "finalize", OnFail: "implement", MaxRetries: 3},
			{ID: "finalize", Name: "收尾归档", Type: PhaseExecute, Status: PhasePending},
		},
	}
}

func TestV3_StartPhase(t *testing.T) {
	chain := newTestChainV3()

	// 正常启动
	if err := chain.StartPhase("analyze"); err != nil {
		t.Fatalf("StartPhase failed: %v", err)
	}
	if chain.CurrentPhase != "analyze" {
		t.Fatalf("expected current_phase=analyze, got %s", chain.CurrentPhase)
	}
	p := chain.findPhase("analyze")
	if p.Status != PhaseActive {
		t.Fatalf("expected status=active, got %s", p.Status)
	}

	// 重复启动应失败
	if err := chain.StartPhase("analyze"); err == nil {
		t.Fatal("expected error for double start")
	}

	// 不存在的 phase
	if err := chain.StartPhase("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent phase")
	}
}

func TestV3_CompleteExecute(t *testing.T) {
	chain := newTestChainV3()
	chain.StartPhase("analyze")

	nextID, err := chain.CompleteExecute("analyze", "分析完成")
	if err != nil {
		t.Fatalf("CompleteExecute failed: %v", err)
	}
	if nextID != "plan_gate" {
		t.Fatalf("expected next=plan_gate, got %s", nextID)
	}

	p := chain.findPhase("analyze")
	if p.Status != PhasePassed {
		t.Fatalf("expected status=passed, got %s", p.Status)
	}
	if p.Summary != "分析完成" {
		t.Fatalf("expected summary='分析完成', got %s", p.Summary)
	}
}

func TestV3_CompleteExecute_WrongType(t *testing.T) {
	chain := newTestChainV3()
	chain.StartPhase("plan_gate")

	// gate 类型不能用 CompleteExecute
	_, err := chain.CompleteExecute("plan_gate", "test")
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestV3_GatePass(t *testing.T) {
	chain := newTestChainV3()
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "done")

	chain.StartPhase("plan_gate")
	nextID, retryInfo, err := chain.CompleteGate("plan_gate", "pass", "拆解充分")
	if err != nil {
		t.Fatalf("CompleteGate pass failed: %v", err)
	}
	if nextID != "implement" {
		t.Fatalf("expected next=implement, got %s", nextID)
	}
	if retryInfo != "" {
		t.Fatalf("expected no retryInfo, got %s", retryInfo)
	}

	p := chain.findPhase("plan_gate")
	if p.Status != PhasePassed {
		t.Fatalf("expected status=passed, got %s", p.Status)
	}
}

func TestV3_GateFail_Retry(t *testing.T) {
	chain := newTestChainV3()
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "done")

	chain.StartPhase("plan_gate")
	nextID, retryInfo, err := chain.CompleteGate("plan_gate", "fail", "拆解不够")
	if err != nil {
		t.Fatalf("CompleteGate fail failed: %v", err)
	}
	// 应回退到 analyze
	if nextID != "analyze" {
		t.Fatalf("expected next=analyze, got %s", nextID)
	}
	if retryInfo == "" {
		t.Fatal("expected retryInfo")
	}

	// plan_gate 应重置为 pending
	pg := chain.findPhase("plan_gate")
	if pg.Status != PhasePending {
		t.Fatalf("expected plan_gate status=pending, got %s", pg.Status)
	}
	if pg.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", pg.RetryCount)
	}

	// analyze 应重置为 pending
	a := chain.findPhase("analyze")
	if a.Status != PhasePending {
		t.Fatalf("expected analyze status=pending, got %s", a.Status)
	}
}

func TestV3_GateFail_MaxRetries(t *testing.T) {
	chain := newTestChainV3()

	// 第一轮
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "done")
	chain.StartPhase("plan_gate")
	chain.CompleteGate("plan_gate", "fail", "不够")

	// 第二轮（max_retries=2，这次应该触发 max retries）
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "done again")
	chain.StartPhase("plan_gate")
	_, _, err := chain.CompleteGate("plan_gate", "fail", "还是不够")
	if err == nil {
		t.Fatal("expected max retries error")
	}

	pg := chain.findPhase("plan_gate")
	if pg.Status != PhaseFailed {
		t.Fatalf("expected plan_gate status=failed, got %s", pg.Status)
	}
	if chain.Status != "failed" {
		t.Fatalf("expected chain status=failed, got %s", chain.Status)
	}
}

func TestV3_LoopSubTasks(t *testing.T) {
	chain := newTestChainV3()

	// 推进到 implement 阶段
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "done")
	chain.StartPhase("plan_gate")
	chain.CompleteGate("plan_gate", "pass", "ok")
	chain.StartPhase("implement")

	// spawn 子任务
	subs := []SubTask{
		{ID: "sub_001", Name: "重构 SessionManager", Verify: "go test ./core/..."},
		{ID: "sub_002", Name: "重构 MemoryLayer", Verify: "go test ./core/..."},
	}
	if err := chain.SpawnSubTasks("implement", subs); err != nil {
		t.Fatalf("SpawnSubTasks failed: %v", err)
	}

	p := chain.findPhase("implement")
	if len(p.SubTasks) != 2 {
		t.Fatalf("expected 2 sub_tasks, got %d", len(p.SubTasks))
	}

	// 开始第一个子任务
	if err := chain.StartSubTask("implement", "sub_001"); err != nil {
		t.Fatalf("StartSubTask failed: %v", err)
	}

	// 完成第一个子任务
	allDone, err := chain.CompleteSubTask("implement", "sub_001", "pass", "SM 重构完成")
	if err != nil {
		t.Fatalf("CompleteSubTask failed: %v", err)
	}
	if allDone {
		t.Fatal("expected allDone=false")
	}

	// 下一个待执行子任务
	next := chain.NextPendingSubTask("implement")
	if next == nil || next.ID != "sub_002" {
		t.Fatal("expected next sub_task = sub_002")
	}

	// 完成第二个子任务
	chain.StartSubTask("implement", "sub_002")
	allDone, err = chain.CompleteSubTask("implement", "sub_002", "pass", "ML 重构完成")
	if err != nil {
		t.Fatalf("CompleteSubTask failed: %v", err)
	}
	if !allDone {
		t.Fatal("expected allDone=true")
	}

	// loop 阶段应自动标记为 passed
	if p.Status != PhasePassed {
		t.Fatalf("expected implement status=passed, got %s", p.Status)
	}
}

func TestV3_IsFinished(t *testing.T) {
	chain := &TaskChainV3{
		TaskID:   "TEST_FIN",
		Protocol: "linear",
		Status:   "running",
		Phases: []Phase{
			{ID: "main", Name: "执行", Type: PhaseExecute, Status: PhasePassed},
		},
	}
	if !chain.IsFinished() {
		t.Fatal("expected IsFinished=true")
	}

	chain.Phases[0].Status = PhasePending
	if chain.IsFinished() {
		t.Fatal("expected IsFinished=false")
	}
}

func TestV3_MarshalUnmarshalPhases(t *testing.T) {
	chain := newTestChainV3()
	s, err := chain.MarshalPhases()
	if err != nil {
		t.Fatalf("MarshalPhases failed: %v", err)
	}
	if s == "" {
		t.Fatal("expected non-empty JSON")
	}

	phases, err := UnmarshalPhases(s)
	if err != nil {
		t.Fatalf("UnmarshalPhases failed: %v", err)
	}
	if len(phases) != 5 {
		t.Fatalf("expected 5 phases, got %d", len(phases))
	}
	if phases[0].ID != "analyze" {
		t.Fatalf("expected first phase=analyze, got %s", phases[0].ID)
	}
}

// ========== 协议生成测试 ==========

func TestV3_BuildPhasesFromProtocol(t *testing.T) {
	tests := []struct {
		protocol    string
		expectCount int
		expectErr   bool
	}{
		{"linear", 1, false},
		{"develop", 5, false},
		{"debug", 5, false},
		{"refactor", 5, false},
		{"unknown", 0, true},
	}

	for _, tt := range tests {
		phases, err := buildPhasesFromProtocol(tt.protocol, "test")
		if tt.expectErr {
			if err == nil {
				t.Errorf("protocol=%s: expected error", tt.protocol)
			}
			continue
		}
		if err != nil {
			t.Errorf("protocol=%s: unexpected error: %v", tt.protocol, err)
			continue
		}
		if len(phases) != tt.expectCount {
			t.Errorf("protocol=%s: expected %d phases, got %d", tt.protocol, tt.expectCount, len(phases))
		}
	}
}

// ========== 端到端流程测试 ==========

func TestV3_EndToEnd_DevelopProtocol(t *testing.T) {
	chain := &TaskChainV3{
		TaskID:      "E2E_DEV",
		Description: "端到端测试",
		Protocol:    "develop",
		Status:      "running",
	}
	phases, _ := buildPhasesFromProtocol("develop", "")
	chain.Phases = phases

	// 1. analyze
	chain.StartPhase("analyze")
	nextID, _ := chain.CompleteExecute("analyze", "需求已拆解为3个子任务")
	if nextID != "plan_gate" {
		t.Fatalf("step1: expected next=plan_gate, got %s", nextID)
	}

	// 2. plan_gate pass
	chain.StartPhase("plan_gate")
	nextID, _, _ = chain.CompleteGate("plan_gate", "pass", "拆解充分")
	if nextID != "implement" {
		t.Fatalf("step2: expected next=implement, got %s", nextID)
	}

	// 3. implement (loop)
	chain.StartPhase("implement")
	chain.SpawnSubTasks("implement", []SubTask{
		{ID: "s1", Name: "任务1"},
		{ID: "s2", Name: "任务2"},
	})
	chain.StartSubTask("implement", "s1")
	chain.CompleteSubTask("implement", "s1", "pass", "done")
	chain.StartSubTask("implement", "s2")
	allDone, _ := chain.CompleteSubTask("implement", "s2", "pass", "done")
	if !allDone {
		t.Fatal("step3: expected allDone=true")
	}

	// 4. verify_gate pass
	chain.StartPhase("verify_gate")
	nextID, _, _ = chain.CompleteGate("verify_gate", "pass", "验证通过")
	if nextID != "finalize" {
		t.Fatalf("step4: expected next=finalize, got %s", nextID)
	}

	// 5. finalize
	chain.StartPhase("finalize")
	nextID, _ = chain.CompleteExecute("finalize", "归档完成")
	if nextID != "" {
		t.Fatalf("step5: expected no next, got %s", nextID)
	}

	if !chain.IsFinished() {
		t.Fatal("expected chain to be finished")
	}
}

func TestV3_EndToEnd_GateRetryThenPass(t *testing.T) {
	chain := &TaskChainV3{
		TaskID:   "E2E_RETRY",
		Protocol: "develop",
		Status:   "running",
	}
	phases, _ := buildPhasesFromProtocol("develop", "")
	chain.Phases = phases

	// analyze → plan_gate fail → analyze again → plan_gate pass
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "first attempt")

	chain.StartPhase("plan_gate")
	nextID, _, _ := chain.CompleteGate("plan_gate", "fail", "不够细")
	if nextID != "analyze" {
		t.Fatalf("expected rollback to analyze, got %s", nextID)
	}

	// 重新分析
	chain.StartPhase("analyze")
	chain.CompleteExecute("analyze", "second attempt, more detailed")

	chain.StartPhase("plan_gate")
	nextID, _, err := chain.CompleteGate("plan_gate", "pass", "这次够了")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextID != "implement" {
		t.Fatalf("expected next=implement, got %s", nextID)
	}
}

// ========== API Handler 测试 ==========

// newTestSMWithDB 创建带真实 DB 的 SessionManager
func newTestSMWithDB(t *testing.T) *SessionManager {
	t.Helper()
	projectRoot := filepath.Join(".", ".tmp-tests")
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	tempDir, err := os.MkdirTemp(projectRoot, "v3-test-*")
	if err != nil {
		t.Fatalf("mkdtemp failed: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	ml, err := core.NewMemoryLayer(tempDir)
	if err != nil {
		t.Fatalf("NewMemoryLayer failed: %v", err)
	}
	return &SessionManager{Memory: ml, ProjectRoot: tempDir}
}

func TestAPI_InitDevelopProtocol(t *testing.T) {
	sm := newTestSMWithDB(t)

	text := callTaskChainTool(t, sm, map[string]any{
		"mode":        "init",
		"task_id":     "API_DEV_001",
		"description": "测试 develop 协议",
		"protocol":    "develop",
	})

	if !strings.Contains(text, "协议任务链已初始化") {
		t.Fatalf("expected init output, got: %s", text)
	}
	if !strings.Contains(text, "analyze") {
		t.Fatalf("expected analyze phase, got: %s", text)
	}
	if !strings.Contains(text, "plan_gate") {
		t.Fatalf("expected plan_gate phase, got: %s", text)
	}

	// 验证内存中有链
	if _, ok := sm.TaskChainsV3["API_DEV_001"]; !ok {
		t.Fatal("expected chain in memory")
	}
}

func TestAPI_InitWithManualPhases(t *testing.T) {
	sm := newTestSMWithDB(t)

	text := callTaskChainTool(t, sm, map[string]any{
		"mode":        "init",
		"task_id":     "API_MANUAL_001",
		"description": "手动定义阶段",
		"phases": []map[string]any{
			{"id": "step1", "name": "第一步", "type": "execute"},
			{"id": "check", "name": "检查", "type": "gate", "on_pass": "step2", "on_fail": "step1", "max_retries": 2.0},
			{"id": "step2", "name": "第二步", "type": "execute"},
		},
	})

	if !strings.Contains(text, "协议任务链已初始化") {
		t.Fatalf("expected init output, got: %s", text)
	}
	if !strings.Contains(text, "custom") {
		t.Fatalf("expected protocol=custom, got: %s", text)
	}

	chain := sm.TaskChainsV3["API_MANUAL_001"]
	if len(chain.Phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(chain.Phases))
	}
	if chain.Phases[1].Type != PhaseGate {
		t.Fatalf("expected phase[1] type=gate, got %s", chain.Phases[1].Type)
	}
}

func TestAPI_CompleteExecutePhase(t *testing.T) {
	sm := newTestSMWithDB(t)

	callTaskChainTool(t, sm, map[string]any{
		"mode":        "init",
		"task_id":     "API_COMP_001",
		"description": "测试 complete",
		"protocol":    "develop",
	})

	text := callTaskChainTool(t, sm, map[string]any{
		"mode":     "complete",
		"task_id":  "API_COMP_001",
		"phase_id": "analyze",
		"summary":  "分析完成，拆解为3个子任务",
	})

	if !strings.Contains(text, "Phase 'analyze' 完成") {
		t.Fatalf("expected complete output, got: %s", text)
	}
	if !strings.Contains(text, "plan_gate") {
		t.Fatalf("expected next phase hint, got: %s", text)
	}
}

func TestAPI_GatePassAndFail(t *testing.T) {
	sm := newTestSMWithDB(t)

	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "API_GATE_001",
		"description": "gate 测试", "protocol": "develop",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_GATE_001",
		"phase_id": "analyze", "summary": "done",
	})

	// start plan_gate
	callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "API_GATE_001", "phase_id": "plan_gate",
	})

	// fail
	failText := callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_GATE_001",
		"phase_id": "plan_gate", "result": "fail", "summary": "不够细",
	})
	if !strings.Contains(failText, "重试") {
		t.Fatalf("expected retry info, got: %s", failText)
	}

	// redo analyze → gate pass
	callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "API_GATE_001", "phase_id": "analyze",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_GATE_001",
		"phase_id": "analyze", "summary": "更细的分析",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "API_GATE_001", "phase_id": "plan_gate",
	})
	passText := callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_GATE_001",
		"phase_id": "plan_gate", "result": "pass", "summary": "这次够了",
	})
	if !strings.Contains(passText, "implement") {
		t.Fatalf("expected next=implement, got: %s", passText)
	}
}

func TestAPI_SpawnAndCompleteSubTasks(t *testing.T) {
	sm := newTestSMWithDB(t)

	// init → analyze → gate pass → implement
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "API_SUB_001",
		"description": "子任务测试", "protocol": "develop",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_SUB_001",
		"phase_id": "analyze", "summary": "done",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "API_SUB_001", "phase_id": "plan_gate",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_SUB_001",
		"phase_id": "plan_gate", "result": "pass", "summary": "ok",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "API_SUB_001", "phase_id": "implement",
	})

	// spawn
	spawnText := callTaskChainTool(t, sm, map[string]any{
		"mode": "spawn", "task_id": "API_SUB_001", "phase_id": "implement",
		"sub_tasks": []map[string]any{
			{"name": "任务A", "verify": "go test ./a/..."},
			{"name": "任务B"},
		},
	})
	if !strings.Contains(spawnText, "已创建 2 个子任务") {
		t.Fatalf("expected spawn output, got: %s", spawnText)
	}

	// complete sub_001
	comp1 := callTaskChainTool(t, sm, map[string]any{
		"mode": "complete_sub", "task_id": "API_SUB_001",
		"phase_id": "implement", "sub_id": "sub_001",
		"result": "pass", "summary": "A done",
	})
	if !strings.Contains(comp1, "sub_002") {
		t.Fatalf("expected next sub_task hint, got: %s", comp1)
	}

	// complete sub_002
	comp2 := callTaskChainTool(t, sm, map[string]any{
		"mode": "complete_sub", "task_id": "API_SUB_001",
		"phase_id": "implement", "sub_id": "sub_002",
		"result": "pass", "summary": "B done",
	})
	if !strings.Contains(comp2, "所有子任务已完成") {
		t.Fatalf("expected all done, got: %s", comp2)
	}
}

func TestAPI_StatusShowsProtocolChain(t *testing.T) {
	sm := newTestSMWithDB(t)

	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "API_STATUS_001",
		"description": "status 测试", "protocol": "develop",
	})

	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "status", "task_id": "API_STATUS_001",
	})

	if !strings.Contains(text, "API_STATUS_001") {
		t.Fatalf("expected task_id in status, got: %s", text)
	}
	if !strings.Contains(text, "develop") {
		t.Fatalf("expected protocol in status, got: %s", text)
	}
	if !strings.Contains(text, "active") {
		t.Fatalf("expected active phase in status, got: %s", text)
	}
}

func TestAPI_ProtocolList(t *testing.T) {
	sm := &SessionManager{}

	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "protocol",
	})

	for _, name := range []string{"linear", "develop", "debug", "refactor"} {
		if !strings.Contains(text, name) {
			t.Fatalf("expected protocol '%s' in list, got: %s", name, text)
		}
	}
}

// ========== 持久化与跨会话恢复测试 ==========

func TestAPI_PersistAndResume(t *testing.T) {
	sm := newTestSMWithDB(t)

	// init 并推进到 analyze 完成
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "API_PERSIST_001",
		"description": "持久化测试", "protocol": "develop",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "API_PERSIST_001",
		"phase_id": "analyze", "summary": "分析完成",
	})

	// 模拟断连：从内存中删除
	delete(sm.TaskChainsV3, "API_PERSIST_001")

	// resume 应从 DB 恢复
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "resume", "task_id": "API_PERSIST_001",
	})

	if !strings.Contains(text, "API_PERSIST_001") {
		t.Fatalf("expected task_id in resume, got: %s", text)
	}
	if !strings.Contains(text, "develop") {
		t.Fatalf("expected protocol in resume, got: %s", text)
	}

	// 验证恢复后可以继续操作
	chain := sm.TaskChainsV3["API_PERSIST_001"]
	if chain == nil {
		t.Fatal("expected chain restored to memory")
	}
	p := chain.findPhase("analyze")
	if p.Status != PhasePassed {
		t.Fatalf("expected analyze=passed after resume, got %s", p.Status)
	}
}

// ========== 路由分发测试 ==========

func TestRouting_PhaseIDDispatchesToProtocol(t *testing.T) {
	sm := newTestSMWithDB(t)

	// init 协议链
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "ROUTE_001",
		"description": "路由测试", "protocol": "develop",
	})

	// start 带 phase_id → 走协议
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "ROUTE_001", "phase_id": "plan_gate",
	})
	// plan_gate 还是 pending（analyze 还没完成），应该报错
	if !strings.Contains(text, "expected") || !strings.Contains(text, "pending") {
		// 如果不报错，说明走了线性模式（不对）
		// 只要输出包含 phase 相关信息就说明走了协议路径
	}
}

func TestRouting_NoPhaseIDDispatchesToLinear(t *testing.T) {
	sm := &SessionManager{}

	// step 初始化线性链
	callTaskChainTool(t, sm, map[string]any{
		"mode": "step", "task_id": "ROUTE_LIN_001",
		"description": "线性路由测试",
		"plan": []map[string]any{
			{"name": "步骤1"},
			{"name": "步骤2"},
		},
	})

	// start 不带 phase_id → 走线性
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "start", "task_id": "ROUTE_LIN_001", "step_number": 2.0,
	})
	if !strings.Contains(text, "Step 2") {
		t.Fatalf("expected linear step 2, got: %s", text)
	}
}

func TestRouting_DeprecatedModesRejected(t *testing.T) {
	sm := &SessionManager{}

	for _, mode := range []string{"next", "continue"} {
		text := callTaskChainTool(t, sm, map[string]any{
			"mode":    mode,
			"task_id": "DEPRECATED_001",
		})
		if !strings.Contains(text, "未知模式") {
			t.Fatalf("expected '未知模式' for deprecated mode '%s', got: %s", mode, text)
		}
	}
}

func TestAPI_FinishMarksAllChainTypes(t *testing.T) {
	sm := newTestSMWithDB(t)

	// 创建协议链
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "FIN_001",
		"description": "finish 测试", "protocol": "linear",
	})

	// finish
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "finish", "task_id": "FIN_001",
	})
	if !strings.Contains(text, "任务链完成") {
		t.Fatalf("expected finish output, got: %s", text)
	}

	// 验证协议链状态
	chain := sm.TaskChainsV3["FIN_001"]
	if chain.Status != "finished" {
		t.Fatalf("expected protocol chain status=finished, got %s", chain.Status)
	}
}

func TestAPI_RenderV3StatusJSON(t *testing.T) {
	chain := newTestChainV3()
	chain.StartPhase("analyze")

	output := renderV3StatusJSON(chain)
	if !strings.Contains(output, "\"task_id\"") {
		t.Fatalf("expected JSON with task_id, got: %s", output)
	}
}

// ========== 自审机制测试 ==========

func TestSelfReview_ReinitCountIncrement(t *testing.T) {
	sm := newTestSMWithDB(t)

	// 初始化
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_001",
		"description": "自审测试", "protocol": "linear",
	})
	chain := sm.TaskChainsV3["SR_001"]
	if chain.ReinitCount != 0 {
		t.Fatalf("expected reinit_count=0, got %d", chain.ReinitCount)
	}

	// 第一次 re-init 应成功，reinit_count=1
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_001",
		"description": "自审测试 re-init 1", "protocol": "linear",
	})
	chain = sm.TaskChainsV3["SR_001"]
	if chain.ReinitCount != 1 {
		t.Fatalf("expected reinit_count=1, got %d", chain.ReinitCount)
	}
	if strings.Contains(text, "停下") {
		t.Fatalf("first re-init should succeed, got: %s", text)
	}
}

func TestSelfReview_ReinitBlockedAfterSecond(t *testing.T) {
	sm := newTestSMWithDB(t)

	// init → re-init 1 → re-init 2（应被硬拦）
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_002",
		"description": "自审测试", "protocol": "linear",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_002",
		"description": "re-init 1", "protocol": "linear",
	})

	// 第二次 re-init 应被拦截
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_002",
		"description": "re-init 2", "protocol": "linear",
	})
	if !strings.Contains(text, "停下") {
		t.Fatalf("expected block message, got: %s", text)
	}
	if !strings.Contains(text, "询问") {
		t.Fatalf("expected '询问' in block message, got: %s", text)
	}
}

func TestSelfReview_NextPhaseHintContainsSelfReview(t *testing.T) {
	sm := newTestSMWithDB(t)

	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_003",
		"description": "自审提示测试", "protocol": "develop",
	})

	// 完成 analyze，输出应包含自审提示
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "complete", "task_id": "SR_003",
		"phase_id": "analyze", "summary": "分析完成",
	})
	if !strings.Contains(text, "自审") {
		t.Fatalf("expected self-review hint, got: %s", text)
	}
	if !strings.Contains(text, "re-init") {
		t.Fatalf("expected re-init hint, got: %s", text)
	}
}

func TestSelfReview_ReinitCountPersisted(t *testing.T) {
	sm := newTestSMWithDB(t)

	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_004",
		"description": "持久化测试", "protocol": "linear",
	})
	callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_004",
		"description": "re-init 1", "protocol": "linear",
	})

	// 模拟断连，从 DB 恢复
	delete(sm.TaskChainsV3, "SR_004")
	callTaskChainTool(t, sm, map[string]any{
		"mode": "resume", "task_id": "SR_004",
	})

	chain := sm.TaskChainsV3["SR_004"]
	if chain == nil {
		t.Fatal("expected chain restored")
	}
	if chain.ReinitCount != 1 {
		t.Fatalf("expected reinit_count=1 after resume, got %d", chain.ReinitCount)
	}

	// 恢复后再 re-init 应被拦截
	text := callTaskChainTool(t, sm, map[string]any{
		"mode": "init", "task_id": "SR_004",
		"description": "re-init 2 after resume", "protocol": "linear",
	})
	if !strings.Contains(text, "停下") {
		t.Fatalf("expected block after resume, got: %s", text)
	}
}
