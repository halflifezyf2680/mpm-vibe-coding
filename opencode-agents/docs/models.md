# OpenCode 模型标准名称：如何获取

OpenCode 的 agent frontmatter 里 `model:` 必须填写你本机 OpenCode 能识别的**标准模型 ID**。

在终端运行：

```bash
opencode models
```

从输出里复制模型 ID 原样填写（通常格式是 `provider/model-id`，例如 `openai/gpt-5.2`）。

补充规则：

- 子代理（subagent）如果不写 `model:`，会默认继承调用它的主代理模型；为了成本/并发池行为可预期，建议本套 MPM agent **都显式写 `model:`**。
- `reasoningEffort` 等 provider 参数应写在 frontmatter **顶层字段**，不要嵌套在 `options:` 下。

示例：

```yaml
---
description: 示例 agent
mode: subagent
model: openai/gpt-5.2
reasoningEffort: high
steps: 30
permission:
  edit: deny
  bash: deny
---
```

# 本仓库 MPM agents 的模型使用建议（与文档闭环）

闭环目标：

1. 用 `opencode models` 拿到标准 ID。
2. 把标准 ID 写回 `.opencode/agents/*.md`，让模型池/成本/能力分配可控。
3. 让“主代理负责拆解与决策 / 子代理负责执行与研究”的分工与模型选择一致。

建议默认值（以 `opencode models` 实际输出为准，不存在就改成存在的等价型号）：

- `.opencode/agents/mpm_pm.md`：`openai/gpt-5.2` + `reasoningEffort: high`（长期 Epic 推进、任务链操作、跨阶段决策）。
- `.opencode/agents/mpm_architect.md`：`openai/gpt-5.2` + `reasoningEffort: high`（架构级判断与方向性任务包；派发子代理落地，不直接改代码）。
- `.opencode/agents/mpm_expert.md`：`openai/gpt-5.3-codex` + `reasoningEffort: high`（跨模块重构、复杂 Debug、图像相关任务）。
- `.opencode/agents/mpm_coder.md`：`zhipuai-coding-plan/glm-5`（高频实现、单点任务，尽量不占 OpenAI 并发池）。
- `.opencode/agents/mpm_spider.md`：`zhipuai-coding-plan/glm-4.7`（外部资料检索与整理，写入 `.tmp/spider_<slug>.md`）。

# 自检清单

- 运行 `opencode models`，逐个核对 `.opencode/agents/*.md` 里出现的每个 `model:` 是否存在。
- 只要有一个不匹配，就把该 agent 的 `model:` 改成终端输出的标准 ID。
- 所有 provider 参数（如 `reasoningEffort`）保持 frontmatter 顶层键；不要使用 `options:`。
