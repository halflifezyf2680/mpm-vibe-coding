# task_chain V3 设计文档：从线性执行器到协议状态机

## 1. 问题

当前 task_chain V2 是线性步骤队列：

```
Step1 → Step2 → Step3 → ... → Finish
```

- 不支持条件分支（验证失败后无法自动回退）
- 不支持循环（多个子任务无法批量处理）
- 不支持门控（阶段之间没有质量检查点）
- 内存存储，断连即丢（大工程跨会话无法恢复）
- "自适应"完全依赖 LLM 手动调 insert/update，引擎不参与决策

这意味着 task_chain 无法驱动真正的大工程（跨多文件、多模块、需要中间验证和回退的任务）。

## 2. 目标

将 task_chain 升级为**协议状态机**：

```
Phase(分析) → Gate(充分?) 
  ├─ pass  → Phase(执行, loop) → Gate(验证?)
  │                                 ├─ pass → 下一个子任务 / 集成
  │                                 └─ fail → 回退修复
  └─ fail  → 回到 Phase(分析)
```

核心约束：**执行者是 LLM，不是程序**。引擎负责状态管理和流转决策，LLM 负责每个阶段内的具体执行。

## 3. 核心概念

### 3.1 Phase（阶段）

替代 V2 的 Step，是状态机的基本节点。

```go
type PhaseType string

const (
    PhaseExecute PhaseType = "execute"  // 普通执行阶段
    PhaseGate    PhaseType = "gate"     // 门控检查点
    PhaseLoop    PhaseType = "loop"     // 循环阶段（内含子任务）
)

type Phase struct {
    ID          string      `json:"id"`           // 阶段唯一标识（如 "analyze", "implement"）
    Name        string      `json:"name"`         // 显示名称
    Type        PhaseType   `json:"type"`         // 阶段类型
    Status      PhaseStatus `json:"status"`       // pending / active / passed / failed / skipped
    Input       string      `json:"input"`        // 建议的工具调用
    Summary     string      `json:"summary"`      // 完成后的总结
    
    // Gate 专用
    OnPass      string      `json:"on_pass"`      // 通过时跳转的 phase ID
    OnFail      string      `json:"on_fail"`      // 失败时跳转的 phase ID
    MaxRetries  int         `json:"max_retries"`  // 最大重试次数（防死循环），默认 3
    RetryCount  int         `json:"retry_count"`  // 当前重试次数
    
    // Loop 专用
    SubTasks    []SubTask   `json:"sub_tasks"`    // 动态子任务列表
}
```

### 3.2 Gate（门控）

门控是特殊的 Phase，LLM 在此提交评估结果（pass/fail），引擎根据结果决定下一个阶段。

```
LLM 调用: complete(phase_id="verify", result="fail", summary="测试失败: 3个用例未通过")
引擎响应: "验证未通过，回退到 phase 'fix'（第 2/3 次重试）"
```

门控解决的问题：
- 强制 LLM 在关键节点做质量判断
- 引擎自动处理回退逻辑，LLM 不需要记住"失败了该回到哪"
- max_retries 防止死循环

### 3.3 Loop（循环阶段）

Loop 阶段内包含动态子任务列表，所有子任务完成后才进入下一阶段。

```
LLM 调用: spawn(phase_id="implement", sub_tasks=[
    {name: "重构 SessionManager", verify: "go test ./internal/core/..."},
    {name: "重构 MemoryLayer", verify: "go test ./internal/core/..."},
    {name: "更新 API 层", verify: "go test ./internal/tools/..."}
])
引擎响应: "已创建 3 个子任务，开始第 1 个: 重构 SessionManager"
```

子任务结构：
```go
type SubTask struct {
    ID       string        `json:"id"`
    Name     string        `json:"name"`
    Verify   string        `json:"verify"`   // 验证命令/标准
    Status   SubTaskStatus `json:"status"`   // pending / active / passed / failed
    Summary  string        `json:"summary"`
}
```

