package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DatabaseManager 数据库连接管理器
type DatabaseManager struct {
	dbPath string
	db     *sql.DB
	mu     sync.Mutex
}

var (
	instances = make(map[string]*DatabaseManager)
	instLock  sync.Mutex
)

// GetDBForProject 获取指定项目的数据库管理器实例
func GetDBForProject(projectRoot string) (*DatabaseManager, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}

	instLock.Lock()
	defer instLock.Unlock()

	if mgr, ok := instances[absRoot]; ok {
		return mgr, nil
	}

	// 验证项目路径
	if !ValidateProjectPath(absRoot) {
		return nil, fmt.Errorf("invalid project path: %s", absRoot)
	}

	dbPath := filepath.Join(absRoot, ".mcp-data", "mcp_memory.db")
	mgr := &DatabaseManager{
		dbPath: dbPath,
	}

	if err := mgr.init(); err != nil {
		return nil, err
	}

	instances[absRoot] = mgr
	return mgr, nil
}

// NewDatabaseManager 创建一个新的数据库管理器实例（用于非项目级数据库，如全局 Prompt 库）
func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	mgr := &DatabaseManager{
		dbPath: dbPath,
	}
	if err := mgr.init(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *DatabaseManager) init() error {
	// 确保目录存在
	dir := filepath.Dir(m.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		return err
	}

	// 性能与并发优化 (WAL 模式)
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 30000",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return err
		}
	}

	m.db = db

	// 执行 Schema 自愈
	if err := m.healSchema(); err != nil {
		fmt.Fprintf(os.Stderr, "[DB][WARN] Schema healing failed: %v\n", err)
	}

	return nil
}

func (m *DatabaseManager) healSchema() error {
	// 1. 确保核心表存在
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS memos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT,
			entity TEXT,
			act TEXT,
			path TEXT,
			content TEXT,
			session_id TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			task_id TEXT PRIMARY KEY,
			description TEXT,
			task_type TEXT,
			parent_task_id TEXT,
			understanding TEXT,
			execution_plan TEXT,
			status TEXT DEFAULT 'in_progress',
			meta_data TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			summary TEXT,
			pitfalls TEXT,
			current_focus TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS known_facts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			summarize TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS system_state (
			key TEXT PRIMARY KEY,
			value TEXT,
			category TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pending_hooks (
			hook_id TEXT PRIMARY KEY,
			description TEXT,
			priority TEXT DEFAULT 'medium',
			context TEXT,
			result_summary TEXT,
			related_task_id TEXT,
			expires_at DATETIME,
			status TEXT DEFAULT 'open',
			tag TEXT,
			summary TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_chains (
			task_id TEXT PRIMARY KEY,
			description TEXT,
			protocol TEXT DEFAULT 'linear',
			status TEXT DEFAULT 'running',
			phases_json TEXT,
			current_phase TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_chain_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			phase_id TEXT,
			sub_id TEXT,
			event_type TEXT NOT NULL,
			payload TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES task_chains(task_id)
		)`,
	}

	for _, s := range schemas {
		if _, err := m.db.Exec(s); err != nil {
			return err
		}
	}

	// 2. 索引优化
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_memos_entity ON memos(entity)",
		"CREATE INDEX IF NOT EXISTS idx_memos_category ON memos(category)",
		"CREATE INDEX IF NOT EXISTS idx_memos_timestamp ON memos(timestamp DESC)",
		"CREATE INDEX IF NOT EXISTS idx_task_chain_events_task ON task_chain_events(task_id, created_at)",
	}
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx); err != nil {
			return err
		}
	}

	// 3. 数据迁移（ADD COLUMN，忽略已存在错误）
	migrations := []string{
		"ALTER TABLE task_chains ADD COLUMN reinit_count INTEGER DEFAULT 0",
	}
	for _, mig := range migrations {
		m.db.Exec(mig) // 忽略错误（列已存在时会报错，属正常）
	}

	return nil
}

// Exec 执行写操作
func (m *DatabaseManager) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

// QueryRow 执行单行查询
func (m *DatabaseManager) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

// Query 执行多行查询
func (m *DatabaseManager) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

// Close 关闭连接
func (m *DatabaseManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
