package main

import (
	"fmt"
	"os"

	"mcp-server-go/internal/core"
	"mcp-server-go/internal/services"
	"mcp-server-go/internal/tools"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 初始化会话管理器与内部服务
	sm := &tools.SessionManager{}
	ai := services.NewASTIndexer()

	// 🚀 [LifeCycle] 探测并尝试自动绑定项目
	projectRoot := core.DetectProjectRoot()
	if projectRoot != "" {
		fmt.Fprintf(os.Stderr, "[MCP-Go] 已锁定项目根目录: %s\n", projectRoot)
		m, err := core.NewMemoryLayer(projectRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[MCP-Go][ERROR] 记忆层初始化受阻: %v\n", err)
		} else {
			sm.Memory = m
			sm.ProjectRoot = projectRoot
			fmt.Fprintf(os.Stderr, "[MCP-Go] 记忆层（SSOT）与项目上下文已就绪。\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "[MCP-Go][WARN] 无法探测项目根目录，请检查环境变量或在项目目录下运行。\n")
	}

	// 启动 MCP Server (StdIO)
	s := server.NewMCPServer(
		"MyProjectManager-Go",
		"1.0.0",
	)

	// 注册工具
	tools.RegisterSystemTools(s, sm)           // 系统初始化
	tools.RegisterMemoryTools(s, sm)           // 备忘与检索
	tools.RegisterSearchTools(s, sm, ai)       // 项目地图与搜索
	tools.RegisterIntelligenceTools(s, sm, ai) // 任务分析与事实存档
	tools.RegisterAnalysisTools(s, sm, ai)     // 影响分析工具
	tools.RegisterSkillTools(s, sm)            // 技能库工具
	tools.RegisterTaskTools(s, sm)             // 任务管理工具
	tools.RegisterEnhanceTools(s, sm)          // 增强工具 (prompt_enhance, persona)
	tools.RegisterDocTools(s, sm)              // 文档工具 (wiki_writer)

	fmt.Fprintf(os.Stderr, "[MCP-Go] MyProjectManager 正在启动...\n")

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "服务运行错误: %v\n", err)
		os.Exit(1)
	}
}
