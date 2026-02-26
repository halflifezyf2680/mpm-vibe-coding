# MyProjectManager (MPM) 一键安装/配置脚本 (Windows)
# 🎯 目标：一键解决所有环境问题，自动注入所有主流 IDE

$scriptDir = $PSScriptRoot
if (-not $scriptDir) { $scriptDir = Get-Location }

# 核心路径定义
# 核心路径定义
$pythonExe = "python"
$launcherPath = Join-Path $scriptDir "smart_launcher.py"
$escapedLauncher = $launcherPath.Replace("\", "\\")

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "   MyProjectManager (MPM) 部署工具" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# 1. 环境自检与依赖安装
Write-Host "[1/4] 正在准备 Python 环境..."
try {
    $pyVer = & python --version 2>&1
    Write-Host " - 找到 Python: $pyVer" -ForegroundColor Green
    
    Write-Host " - 正在安装依赖 (requirements.txt)..."
    & python -m pip install -r requirements.txt --quiet
    if ($LASTEXITCODE -ne 0) {
        Write-Host " - ⚠️ 依赖安装可能不完整，建议稍后查看报错内容。" -ForegroundColor Yellow
    }
}
catch {
    Write-Host " - ❌ 错误: 未能在系统中找到 Python！" -ForegroundColor Red
    Write-Host " 请先安装 Python 3.10+: https://www.python.org/downloads/"
    pause
    exit 1
}

# 2. 生成通用配置模板
$mcpConfig = @{
    command = "python"
    args    = @($escapedLauncher)
    env     = @{
        PYTHONIOENCODING = "utf-8"
    }
}

# 3. 自动扫描并注入 IDE
Write-Host "[2/4] 正在搜寻 IDE 配置..."

# A. Claude Desktop
$claudeConfig = "$env:APPDATA\Claude\mcp_config.json"
if (Test-Path $claudeConfig) {
    Write-Host " - 发现 Claude Desktop: $claudeConfig" -ForegroundColor Gray
    try {
        $cfg = Get-Content $claudeConfig | ConvertFrom-Json
        if (-not $cfg.mcpServers) { $cfg | Add-Member -Name "mcpServers" -Value @{} -NoteProperty }
        $cfg.mcpServers | Add-Member -Name "my-project-manager" -Value $mcpConfig -Force
        $cfg | ConvertTo-Json -Depth 10 | Out-File $claudeConfig -Encoding UTF8
        Write-Host " - ✅ 已成功注入 Claude Desktop！" -ForegroundColor Green
    }
    catch { Write-Host " - ❌ Claude 注入失败: $($_.Exception.Message)" -ForegroundColor Red }
}

# B. Windsurf
$windsurfConfig = "$env:USERPROFILE\.codeium\windsurf\mcp_config.json"
if (Test-Path $windsurfConfig) {
    Write-Host " - 发现 Windsurf: $windsurfConfig" -ForegroundColor Gray
    try {
        $cfg = Get-Content $windsurfConfig | ConvertFrom-Json
        if (-not $cfg.mcpServers) { $cfg | Add-Member -Name "mcpServers" -Value @{} -NoteProperty }
        $cfg.mcpServers | Add-Member -Name "my-project-manager" -Value $mcpConfig -Force
        $cfg | ConvertTo-Json -Depth 10 | Out-File $windsurfConfig -Encoding UTF8
        Write-Host " - ✅ 已成功注入 Windsurf！" -ForegroundColor Green
    }
    catch { Write-Host " - ❌ Windsurf 注入失败: $($_.Exception.Message)" -ForegroundColor Red }
}

# 4. 特殊：一键安装到 Claude Code (命令行版本)
Write-Host "[3/4] 检测 Claude Code..."
$claudeCode = Get-Command claude -ErrorAction SilentlyContinue
if ($claudeCode) {
    Write-Host " - 发现 Claude Code 命令行工具。" -ForegroundColor Gray
    Write-Host " - 正在通过命令自动配置..."
    try {
        & claude mcp add my-project-manager python $escapedLauncher
        Write-Host " - ✅ Claude Code 配置指令已发动。" -ForegroundColor Green
    }
    catch { }
}

# 5. 完成提示
Write-Host "[4/4] 正在保存备份配置..."
$backupPath = Join-Path $scriptDir "mcp_config_backup.json"
@{ mcpServers = @{ "my-project-manager" = $mcpConfig } } | ConvertTo-Json -Depth 10 | Out-File $backupPath -Encoding UTF8

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "🎉 完美！MyProjectManager 部署完成！" -ForegroundColor Green
Write-Host "当前安装目录: $scriptDir"
Write-Host "备份配置已存至: $backupPath"
Write-Host ""
Write-Host "💡 现在您可以打开您的 IDE (Windsurf/Cursor/Claude)，"
Write-Host "   选择任何项目并开始与您的专家团队合作了！"
Write-Host "==========================================" -ForegroundColor Cyan

pause
