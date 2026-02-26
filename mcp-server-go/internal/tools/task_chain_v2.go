package tools

// StepStatus 步骤状态
type StepStatus string

const (
	StepStatusTodo       StepStatus = "todo"        // 待执行
	StepStatusInProgress StepStatus = "in_progress" // 执行中
	StepStatusComplete   StepStatus = "complete"    // 已完成
)

// Step 任务链中的单个步骤（线性自适应版本）
type Step struct {
	Number  float64    `json:"number"`  // 步骤编号（支持小数：1.0, 1.5, 2.0 等）
	Name    string     `json:"name"`    // 步骤名称
	Input   string     `json:"input"`   // 建议的工具调用参数（可选）
	Output  string     `json:"output"`  // 执行输出（完成后填充）
	Summary string     `json:"summary"` // LLM 提炼的总结（必须）
	Status  StepStatus `json:"status"`  // 当前状态
}

// TaskChainV2 任务链（线性自适应版本）
type TaskChainV2 struct {
	TaskID      string  `json:"task_id"`      // 任务唯一标识
	Description string  `json:"description"`  // 任务整体描述
	Steps       []Step  `json:"steps"`        // 步骤列表（按编号排序）
	CurrentStep float64 `json:"current_step"` // 当前执行到的步骤编号
	Status      string  `json:"status"`       // 任务状态：running, paused, finished
}

// TaskChainArgsV2 任务链参数（线性版本）
type TaskChainArgsV2 struct {
	Mode        string                   `json:"mode" jsonschema:"required,enum=start,enum=complete,enum=step,enum=insert,enum=update,enum=delete,enum=finish,enum=status,description=操作模式"`
	TaskID      string                   `json:"task_id" jsonschema:"description=任务ID"`
	Description string                   `json:"description" jsonschema:"description=任务描述（step模式）"`
	Plan        []map[string]interface{} `json:"plan" jsonschema:"description=任务计划列表（step模式）"`
	// start 模式参数
	StepNumber float64 `json:"step_number" jsonschema:"description=步骤编号（start模式）"`
	// complete 模式参数
	Summary string `json:"summary" jsonschema:"description=步骤总结（complete模式，必填）"`
	// insert 模式参数
	After      float64                  `json:"after" jsonschema:"description=插入到某步骤之后（insert模式）"`
	InsertPlan []map[string]interface{} `json:"insert_plan" jsonschema:"description=插入的步骤列表（insert模式）"`
	// update 模式参数
	From       float64                  `json:"from" jsonschema:"description=从某步骤开始更新（update模式）"`
	UpdatePlan []map[string]interface{} `json:"update_plan" jsonschema:"description=更新后的步骤列表（update模式）"`
	// delete 模式参数
	StepToDelete float64 `json:"step_to_delete" jsonschema:"description=要删除的步骤编号（delete模式）"`
	DeleteScope  string  `json:"delete_scope" jsonschema:"description=删除范围（delete模式）: single, remaining"`
}
