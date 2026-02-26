---
name: "swe-bench"
description: "SWE-Bench 标准解题专家。指导从 Issue 分析、环境复现、测试编写到最终修复的全流程。适用于解决 GitHub Issue 或 SWE-Bench 评测任务。"
---

# SWE-BenchStandard Solving Workflow

本技能指导你按照 SWE-Bench 的严苛标准解决 GitHub Issue。不仅是修复代码，更是要证明修复的正确性和无副作用。

## 🏆 核心原则 (Core Principles)

1.  **Reproduction First**: 修改代码前，**必须**先编写复现脚本，证明 Bug 存在。
2.  **Test Driven**: 只有当复现脚本从 Fail 变为 Pass，且不破坏原有测试时，任务才算完成。
3.  **Minimal Changes**: 只修改必要的文件，避免重构无关代码。

## 🚀 标准工作流 (Standard Workflow)

请严格按照以下 5 个阶段执行：

### Phase 1: Issue Analysis (审题)

1.  **阅读 Issue**: 理解用户报告的 Bug 现象、环境和复现步骤。
2.  **调用工具**: 使用 `code_search` 初步探索 Issue 提到的报错信息或关键词。
3.  **输出**: 明确 "Bug 预期行为" vs "实际行为"。

### Phase 2: Reproduction (复现 - 最关键一步)

❌ **严禁跳过此步直接改代码！**

1.  **创建脚本**: 使用 `scripts/reproduce_template.py` 模板，在项目根目录创建 `reproduce_issue.py`。
2.  **编写断言**: 脚本必须包含 `assert` 语句：
    *   在 Bug 存在时，脚本应抛出 `AssertionError` 或 crash (Exit Code != 0)。
    *   在 Bug 修复后，脚本应正常退出 (Exit Code = 0)。
3.  **验证复现**:
    ```bash
    python reproduce_issue.py
    # 预期输出: AssertionError 或 Traceback
    ```

### Phase 3: Localization (定位)

1.  **AST 定位**: 使用 `code_search(search_type="function")` 查找相关函数定义。
2.  **调用分析**: 使用 `code_impact(direction="both")` 查看调用链，确定修改的影响范围。
3.  **确认根因**: 阅读源码，找到逻辑漏洞的确切位置。

### Phase 4: TDD Fixing (修复)

1.  **开发修复**: 修改代码，修复 Bug。
2.  **增量测试**: 如果需要，在项目测试套件（如 `tests/` 目录）中添加新的测试用例文件。
3.  ** lint 检查**: 确保代码风格符合项目规范。

### Phase 5: Verification (验证)

1.  **验证复现脚本 (Fail-to-Pass)**:
    ```bash
    python reproduce_issue.py
    # 预期输出: 无报错，正常退出
    ```
2.  **验证原有测试 (Pass-to-Pass)**:
    运行与修改模块相关的原有测试，确保无 Regression。
    ```bash
    pytest tests/path/to/relevant_tests.py
    ```

## 🛠️ 常用工具集

- `reproduce_issue.py`: 必须创建的复现脚本。
- `code_search`: 查找定义。
- `code_impact`: 评估影响。
- `run_command`: 执行测试命令。

## ⚠️ 常见陷阱 (Pitfalls)

*   **陷阱1**: 没写复现脚本就改代码。 -> **后果**: 无法证明你通过了 SWE-Bench 评测。
*   **陷阱2**: 修改了太多无关文件。 -> **后果**: 引入新 Bug，评分降低。
*   **陷阱3**: 这里的测试通过了，但破坏了其他模块。 -> **后果**: 必须运行相关回归测试。
