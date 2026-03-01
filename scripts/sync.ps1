# Sync and Push script for MPM project

$CommitMessage = $args[0]
if (-not $CommitMessage) {
    $CommitMessage = "Update project: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
}

Write-Host "🚀 Starting Sync Process..." -ForegroundColor Cyan

# 1. Add all changes
Write-Host "📦 Staging changes..." -ForegroundColor Yellow
git add .

# 2. Commit
Write-Host "📝 Committing changes with message: '$CommitMessage'..." -ForegroundColor Yellow
git commit -m $CommitMessage

# 3. Push
Write-Host "📤 Pushing to remote repository (main)..." -ForegroundColor Yellow
git push origin main

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Sync completed successfully!" -ForegroundColor Green
} else {
    Write-Host "❌ Sync failed. Please check the errors above." -ForegroundColor Red
}
