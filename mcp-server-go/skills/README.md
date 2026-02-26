# Skills 目录说明

本目录用于存放 **Claude Skill** 格式的领域知识库，完全兼容 Claude MCP Skills 标准。

## 📋 什么是 Skill？

Skill 是一个包含专家指导、脚本、示例代码的文件夹结构，用于扩展 AI 在特定领域的能力。

## 🎯 与 MPM 的关系

MPM 项目通过 `skill_load` 工具支持加载和使用 Skills：
- 支持项目本地 Skills（优先级更高）
- 支持全局 Skills（跨项目共享）
- 自动解析 `SKILL.md` 的 YAML frontmatter

## 📁 标准 Skill 结构

```
skills/
└── your_skill_name/
    ├── SKILL.md          # 必需：主指导文档（含 YAML frontmatter）
    ├── references/       # 可选：参考文档
    ├── scripts/          # 可选：辅助脚本
    └── examples/         # 可选：示例代码
```

### SKILL.md 示例

```markdown
---
name: "react_development"
description: "React 全栈开发专家指南"
---

# React Development Skill

## 核心原则
1. 使用函数组件和 Hooks
2. ...

## 最佳实践
...
```

## 🚀 如何使用

### 1. 添加 Skill

将 Skill 文件夹复制到此目录：

```bash
skills/
├── react_development/
├── python_optimization/
└── ...
```

### 2. 加载 Skill

在 MPM 中使用：

```python
# 查看可用 Skills
mpm skill_list

# 加载指定 Skill
mpm skill_load react_development
```

### 3. Skill 来源

- **官方仓库**: [Claude Skills](https://github.com/anthropics/claude-skills)（如果存在）
- **自定义**: 根据项目需求自行编写
- **社区分享**: 从开源社区获取

## ⚙️ 扫描机制

MPM 按以下顺序扫描 Skills（后扫描的会覆盖同名 Skill）：

1. **MPM 安装目录**: `<mpm_install_dir>/skills/`
   - 随 MPM 一起分发的官方 Skills（**最低优先级**）
   - 用户克隆 MPM 仓库后可直接使用
   
2. **用户全局目录**: `~/.mpm/skills/`
   - 用户自定义的全局 Skills，跨项目共享
   - 可以覆盖官方 Skills 的特定版本
   
3. **项目本地目录**: `<project_root>/skills/`  
   - 项目特定的 Skills（**最高优先级**）
   - 适用于项目特有的业务规则和约束

### 覆盖规则

如果多个路径中存在同名 Skill（基于 `name` 字段或目录名），**优先级为：项目本地 > 用户全局 > MPM 官方**。

### 推荐实践

- **官方 Skills**: 由 MPM 维护，随仓库分发（用户可直接使用）
- **通用自定义**: 放在 `~/.mpm/skills/`（如公司内部规范、个人偏好配置）
- **项目特定**: 放在项目的 `skills/`（如项目特定的 API 规范）

### 注意事项

- 每个 Skill 目录必须包含 `SKILL.md` 或 `skill.md`
- 用户全局目录需手动创建（首次使用时）
- 官方 Skills 会随 Git 更新自动同步

## 📝 注意事项

1. **Skill 文件夹被 Git 忽略**：本目录下的具体 Skill 内容不会提交到仓库
2. **README.md 会被保留**：此说明文件会提交，提醒用户自行配置
3. **完全兼容 Claude**：可直接使用 Claude 官方 Skill 格式
4. **自主管理**：根据项目需要自行添加/删除 Skills

## 🔗 相关文档

- [MPM Skill 系统文档](../docs/user-manual/06-SKILL-SYSTEM.md)
- [Skill 工具参考](../docs/user-manual/08-TOOLS.md#skill_load)

---

**提示**: 如果你不需要使用 Skills，可以保持此目录为空。
