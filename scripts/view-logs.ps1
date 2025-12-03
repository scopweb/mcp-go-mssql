# Quick script to view Claude Desktop logs in real-time
# This helps diagnose MCP server connection issues

$logPath = "$env:APPDATA\Claude\logs"

Write-Host "=== Claude Desktop Logs ===" -ForegroundColor Cyan
Write-Host "Log path: $logPath" -ForegroundColor Yellow
Write-Host ""

if (-not (Test-Path $logPath)) {
    Write-Host "ERROR: Log directory not found at $logPath" -ForegroundColor Red
    exit 1
}

# Get the most recent log file
$latestLog = Get-ChildItem -Path $logPath -Filter "mcp*.log" | Sort-Object LastWriteTime -Descending | Select-Object -First 1

if ($latestLog) {
    Write-Host "Latest log file: $($latestLog.Name)" -ForegroundColor Green
    Write-Host "Last modified: $($latestLog.LastWriteTime)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "=== Tailing log (Ctrl+C to stop) ===" -ForegroundColor Cyan
    Write-Host ""
    
    Get-Content -Path $latestLog.FullName -Wait -Tail 50
} else {
    Write-Host "No MCP log files found" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "All log files:" -ForegroundColor Yellow
    Get-ChildItem -Path $logPath -File | Format-Table Name, LastWriteTime, Length -AutoSize
}
