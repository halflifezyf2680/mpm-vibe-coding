# MPM 完整手册

> **从"聊天"到"可控交付"**

中文 | [English](MANUAL_EN.md)

---

## 目录

1. [核心概念](#1-核心概念)
2. [工具详解](#2-工具详解)
3. [最佳实践](#3-最佳实践)
4. [效能对比](#4-效能对比)
5. [FAQ](#5-faq)

---

## 1. 核心概念

### 1.1 MPM 解决什么问题？

AI 编程的三大痛点：

| 痛点 | 表现 | MPM 方案 |
|------|------|---------|
| **上下文迷失** | AI 不知道代码在哪 | `code_search` AST 精确定位 |
| **盲目修改** | 改了这里，漏了那里 | `code_impact` 调用链分析 |
| **记忆流失** | 每次对话从零开始 | `memo` + `system_recall` |

### 1.2 三层架构

```
感知层          调度层          记忆层
────────────────────────────────────────
code_search     manager_analyze   memo
code_impact     task_chain        system_recall
project_map     index_status      known_facts
flow_trace
```

- **感知层**：看代码（定位、分析、地图）
- **调度层**：管任务（规划、执行、断点）
- **记忆层**：存经验（备忘、召回、铁律）

### 1.3 AST 索引原理

MPM 使用 Rust AST 引擎解析代码，维护三个核心字段：

| 字段 | 说明 | 示例 |
|------|------|------|
| `canonical_id` | 全局唯一标识 | `func:core/auth.go::Login` |
| `scope_path` | 层级作用域 | `AuthManager::Login` |
| `callee_id` | 精确调用链 | `func:db/query.go::Exec` |

**为什么重要**：消除了"同名函数"歧义，`code_impact` 可以精确追踪多层调用链。

---

## 2. 工具详解

### 2.1 代码定位（4个）

#### project_map - 项目地图

**触发词**：`mpm 地图`、`mpm 结构`

**用途**：接手新项目时的第一步，快速建立认知。

**参数**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `scope` | 目录范围 | 整个项目 |
| `level` | `structure`(目录) / `symbols`(符号) | `symbols` |

**输出示例**：
```
📊 项目统计: 156 文件, 892 符号

🔴 高复杂度热点:
  - SessionManager::Handle (Score: 85)
  - PaymentService::Process (Score: 72)

📁 src/core/ (12 文件)
  ├── session.go
  │   └── func GetSession (L45-80) 🔴
  └── config.go
      └── func LoadConfig (L20-40) 🟢
```

---

#### code_search - 符号定位

**触发词**：`mpm 搜索`、`mpm 定位`

**用途**：精确定位函数/类定义，不靠字符串猜测。

**参数**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `query` | 搜索关键词 | 必填 |
| `scope` | 目录范围 | 整个项目 |
| `search_type` | `any`/`function`/`class` | `any` |

**5层降级搜索**：
```
1. 精确匹配 (exact)
2. 前缀/后缀匹配 (prefix/suffix)
3. 子串匹配 (substring)
4. 编辑距离 (levenshtein)
5. 词根匹配 (stem)
```

**输出示例**：
```
✅ 精确定义 (exact):
  func Login @ src/auth/login.go L45-67
  签名: func Login(ctx context.Context, cred Credentials) (*Token, error)

🔍 相似符号:
  [func] LoginUser @ src/api/user.go (score: 0.85)
```

---

#### code_impact - 影响分析

**触发词**：`mpm 影响`、`mpm 依赖`

**用途**：**修改前必做**，评估影响范围。

**参数**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `symbol_name` | 符号名 | 必填 |
| `direction` | `backward`(谁调用我)/`forward`(我调用谁)/`both` | `backward` |

**输出示例**：
```
CODE_IMPACT_REPORT: GetSession
RISK_LEVEL: high
AFFECTED_NODES: 15

#### POLLUTION_PROPAGATION_GRAPH
LAYER_1_DIRECT (4):
  - [api/handler.go:45-80] SYMBOL: HandleRequest
  - [service/auth.go:100-130] SYMBOL: Authenticate

LAYER_2_INDIRECT (11):
  - [main.go:50-100] SYMBOL: main
  ... and 9 more

#### ACTION_REQUIRED_CHECKLIST
- [ ] MODIFY_TARGET: [core/session.go:45-80]
- [ ] VERIFY_CALLER: [api/handler.go:45-80]
- [ ] VERIFY_CALLER: [service/auth.go:100-130]
```

---

#### flow_trace - 业务流程追踪

**触发词**：`mpm 流程`、`mpm flow`

**用途**：读取“入口-上游-下游”业务主链，比 `code_impact` 更偏流程理解。

**参数**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `symbol_name` / `file_path` | 二选一（同时提供时优先 symbol） | - |
| `scope` | 限定范围（大仓建议必填） | 空 |
| `direction` | `backward` / `forward` / `both` | `both` |
| `mode` | `brief` / `standard` / `deep`（渐进披露） | `brief` |
| `max_nodes` | 输出节点预算上限 | `40` |

**输出重点**：
- 入口点与位置
- 上下游关键节点
- 关键路径 Top3
- 阶段摘要 / 副作用（standard/deep）
- 截断提示（避免 LLM 被超大输出淹没）

---

### 2.2 任务管理（4个）

#### manager_analyze - 任务情报简报

**触发词**：`mpm 分析`、`mpm mg`

**适用场景**：满足以下任意一个条件时可选使用：
- 刚接手全陌生项目，没有任何代码线索
- 上下文过多过杂，LLM 需要收敛注意力再出发

**单点 Bug 修复、问题边界已清晰的场景不需要使用，直接用 `code_search` 定位即可。**

**参数**：
| 参数 | 说明 | 必填 |
|------|------|------|
| `task_description` | 原始任务描述 | ✅ |
| `intent` | `DEBUG`/`DEVELOP`/`REFACTOR`/`RESEARCH` | ✅ |
| `symbols` | 相关符号列表 | ✅ |
| `step` | 1=分析, 2=生成策略 | 默认1 |

**两步流程**：
```
Step 1: 分析
  → AST 搜索定位符号
  → 加载历史经验
  → 复杂度评估
  → 返回 task_id

Step 2: 生成策略
  → 基于分析结果动态生成战术建议
  → 返回 strategic_handoff
```

---

#### task_chain - 协议状态机任务链

**触发词**：`mpm 任务链`、`mpm chain`

**用途**：大工程/长任务专用，用预定义“协议”驱动多阶段流转。支持 Gate 门控、Loop 子任务分发和跨会话持久化。

**核心概念**：
1. **协议 (Protocol)**：预定义的任务模板（如 `develop`, `debug`）。
2. **阶段 (Phase)**：任务的不同生命周期，分为 `execute` (执行)、`gate` (校验)、`loop` (循环子任务)。
3. **门控 (Gate)**：强制自审点，通过 `result=pass|fail` 控制流向。

**操作模式 (Mode)**：

| 模式 | 必填参数 | 说明 |
|------|----------|------|
| `init` | `task_id`, `protocol` | 初始化任务链。默认 `protocol=linear` |
| `start` | `task_id`, `phase_id` | 开始一个特定阶段 |
| `complete` | `task_id`, `phase_id`, `summary` | 完成阶段。`gate` 类型需加 `result=pass\|fail` |
| `spawn` | `task_id`, `phase_id`, `sub_tasks` | 在 `loop` 阶段批量分发子任务 |
| `complete_sub` | `task_id`, `phase_id`, `sub_id`, `summary` | 完成单个子任务 |
| `status` | `task_id` | 查看当前进度墙（自动识别协议） |
| `resume` | `task_id` | 跨会话恢复任务（自动从数据库加载） |
| `protocol` | - | 查看所有可用协议列表及其阶段定义 |
| `finish` | `task_id` | 彻底关闭任务链 |

**内置协议流程表**：

| 协议 | 流程 (Phases) | 场景 |
|------|--------------|------|
| `linear` | main (execute) | 确定性极强的一步走任务 |
| `develop` | analyze → plan_gate → implement(loop) → verify_gate → finalize | 跨模块开发 |
| `debug` | reproduce → locate → fix(loop) → verify_gate → finalize | Bug 排查 |
| `refactor` | baseline → analyze → refactor(loop) → verify_gate → finalize | 大范围重构 |

**自审与升级机制**：

系统内置了 Re-init 拦截逻辑，防止 LLM 在同一个 TaskID 上反复原地打转：
- **第 1 次**：允许 Re-init（重置进度）。
- **第 2 次**：强制拦截，要求 LLM 必须解释原因并请求人类介入。

**典型示例**：

```javascript
// 1. 初始化一个重构任务
task_chain(mode="init", task_id="AUTH_REFACTOR", protocol="refactor", description="重构登录鉴权模块")

// 2. 完成基线检查
task_chain(mode="complete", task_id="AUTH_REFACTOR", phase_id="baseline", summary="当前测试全绿")

// 3. 进入重构循环
task_chain(mode="spawn", task_id="AUTH_REFACTOR", phase_id="refactor", sub_tasks=[
  {"name": "解耦 SessionStore"},
  {"name": "重写 JWT 签名逻辑"}
])

// 4. 完成子任务
task_chain(mode="complete_sub", task_id="AUTH_REFACTOR", phase_id="refactor", sub_id="sub_001", summary="Store 已提取为接口")
```

**为什么在 V3 弃用了 Linear Step 模式？**
因为 V3 的 `linear` 协议通过 `loop` 阶段可以完美实现旧版的动态步骤能力，且具备数据库持久化和多级自审能力，不再依赖不稳定的内存状态。

| 工具 | 触发词 | 用途 |
|------|--------|------|
| `manager_create_hook` | `mpm 挂起` | 创建待办/断点 |
| `manager_list_hooks` | `mpm 待办列表` | 查看待办 |
| `manager_release_hook` | `mpm 释放` | 完成待办 |

**Hook 特性**：支持 `expires_in_hours` 过期时间。

---

### 2.3 记忆系统（3个）

#### memo - 变更备忘录

**触发词**：`mpm 记录`、`mpm memo`

**用途**：**任何代码修改后必调用**，记录"为什么改"。

**参数**：
| 字段 | 说明 | 示例 |
|------|------|------|
| `category` | 分类 | `修改`/`开发`/`决策`/`避坑` |
| `entity` | 改动实体 | `session.go` |
| `act` | 行为 | `修复幂等问题` |
| `path` | 文件路径 | `core/session.go` |
| `content` | 详细说明 | 为什么这么改 |

**示例**：
```javascript
memo(items=[{
  category: "修复",
  entity: "GetSession",
  act: "添加幂等检查",
  path: "core/session.go",
  content: "防止重复请求创建多个 session"
}])
```

---

#### system_recall - 记忆召回

**触发词**：`mpm 历史`、`mpm recall`

**用途**：检索过去的决策和修改，**"宽进严出"** 策略。

**参数**：
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `keywords` | 关键词（多字段模糊匹配） | 必填 |
| `category` | 类型过滤 | 全部 |
| `limit` | 返回条数 | 20 |

**宽进严出策略**：
- **宽进**：在 `Entity` / `Act` / `Content` 多字段中 OR 匹配
- **严出**：通过 `category` 过滤 + `limit` 限制
- **精细输出**：分类展示（Known Facts 优先）+ 时间戳（近→远）

**输出示例**：
```
## 📌 Known Facts (2)

- **[避坑]** 修改 session 逻辑前必须先检查依赖 _(ID: 1, 2026-01-15)_

## 📝 Memos (3)

- **[42] 2026-02-15 14:30** (修复) 添加幂等检查: 防止重复请求...
- **[41] 2026-02-14 10:00** (开发) 新增 timeout 参数: 适配阿里云...
```

**典型用法**：
```
system_recall(keywords="session timeout")
  → 找到所有涉及 session/timeout 的历史记录

system_recall(keywords="幂等", category="避坑")
  → 只返回避坑类记录
```

---

#### known_facts - 铁律存档

**触发词**：`mpm 铁律`、`mpm 避坑`

**用途**：存档经过验证的规则，`manager_analyze` 会自动加载。

**示例**：
```javascript
known_facts(type="避坑", summarize="修改 session 逻辑前必须先检查依赖")
```

---

### 2.4 增强工具（3个）

#### persona - 人格管理

**触发词**：`mpm 人格`

**设计理念**：人格是 **Buff 机制**，不是持久配置。

| 特性 | 说明 |
|------|------|
| **临时性** | 切换人格 = 临时加 buff，用完即走 |
| **不持久化** | 不存数据库，不跨会话 |
| **健康指示** | 人格表现模糊 = 上下文已稀释，需要处理 |

**上下文稀释判断**：

人格表现强度可作为上下文健康的**信号**：

| 人格表现 | 含义 | 建议 |
|---------|------|------|
| 风格鲜明 | 上下文健康 | 继续当前对话 |
| 表现模糊 | context 已稀释 | 新开对话 / compact / 输入提示词收敛注意力 |

**操作模式**：

| 模式 | 说明 | 示例 |
|------|------|------|
| `list` | 列出所有人格 | `persona(mode="list")` |
| `activate` | 激活人格 | `persona(mode="activate", name="zhuge")` |
| `create` | 新增人格 | `persona(mode="create", name="my_expert", ...)` |
| `update` | 更新人格 | `persona(mode="update", name="my_expert", ...)` |
| `delete` | 删除人格 | `persona(mode="delete", name="my_expert")` |

**创建人格参数**：
| 参数 | 说明 |
|------|------|
| `display_name` | 显示名称 |
| `hard_directive` | 核心指令 |
| `style_must` | 必须遵守的风格 |
| `style_signature` | 标志性表达 |
| `style_taboo` | 禁用表达 |
| `triggers` | 触发词 |

**内置人格**：
| 人格 | 代号 | 风格强度 | 适用场景 |
|------|------|---------|---------|
| 孔明 | `zhuge` | 中 | 架构设计、代码审查 |
| 懂王 | `trump` | 强 | 头脑风暴、打破僵局 |
| 哆啦 | `doraemon` | 中 | 新手引导、编写教程 |
| 柯南 | `detective_conan` | 中 | Bug 排查、日志分析 |

---

#### skill_load - 技能加载

**触发词**：`mpm 技能`

**用途**：加载领域专家指南（如 Refactoring、Go-expert）。

---

#### open_timeline - 项目演进

**触发词**：`mpm 时间线`

**用途**：生成 HTML 可视化项目演进历史。

---

## 3. 最佳实践

### 3.0 先接管规则（必须）

初始化后，优先将 `_MPM_PROJECT_RULES.md` 注入你使用的客户端系统规则，再开始任何开发任务。

**最小步骤**：

1. 执行 `initialize_project`
2. 打开项目根目录 `_MPM_PROJECT_RULES.md`
3. 将全文粘贴到客户端的系统规则区域

**常见客户端放置位置**（不同版本命名可能略有差异）：

| 客户端 | 建议放置位置 |
|------|-------------|
| Claude Code | System Prompt / Project Instructions |
| OpenCode | System Rules / Workspace Rules |
| Cursor | Rules for AI / Project Rules |

**推荐首句**：

`先读取并严格遵守 _MPM_PROJECT_RULES.md，再执行任务。`

### 3.1 标准工作流

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│    定位     │ ─▶ │    执行     │ ─▶ │    记录     │
│ code_search │     │ 代码修改    │     │   memo      │
│ code_impact │     │             │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   ▲                   │
       └───────────────────┴───────────────────┘

[可选] 项目文件数 > 100 且用户未提及任何符号时，在定位阶段前可先调用 manager_analyze。
```

### 3.2 黄金法则

| 法则 | 说明 |
|------|------|
| **修改前必定位** | 先 `code_search` 再改代码 |
| **大改动必评估** | 先 `code_impact` 看影响 |
| **变更即记录** | 修改后必调用 `memo` |
| **新对话读日志** | 先读 `dev-log.md` 恢复上下文 |

### 3.3 修改代码标准流程

```
1. code_search(query="目标函数")      # 定位
2. code_impact(symbol_name="目标函数") # 评估影响
3. (阅读代码)
4. (执行修改)
5. memo(items=[{...}])                # 记录
```

### 3.4 命名规范（Vibe Coding）

**三大法则**：

1. **符号锚定**：拒绝通用词
   - ❌ `data = get_data()`
   - ✅ `verified_payload = auth_service.fetch_verified_payload()`

2. **前缀即领域**：使用 `domain_action_target`
   - `ui_btn_submit`、`api_req_login`、`db_conn_main`

3. **可检索性优先**：名字越长，冲突越少
   - `transaction_unique_trace_id` 比 `id` 更易搜索

---

## 4. 效能对比

### 4.1 Case 1：符号定位

**任务**：分析 `memo` 工具的实现逻辑

| 指标 | 无 MPM | 有 MPM | 提升 |
|------|--------|--------|------|
| 步骤数 | 12+ 步 | 3 步 | **300%** |
| 工具调用 | 10+ 次 | 2 次 | **400%** |
| 首步命中率 | 0% | 100% | **∞** |

**原因**：`code_search` 直接返回精确坐标（文件:行号），无需反复试错。

---

### 4.2 Case 2：影响评估

**任务**：评估修改 `session.go` 的风险

| 维度 | 无 MPM | 有 MPM |
|------|--------|--------|
| 风险感知 | 基于局部猜测 | **AST 调用链分析** |
| Token 消耗 | 通读文件 (4000+) | 地图摘要 (~800) |
| 输出 | 模糊反问 | **精确修改清单** |

---

### 4.3 Case 3：项目认知

**任务**：冷启动理解 300+ 文件的新项目

| 指标 | 无 MPM | 有 MPM |
|------|--------|--------|
| 总耗时 | 40 秒 | **15 秒** |
| 工具调用 | 4+ 次 | 1 次 |
| 认知路径 | 配置→源码→拼装 | **直达结构化地图** |

---

### 4.4 Case 4：灾难恢复

**场景**：误执行 `git reset --hard`，丢失一天未提交的代码

| 维度 | Git | MPM 数据库 |
|------|-----|-----------|
| 记录触发 | 显式 commit | **修改即 memo** |
| 覆盖范围 | 物理文本 | **意图 + 语义** |
| 恢复成本 | 手动重写 | **指导性恢复** |

**结论**：MPM 保护的是开发过程，Git 保护的是代码。

---

## 5. FAQ

### Q1: `initialize_project` 什么时候需要调用？

**只在以下情况**：
- 重启了 MCP Server / IDE
- 首次使用 MPM

**高级选项**：
- `force_full_index=true`：强制全量索引（禁用大仓 bootstrap 策略）
- `index_status`：查看后台索引进度/心跳/数据库体积

**如果只是新开对话**：直接读 `dev-log.md` 即可，无需重新初始化。

---

### Q2: `code_search` 和 IDE 自带搜索有什么区别？

| 维度 | IDE 搜索 | `code_search` |
|------|---------|---------------|
| 匹配方式 | 文本级 | **AST 符号级** |
| 同名歧义 | 无法区分 | **canonical_id 精确** |
| 上下文 | 需手动查看 | **自动返回签名** |

**建议**：先用 `code_search` 定位，再用 IDE 精读。

---

### Q3: DICE 复杂度算法是什么？

基于 **精确调用链** 计算复杂度：

```
complexity_score = 
    (out_degree × 2.0) +   // Fan-out: 调用了多少
    (in_degree × 1.0)      // Fan-in: 被多少人依赖
```

**评分等级**：
| 分数 | 等级 | 建议 |
|------|------|------|
| 0-20 | Simple | 直接修改 |
| 20-50 | Medium | 先看调用者 |
| 50-80 | High | 先 `code_impact` |
| 80+ | Extreme | 需要详细规划 |

---

### Q4: 数据存储在哪里？

| 数据 | 位置 |
|------|------|
| AST 索引 | `.mcp-data/symbols.db` (SQLite) |
| Memos | `.mcp-data/mcp_memory.db` |
| 人类可读日志 | `dev-log.md` |
| 项目规则 | `_MPM_PROJECT_RULES.md` |

**建议**：`.mcp-data/` 加入 `.gitignore`，但 `dev-log.md` 可提交。

---

### Q5: 支持哪些语言？

| 语言 | 扩展名 |
|------|--------|
| Python | .py |
| Go | .go |
| JavaScript/TypeScript | .js, .ts, .tsx, .mjs |
| Rust | .rs |
| Java | .java |
| C/C++ | .c, .cpp, .h, .hpp |

---

## 触发词速查表

| 分类 | 触发词 | 工具 |
|------|--------|------|
| 系统 | `mpm 初始化` | `initialize_project` |
| 系统 | `mpm 索引状态` `mpm index status` | `index_status` |
| 定位 | `mpm 搜索` `mpm 定位` | `code_search` |
| 分析 | `mpm 影响` `mpm 依赖` | `code_impact` |
| 地图 | `mpm 地图` `mpm 结构` | `project_map` |
| 流程 | `mpm 流程` `mpm flow` | `flow_trace` |
| 任务 | `mpm 分析` `mpm mg` | `manager_analyze` |
| 链式 | `mpm 任务链` `mpm chain` | `task_chain` |
| 待办 | `mpm 挂起` `mpm 待办列表` `mpm 释放` | Hook 系列 |
| 记忆 | `mpm 记录` `mpm 历史` `mpm 铁律` | 记忆系列 |
| 人格 | `mpm 人格` | `persona` |
| 技能 | `mpm 技能列表` `mpm 加载技能` | 技能系列 |
| 可视 | `mpm 时间线` | `open_timeline` |

---

*MPM Manual v2.1 - 2026-02*
