package utils

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// URIToPath 将 MCP file:/// URI 转换为本地绝对路径
func URIToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	path := u.Path

	// Windows 处理: /C:/foo -> C:/foo
	// 在 Windows 上，path 可能以 /C:/ 开头
	if os.PathSeparator == '\\' && len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}
