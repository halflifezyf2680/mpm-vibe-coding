# MPM Complete Manual

> **From "Chatting" to "Controlled Delivery"**

[中文](MANUAL.md) | English

---

## Table of Contents

1. [Core Concepts](#1-core-concepts)
2. [Tool Reference](#2-tool-reference)
3. [Best Practices](#3-best-practices)
4. [Performance Comparison](#4-performance-comparison)
5. [FAQ](#5-faq)

---

## 1. Core Concepts

### 1.1 What Problems Does MPM Solve?

Three major pain points in AI coding:

| Pain Point | Symptom | MPM Solution |
|------------|---------|--------------|
| **Context Lost** | AI doesn't know where code is | `code_search` AST precision |
| **Blind Changes** | Fixed here, broke there | `code_impact` call chain analysis |
| **Memory Loss** | Start from zero every session | `memo` + `system_recall` |

### 1.2 Three-Layer Architecture

```
Perception      Scheduling      Memory
────────────────────────────────────────
code_search     manager_analyze   memo
code_impact     task_chain        system_recall
project_map     index_status      known_facts
flow_trace
```

- **Perception**: See code (locate, analyze, map)
- **Scheduling**: Manage tasks (plan, execute, checkpoint)
- **Memory**: Store experience (memo, recall, rules)

### 1.3 AST Indexing Principles

MPM uses a Rust AST engine to parse code, maintaining three core fields:

| Field | Description | Example |
|-------|-------------|---------|
| `canonical_id` | Globally unique identifier | `func:core/auth.go::Login` |
| `scope_path` | Hierarchical scope | `AuthManager::Login` |
| `callee_id` | Precise call chain | `func:db/query.go::Exec` |

**Why it matters**: Eliminates "same-name function" ambiguity, `code_impact` can precisely track multi-layer call chains.

---

## 2. Tool Reference

### 2.1 Code Location (4 tools)

#### project_map - Project Map

**Triggers**: `mpm map`, `mpm structure`

**Purpose**: First step when taking over a new project, quickly build understanding.

**Parameters**:
| Parameter | Description | Default |
|-----------|-------------|---------|
| `scope` | Directory scope | Entire project |
| `level` | `structure`(dirs) / `symbols`(symbols) | `symbols` |

**Output Example**:
```
📊 Project Stats: 156 files, 892 symbols

🔴 High Complexity Hotspots:
  - SessionManager::Handle (Score: 85)
  - PaymentService::Process (Score: 72)

📁 src/core/ (12 files)
  ├── session.go
  │   └── func GetSession (L45-80) 🔴
  └── config.go
      └── func LoadConfig (L20-40) 🟢
```

---

#### code_search - Symbol Lookup

**Triggers**: `mpm search`, `mpm locate`

**Purpose**: Precisely locate function/class definitions, no string guessing.

**Parameters**:
| Parameter | Description | Default |
|-----------|-------------|---------|
| `query` | Search keyword | Required |
| `scope` | Directory scope | Entire project |
| `search_type` | `any`/`function`/`class` | `any` |

**5-Layer Fallback Search**:
```
1. Exact match
2. Prefix/suffix match
3. Substring match
4. Levenshtein distance
5. Stem match
```

**Output Example**:
```
✅ Exact Definition:
  func Login @ src/auth/login.go L45-67
  Signature: func Login(ctx context.Context, cred Credentials) (*Token, error)

🔍 Similar Symbols:
  [func] LoginUser @ src/api/user.go (score: 0.85)
```

---

#### code_impact - Impact Analysis

**Triggers**: `mpm impact`, `mpm dependency`

**Purpose**: **Must do before modifications**, assess impact scope.

**Parameters**:
| Parameter | Description | Default |
|-----------|-------------|---------|
| `symbol_name` | Symbol name | Required |
| `direction` | `backward`(who calls me)/`forward`(I call whom)/`both` | `backward` |

**Output Example**:
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

#### flow_trace - Business Flow Trace

**Triggers**: `mpm flow`

**Purpose**: Read the main business chain as "entry-upstream-downstream". Compared to `code_impact`, this focuses on flow comprehension.

**Parameters**:
| Parameter | Description | Default |
|-----------|-------------|---------|
| `symbol_name` / `file_path` | One of the two (symbol wins if both provided) | - |
| `scope` | Scope filter (recommended for large repositories) | empty |
| `direction` | `backward` / `forward` / `both` | `both` |
| `mode` | `brief` / `standard` / `deep` (progressive disclosure) | `brief` |
| `max_nodes` | Output node budget | `40` |

**Output Focus**:
- Entry point and location
- Upstream/downstream key nodes
- Critical paths Top 3
- Stage summary / side-effects (standard/deep)
- Budget truncation hints

---

### 2.2 Task Management (4 tools)

#### manager_analyze - Task Intelligence Briefing

**Triggers**: `mpm analyze`, `mpm mg`

**When to use**: This tool is **optional**. Use it when any of the following applies:
- Just took over a completely unfamiliar project with no code clues at all
- Context is too noisy and scattered; the LLM needs to converge attention before proceeding

**For single-point bug fixes or tasks with a clear target, skip this tool and go directly to `code_search`.**

**Parameters**:
| Parameter | Description | Required |
|-----------|-------------|----------|
| `task_description` | Original task description | ✅ |
| `intent` | `DEBUG`/`DEVELOP`/`REFACTOR`/`RESEARCH` | ✅ |
| `symbols` | Related symbol list | ✅ |
| `step` | 1=analyze, 2=generate strategy | Default 1 |

**Two-Step Process**:
```
Step 1: Analyze
  → AST search to locate symbols
  → Load historical experience
  → Complexity assessment
  → Return task_id

Step 2: Generate Strategy
  → Dynamically generate tactical suggestions based on analysis
  → Return strategic_handoff
```

---

#### task_chain - Protocol State Machine

**Triggers**: `mpm chain`, `mpm taskchain`

**Purpose**: Designed for large projects and long-running tasks. Uses predefined "protocols" to drive multi-phase workflows with built-in Gate checkpoints, Loop sub-tasks, and cross-session persistence.

**Core Concepts**:
1. **Protocol**: A predefined task template (e.g., `develop`, `debug`).
2. **Phase**: Different lifecycle stages of a task, categorized as `execute`, `gate`, or `loop`.
3. **Gate**: A mandatory self-review checkpoint controlled by `result=pass|fail`.

**Operation Modes**:

| Mode | Required Parameters | Description |
|------|--------------------|-------------|
| `init` | `task_id`, `protocol` | Initialize the task chain. Default `protocol=linear` |
| `start` | `task_id`, `phase_id` | Start a specific phase |
| `complete` | `task_id`, `phase_id`, `summary` | Complete a phase. `gate` types require `result=pass\|fail` |
| `spawn` | `task_id`, `phase_id`, `sub_tasks` | Dispatch sub-tasks in a `loop` phase |
| `complete_sub` | `task_id`, `phase_id`, `sub_id`, `summary` | Complete a single sub-task |
| `status` | `task_id` | View current progress wall (auto-identifies protocol) |
| `resume` | `task_id` | Restore task across sessions (loads automatically from DB) |
| `protocol` | - | List all available protocols and their phase definitions |
| `finish` | `task_id` | Permanently close the task chain |

**Built-in Protocol Flow Table**:

| Protocol | Flow (Phases) | Use Case |
|----------|---------------|----------|
| `linear` | main (execute) | Highly deterministic, single-step tasks |
| `develop` | analyze → plan_gate → implement(loop) → verify_gate → finalize | Cross-module development |
| `debug` | reproduce → locate → fix(loop) → verify_gate → finalize | Bug investigation |
| `refactor` | baseline → analyze → refactor(loop) → verify_gate → finalize | Large-scale refactoring |

**Self-Review & Escalation**:

Built-in Re-init interception prevents the LLM from spinning in circles on the same TaskID:
- **1st time**: Re-init allowed (resets progress).
- **2nd time**: Hard block, requiring the LLM to explain the deviation and request human intervention.

**Typical Usage**:

```javascript
// 1. Initialize a refactoring task
task_chain(mode="init", task_id="AUTH_REFACTOR", protocol="refactor", description="Refactor auth module")

// 2. Complete baseline check
task_chain(mode="complete", task_id="AUTH_REFACTOR", phase_id="baseline", summary="Current tests pass")

// 3. Enter refactor loop
task_chain(mode="spawn", task_id="AUTH_REFACTOR", phase_id="refactor", sub_tasks=[
  {"name": "Decouple SessionStore"},
  {"name": "Rewrite JWT signing"}
])

// 4. Complete a sub-task
task_chain(mode="complete_sub", task_id="AUTH_REFACTOR", phase_id="refactor", sub_id="sub_001", summary="Store extracted to interface")
```

**Why was Linear Step Mode deprecated in V3?**
The V3 `linear` protocol, combined with `loop` phases, perfectly replaces the dynamic step capabilities of the old mode while providing robust DB persistence and multi-level self-review, no longer relying on volatile memory state.

---

#### Hook Series (3 tools)

| Tool | Trigger | Purpose |
|------|---------|---------|
| `manager_create_hook` | `mpm suspend` | Create todo/checkpoint |
| `manager_list_hooks` | `mpm todolist` | View todos |
| `manager_release_hook` | `mpm release` | Complete todo |

**Hook Feature**: Supports `expires_in_hours` expiration time.

---

### 2.3 Memory System (3 tools)

#### memo - Change Documentation

**Triggers**: `mpm memo`, `mpm record`

**Purpose**: **Must call after any code change**, record "why changed".

**Parameters**:
| Field | Description | Example |
|-------|-------------|---------|
| `category` | Category | `fix`/`develop`/`decision`/`pitfall` |
| `entity` | Changed entity | `session.go` |
| `act` | Action | `fix idempotency issue` |
| `path` | File path | `core/session.go` |
| `content` | Detailed explanation | Why this change |

**Example**:
```javascript
memo(items=[{
  category: "fix",
  entity: "GetSession",
  act: "add idempotency check",
  path: "core/session.go",
  content: "prevent duplicate requests from creating multiple sessions"
}])
```

---

#### system_recall - Memory Retrieval

**Triggers**: `mpm recall`, `mpm history`

**Purpose**: Retrieve past decisions and changes, **"Wide-In Strict-Out"** strategy.

**Parameters**:
| Parameter | Description | Default |
|-----------|-------------|---------|
| `keywords` | Keywords (multi-field fuzzy match) | Required |
| `category` | Type filter | All |
| `limit` | Return count | 20 |

**Wide-In Strict-Out Strategy**:
- **Wide-In**: OR match across `Entity` / `Act` / `Content` fields
- **Strict-Out**: Filter by `category` + limit by `limit`
- **Refined Output**: Categorized display (Known Facts first) + timestamp (recent→old)

**Output Example**:
```
## 📌 Known Facts (2)

- **[pitfall]** Must check dependencies before modifying session logic _(ID: 1, 2026-01-15)_

## 📝 Memos (3)

- **[42] 2026-02-15 14:30** (fix) add idempotency check: prevent duplicate requests...
- **[41] 2026-02-14 10:00** (develop) add timeout parameter: adapt to Alibaba Cloud...
```

---

#### known_facts - Rules Archive

**Triggers**: `mpm rule`, `mpm pitfall`

**Purpose**: Archive verified rules, `manager_analyze` auto-loads them.

**Example**:
```javascript
known_facts(type="pitfall", summarize="Must check dependencies before modifying session logic")
```

---

### 2.4 Enhancement Tools (3 tools)

#### persona - Personality Management

**Triggers**: `mpm persona`

**Design Philosophy**: Personality is a **Buff mechanism**, not persistent config.

| Feature | Description |
|---------|-------------|
| **Temporary** | Switch personality = temporary buff, done when finished |
| **No Persistence** | Not stored in DB, not cross-session |
| **Health Indicator** | Blurred personality = context diluted, needs attention |

**Context Dilution Detection**:

Personality expression strength serves as a **signal** for context health:

| Personality Expression | Meaning | Suggestion |
|----------------------|---------|------------|
| Distinct style | Context healthy | Continue current session |
| Blurred expression | Context diluted | New session / compact / input prompt to converge attention |

**Operation Modes**:

| Mode | Description | Example |
|------|-------------|---------|
| `list` | List all personalities | `persona(mode="list")` |
| `activate` | Activate personality | `persona(mode="activate", name="zhuge")` |
| `create` | Create personality | `persona(mode="create", name="my_expert", ...)` |
| `update` | Update personality | `persona(mode="update", name="my_expert", ...)` |
| `delete` | Delete personality | `persona(mode="delete", name="my_expert")` |

**Built-in Personalities**:
| Personality | Code | Style Strength | Use Case |
|-------------|------|----------------|----------|
| Zhuge Liang | `zhuge` | Medium | Architecture design, code review |
| Trump | `trump` | Strong | Brainstorming, break deadlock |
| Doraemon | `doraemon` | Medium | Beginner guidance, tutorial writing |
| Detective Conan | `detective_conan` | Medium | Bug investigation, log analysis |

---

#### skill_load - Skill Loading

**Triggers**: `mpm skill`

**Purpose**: Load domain expert guides (e.g., Refactoring, Go-expert).

---

#### open_timeline - Project Evolution

**Triggers**: `mpm timeline`

**Purpose**: Generate HTML visualization of project evolution history.

---

## 3. Best Practices

### 3.0 Rules First (Required)

After initialization, apply `_MPM_PROJECT_RULES.md` to your client system rules before starting any coding task.

**Minimal steps**:

1. Run `initialize_project`
2. Open `_MPM_PROJECT_RULES.md` in project root
3. Paste it into your client's system-rules area

**Common client locations** (labels vary by version):

| Client | Recommended location |
|--------|----------------------|
| Claude Code | System Prompt / Project Instructions |
| OpenCode | System Rules / Workspace Rules |
| Cursor | Rules for AI / Project Rules |

**Recommended first prompt**:

`Read and strictly follow _MPM_PROJECT_RULES.md before executing tasks.`

### 3.1 Standard Workflow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Locate    │ ──▶ │   Execute   │ ──▶ │   Record    │
│ code_search │     │ Code Change │     │    memo     │
│ code_impact │     │             │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   ▲                   │
       └───────────────────┴───────────────────┘

[Optional] When project has > 100 files and user mentions no symbols,
call manager_analyze before the Locate step.
```

### 3.2 Golden Rules

| Rule | Description |
|------|-------------|
| **Locate Before Modify** | `code_search` before changing code |
| **Assess Before Big Change** | `code_impact` to see impact |
| **Record Every Change** | Must call `memo` after modification |
| **Read Log on New Session** | Read `dev-log.md` to restore context |

### 3.3 Standard Code Modification Flow

```
1. code_search(query="target_function")      # Locate
2. code_impact(symbol_name="target_function") # Assess impact
3. (Read code)
4. (Execute modification)
5. memo(items=[{...}])                        # Record
```

### 3.4 Naming Conventions (Vibe Coding)

**Three Rules**:

1. **Symbol Anchoring**: Reject generic words
   - ❌ `data = get_data()`
   - ✅ `verified_payload = auth_service.fetch_verified_payload()`

2. **Prefix as Domain**: Use `domain_action_target`
   - `ui_btn_submit`, `api_req_login`, `db_conn_main`

3. **Searchability First**: Longer names, fewer conflicts
   - `transaction_unique_trace_id` is easier to search than `id`

---

## 4. Performance Comparison

### 4.1 Case 1: Symbol Location

**Task**: Analyze `memo` tool implementation logic

| Metric | Without MPM | With MPM | Improvement |
|--------|-------------|----------|-------------|
| Steps | 12+ | 3 | **300%** |
| Tool calls | 10+ | 2 | **400%** |
| First-step accuracy | 0% | 100% | **∞** |

**Reason**: `code_search` directly returns precise coordinates (file:line), no trial and error.

---

### 4.2 Case 2: Impact Assessment

**Task**: Assess risk of modifying `session.go`

| Dimension | Without MPM | With MPM |
|-----------|-------------|----------|
| Risk perception | Based on local guessing | **AST call chain analysis** |
| Token consumption | Read entire files (4000+) | Map summary (~800) |
| Output | Vague questions | **Precise modification checklist** |

---

### 4.3 Case 3: Project Understanding

**Task**: Cold-start understanding of a 300+ file project

| Metric | Without MPM | With MPM |
|--------|-------------|----------|
| Total time | 40 seconds | **15 seconds** |
| Tool calls | 4+ | 1 |
| Cognitive path | Config→Source→Assembly | **Direct to structured map** |

---

### 4.4 Case 4: Disaster Recovery

**Scenario**: Accidentally ran `git reset --hard`, lost a day of uncommitted code

| Dimension | Git | MPM Database |
|-----------|-----|--------------|
| Record trigger | Explicit commit | **Memo on every change** |
| Coverage | Physical text | **Intent + Semantics** |
| Recovery cost | Manual rewrite | **Guided recovery** |

**Conclusion**: MPM protects the development process, Git protects the code.

---

## 5. FAQ

### Q1: When should I call `initialize_project`?

**Only in these cases**:
- Restarted MCP Server / IDE
- First time using MPM

**Advanced options**:
- `force_full_index=true`: force full indexing (disable bootstrap strategy for large repositories)
- `index_status`: inspect background indexing progress / heartbeat / database file sizes

**If just starting a new conversation**: Just read `dev-log.md`, no need to reinitialize.

---

### Q2: What's the difference between `code_search` and IDE search?

| Dimension | IDE Search | `code_search` |
|-----------|------------|---------------|
| Match method | Text level | **AST symbol level** |
| Same-name ambiguity | Cannot distinguish | **canonical_id precision** |
| Context | Need manual viewing | **Auto-return signature** |

**Suggestion**: Use `code_search` to locate, then IDE to read in detail.

---

### Q3: What is the DICE complexity algorithm?

Calculates complexity based on **precise call chains**:

```
complexity_score = 
    (out_degree × 2.0) +   // Fan-out: how many it calls
    (in_degree × 1.0)      // Fan-in: how many depend on it
```

**Rating Levels**:
| Score | Level | Suggestion |
|-------|-------|------------|
| 0-20 | Simple | Modify directly |
| 20-50 | Medium | Check callers first |
| 50-80 | High | Run `code_impact` first |
| 80+ | Extreme | Need detailed planning |

---

### Q4: Where is data stored?

| Data | Location |
|------|----------|
| AST index | `.mcp-data/symbols.db` (SQLite) |
| Memos | `.mcp-data/mcp_memory.db` |
| Human-readable log | `dev-log.md` |
| Project rules | `_MPM_PROJECT_RULES.md` |

**Suggestion**: Add `.mcp-data/` to `.gitignore`, but `dev-log.md` can be committed.

---

### Q5: Which languages are supported?

| Language | Extensions |
|----------|------------|
| Python | .py |
| Go | .go |
| JavaScript/TypeScript | .js, .ts, .tsx, .mjs |
| Rust | .rs |
| Java | .java |
| C/C++ | .c, .cpp, .h, .hpp |

---

## Trigger Quick Reference

| Category | Triggers | Tool |
|----------|----------|------|
| System | `mpm init` | `initialize_project` |
| System | `mpm index status` | `index_status` |
| Location | `mpm search` `mpm locate` | `code_search` |
| Analysis | `mpm impact` `mpm dependency` | `code_impact` |
| Map | `mpm map` `mpm structure` | `project_map` |
| Flow | `mpm flow` | `flow_trace` |
| Task | `mpm analyze` `mpm mg` | `manager_analyze` |
| Chain | `mpm chain` `mpm taskchain` | `task_chain` |
| Todo | `mpm suspend` `mpm todolist` `mpm release` | Hook Series |
| Memory | `mpm memo` `mpm recall` `mpm rule` | Memory Series |
| Persona | `mpm persona` | `persona` |
| Skill | `mpm skilllist` `mpm loadskill` | Skill Series |
| Visual | `mpm timeline` | `open_timeline` |

---

*MPM Manual v2.1 - 2026-02*
