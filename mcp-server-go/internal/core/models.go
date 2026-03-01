package core

import (
	"database/sql"
	"time"
)

// Memo 原子操作备忘 (SSOT)
type Memo struct {
	ID        int64          `db:"id"`
	Category  string         `db:"category"`
	Entity    string         `db:"entity"`
	Act       string         `db:"act"`
	Path      string         `db:"path"`
	Content   string         `db:"content"`
	SessionID sql.NullString `db:"session_id"`
	Timestamp time.Time      `db:"timestamp"`
}

// Task 任务上下文
type Task struct {
	TaskID          string         `db:"task_id"`
	Description     string         `db:"description"`
	TaskType        sql.NullString `db:"task_type"`
	ParentTaskID    sql.NullString `db:"parent_task_id"`
	Understanding   sql.NullString `db:"understanding"`
	ExecutionPlan   sql.NullString `db:"execution_plan"` // JSON string
	Status          string         `db:"status"`
	MetaData        sql.NullString `db:"meta_data"` // JSON string
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
	CompletedAt     sql.NullTime   `db:"completed_at"`
	Summary         sql.NullString `db:"summary"`
	Pitfalls        sql.NullString `db:"pitfalls"`
	CurrentFocus    sql.NullString `db:"current_focus"`
	TopLevelMission string         `db:"-"` // Virtual field for meta_data mapping
}

// TaskStep 任务步骤历史
type TaskStep struct {
	ID            int64     `db:"id"`
	TaskID        string    `db:"task_id"`
	StepNumber    int       `db:"step_number"`
	CurrentStep   string    `db:"current_step"`
	Observation   string    `db:"observation"`
	Reflection    string    `db:"reflection"`
	Status        string    `db:"status"`
	YieldType     string    `db:"yield_type"`
	SuggestedTool string    `db:"suggested_tool"`
	SubtaskID     string    `db:"subtask_id"`
	CreatedAt     time.Time `db:"created_at"`
}

// KnownFact 原子化事实
type KnownFact struct {
	ID        int64     `db:"id"`
	Type      string    `db:"type"`
	Summarize string    `db:"summarize"`
	CreatedAt time.Time `db:"created_at"`
}

// ConstraintRule 约束规则
type ConstraintRule struct {
	ID             int64     `db:"id"`
	RuleName       string    `db:"rule_name"`
	Category       string    `db:"category"`
	RuleDefinition string    `db:"rule_definition"` // JSON string
	Description    string    `db:"description"`
	Priority       int       `db:"priority"`
	FilePatterns   string    `db:"file_patterns"` // JSON string
	ExpertScope    string    `db:"expert_scope"`  // JSON string
	IsActive       bool      `db:"is_active"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// SystemState 全局状态
type SystemState struct {
	Key       string    `db:"key"`
	Value     string    `db:"value"`
	Category  string    `db:"category"`
	UpdatedAt time.Time `db:"updated_at"`
}
