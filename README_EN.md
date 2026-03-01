# MPM - Vibe Coding MCP

> **Turning AI Coding from "Demos" into "Delivery"**

[中文](README.md) | English

![License](https://img.shields.io/badge/license-MIT-blue.svg) ![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg) ![MCP](https://img.shields.io/badge/MCP-v1.0-FF4F5E.svg)

---

## Industry Status

The rise of AI coding tools has lowered the programming barrier, but also exposed some issues: many projects perform well in demos, yet lack systematic consideration for the following engineering concerns in production:

- **Reproducibility**: Can execution results be reliably reproduced?
- **Fault Recovery**: Can execution resume from checkpoints after failure?
- **Permission Boundaries**: Are default permissions restrictive enough? Can privilege escalation be prevented?
- **Acceptance & Audit**: Are there clear acceptance criteria? Are operations traceable?
- **Self-Iteration**: How does state progress? What's the method?

Common patterns in current AI coding practice:
- Relying on natural-language workflows and agent templates with insufficient enforcement
- LLM-generated documentation that appears professional but fails to form a verifiable system
- Users struggle to assess the reliability of LLM-generated content

**Core Insight**: LLM engineering isn't about stacking prompts or multi-agent orchestration—it requires a **programmatic framework**: information cleaning, externalized state, workflow state machines, pre-verification, clear tool boundaries, and solidified + retrievable memory.

The tooling framework determines the upper bound of delivery capability. LLM is the driver.

---

## What is MPM?

MPM is an **MCP engineering layer** that upgrades AI coding from "chatting" to "controlled delivery workflow".
You can start with almost zero cognitive overhead: initialize first, then apply `_MPM_PROJECT_RULES.md` as your system rules.

### 🚀 30-Second Start (Do This First)

```text
1) initialize_project
2) Paste _MPM_PROJECT_RULES.md into client system rules
3) Ask directly: "Help me fix XXX and follow the rules"
```

If you do this first, you can start effectively without learning every tool in advance.

**Core Differentiators**:

| Traditional Approach | MPM Approach |
|---------------------|--------------|
| `grep "Login"` → 500 results | `code_search("Login")` → exact file:line |
| "I think this change should work" | `code_impact` → full call chain analysis |
| Starting from scratch every session | `system_recall` → cross-session memory |
| AI improvises freely | `manager_analyze` → structured task planning |

---

## Key Features

### 1. AST-based Precision, Not Text Search

```text
You: Search for Login function
AI: Found func:src/auth/login.go::Login (L45-67)
    Signature: func Login(ctx context.Context, cred Credentials) (*Token, error)
    Callers: 3 direct, 12 indirect
```

**Powered by**: Rust AST engine + `canonical_id` for disambiguation

### 2. Complete Call Chain Tracking

```text
You: Analyze impact of modifying SessionManager
AI: CODE_IMPACT_REPORT
    Risk Level: HIGH
    Direct Impact: 4 functions
    Indirect Impact: 23 functions (3-layer call chain)
    
    Modification Checklist:
    ▶ [core/session.go:100-150] MODIFY_TARGET
    ▶ [api/handler.go:45-80] VERIFY_CALLER
    ▶ [service/auth.go:200-250] VERIFY_CALLER
```

### 3. Cross-Session Memory Persistence

```text
You: Why did we change timeout to 30s last time?
AI: (system_recall) Memo from 2024-01-15:
    "Changed timeout from 10s to 30s due to Alibaba Cloud ECS cold start delay"
```

---

## Quick Start

### 1. Build

```powershell
# Windows
powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1

# Linux/macOS
./scripts/build-unix.sh
```

### 2. Configure MCP

Point to the build output: `mcp-server-go/bin/mpm-go(.exe)`

### 3. Start Using

```text
Initialize project
Help me analyze and fix the Login callback idempotency issue
```

After initialization, MPM generates `_MPM_PROJECT_RULES.md` automatically. Treat it as the project's operating playbook:

- It tells the LLM naming conventions, tool order, and hard constraints
- You can start effectively without learning every tool detail first
- In a new chat, ask the LLM to read this file first to reduce mistakes

Recommended first prompt: `Read _MPM_PROJECT_RULES.md and follow it`

### 4. Release Packaging (Fixed Directory)

```powershell
python package_product.py
```

Notes:

- Output directory is fixed: `mpm-release/MyProjectManager`
- Each run removes previous `mpm-release` first, then rebuilds clean package contents

---

## Tool Quick Reference

| Trigger | Tool | Purpose |
|---------|------|---------|
| `mpm init` | `initialize_project` | Project binding & AST indexing (supports `force_full_index`) |
| `mpm index status` | `index_status` | Check background indexing progress/heartbeat/DB size |
| `mpm search` | `code_search` | AST-based symbol lookup |
| `mpm impact` | `code_impact` | Call chain impact analysis |
| `mpm map` | `project_map` | Project structure + heat map |
| `mpm flow` | `flow_trace` | Business flow trace (entry/upstream/downstream) |
| `mpm analyze` | `manager_analyze` | Task intelligence briefing |
| `mpm chain` | `task_chain` | Protocol state machine driven (linear/develop/debug/refactor), supporting gates & sub-tasks |
| `mpm memo` | `memo` | Change documentation |
| `mpm recall` | `system_recall` | Memory retrieval |
| `mpm persona` | `persona` | Switch AI personality |
| `mpm skill` | `skill_load` | Load domain expert guides |
| `mpm timeline` | `open_timeline` | Project evolution visualization |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        MCP Client                           │
│              (Claude Code / Cursor / Windsurf)              │
└─────────────────────────┬───────────────────────────────────┘
                          │ MCP Protocol
┌─────────────────────────▼───────────────────────────────────┐
│                     Go MCP Server                           │
├──────────────┬──────────────┬───────────────┬───────────────┤
│  Perception  │  Scheduling  │    Memory     │   Enhancement │
│ code_search  │ manager_     │ memo          │ persona       │
│ code_impact  │ analyze      │ system_recall │ skill_load    │
│ project_map  │ task_chain   │ known_facts   │ open_timeline │
└──────────────┴──────────────┴───────────────┴───────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                   Rust AST Indexer                          │
│  • Tree-sitter multi-language parsing (Go/Python/JS/TS/...) │
│  • canonical_id for precise identification                  │
│  • callee_id for exact call chains                          │
│  • DICE complexity algorithm                                │
└─────────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                SQLite multi-db (.mcp-data/*)                │
│  • symbols.db: canonical_id/scope_path/callee_id            │
│  • mcp_memory.db: memos/tasks/known_facts                    │
│  • arch-ast.db: revisions/nodes/edges/proposals/events       │
└─────────────────────────────────────────────────────────────┘
```

---

## AST Indexing Core Fields

MPM's AST engine maintains **precise call chains**:

| Field | Example | Value |
|-------|---------|-------|
| `canonical_id` | `func:core/session.go::GetSession` | Globally unique, no ambiguity |
| `scope_path` | `SessionManager::GetSession` | Hierarchical scope |
| `callee_id` | `func:core/db.go::Query` | Exact call chain (not guessing) |

**Result**: `code_impact` supports **3-layer BFS traversal**, showing complete impact propagation.

---

## Performance Comparison

| Metric | Without MPM | With MPM |
|--------|-------------|----------|
| Symbol location | 10+ search steps | 1 exact hit |
| First-step accuracy | 0% | 100% |
| Impact assessment | Based on guessing | AST call chain |
| Token consumption | 4000+ | ~800 |
| Context recovery | Start from zero | Memory recall |

See [MANUAL_EN.md](./docs/MANUAL_EN.md#performance-comparison) for details.

---

## Documentation

- **[MANUAL_EN.md](./docs/MANUAL_EN.md)** - Complete manual (tools + best practices + case studies)
- **[README.md](./README.md)** - 中文版
- **[MANUAL.md](./docs/MANUAL.md)** - 中文版手册

---

## Common Search Questions

- `How to do impact analysis in MCP?` -> use `code_impact`
- `How to make LLM understand business logic flow?` -> use `flow_trace`
- `How to monitor indexing progress for large repositories?` -> use `index_status`
- `How to force full indexing?` -> `initialize_project(force_full_index=true)`

See [MANUAL_EN.md](./docs/MANUAL_EN.md) for detailed examples.

---

## OpenCode Multi-Agent Mode

MPM provides a 5-role Agent pack (PM / Architect / Coder / Expert / Spider) for direct use in OpenCode. See [opencode-agents/README.md](./opencode-agents/README.md).

---

## Contact

- Support: GitHub Issues
- Email: `halflifezyf2680@gmail.com`

---

## License

MIT License
