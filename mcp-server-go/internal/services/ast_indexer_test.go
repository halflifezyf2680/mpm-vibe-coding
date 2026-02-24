package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasArg(args []string, key string) bool {
	for _, arg := range args {
		if arg == key {
			return true
		}
	}
	return false
}

func argValue(args []string, key string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			return args[i+1]
		}
	}
	return ""
}

func toSet(csv string) map[string]bool {
	set := make(map[string]bool)
	for _, item := range strings.Split(csv, ",") {
		v := strings.TrimSpace(item)
		if v != "" {
			set[v] = true
		}
	}
	return set
}

func TestDetectTechStackAndConfig_DetectsGoWithoutRootGoMod(t *testing.T) {
	root := t.TempDir()
	goFile := filepath.Join(root, "nested", "service", "handler.go")
	if err := os.MkdirAll(filepath.Dir(goFile), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(goFile, []byte("package service\n\nfunc Handle() {}\n"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	exts, _ := detectTechStackAndConfig(root)
	extSet := toSet(exts)

	if !extSet["go"] {
		t.Fatalf("expected go extension to be detected, got %q", exts)
	}
}

func TestDetectTechStackAndConfig_GoDoesNotIgnorePkgDir(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	if err := os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatalf("write go.mod failed: %v", err)
	}

	_, ignores := detectTechStackAndConfig(root)
	ignoreSet := toSet(ignores)

	if ignoreSet["pkg"] {
		t.Fatalf("pkg should not be ignored for go projects, got ignores %q", ignores)
	}
}

func TestBuildIndexArgs_DefaultDoesNotPassExtensions(t *testing.T) {
	args := buildIndexArgs("D:/repo", "D:/repo/.mcp-data/symbols.db", "D:/repo/.mcp-data/.ast_result_index.json", "node_modules,.git", "go,py", "", false, false)

	if hasArg(args, "--extensions") {
		t.Fatalf("default args should not include --extensions, got %v", args)
	}
	if !hasArg(args, "--ignore-dirs") {
		t.Fatalf("expected --ignore-dirs in args, got %v", args)
	}
}

func TestBuildIndexArgs_RetryCanPassExtensions(t *testing.T) {
	args := buildIndexArgs("D:/repo", "D:/repo/.mcp-data/symbols.db", "D:/repo/.mcp-data/.ast_result_index.json", "node_modules,.git", "go,py", "", true, false)

	if !hasArg(args, "--extensions") {
		t.Fatalf("retry args should include --extensions, got %v", args)
	}
	if got := argValue(args, "--extensions"); got != "go,py" {
		t.Fatalf("unexpected --extensions value: got %q", got)
	}
}

func TestBuildIndexArgs_ForceFullAddsFlag(t *testing.T) {
	args := buildIndexArgs("D:/repo", "D:/repo/.mcp-data/symbols.db", "D:/repo/.mcp-data/.ast_result_index.json", "node_modules,.git", "go,py", "", false, true)

	if !hasArg(args, "--force-full") {
		t.Fatalf("expected --force-full in args, got %v", args)
	}
}
