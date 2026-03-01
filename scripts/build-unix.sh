#!/usr/bin/env bash
# MyProjectManager one-click build script for Linux/macOS
# Usage:
#   chmod +x scripts/build-unix.sh
#   ./scripts/build-unix.sh

set -euo pipefail

step() { echo "[STEP] $1"; }
ok() { echo "[OK]   $1"; }
fail() { echo "[FAIL] $1"; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing command: $1"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

GO_ROOT="$PROJECT_ROOT/mcp-server-go"
BIN_DIR="$GO_ROOT/bin"
AST_ROOT="$GO_ROOT/internal/services/ast_indexer_rust"

echo "=== MyProjectManager one-click build (Linux/macOS) ==="
echo "Project root: $PROJECT_ROOT"
echo "Output dir:   $BIN_DIR"
echo

step "Check toolchain"
require_cmd go
require_cmd cargo
ok "$(go version)"
ok "$(rustc --version)"

mkdir -p "$BIN_DIR"

step "Build mpm-go"
(
  cd "$GO_ROOT"
  go build -o "$BIN_DIR/mpm-go" ./cmd/server
)
chmod +x "$BIN_DIR/mpm-go" || true
ok "Built mpm-go"

step "Build ast_indexer"
(
  cd "$AST_ROOT"
  cargo build --release
)

if [ -f "$AST_ROOT/target/release/ast_indexer_rust" ]; then
  cp "$AST_ROOT/target/release/ast_indexer_rust" "$BIN_DIR/ast_indexer"
elif [ -f "$AST_ROOT/target/release/ast_indexer" ]; then
  cp "$AST_ROOT/target/release/ast_indexer" "$BIN_DIR/ast_indexer"
else
  fail "ast_indexer release binary not found"
fi

chmod +x "$BIN_DIR/ast_indexer" || true
ok "Built ast_indexer"

echo
step "Verify outputs"
for name in mpm-go ast_indexer; do
  file="$BIN_DIR/$name"
  if [ ! -f "$file" ]; then
    fail "$name missing"
  fi
  size="$(du -h "$file" | cut -f1)"
  ok "$name ($size)"
done

echo
echo "=== Build completed ==="
echo "Output dir: $BIN_DIR"
