# MCP 工具设计最佳实践：Token 经济学与语义约束

> **摘要**: 本文档综合了 MCP-Universe 基准测试、ToolACE 研究及实战经验，旨在解决 **"Token 消耗 vs 模型理解力"** 的核心权衡问题。

## 一、核心权衡 (The Trade-off)

在 MCP 工具设计中，我们面临一个根本矛盾：
*   **精简 (Conciseness)**：为了节省 Token 和防止 Context Rot（上下文衰减），工具定义应尽可能短。
*   **详尽 (Detail)**：为了让 LLM 准确理解何时调用及如何传参，描述需要尽可能详细。

**结论**：在当前（2025/2026）的模型能力下，**准确性 > Token 成本**。对于关键的 Agent 工具，投入 Token 编写高质量的 Docstring 和 Schema 是 ROI 最高的投资。

---

## 二、黄金三要素 (The Trinity of Tool Definition)

一个完美的 MCP 工具定义由三层构成，缺一不可：

| 层级 | 载体 (Python) | 传输形式 (JSON) | 作用域 | 核心价值 |
| :--- | :--- | :--- | :--- | :--- |
| **L1: 语义意图** | `Docstring` | `"description"` | **Soft Guide** | 告诉 LLM **"何时 (When)"** 和 **"为何 (Why)"** 使用此工具。 |
| **L2: 结构约束** | `Type Hint` | `"inputSchema"` | **Hard Constraint** | 强制约束参数的**类型、枚举值**，防止幻觉参数。 |
| **L3: 语义暗示** | `Parameter Name` | Argument Key | **Implicit Signal** | 通过变量名（如 `symbol_name` vs `arg1`）传递隐式语义。 |

### ❌ 错误示范
```python
def search(query: str, type: str):
    """Search code."""
    pass
```
*   **问题**：`type` 是什么？文件名还是函数名？`query` 支持正则吗？

### ✅ 最佳实践
```python
def code_search(
    query: str, 
    search_type: Literal["function", "class", "variable"] = "function"
):
    """
    Search for code symbols (functions, classes) in the project.
    Use this when you need to find the definition of a specific symbol.
    """
    pass
```
*   **改进**：`code_search` 暗示用途；`Literal` 生成枚举约束；Docstring 说明场景。

---

## 三、文档编写策略 (Docstring Engineering)

Docstring 不仅仅是文档，它是 **Micro-Prompt (微提示词)**。

### 1. 内置思维链 (CoT)
对于复杂工具（如编排器），不要只列出参数，要教模型**如何决策**。

```python
"""
3秒决策：
- 任务刚开始？ -> mode="continue"
- 需要细分步骤？ -> mode="step"
- 遇到阻塞？ -> mode="hook"
"""
```
这种 "If-This-Then-That" 的引导比枯燥的参数列表有效得多。

### 2. 负面约束与铁律
使用带有情感色彩的强指令词汇（如 **"铁律"**, **"必须"**, **"严禁"**），能显著降低模型违规率。

```python
"""
!! 最佳实践: 修改任何代码前，必须先调用 code_impact 分析影响 !!
"""
```

### 3. 微观示例 (Micro-Examples)
在 Docstring 中保留简短的调用示例，特别是对于复杂的 List/Dict 参数。

---

## 四、Schema 约束策略 (Schema Engineering)

**原则：能用 Schema 解决的，绝对不要只写在 Docstring 里。**

### 1. 拥抱 `Literal` (Enum)
LLM 生成自由文本（String）出错率高，生成枚举（Enum）出错率极低。

*   ❌ `mode: str` (描述写: "可选 a, b, c")
*   ✅ `mode: Literal["a", "b", "c"]` (Pydantic 自动生成 JSON Enum)

### 2. 也是 Pydantic
利用 Pydantic 的 `Field(description="...")` 为每个参数添加微观描述，这会被转换成 JSON Schema 中的 `description` 字段。

---

## 五、架构设计原则

### 1. 原子性与数量级
*   **甜点区**：单个 MCP Server 暴露 **15-20 个** 工具最为适宜。
*   **过少 (<5)**：导致单个工具参数爆炸（God Object），模型难以驾驭参数组合。
*   **过多 (>50)**：导致工具检索（Tool Selection）精度下降，且 Context 占用过大。

### 2. 动态加载 (Advanced)
对于超大型项目，可以使用 `context-dependent tools` 策略：
*   初始只暴露 `list_capabilities`。
*   根据任务进入特定领域（如 "Database Mode"）后，再动态注入相关工具集。

---

## 六、主要参考

*   **MCP-Universe Benchmark**: 证明了结构化 Schema 优于自然语言描述 (Accuracy +18.4%)。
*   **ToolACE Study**: 证明了“中等复杂度”工具集的泛化能力最强。
*   **Google ADK Docs**: 明确指出 Docstring 就是发给 LLM 的 Description。

> **最后建议**：把工具定义当成代码来写，把 Docstring 当成 Prompt 来写。

