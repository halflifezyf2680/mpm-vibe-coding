# MPM Agents For OpenCode

本目录是一个可直接发布/分发的 OpenCode agent 包。

## 安装（项目级）

把本仓库的 `.opencode/agents/` 复制到你的项目根目录下：

- `<your-project>/.opencode/agents/mpm_pm.md`
- `<your-project>/.opencode/agents/mpm_architect.md`
- `<your-project>/.opencode/agents/mpm_coder.md`
- `<your-project>/.opencode/agents/mpm_expert.md`
- `<your-project>/.opencode/agents/mpm_spider.md`

然后在项目目录启动 OpenCode。

## 前置条件

本套 agents 依赖 MPM-coding MCP 工具（例如 `MPM-coding__task_chain`、`MPM-coding__memo`、`MPM-coding__code_search` 等）。
如果你的 OpenCode 没有加载对应 MCP server，这些工具调用会失败。

## 模型配置（必须校验）

在终端运行：

```bash
opencode models
```

把 `.opencode/agents/*.md` 里的 `model:` 改成你本机输出里存在的标准 ID（逐字匹配）。
更多说明见：`docs/models.md`。

## 角色与用法

- `mpm_pm`（primary）：推进长期 Epic；唯一操作者 `MPM-coding__task_chain`；通过 Task tool 派发子代理。
- `mpm_architect`（primary）：短期任务的架构级分析与任务包发布；不改代码；通过 Task tool 派发子代理。
- `mpm_coder`（subagent）：高频落地实现；改完必须 `MPM-coding__memo`。
- `mpm_expert`（subagent）：跨模块/高复杂任务；改完必须 `MPM-coding__memo`。
- `mpm_spider`（subagent, hidden）：外部资料调研，写入 `.tmp/spider_<topic_slug>.md`。

LLM 文档写法规范与对齐策略见：`docs/LLM_FRIENDLY_AGENT_DOCS.md`。

---

# MPM Agents For OpenCode (English)

This directory is a ready-to-publish/distribute OpenCode agent pack.

## Installation (Project-level)

Copy `.opencode/agents/` from this repository to your project root:

- `<your-project>/.opencode/agents/mpm_pm.md`
- `<your-project>/.opencode/agents/mpm_architect.md`
- `<your-project>/.opencode/agents/mpm_coder.md`
- `<your-project>/.opencode/agents/mpm_expert.md`
- `<your-project>/.opencode/agents/mpm_spider.md`

Then launch OpenCode in your project directory.

## Prerequisites

These agents depend on MPM-coding MCP tools (e.g., `MPM-coding__task_chain`, `MPM-coding__memo`, `MPM-coding__code_search`, etc.).
If your OpenCode doesn't have the corresponding MCP server loaded, these tool calls will fail.

## Model Configuration (Required)

Run in terminal:

```bash
opencode models
```

Update the `model:` field in `.opencode/agents/*.md` to match the exact model IDs shown in your output (character-for-character match).
See `docs/models.md` for more details.

## Roles and Usage

- `mpm_pm` (primary): Drives long-term Epics; sole operator of `MPM-coding__task_chain`; dispatches sub-agents via Task tool.
- `mpm_architect` (primary): Architecture-level analysis and task package publishing for short-term tasks; no code changes; dispatches sub-agents via Task tool.
- `mpm_coder` (subagent): High-frequency implementation; must call `MPM-coding__memo` after changes.
- `mpm_expert` (subagent): Cross-module / high-complexity tasks; must call `MPM-coding__memo` after changes.
- `mpm_spider` (subagent, hidden): External research; writes to `.tmp/spider_<topic_slug>.md`.

For LLM documentation conventions and alignment strategy, see: `docs/LLM_FRIENDLY_AGENT_DOCS.md`.
