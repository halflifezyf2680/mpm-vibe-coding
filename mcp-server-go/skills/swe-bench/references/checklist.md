# SWE-Bench 提交前自检清单 (Pre-Flight Checklist)

在提交任务前，请逐项核对：

## 1. 复现性 (Reproducibility)
- [ ] 是否创建了 `reproduce_issue.py`？
- [ ] **复现阶段**：运行脚本是否报错/失败？(证明 Bug 存在)
- [ ] **验证阶段**：修复后运行脚本是否通过？(证明 Bug 已修)

## 2. 完整性 (Completeness)
- [ ] 是否处理了 Issue 描述中的所有边缘情况？
- [ ] 是否添加了新的单元测试文件（如 `tests/test_issue_xxx.py`）？

## 3. 安全性 (Safety)
- [ ] 是否运行了相关模块的原有测试？
- [ ] 确保没有引入新的 Regression？

## 4. 规范性 (Style)
- [ ] 是否删除了临时的 print/log 语句？
- [ ] 代码风格是否与原项目保持一致？

---
> **黄金法则**: 没有复现脚本的修复，在 SWE-Bench 中等于 0 分。
