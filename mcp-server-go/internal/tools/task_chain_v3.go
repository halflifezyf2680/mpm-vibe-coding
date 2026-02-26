package tools

import (
	"encoding/json"
	"fmt"
)

// ========== 协议状态机数据结构 ==========

// PhaseType 阶段类型
type PhaseType string

const (
	PhaseExecute PhaseType = "execute" // 普通执行阶段
	PhaseGate    PhaseType = "gate"    // 门控检查点
	PhaseLoop    PhaseType = "loop"    // 循环阶段（内含子任务）
)

// PhaseStatus 阶段状态
type PhaseStatus string

const (
	PhasePending PhaseStatus = "pending" // 待执行
	PhaseActive  PhaseStatus = "active"  // 执行中
	PhasePassed  PhaseStatus = "passed"  // 已通过
	PhaseFailed  PhaseStatus = "failed"  // 已失败（gate 专用）
	PhaseSkipped PhaseStatus = "skipped" // 已跳过
)

// SubTaskStatus 子任务状态
type SubTaskStatus string

const (
	SubTaskPending SubTaskStatus = "pending"
	SubTaskActive  SubTaskStatus = "active"
	SubTaskPassed  SubTaskStatus = "passed"
	SubTaskFailed  SubTaskStatus = "failed"
)

// Phase 状态机阶段
type Phase struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Type    PhaseType   `json:"type"`
	Status  PhaseStatus `json:"status"`
	Input   string      `json:"input,omitempty"`
	Summary string      `json:"summary,omitempty"`

	// Gate 专用
	OnPass     string `json:"on_pass,omitempty"`
	OnFail     string `json:"on_fail,omitempty"`
	MaxRetries int    `json:"max_retries,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`

	// Loop 专用
	SubTasks []SubTask `json:"sub_tasks,omitempty"`
}

// SubTask 子任务
type SubTask struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Verify  string        `json:"verify,omitempty"`
	Status  SubTaskStatus `json:"status"`
	Summary string        `json:"summary,omitempty"`
}

// TaskChainV3 协议状态机任务链
type TaskChainV3 struct {
	TaskID       string  `json:"task_id"`
	Description  string  `json:"description"`
	Protocol     string  `json:"protocol"`
	Status       string  `json:"status"` // running / paused / finished / failed
	Phases       []Phase `json:"phases"`
	CurrentPhase string  `json:"current_phase"`
	ReinitCount  int     `json:"reinit_count,omitempty"` // 重新初始化次数，用于自审升级判断
}

// ========== 状态流转引擎 ==========

// findPhase 按 ID 查找阶段
func (tc *TaskChainV3) findPhase(phaseID string) *Phase {
	for i := range tc.Phases {
		if tc.Phases[i].ID == phaseID {
			return &tc.Phases[i]
		}
	}
	return nil
}

// findPhaseIndex 按 ID 查找阶段索引
func (tc *TaskChainV3) findPhaseIndex(phaseID string) int {
	for i := range tc.Phases {
		if tc.Phases[i].ID == phaseID {
			return i
		}
	}
	return -1
}

// nextPhaseAfter 获取指定阶段之后的下一个 pending 阶段
func (tc *TaskChainV3) nextPhaseAfter(phaseID string) *Phase {
	idx := tc.findPhaseIndex(phaseID)
	if idx < 0 {
		return nil
	}
	for i := idx + 1; i < len(tc.Phases); i++ {
		if tc.Phases[i].Status == PhasePending {
			return &tc.Phases[i]
		}
	}
	return nil
}

// StartPhase 开始一个阶段
func (tc *TaskChainV3) StartPhase(phaseID string) error {
	p := tc.findPhase(phaseID)
	if p == nil {
		return errPhaseNotFound(phaseID)
	}
	if p.Status != PhasePending {
		return errPhaseWrongStatus(phaseID, p.Status, PhasePending)
	}
	p.Status = PhaseActive
	tc.CurrentPhase = phaseID
	return nil
}

// CompleteExecute 完成 execute 阶段
func (tc *TaskChainV3) CompleteExecute(phaseID, summary string) (nextPhaseID string, err error) {
	p := tc.findPhase(phaseID)
	if p == nil {
		return "", errPhaseNotFound(phaseID)
	}
	if p.Status != PhaseActive {
		return "", errPhaseWrongStatus(phaseID, p.Status, PhaseActive)
	}
	if p.Type != PhaseExecute {
		return "", errPhaseWrongType(phaseID, p.Type, PhaseExecute)
	}

	p.Status = PhasePassed
	p.Summary = summary

	// 返回下一个阶段
	next := tc.nextPhaseAfter(phaseID)
	if next != nil {
		return next.ID, nil
	}
	return "", nil
}

// CompleteGate 完成 gate 阶段（pass/fail 路由）
func (tc *TaskChainV3) CompleteGate(phaseID, result, summary string) (nextPhaseID string, retryInfo string, err error) {
	p := tc.findPhase(phaseID)
	if p == nil {
		return "", "", errPhaseNotFound(phaseID)
	}
	if p.Status != PhaseActive {
		return "", "", errPhaseWrongStatus(phaseID, p.Status, PhaseActive)
	}
	if p.Type != PhaseGate {
		return "", "", errPhaseWrongType(phaseID, p.Type, PhaseGate)
	}

	p.Summary = summary

	if result == "pass" {
		p.Status = PhasePassed
		if p.OnPass != "" {
			return p.OnPass, "", nil
		}
		next := tc.nextPhaseAfter(phaseID)
		if next != nil {
			return next.ID, "", nil
		}
		return "", "", nil
	}

	// fail 路径
	p.RetryCount++
	maxRetries := p.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	if p.RetryCount >= maxRetries {
		p.Status = PhaseFailed
		tc.Status = "failed"
		return "", "", errGateMaxRetries(phaseID, maxRetries)
	}

	// 重置为 pending，准备下次进入
	p.Status = PhasePending
	retryInfo = formatRetryInfo(p.RetryCount, maxRetries)

	// 回退目标
	if p.OnFail != "" {
		target := tc.findPhase(p.OnFail)
		if target != nil {
			target.Status = PhasePending
			target.Summary = ""
			return p.OnFail, retryInfo, nil
		}
	}

	return "", retryInfo, nil
}

