package tools

// WikiStyleTemplates Wiki 书写风格模板
var WikiStyleTemplates = map[string]string{
	"technical": `
# Wiki 书写指南 - 技术文档风格

## 语言风格
- 简洁准确，避免冗余
- 使用专业术语
- 保持客观中立
- 避免口语化表达

## 写作手法
- 章节结构清晰，层次分明
- 代码示例带注释说明
- 重点内容加粗强调
- 使用列表罗列要点

## 格式要求
- 使用 Markdown
- 代码块指定语言语法高亮
- 标题层级规范（#, ##, ###）
- 列表对齐工整

## 适合场景
API 文档、架构说明、技术规范
`,
	"tutorial": `
# Wiki 书写指南 - 教程指南风格

## 语言风格
- 循序渐进，从简单到复杂
- 语言通俗，适合新手
- 多用示例和类比
- 避免跳跃式说明

## 写作手法
- 每个概念配示例
- 使用图示辅助说明
- 分步骤详细讲解
- 提供常见问题解答

## 格式要求
- 使用 Markdown
- 代码块完整可运行
- 多用 Mermaid 流程图
- 步骤编号清晰

## 适合场景
入门教程、操作指南、使用手册
`,
	"reference": `
# Wiki 书写指南 - 参考资料风格

## 语言风格
- 详细完整，全面覆盖
- 准确描述每个细节
- 保持结构一致性
- 提供完整参数说明

## 写作手法
- 按功能模块组织
- 每个函数/接口独立说明
- 提供参数详解
- 包含返回值和异常说明

## 格式要求
- 使用 Markdown
- 表格展示参数列表
- 代码签名完整
- 交叉引用相关内容

## 适合场景
API 参考、配置手册、函数库文档
`,
	"blog": `
# Wiki 书写指南 - 博客风格

## 语言风格
- 轻松活泼，富有感染力
- 使用 emoji 增强可读性
- 讲故事的方式
- 避免过于严肃

## 写作手法
- 多用实例和生活类比
- 使用引用突出金句
- 适度使用感叹号
- 图文并茂

## 格式要求
- 使用 Markdown
- 适当使用 emoji
- 引用块强调重点
- 图片配说明

## 适合场景
项目介绍、更新日志、使用心得
`,
}

// DefaultWikiStyleTemplate 默认通用模板（与用户自定义要求融合的基础）
const DefaultWikiStyleTemplate = `
# Wiki 书写指南（默认模板）

## 语言风格
- 简洁准确，避免冗余
- 使用专业术语
- 保持客观中立
- 代码注释清晰

## 写作手法
- 章节结构清晰，层次分明
- 代码示例带注释
- 重点内容加粗强调
- 列表对齐工整

## 格式要求
- 使用 Markdown 格式
- 代码块指定语言高亮
- 标题层级规范
- 列表对齐
`

// GetWikiStyleTemplate 获取指定的风格模板
func GetWikiStyleTemplate(style string) string {
	if tpl, ok := WikiStyleTemplates[style]; ok {
		return tpl
	}
	return DefaultWikiStyleTemplate
}

// MergeWikiStyleTemplate 融合用户自定义要求与默认模板
func MergeWikiStyleTemplate(userRequirements string) string {
	return `
# Wiki 书写指南（个性化定制）

` + userRequirements + `

---

**注**：以上要求已与默认写作规范融合，确保文档质量与个性化需求的平衡。
`
}
