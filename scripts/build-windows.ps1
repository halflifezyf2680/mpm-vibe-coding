# MyProjectManager one-click build script for Windows
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1

$ErrorActionPreference = "Stop"

function Write-Step($msg) { Write-Host "[STEP] $msg" -ForegroundColor Cyan }
function Write-Ok($msg) { Write-Host "[OK]   $msg" -ForegroundColor Green }
function Write-Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red }

function Require-Command($name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "Missing command: $name"
    }
}

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptRoot

$goRoot = Join-Path $projectRoot "mcp-server-go"
$binDir = Join-Path $goRoot "bin"
$astRoot = Join-Path $goRoot "internal\services\ast_indexer_rust"

Write-Host "=== MyProjectManager one-click build (Windows) ===" -ForegroundColor Cyan
Write-Host "Project root: $projectRoot"
Write-Host "Output dir:   $binDir"
Write-Host ""

Write-Step "Check toolchain"
try {
    Require-Command go
    Require-Command cargo
    Write-Ok "Go: $(go version)"
    Write-Ok "Rust: $(rustc --version)"
}
catch {
    Write-Fail $_.Exception.Message
    exit 1
}

New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Write-Step "Build mpm-go.exe"
Push-Location $goRoot
try {
    go build -o "bin\mpm-go.exe" ".\cmd\server"
    Write-Ok "Built mpm-go.exe"
}
finally {
    Pop-Location
}

Write-Step "Build ast_indexer.exe"
Push-Location $astRoot
try {
    cargo build --release
}
finally {
    Pop-Location
}

$astSrc = Join-Path $astRoot "target\release\ast_indexer_rust.exe"
if (-not (Test-Path $astSrc)) {
    Write-Fail "ast_indexer_rust.exe not found: $astSrc"
    exit 1
}
Copy-Item $astSrc (Join-Path $binDir "ast_indexer.exe") -Force
Write-Ok "Built ast_indexer.exe"

Write-Host ""
Write-Step "Verify outputs"
$required = @("mpm-go.exe", "ast_indexer.exe")

$missing = @()
foreach ($name in $required) {
    $file = Join-Path $binDir $name
    if (Test-Path $file) {
        $size = [math]::Round((Get-Item $file).Length / 1MB, 2)
        Write-Ok ("{0} ({1} MB)" -f $name, $size)
    }
    else {
        $missing += $name
        Write-Fail ("{0} missing" -f $name)
    }
}

if ($missing.Count -gt 0) {
    Write-Fail ("Build failed, missing files: {0}" -f ($missing -join ", "))
    exit 1
}

Write-Host ""
Write-Host "=== Build completed ===" -ForegroundColor Cyan
Write-Host "Output dir: $binDir"
