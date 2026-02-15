# MPM 快速配置指南

## 1. 解压文件

将下载的压缩包解压到任意目录，例如：
- Windows: `D:\mpm-windows-amd64\`
- macOS/Linux: `~/mpm-darwin-arm64/` 或 `~/mpm-linux-amd64/`

目录结构：
```
mpm-windows-amd64/
├── mpm-go.exe        # MCP 服务器（主程序）
├── ast_indexer.exe   # AST 索引器（被 mpm-go 调用）
├── docs/             # 文档
├── README.md
└── README_EN.md
```

## 2. 配置 MCP 客户端

### Claude Code (Claude Desktop)

编辑配置文件：
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mpm": {
      "command": "D:/mpm-windows-amd64/mpm-go.exe"
    }
  }
}
```

macOS/Linux 示例：
```json
{
  "mcpServers": {
    "mpm": {
      "command": "/Users/yourname/mpm-darwin-arm64/mpm-go"
    }
  }
}
```

### Cursor

1. 打开 Settings → Features → Model Context Protocol
2. 点击 "Add new MCP server"
3. 填写：
   - Name: `mpm`
   - Command: `D:/mpm-windows-amd64/mpm-go.exe`
4. 保存

### Windsurf

1. 打开 Settings → MCP
2. 添加服务器：
   - Name: `mpm`
   - Command: `/path/to/mpm-go`

### 其他 MCP 客户端

参考上述配置，核心是：
```json
{
  "mcpServers": {
    "mpm": {
      "command": "/完整路径/mpm-go"
    }
  }
}
```

## 3. 重启客户端

配置完成后，重启 MCP 客户端（Claude/Cursor/Windsurf）。

## 4. 验证

在对话中输入：
```
mpm 初始化
```

如果看到项目初始化成功的提示，说明配置正确。

## 常见问题

**Q: 提示找不到 ast_indexer？**
A: 确保 `ast_indexer` 和 `mpm-go` 在同一目录下。

**Q: Windows 提示权限错误？**
A: 右键 `mpm-go.exe` → 属性 → 解除锁定（如有）。

**Q: macOS 提示无法验证开发者？**
A: 运行 `xattr -cr /path/to/mpm-darwin-arm64/`
