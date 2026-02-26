#!/usr/bin/env bash
# MyProjectManager cross-platform build helper
#
# What it does:
# - Cross-compiles Go server binary (mpm-go) for common targets
# - Optionally builds host ast_indexer binary
#
# Usage:
#   chmod +x scripts/build-cross-platform.sh
#   ./scripts/build-cross-platform.sh
#   ./scripts/build-cross-platform.sh --with-rust-host

set -euo pipefail

WITH_RUST_HOST=false

for arg in "$@"; do
  case "$arg" in
    --with-rust-host)
      WITH_RUST_HOST=true
      ;;
    *)
      echo "Unknown argument: $arg"
      exit 1
      ;;
  esac
done

step() { echo "[STEP] $1"; }
ok() { echo "[OK]   $1"; }
warn() { echo "[WARN] $1"; }
fail() { echo "[FAIL] $1"; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing command: $1"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

GO_ROOT="$PROJECT_ROOT/mcp-server-go"
BIN_DIR="$GO_ROOT/bin"
RELEASE_DIR="$PROJECT_ROOT/release_cross_platform"
AST_ROOT="$GO_ROOT/internal/services/ast_indexer_rust"

TARGETS=(
  "windows/amd64"
  "windows/arm64"
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
)

echo "=== MyProjectManager cross-platform build helper ==="
echo "Project root: $PROJECT_ROOT"
echo "Go release dir: $RELEASE_DIR"
echo

step "Check Go toolchain"
require_cmd go
ok "$(go version)"

mkdir -p "$RELEASE_DIR"
mkdir -p "$BIN_DIR"

step "Build Go server for target matrix"
go_failed=()
for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"

  if [ "$goos" = "windows" ]; then
    out="$RELEASE_DIR/mpm-go-${goos}-${goarch}.exe"
  else
    out="$RELEASE_DIR/mpm-go-${goos}-${goarch}"
  fi

  echo "  -> $goos/$goarch"
  if (
    cd "$GO_ROOT"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -o "$out" ./cmd/server
  ); then
    size="$(du -h "$out" | cut -f1)"
    ok "mpm-go-${goos}-${goarch} ($size)"
  else
    warn "Failed: $goos/$goarch"
    go_failed+=("$goos/$goarch")
  fi
done

if [ "${#go_failed[@]}" -gt 0 ]; then
  warn "Some Go targets failed: ${go_failed[*]}"
fi

if [ "$WITH_RUST_HOST" = true ]; then
  step "Build host ast_indexer"
  require_cmd cargo
  ok "$(rustc --version)"

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
  ok "Built host ast_indexer"
else
  warn "Skipped host Rust build (use --with-rust-host to enable)"
fi

echo
echo "=== Build completed ==="
echo "Go binaries:  $RELEASE_DIR"
echo "Host binaries: $BIN_DIR"