### 3.4 Protocol（协议）

协议 = Phase 列表 + 连接关系。替代 V2 的模板。

```yaml
protocols:
  - name: large_develop
    description: "大工程开发协议"
    phases:
      - id: analyze
        name: "需求分析与拆解"
        type: execute
        
      - id: plan_gate
        name: "拆解是否充分？"
        type: gate
        on_pass: implement
        on_fail: analyze
        max_retries: 2
        
      - id: implement
        name: "逐个实现子任务"
        type: loop
        
      - id: verify_gate
        name: "集成验证"
        type: gate
        on_pass: finalize
        on_fail: implement
        max_retries: 3
        
      - id: finalize
        name: "收尾归档"
        type: execute
```

## 4. 状态流转

```
                    ┌──────────┐
                    │ analyze  │ (execute)
                    └────┬─────┘
                         │ complete
                    ┌────▼─────┐
                ┌───│plan_gate │ (gate)
                │   └────┬─────┘
           fail │        │ pass
           (retry)       │
                │   ┌────▼─────┐
                └──►│implement │ (loop)
                    │ [子任务1] │──► 逐个执行
                    │ [子任务2] │
                    │ [子任务3] │
                    └────┬─────┘
                         │ 全部完成
                    ┌────▼──────┐
                ┌───│verify_gate│ (gate)
                │   └────┬──────┘
           fail │        │ pass
           (retry)       │
                │   ┌────▼─────┐
                └──►│ finalize │ (execute)
                    └──────────┘
```

## 5. API 设计

### 5.1 初始化

```
task_chain(mode="init", task_id="PROJ_001", description="...", protocol="large_develop")
// 或手动定义 phases
task_chain(mode="init", task_id="PROJ_001", description="...", phases=[...])
```

### 5.2 执行阶段

```
task_chain(mode="start", task_id="PROJ_001", phase_id="analyze")
```

### 5.3 完成阶段

```
// execute 类型
task_chain(mode="complete", task_id="PROJ_001", phase_id="analyze", summary="...")

// gate 类型（必须传 result）
task_chain(mode="complete", task_id="PROJ_001", phase_id="plan_gate", result="pass", summary="...")
```

### 5.4 生成子任务（loop 阶段）

```
task_chain(mode="spawn", task_id="PROJ_001", phase_id="implement", sub_tasks=[
    {name: "重构 SessionManager", verify: "go test ./internal/core/..."},
    {name: "重构 MemoryLayer", verify: "go test ./internal/core/..."}
])
```

### 5.5 完成子任务

```
task_chain(mode="complete_sub", task_id="PROJ_001", phase_id="implement", sub_id="sub_001", result="pass", summary="...")
```

### 5.6 查看状态

```
task_chain(mode="status", task_id="PROJ_001")
// 返回 JSON：当前 phase、各 phase 状态、子任务进度、重试计数
```

### 5.7 恢复任务（跨会话）

```
task_chain(mode="resume", task_id="PROJ_001")
// 从持久化存储加载，返回当前状态和下一步建议
```

## 6. 持久化

### 6.1 存储位置

使用项目级 SQLite 数据库（与 memo/facts 共用 `symbols.db`）。

### 6.2 表结构

```sql
CREATE TABLE task_chains (
    task_id     TEXT PRIMARY KEY,
    description TEXT,
    protocol    TEXT,          -- 协议名称
    status      TEXT,          -- running / paused / finished / failed
    phases_json TEXT,          -- 完整 phases JSON
    created_at  DATETIME,
    updated_at  DATETIME
);

CREATE TABLE task_chain_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id     TEXT,
    phase_id    TEXT,
    event_type  TEXT,          -- start / complete / fail / retry / spawn
    payload     TEXT,          -- JSON: summary, result, sub_tasks 等
    created_at  DATETIME,
    FOREIGN KEY (task_id) REFERENCES task_chains(task_id)
);
```

### 6.3 事件溯源

