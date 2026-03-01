# LLM 友好的 Agent 文档写法（OpenCode / MPM 协作）

本文件是长期保留的规范，用于让 agent system prompt（Markdown agent 文档）对 LLM 更友好、更稳定、更不易跑偏。

## 目标

- 降低“格式噪音”（无意义的 token），提升指令命中率。
- 消除前后矛盾与边界歧义，减少 LLM 自行脑补。
- 在异常中断/工具失败时，能从外部状态恢复（MPM-coding__task_chain / 文件锚点）。

## 总原则（越少越强）

1. **清晰优先**：写给“聪明但不了解你工作流的新同事”。如果人类同事会疑惑，LLM 也会疑惑。（Anthropic prompting best practices 强调清晰、直接与可执行的约束）

2. **把状态外置**：长期进度用 `MPM-coding__task_chain`（结构化、可恢复）；语义锚点用文件（`.tmp/` 调研、任务包/白板）。不要指望模型“记住”。

3. **少写“禁止……”，多写“应该怎么做”**：给正向规则和模板，减少负向约束导致的钻空子与误解。（Anthropic 建议“tell what to do instead of what not to do”）

4. **避免过度格式化**：Markdown 适度即可。表格/复杂嵌套列表会增加 token 噪音，且对 LLM 的“视觉对齐”价值有限。

## 推荐的文档结构（Markdown agent body）

- 一句话角色定位（做什么 + 不做什么）
- 3-7 条铁律（稳定、可验证、互不冲突）
- 工作流（只写关键分叉点：何时派发/何时自查/何时回报）
- 回报模板（强制字段名，减少口径漂移）
- （可选）异常处理（工具失败/断网/崩溃恢复）

## 格式与表达（对 LLM 友好）

- **避免表格**：表格主要服务人类视觉扫描，对 LLM 是额外分隔符 token。
- **少用装饰性强调**：只在“硬约束/关键字段名”处使用 `inline code`。
- **列表只用在离散项**：步骤/字段清单用 bullet 或 numbered list；不要把连续叙述切成碎片。
- **模板统一用 code block**：例如 `[UPSTREAM REPORT]`、`[EPIC STATE]`。
- **术语统一**：同一个概念永远用同一个名字（例如“上游凭证”）。

OpenAI 提示：可以用 Markdown 标题/列表标出逻辑边界，也可以用 XML tags 明确区分“指令/上下文/示例”。
参考：OpenAI Prompt Engineering（Message formatting with Markdown and XML）

## “上游 ID”与“内部 ID”隔离（防串台）

为什么：OpenCode 的 Task tool 子代理可以自建内部 task_chain；如果回报里混用 ID，会让上级销账失败。

约定：

- **Upstream Task/Phase/Sub**：只指上级给你的那一套；用于上级调用 `MPM-coding__task_chain(complete/complete_sub/...)`。
- **Internal TaskChain ID**：仅用于子代理内部续传；默认不回传，除非上游明确要求。
- **禁止复用上游 task_id**：子代理内部 task_id 必须使用独立命名空间（例如 `local_coder_*`、`local_expert_*`）。

## 并发派发（可选但要有刹车）

并发可以提升吞吐，但会带来“交付物覆盖/锚点混乱/结果对不上号”的风险。

建议：

- 默认串行派发。
- 只有在“互不依赖 + 不写同一输出路径 + 每个子代理都有独立交付物与验收”时才并发。

## 异常中断与恢复

现实：子代理执行不是后台 job；工具失败/abort/崩溃会中止当前执行。恢复依赖“外部状态”。

建议：

- PM 每回合输出 `[EPIC STATE]`（含可复制的 `MPM-coding__task_chain(mode="resume")`）。
- 任何需要跨回合的中间产物必须落盘（`.tmp/` 或任务包/白板文件），不要只存在对话里。

## 参考资料

- OpenCode Agents 文档（Markdown agent / permissions / task permissions）：https://opencode.ai/docs/agents/
- OpenAI Prompt engineering（Markdown + XML 边界、角色/指令分层等）：https://platform.openai.com/docs/guides/prompt-engineering
- Anthropic Prompting best practices（清晰直接、结构化、避免过度格式化与过度指令等）：https://docs.anthropic.com/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