// SpawnSubTasks 在 loop 阶段生成子任务
func (tc *TaskChainV3) SpawnSubTasks(phaseID string, subs []SubTask) error {
	p := tc.findPhase(phaseID)
	if p == nil {
		return errPhaseNotFound(phaseID)
	}
	if p.Type != PhaseLoop {
		return errPhaseWrongType(phaseID, p.Type, PhaseLoop)
	}
	if p.Status != PhaseActive {
		return errPhaseWrongStatus(phaseID, p.Status, PhaseActive)
	}

	for i := range subs {
		if subs[i].Status == "" {
			subs[i].Status = SubTaskPending
		}
	}
	p.SubTasks = append(p.SubTasks, subs...)
	return nil
}

// StartSubTask 开始子任务
func (tc *TaskChainV3) StartSubTask(phaseID, subID string) error {
	p := tc.findPhase(phaseID)
	if p == nil {
		return errPhaseNotFound(phaseID)
	}
	sub := findSubTask(p, subID)
	if sub == nil {
		return errSubTaskNotFound(phaseID, subID)
	}
	if sub.Status != SubTaskPending {
		return errSubTaskWrongStatus(subID, sub.Status, SubTaskPending)
	}
	sub.Status = SubTaskActive
	return nil
}

// CompleteSubTask 完成子任务，返回是否全部完成
func (tc *TaskChainV3) CompleteSubTask(phaseID, subID, result, summary string) (allDone bool, err error) {
	p := tc.findPhase(phaseID)
	if p == nil {
		return false, errPhaseNotFound(phaseID)
	}
	sub := findSubTask(p, subID)
	if sub == nil {
		return false, errSubTaskNotFound(phaseID, subID)
	}
	if sub.Status != SubTaskActive {
		return false, errSubTaskWrongStatus(subID, sub.Status, SubTaskActive)
	}

	sub.Summary = summary
	if result == "pass" {
		sub.Status = SubTaskPassed
	} else {
		sub.Status = SubTaskFailed
	}

	// 检查是否全部完成
	allDone = true
	for _, s := range p.SubTasks {
		if s.Status == SubTaskPending || s.Status == SubTaskActive {
			allDone = false
			break
		}
	}

	if allDone {
		p.Status = PhasePassed
		// 汇总 summary
		p.Summary = summary
	}

	return allDone, nil
}

// NextPendingSubTask 获取 loop 阶段下一个待执行的子任务
func (tc *TaskChainV3) NextPendingSubTask(phaseID string) *SubTask {
	p := tc.findPhase(phaseID)
	if p == nil {
		return nil
	}
	for i := range p.SubTasks {
		if p.SubTasks[i].Status == SubTaskPending {
			return &p.SubTasks[i]
		}
	}
	return nil
}

// IsFinished 检查所有阶段是否完成
func (tc *TaskChainV3) IsFinished() bool {
	for _, p := range tc.Phases {
		if p.Status == PhasePending || p.Status == PhaseActive {
			return false
		}
	}
	return true
}

// MarshalPhases 序列化 phases
func (tc *TaskChainV3) MarshalPhases() (string, error) {
	data, err := json.Marshal(tc.Phases)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalPhases 反序列化 phases
func UnmarshalPhases(s string) ([]Phase, error) {
	var phases []Phase
	if s == "" {
		return phases, nil
	}
	err := json.Unmarshal([]byte(s), &phases)
	return phases, err
}

// ========== 错误辅助函数 ==========

func errPhaseNotFound(phaseID string) error {
	return fmt.Errorf("phase '%s' not found", phaseID)
}

func errPhaseWrongStatus(phaseID string, current, expected PhaseStatus) error {
	return fmt.Errorf("phase '%s' status is '%s', expected '%s'", phaseID, current, expected)
}

func errPhaseWrongType(phaseID string, current, expected PhaseType) error {
	return fmt.Errorf("phase '%s' type is '%s', expected '%s'", phaseID, current, expected)
}

func errGateMaxRetries(phaseID string, max int) error {
	return fmt.Errorf("gate '%s' reached max retries (%d), task failed", phaseID, max)
}

func errSubTaskNotFound(phaseID, subID string) error {
	return fmt.Errorf("sub_task '%s' not found in phase '%s'", subID, phaseID)
}

func errSubTaskWrongStatus(subID string, current, expected SubTaskStatus) error {
	return fmt.Errorf("sub_task '%s' status is '%s', expected '%s'", subID, current, expected)
}

// ========== 辅助函数 ==========

func findSubTask(p *Phase, subID string) *SubTask {
	for i := range p.SubTasks {
		if p.SubTasks[i].ID == subID {
			return &p.SubTasks[i]
		}
	}
	return nil
}

func formatRetryInfo(current, max int) string {
	return formatf("第 %d/%d 次重试", current, max)
}

func formatf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
