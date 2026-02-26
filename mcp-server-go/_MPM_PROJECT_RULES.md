# MPM 强制协议

## 🚨 死规则 (违反即失败)

1. **复杂任务前** → 必须先 `manager_analyze` (主动填 Intent)，获取战术简报
2. **改代码前** → 必须先 `code_search` 或 `project_map` 定位，严禁凭记忆改
3. **预计任务很长** → 必须使用 `task_chain` 分步执行，禁止单次并发操作
4. **改代码后** → 必须立即 `memo` 记录
5. **准备改函数时** → 必须先 `code_impact` 分析谁在调用它
6. **code_search 失败** → 必须换词重试（同义词/缩写/驼峰变体），禁止放弃
7. **阅读业务流程时** → 优先使用 `flow_trace`，禁止只看文件名凭感觉推断

---

## 🔧 工具使用时机

| 场景 | 必须使用的工具 |
|------|---------------|
| **任务复杂/模糊** | `manager_analyze` (必填 Intent) |
| **任务 > 2 步** | `task_chain` (防止搞砸) |
| 刚接手项目 / 宏观探索 | `project_map` |
| 理解业务逻辑主链 | `flow_trace` |
| 找具体函数/类的定义 | `code_search` |
| 准备修改某函数 | `code_impact` |
| 代码改完了 | `memo` (SSOT) |

---

## 🚫 禁止

- 禁止凭记忆修改代码
- 禁止 code_search 失败后直接放弃
- 禁止修改代码后不调用 memo
- 禁止并发调用工具


# 项目命名规范 (由 MPM 自动分析生成)

> **重要**: 此规范基于项目现有代码自动提取。LLM 必须严格遵守以确保风格一致。

## 检测结果

| 项目类型 | 旧项目 (检测到 72 个源码文件，477 个符号) |
|---------|------|
| **函数/变量风格** | snake_case (55.8%) |
| **类名风格** | PascalCase |
| **常见前缀** | validate_, _ |

## 命名约定

-   **函数/变量**: 使用 snake_case，示例: `get_task`, `session_manager`
-   **类名**: 使用 PascalCase，示例: `TaskContext`, `SessionManager`
-   **禁止模糊修改**: 修改前必须用 code_search 确认目标唯一性。

## 代码示例 (从项目中提取)

init, main, main, pack_document, validate_document, condense_xml, GetDBForProject, NewDatabaseManager, init, healSchema

---

> **提示**: 如需修改规范，请直接编辑此文件。IDE 会自动读取更新后的内容。