`task_chain_events` 记录每一次状态变更，支持：
- 跨会话恢复时重建上下文
- 事后复盘任务执行过程
- 与 memo/timeline 联动

## 7. 向后兼容性说明 (已于 2026-02 变更)

**重大变更**：在 2026-02 的版本中，我们正式移除了旧版的 `mode=step/insert/update/delete` 等基于内存的线性模式。

**原因**：
- **一致性**：V3 的 `linear` 协议可以覆盖旧版所有场景。
- **稳定性**：旧版内存模式在 Server 重启后数据即丢，不符合 Vibe Coding 长期任务的要求。
- **代码精简**：移除旧版逻辑后，MCP Server 代码体积减少约 500+ 行，显著降低维护成本。

**现状**：
- `task_chain` 现在强制使用协议状态机模式。
- 若需“线性步骤”，请使用 `init` 模式并选择 `protocol="linear"` (默认值)。
- 旧版的 `mode=step` 调用将返回错误，引导用户转向 `init` 模式。

## 8. 内置协议

### 8.1 linear（默认，兼容 V2）

纯线性执行，无门控无循环。等价于当前 V2 行为。

### 8.2 develop

```
analyze → plan_gate → implement(loop) → verify_gate → finalize
```

### 8.3 debug

```
reproduce → locate → fix(loop) → verify_gate → finalize
```

### 8.4 refactor

```
baseline → analyze → refactor(loop) → verify_gate → finalize
```

## 9. 实施计划

1. **Phase 1: 数据层**
   - 新增 DB 表（task_chains, task_chain_events）
   - 实现持久化读写

2. **Phase 2: 状态机引擎**
   - Phase/Gate/Loop 状态流转逻辑
   - 门控的 pass/fail 路由
   - Loop 的子任务管理
   - max_retries 防死循环

3. **Phase 3: API 层**
   - 新增 init/spawn/complete_sub/resume 模式
   - 改造 complete 支持 gate result
   - status 输出完整状态机视图

4. **Phase 4: 协议与兼容**
   - 内置协议定义（YAML 热加载）
   - V2 向后兼容层
   - 协议自定义支持

5. **Phase 5: 验证**
   - 单元测试
   - 端到端测试（用真实任务跑一遍协议）
   - 跨会话恢复测试

## 10. 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 在 gate 阶段给出错误的 pass/fail 判断 | 流程走偏 | gate 提示里明确评估标准；max_retries 兜底 |
| 死循环（gate 反复 fail） | 任务卡死 | max_retries 强制终止，提示 LLM 换策略 |
| 持久化数据损坏 | 任务无法恢复 | 事件溯源可重建；定期备份 |
| V2 兼容性破坏 | 现有用法失效 | V2 API 保持不变，内部映射到 linear 协议 |

## 11. 协议选择策略

### 11.1 不做自动推荐

`manager_analyze` 的复杂度评估基于 AST 静态分析，衡量的是代码结构复杂度，不是任务复杂度。一个改 3 行的 bug 可能需要跨 5 个模块验证（任务复杂度 High），但 AST 复杂度是 Low。因此不用 `manager_analyze` 自动推荐协议。

### 11.2 默认 linear，显式升级

- 不传 `protocol` 参数时，默认使用 `linear`（纯线性，等价于 V2）
- 用户或 LLM 认为需要门控/循环时，显式指定协议
- 工具描述里给出选择标准，LLM 自行判断

### 11.3 工具描述中的选择标准

```
协议选择：
- 不传 protocol（默认 linear）：任务步骤明确，线性推进即可
- protocol="develop"：跨模块开发，需要拆解子任务并逐个验证
- protocol="debug"：问题复现→定位→修复→验证，可能需要多轮重试
- protocol="refactor"：大范围重构，需要基线验证和逐步替换
```

### 11.4 用户显式触发

用户可以直接说：
- "用 develop 协议"→ LLM 传 `protocol="develop"`
- "这个任务比较大"→ LLM 判断后选择合适协议
- 什么都不说 → 默认 linear
