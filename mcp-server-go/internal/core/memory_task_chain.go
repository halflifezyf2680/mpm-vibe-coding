package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ========== Task Chain V3 持久化 ==========

// TaskChainRecord 任务链持久化记录
type TaskChainRecord struct {
	TaskID       string `json:"task_id"`
	Description  string `json:"description"`
	Protocol     string `json:"protocol"`
	Status       string `json:"status"`
	PhasesJSON   string `json:"phases_json"`
	CurrentPhase string `json:"current_phase"`
	ReinitCount  int    `json:"reinit_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// TaskChainEvent 任务链事件
type TaskChainEvent struct {
	ID        int64  `json:"id"`
	TaskID    string `json:"task_id"`
	PhaseID   string `json:"phase_id"`
	SubID     string `json:"sub_id"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"created_at"`
}

// SaveTaskChain 保存或更新任务链
func (m *MemoryLayer) SaveTaskChain(ctx context.Context, rec *TaskChainRecord) error {
	query := `INSERT INTO task_chains (task_id, description, protocol, status, phases_json, current_phase, reinit_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			description=excluded.description,
			protocol=excluded.protocol,
			status=excluded.status,
			phases_json=excluded.phases_json,
			current_phase=excluded.current_phase,
			reinit_count=excluded.reinit_count,
			updated_at=excluded.updated_at`

	now := time.Now().Format(time.RFC3339)
	createdAt := rec.CreatedAt
	if createdAt == "" {
		createdAt = now
	}

	_, err := m.dbManager.Exec(query,
		rec.TaskID, rec.Description, rec.Protocol, rec.Status,
		rec.PhasesJSON, rec.CurrentPhase, rec.ReinitCount, createdAt, now)
	return err
}

// LoadTaskChain 加载任务链
func (m *MemoryLayer) LoadTaskChain(ctx context.Context, taskID string) (*TaskChainRecord, error) {
	query := `SELECT task_id, description, protocol, status, phases_json, current_phase, reinit_count, created_at, updated_at
		FROM task_chains WHERE task_id = ?`

	var rec TaskChainRecord
	err := m.dbManager.QueryRow(query, taskID).Scan(
		&rec.TaskID, &rec.Description, &rec.Protocol, &rec.Status,
		&rec.PhasesJSON, &rec.CurrentPhase, &rec.ReinitCount, &rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListTaskChains 列出任务链（按更新时间倒序）
func (m *MemoryLayer) ListTaskChains(ctx context.Context, status string, limit int) ([]TaskChainRecord, error) {
	query := `SELECT task_id, description, protocol, status, phases_json, current_phase, created_at, updated_at
		FROM task_chains`
	var params []interface{}

	if status != "" {
		query += " WHERE status = ?"
		params = append(params, status)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	params = append(params, limit)

	rows, err := m.dbManager.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TaskChainRecord
	for rows.Next() {
		var rec TaskChainRecord
		if err := rows.Scan(&rec.TaskID, &rec.Description, &rec.Protocol, &rec.Status,
			&rec.PhasesJSON, &rec.CurrentPhase, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			continue
		}
		results = append(results, rec)
	}
	return results, nil
}

// DeleteTaskChain 删除任务链及其事件
func (m *MemoryLayer) DeleteTaskChain(ctx context.Context, taskID string) error {
	if _, err := m.dbManager.Exec("DELETE FROM task_chain_events WHERE task_id = ?", taskID); err != nil {
		return err
	}
	_, err := m.dbManager.Exec("DELETE FROM task_chains WHERE task_id = ?", taskID)
	return err
}

// AppendTaskChainEvent 追加事件
func (m *MemoryLayer) AppendTaskChainEvent(ctx context.Context, evt *TaskChainEvent) (int64, error) {
	query := `INSERT INTO task_chain_events (task_id, phase_id, sub_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	now := time.Now().Format(time.RFC3339)
	res, err := m.dbManager.Exec(query, evt.TaskID, evt.PhaseID, evt.SubID, evt.EventType, evt.Payload, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// QueryTaskChainEvents 查询任务链事件
func (m *MemoryLayer) QueryTaskChainEvents(ctx context.Context, taskID string, limit int) ([]TaskChainEvent, error) {
	query := `SELECT id, task_id, phase_id, sub_id, event_type, payload, created_at
		FROM task_chain_events WHERE task_id = ? ORDER BY id ASC LIMIT ?`

	rows, err := m.dbManager.Query(query, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TaskChainEvent
	for rows.Next() {
		var evt TaskChainEvent
		if err := rows.Scan(&evt.ID, &evt.TaskID, &evt.PhaseID, &evt.SubID,
			&evt.EventType, &evt.Payload, &evt.CreatedAt); err != nil {
			continue
		}
		results = append(results, evt)
	}
	return results, nil
}

// MarshalPhasesJSON 辅助：将 phases 序列化为 JSON 字符串
func MarshalPhasesJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("序列化 phases 失败: %w", err)
	}
	return string(data), nil
}

// UnmarshalPhasesJSON 辅助：将 JSON 字符串反序列化为 phases
func UnmarshalPhasesJSON(s string, v interface{}) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), v)
}
