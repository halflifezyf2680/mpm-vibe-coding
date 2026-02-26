package core

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MemoryLayer 记忆层 (SSOT)
type MemoryLayer struct {
	dbManager   *DatabaseManager
	projectRoot string
}

// NewMemoryLayer 创建记忆层实例
func NewMemoryLayer(projectRoot string) (*MemoryLayer, error) {
	mgr, err := GetDBForProject(projectRoot)
	if err != nil {
		return nil, err
	}
	ml := &MemoryLayer{
		dbManager:   mgr,
		projectRoot: projectRoot,
	}

	if err := ml.ensureMemoData(); err != nil {
		fmt.Fprintf(os.Stderr, "[Memory][WARN] memo bootstrap failed: %v\n", err)
	}

	return ml, nil
}

// ========== Task Management ==========

// CreateTask 创建任务记录
func (m *MemoryLayer) CreateTask(ctx context.Context, task Task) error {
	query := `INSERT INTO tasks (
		task_id, description, task_type, parent_task_id,
		understanding, execution_plan, status, meta_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.dbManager.Exec(query,
		task.TaskID,
		task.Description,
		task.TaskType,
		task.ParentTaskID,
		task.Understanding,
		task.ExecutionPlan,
		task.Status,
		task.MetaData,
	)
	return err
}

// GetTask 获取任务详情
func (m *MemoryLayer) GetTask(ctx context.Context, taskID string) (*Task, error) {
	row := m.dbManager.QueryRow(`
		SELECT 
			task_id, description, task_type, parent_task_id, 
			understanding, execution_plan, status, meta_data, 
			created_at, updated_at, completed_at, summary, 
			pitfalls, current_focus 
		FROM tasks WHERE task_id = ?`, taskID)
	var t Task
	err := row.Scan(
		&t.TaskID, &t.Description, &t.TaskType, &t.ParentTaskID,
		&t.Understanding, &t.ExecutionPlan, &t.Status, &t.MetaData,
		&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt, &t.Summary,
		&t.Pitfalls, &t.CurrentFocus,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

// ========== Memo Management ==========

// memoArchiveEntry 用于持久化到 dev-log-archive 的备份条目
// 设计目标：即使 .mcp-data/mcp_memory.db 丢失，也可以通过重放此日志恢复 memos 表的核心字段。
type memoArchiveEntry struct {
	ID        int64     `json:"id"`
	Category  string    `json:"category"`
	Entity    string    `json:"entity"`
	Act       string    `json:"act"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	SessionID string    `json:"session_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

var devLogMemoLinePattern = regexp.MustCompile(`^- \[(.*)\] \*\*([^*]+)\*\*: (.*?) \((.*?)\)\s*(.*)$`)

func (m *MemoryLayer) ensureMemoData() error {
	var count int
	if err := m.dbManager.QueryRow("SELECT COUNT(*) FROM memos").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	archiveRecovered, err := m.recoverMemosFromArchive()
	if err != nil {
		return err
	}
	if archiveRecovered > 0 {
		fmt.Fprintf(os.Stderr, "[Memory] Recovered %d memos from archive\n", archiveRecovered)
		return nil
	}

	devLogRecovered, err := m.recoverMemosFromDevLog()
	if err != nil {
		return err
	}
	if devLogRecovered > 0 {
		fmt.Fprintf(os.Stderr, "[Memory] Recovered %d memos from dev-log.md\n", devLogRecovered)
	}

	return nil
}

func (m *MemoryLayer) recoverMemosFromArchive() (int, error) {
	archivePath := filepath.Join(m.projectRoot, "dev-log-archive", "memo_archive.jsonl")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return 0, nil
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	recovered := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry memoArchiveEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		ts := entry.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}

		_, err := m.dbManager.Exec(
			"INSERT INTO memos (category, entity, act, path, content, session_id, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)",
			entry.Category, entry.Entity, entry.Act, entry.Path, entry.Content, entry.SessionID, ts.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			continue
		}
		recovered++
	}

	if err := scanner.Err(); err != nil {
		return recovered, err
	}

	return recovered, nil
}

func (m *MemoryLayer) recoverMemosFromDevLog() (int, error) {
	devLogPath := filepath.Join(m.projectRoot, "dev-log.md")
	if _, err := os.Stat(devLogPath); os.IsNotExist(err) {
		return 0, nil
	}

	f, err := os.Open(devLogPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	recovered := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := devLogMemoLinePattern.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}

		content := strings.TrimSpace(matches[1])
		timestampStr := strings.TrimSpace(matches[2])
		category := strings.TrimSpace(matches[3])
		entity := strings.TrimSpace(matches[4])
		act := strings.TrimSpace(matches[5])

		ts := parseMemoTimestamp(timestampStr)
		_, err := m.dbManager.Exec(
			"INSERT INTO memos (category, entity, act, path, content, session_id, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)",
			category, entity, act, "", content, "rebuild-devlog", ts.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			continue
		}
		recovered++
	}

	if err := scanner.Err(); err != nil {
		return recovered, err
	}

	return recovered, nil
}

func parseMemoTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now()
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}

	return time.Now()
}

// AddMemos 批量添加原子操作备忘
func (m *MemoryLayer) AddMemos(ctx context.Context, items []Memo) ([]int64, error) {
	if len(items) == 0 {
		return nil, nil
	}

	sessionID := fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	var ids []int64
	var archives []memoArchiveEntry

	now := time.Now()

	for _, item := range items {
		res, err := m.dbManager.Exec(
			"INSERT INTO memos (category, entity, act, path, content, session_id) VALUES (?, ?, ?, ?, ?, ?)",
			item.Category, item.Entity, item.Act, item.Path, item.Content, sessionID,
		)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		ids = append(ids, id)

		// 构造归档条目（与 DB 解耦，作为物理备份和重放来源）
		entry := memoArchiveEntry{
			ID:       id,
			Category: item.Category,
			Entity:   item.Entity,
			Act:      item.Act,
			Path:     item.Path,
			Content:  item.Content,
			// 这里使用 AddMemos 调用时的时间戳，精度足以支撑后续审计与恢复
			Timestamp: now,
		}
		if sessionID != "" {
			entry.SessionID = sessionID
		}
		archives = append(archives, entry)
	}

	// 触发同步 dev-log.md
	go m.SyncDevLog()

	// 异步追加写入 dev-log-archive 作为独立物理备份
	if len(archives) > 0 {
		go m.appendMemoArchive(archives)
	}

	return ids, nil
}

// SearchMemos 搜索备忘录
func (m *MemoryLayer) SearchMemos(ctx context.Context, keywords string, category string, limit int) ([]Memo, error) {
	query := "SELECT id, category, entity, act, path, content, session_id, timestamp FROM memos WHERE 1=1"
	var args []interface{}

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	if keywords != "" {
		// 宽进严出：支持空格和逗号拆分关键词，实现逻辑或(OR)匹配
		keywords = strings.ReplaceAll(keywords, ",", " ")
		words := strings.Fields(keywords)
		if len(words) > 0 {
			var orConditions []string
			for _, word := range words {
				orConditions = append(orConditions, "(content LIKE ? OR entity LIKE ? OR act LIKE ?)")
				pattern := "%" + word + "%"
				args = append(args, pattern, pattern, pattern)
			}
			query += " AND (" + strings.Join(orConditions, " OR ") + ")"
		}
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit)

	// DEBUG: Log the final query and args
	debugPath := filepath.Join(m.projectRoot, ".mcp-data", "recall_debug.log")
	debugMsg := fmt.Sprintf("Query: %s\nArgs: %v\n", query, args)
	_ = os.WriteFile(debugPath, []byte(debugMsg), 0644)

	rows, err := m.dbManager.Query(query, args...)
	if err != nil {
		_ = os.WriteFile(debugPath, []byte(fmt.Sprintf("%sERR: %v\n", debugMsg, err)), 0644)
		return nil, err
	}
	defer rows.Close()

	var memos []Memo
	for rows.Next() {
		var m Memo
		if err := rows.Scan(&m.ID, &m.Category, &m.Entity, &m.Act, &m.Path, &m.Content, &m.SessionID, &m.Timestamp); err != nil {
			return nil, err
		}
		memos = append(memos, m)
	}
	return memos, nil
}

// SyncDevLog 同步更新 dev-log.md
func (m *MemoryLayer) SyncDevLog() {
	rows, err := m.dbManager.Query(`
		SELECT 
			id, content, timestamp, category, entity, act, path, session_id 
		FROM memos ORDER BY id DESC LIMIT 100`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SyncDevLog] Query failed: %v\n", err)
		return
	}
	defer rows.Close()

	var memos []Memo
	for rows.Next() {
		var m Memo
		// Physical order: 0:id, 1:content, 2:timestamp, 3:category, 4:entity, 5:act, 6:path, 7:session_id
		err := rows.Scan(
			&m.ID, &m.Content, &m.Timestamp, &m.Category, &m.Entity, &m.Act,
			&m.Path, &m.SessionID,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[SyncDevLog] Scan failed: %v\n", err)
			continue
		}
		memos = append(memos, m)
	}

	// 保持倒序（最新的在上面），不进行排序
	// memos 已经是从数据库按 id DESC 取出的，直接使用

	projectName := filepath.Base(m.projectRoot)
	var lines []string
	lines = append(lines, fmt.Sprintf("# Dev Log: %s (Surgical Snapshot)", projectName))
	lines = append(lines, "")
	lines = append(lines, "<!-- 由 MPM-Go 自动生成，请勿手动编辑 -->")
	lines = append(lines, "")

	for _, memo := range memos {
		// Convert UTC timestamp to Local time
		// Assuming DB stores UTC, and Scan reads it as UTC (or we treat it as such)
		// We explicitly convert to Local for display.
		displayTime := memo.Timestamp.In(time.Local).Format("2006-01-02 15:04:05")

		// Revert to Python-like format: - [Content] **Time**: Category (Entity) Act
		// This matches the format expected by the user and legacy logs.
		line := fmt.Sprintf("- [%s] **%s**: %s (%s) %s",
			memo.Content, displayTime, memo.Category, memo.Entity, memo.Act)
		lines = append(lines, line)
	}

	devLogPath := filepath.Join(m.projectRoot, "dev-log.md")
	os.WriteFile(devLogPath, []byte(strings.Join(lines, "\n")), 0644)
}

// appendMemoArchive 将新增的 memo 以 JSONL 形式追加写入 dev-log-archive 目录
// 路径示例：<project_root>/dev-log-archive/memo_archive.jsonl
// 说明：
// - 采用 append-only 设计，不做就地修改，便于事后重放恢复数据库
// - 写入失败不会影响主流程，只在 stderr 打印告警
func (m *MemoryLayer) appendMemoArchive(entries []memoArchiveEntry) {
	if len(entries) == 0 {
		return
	}

	archiveDir := filepath.Join(m.projectRoot, "dev-log-archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[MemoArchive] MkdirAll failed: %v\n", err)
		return
	}

	archivePath := filepath.Join(archiveDir, "memo_archive.jsonl")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[MemoArchive] OpenFile failed: %v\n", err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, e := range entries {
		if err := encoder.Encode(e); err != nil {
			fmt.Fprintf(os.Stderr, "[MemoArchive] Encode failed: %v\n", err)
			// 不中断后续写入，尽可能多地保留可用记录
		}
	}
}

// ========== Retrieval Operations ==========

// QueryMemos 检索备忘
func (m *MemoryLayer) QueryMemos(ctx context.Context, keywords, category string, limit int) ([]Memo, error) {
	query := `
		SELECT 
			id, content, timestamp, category, entity, act, path, session_id 
		FROM memos WHERE 1=1`
	var params []interface{}

	if category != "" {
		query += " AND category = ?"
		params = append(params, category)
	}

	if keywords != "" {
		// 亮窃谓：此处将词句拆解，若有一词相合，即入奏报。
		// 待日后功力深厚，再行复杂之权重排序。
		words := strings.Fields(strings.ReplaceAll(keywords, ",", " "))
		if len(words) > 0 {
			var subConditions []string
			for _, w := range words {
				subConditions = append(subConditions, "(entity LIKE ? OR act LIKE ? OR content LIKE ?)")
				pattern := "%" + w + "%"
				params = append(params, pattern, pattern, pattern)
			}
			query += " AND (" + strings.Join(subConditions, " OR ") + ")"
		}
	}

	query += " ORDER BY id DESC LIMIT ?"
	params = append(params, limit)

	rows, err := m.dbManager.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Memo
	for rows.Next() {
		var item Memo
		// Physical order: 0:id, 1:content, 2:timestamp, 3:category, 4:entity, 5:act, 6:path, 7:session_id
		err := rows.Scan(
			&item.ID, &item.Content, &item.Timestamp, &item.Category, &item.Entity, &item.Act,
			&item.Path, &item.SessionID,
		)
		if err != nil {
			continue
		}
		results = append(results, item)
	}
	return results, nil
}

// QueryTasks 检索任务
func (m *MemoryLayer) QueryTasks(ctx context.Context, keywords string, limit int) ([]Task, error) {
	query := `
		SELECT 
			task_id, description, task_type, parent_task_id, 
			understanding, execution_plan, status, meta_data, 
			created_at, updated_at, completed_at, summary, 
			pitfalls, current_focus 
		FROM tasks WHERE 1=1`
	var params []interface{}

	if keywords != "" {
		words := strings.Fields(strings.ReplaceAll(keywords, ",", " "))
		if len(words) > 0 {
			var subConditions []string
			for _, w := range words {
				subConditions = append(subConditions, "(description LIKE ? OR summary LIKE ?)")
				pattern := "%" + w + "%"
				params = append(params, pattern, pattern)
			}
			query += " AND (" + strings.Join(subConditions, " OR ") + ")"
		}
	}

	query += " ORDER BY updated_at DESC LIMIT ?"
	params = append(params, limit)

	rows, err := m.dbManager.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(
			&t.TaskID, &t.Description, &t.TaskType, &t.ParentTaskID,
			&t.Understanding, &t.ExecutionPlan, &t.Status, &t.MetaData,
			&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt, &t.Summary,
			&t.Pitfalls, &t.CurrentFocus,
		)
		if err != nil {
			continue
		}
		results = append(results, t)
	}
	return results, nil
}

// QueryFacts 检索事实
func (m *MemoryLayer) QueryFacts(ctx context.Context, keywords string, limit int) ([]KnownFact, error) {
	query := `
		SELECT 
			id, type, summarize, created_at 
		FROM known_facts WHERE 1=1`
	var params []interface{}

	if keywords != "" {
		words := strings.Fields(strings.ReplaceAll(keywords, ",", " "))
		if len(words) > 0 {
			var subConditions []string
			for _, w := range words {
				subConditions = append(subConditions, "(summarize LIKE ? OR type LIKE ?)")
				pattern := "%" + w + "%"
				params = append(params, pattern, pattern)
			}
			query += " AND (" + strings.Join(subConditions, " OR ") + ")"
		}
	}

	query += " ORDER BY id DESC LIMIT ?"
	params = append(params, limit)

	rows, err := m.dbManager.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []KnownFact
	for rows.Next() {
		var f KnownFact
		err := rows.Scan(&f.ID, &f.Type, &f.Summarize, &f.CreatedAt)
		if err != nil {
			continue
		}
		results = append(results, f)
	}
	return results, nil
}

// SaveFact 保存事实
func (m *MemoryLayer) SaveFact(ctx context.Context, factType, summarize string) (int64, error) {
	query := "INSERT INTO known_facts (type, summarize, created_at) VALUES (?, ?, ?)"
	res, err := m.dbManager.Exec(query, factType, summarize, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetRecentTasks 获取近期任务
func (m *MemoryLayer) GetRecentTasks(ctx context.Context, limit int) ([]Task, error) {
	query := `
		SELECT 
			task_id, description, task_type, parent_task_id, 
			understanding, execution_plan, status, meta_data, 
			created_at, updated_at, completed_at, summary, 
			pitfalls, current_focus 
		FROM tasks ORDER BY updated_at DESC LIMIT ?`
	rows, err := m.dbManager.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Task
	for rows.Next() {
		var t Task
		err := rows.Scan(
			&t.TaskID, &t.Description, &t.TaskType, &t.ParentTaskID,
			&t.Understanding, &t.ExecutionPlan, &t.Status, &t.MetaData,
			&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt, &t.Summary,
			&t.Pitfalls, &t.CurrentFocus,
		)
		if err != nil {
			continue
		}
		results = append(results, t)
	}
	return results, nil
}

// SaveState 保存系统状态
func (m *MemoryLayer) SaveState(ctx context.Context, key, value, category string) error {
	query := `INSERT INTO system_state (key, value, category, updated_at) 
			  VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			  ON CONFLICT(key) DO UPDATE SET 
			  value=excluded.value, 
			  category=excluded.category, 
			  updated_at=CURRENT_TIMESTAMP`
	_, err := m.dbManager.Exec(query, key, value, category)
	return err
}

// GetState 获取系统状态
func (m *MemoryLayer) GetState(ctx context.Context, key string) (string, error) {
	var value string
	err := m.dbManager.QueryRow("SELECT value FROM system_state WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// ========== Hook Management ==========

// Hook 待办钩子
// Hook 待办钩子
type Hook struct {
	HookID        string // mapped to hook_id
	Description   string
	Priority      string
	Tag           string
	Status        string
	RelatedTaskID string // mapped to related_task_id
	ExpiresAt     sql.NullTime
	CreatedAt     time.Time
	Summary       string
}

// CreateHook 创建待办钩子
func (m *MemoryLayer) CreateHook(ctx context.Context, description, priority, tag, taskID string, expiresHours int) (string, error) {
	// 生成 Hook ID (hook_hex5)
	// 使用纳秒的低 20 位生成 5 位 16 进制字符串 (约 100 万空间，足以区分)
	nano := time.Now().UnixNano()
	suffix := fmt.Sprintf("%x", nano&0xFFFFF)
	hookID := fmt.Sprintf("hook_%s", suffix)

	var expiresAt sql.NullTime
	if expiresHours > 0 {
		expiresAt.Time = time.Now().Add(time.Duration(expiresHours) * time.Hour)
		expiresAt.Valid = true
	}

	query := `INSERT INTO pending_hooks (
		hook_id, description, priority, tag, status, 
		related_task_id, expires_at, summary
	) VALUES (?, ?, ?, ?, 'open', ?, ?, ?)`

	// summary 显示为 #后缀
	summary := fmt.Sprintf("#%s", suffix)

	_, err := m.dbManager.Exec(
		query,
		hookID, description, priority, tag, taskID, expiresAt, summary,
	)
	if err != nil {
		return "", err
	}
	return hookID, nil
}

// ListHooks 列出钩子
func (m *MemoryLayer) ListHooks(ctx context.Context, status string) ([]Hook, error) {
	query := `
		SELECT 
			hook_id, description, priority, tag, status, 
			created_at, related_task_id, expires_at, summary 
		FROM pending_hooks 
		WHERE status = ? 
		ORDER BY created_at DESC`

	rows, err := m.dbManager.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []Hook
	for rows.Next() {
		var h Hook
		var relatedTaskID sql.NullString
		var summary sql.NullString
		if err := rows.Scan(
			&h.HookID, &h.Description, &h.Priority, &h.Tag, &h.Status,
			&h.CreatedAt, &relatedTaskID, &h.ExpiresAt, &summary,
		); err != nil {
			continue
		}
		h.RelatedTaskID = relatedTaskID.String
		h.Summary = summary.String
		hooks = append(hooks, h)
	}
	return hooks, nil
}

// ReleaseHook 释放钩子
func (m *MemoryLayer) ReleaseHook(ctx context.Context, hookID string, resultSummary string) error {
	_, err := m.dbManager.Exec(
		"UPDATE pending_hooks SET status = 'closed', result_summary = ? WHERE hook_id = ?",
		resultSummary, hookID,
	)
	return err
}
