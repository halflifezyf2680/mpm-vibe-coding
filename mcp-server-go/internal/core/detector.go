package core

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectProjectRoot 探测当前活跃项目路径
func DetectProjectRoot() string {
	// 1. 优先信任 IDE 显式提供的环境变量
	envKeys := []string{"MPM_PROJECT_ROOT", "WORKSPACE_FOLDER", "SESSION_DIR", "VSCODE_CWD", "INIT_CWD"}
	for _, k := range envKeys {
		val := strings.TrimSpace(os.Getenv(k))
		if val != "" {
			abs, err := filepath.Abs(val)
			if err == nil && ValidateProjectPath(abs) {
				return abs
			}
		}
	}

	// 2. 其次选择当前工作目录 (CWD)
	cwd, err := os.Getwd()
	if err == nil {
		abs, err := filepath.Abs(cwd)
		if err == nil && ValidateProjectPath(abs) {
			return abs
		}
	}

	return ""
}

// ValidateProjectPath 验证路径是否安全且合法
func ValidateProjectPath(path string) bool {
	if path == "" || path == "Unknown" {
		return false
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return false
	}

	// 0. 基础过滤：绝对禁止直接在盘符根目录 (如 D:\, C:\) 创建项目
	// 如果路径等于其体积标识（如 C:\），则拒绝
	if abs == filepath.VolumeName(abs)+string(filepath.Separator) {
		return false
	}

	pLow := strings.ToLower(abs)

	// 1. 严格系统禁区
	systemTraps := []string{"c:\\windows", "system32", "program files", "programdata", "users\\all users"}
	for _, trap := range systemTraps {
		if strings.Contains(pLow, trap) {
			return false
		}
	}

	// 2. 严格排除 IDE 运行时目录、应用数据区、临时目录等容器环境
	ideTraps := []string{"antigravity", "kiro", "vscode", "cursor", "claude", "windsurf", "pycharm", "intellij"}
	for _, trap := range ideTraps {
		if strings.Contains(pLow, trap) {
			return false
		}
	}

	sensitiveDirs := []string{"appdata", "local/programs", "local\\programs", "application data", "temp", "prefetch"}
	for _, sd := range sensitiveDirs {
		if strings.Contains(pLow, sd) {
			// 除非明确包含 .git 标记
			if _, err := os.Stat(filepath.Join(abs, ".git")); err != nil {
				return false
			}
		}
	}

	return true
}
