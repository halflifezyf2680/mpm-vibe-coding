# MPM 更新报告（2026-02-24）

## 1. 本次目标

- 解决大仓库索引阻塞、WAL 膨胀、初始化超时问题。
- 提升 `project_map` 与 `flow_trace` 对 LLM 的可用性（降低认知负担）。
- 统一设计哲学：
  - 真相层（全量图数据）
  - 视图层（渐进披露）
  - 推理层（LLM 按需下钻）

## 2. 关键设计决策

### 2.1 三层分离

- **Truth Layer**: Rust AST 引擎 + `symbols.db` 维持可追溯全量事实。
- **View Layer**: Go 工具层输出预算化摘要（`brief/standard/deep`）。
- **Reasoning Layer**: LLM 先读摘要，再按 `scope` 和模式下钻。

### 2.2 默认易用优先

- 默认参数应可直接用（减少 LLM 参数决策）。
- 输出格式固定化（入口、上下游、风险、建议、截断说明）。

## 3. 功能改动总览

### 3.1 初始化与索引链路

- `initialize_project` 改为异步索引启动，避免初始化阻塞。
- 新增 `force_full_index` 参数，支持显式强制全量索引。
- 新增 `index_status` 工具，查看后台索引状态、心跳、DB 文件大小。

涉及文件：

- `mcp-server-go/internal/tools/system_tools.go`

### 3.2 Rust AST 引擎优化

- 增加默认忽略目录（第三方依赖/构建产物）。
- 增量索引元数据字段：`file_size`、`file_mtime`。
- 新增分层索引状态字段：`index_level`、`indexed_at`。
- 大仓策略：bootstrap + 预算解析（支持后续补录）。
- 支持 `--force-full` 强制全量。
- 分批事务提交 + checkpoint，降低 WAL 长事务风险。
- `structure` 模式支持 `scope` 且默认限制文件列表输出体积。

涉及文件：

- `mcp-server-go/internal/services/ast_indexer_rust/src/main.rs`

### 3.3 Go 服务层与工具层

- `ASTIndexer` 增加 `IndexScope` 与 `IndexFull` 路径。
- `buildIndexArgs` 支持 `scope` / `force-full`。
- `project_map(level=structure)` 走 Rust `structure` 模式，减少超大 JSON 风险。
- 修复 structure 扫描报错：补齐 `--db` 参数。
- `code_search` / `project_map` 优先范围补录或按新鲜度刷新。

涉及文件：

- `mcp-server-go/internal/services/ast_indexer.go`
- `mcp-server-go/internal/tools/search_tools.go`
- `mcp-server-go/internal/tools/analysis_tools.go`

### 3.4 新增流程工具 `flow_trace`

- 新增工具：`flow_trace`（symbol/file 双入口）。
- 新增模式：`brief` / `standard` / `deep`（渐进披露）。
- 输出增强：
  - 入口点
  - 上下游摘要
  - 风险建议
  - 预算截断提示
- 语言无关抽象：`callable/type/module/other`。
- 入口排序改造：按结构优先级 + 影响评分筛选高价值入口。
- 副作用标签优化：从粗略 contains 改为打分阈值模型（降低误报）。

涉及文件：

- `mcp-server-go/internal/tools/analysis_tools.go`

### 3.5 规则文档更新

- 在规则模板与项目规则中加入 `flow_trace` 使用时机。
- 强化“阅读业务流程优先 flow_trace”规范。

涉及文件：

- `mcp-server-go/internal/tools/system_tools.go`
- `mcp-server-go/_MPM_PROJECT_RULES.md`
- `_MPM_PROJECT_RULES.md`

### 3.6 清理项

- 删除重复脚本副本：`mcp-server-go/visualize_history.py`。

## 4. 回归测试与验证

已执行并通过：

- `go test ./internal/services/... ./internal/tools/...`
- `go build -o bin/mpm-go.exe ./cmd/server`
- `cargo build --release`（Rust 索引器）

场景验证（代表性）：

- `initialize_project` 异步成功，返回状态文件路径。
- `project_map(level=structure, scope=...)` 可返回目录级结果。
- `flow_trace(mode=standard/deep)` 能返回入口、上下游、风险与建议。

## 5. 当前已知问题

- `flow_trace` file 模式入口选择已明显改善，但在部分文件中仍可能选到高复杂工具函数而非业务主入口。
- 副作用标签已降噪，但仍属于启发式结果，不是语义执行证明。

## 6. 下一步建议

- 为 `flow_trace` 增加“关键路径 Top-K 固定槽位”与更强业务入口优先规则。
- 增加 `index_status` 的人类可读摘要字段（running/success + processed/total + wal_size）。
- 为结构视图与流程视图补充更系统的 E2E 回归用例。

## 6.1 当日补充（一次到位收敛）

为降低多轮微调成本，已对 `flow_trace` 入口排序进行一轮收敛式 hardening：

- 入口候选优先 `callable`，仅在 callable 缺失时回退 `type/module`。
- 评分函数改为“跨文件入边优先 + 影响面 + 复杂度上限”组合，降低辅助函数误排前列概率。
- 文件模式排序增加 tie-break：`ExternalIn` > 直接上游数 > 间接上游数 > score。
- 输出保留关键路径 Top3、阶段摘要、副作用和预算截断提示。

目的：尽量减少后续“同类问题小修小补”的反复。

## 7. 受影响文件清单（本轮核心）

- `mcp-server-go/internal/tools/system_tools.go`
- `mcp-server-go/internal/tools/analysis_tools.go`
- `mcp-server-go/internal/tools/search_tools.go`
- `mcp-server-go/internal/services/ast_indexer.go`
- `mcp-server-go/internal/services/ast_indexer_test.go`
- `mcp-server-go/internal/services/ast_indexer_rust/src/main.rs`
- `mcp-server-go/_MPM_PROJECT_RULES.md`
- `_MPM_PROJECT_RULES.md`
- `mcp-server-go/bin/mpm-go.exe`
- `mcp-server-go/bin/ast_indexer.exe`
