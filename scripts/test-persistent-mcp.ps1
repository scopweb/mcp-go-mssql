# Test MCP Server with persistent connection

Write-Host "Testing MCP Server with persistent connection..." -ForegroundColor Green
Write-Host "This test will start the server and send multiple commands" -ForegroundColor Yellow
Write-Host ""

# Load environment from .env file
$envFile = Join-Path $PSScriptRoot "..\.env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim())
        }
    }
    Write-Host "Loaded environment from .env" -ForegroundColor Cyan
} else {
    Write-Host "WARNING: .env file not found at $envFile" -ForegroundColor Red
    Write-Host "Copy .env.example to .env and configure your credentials" -ForegroundColor Yellow
    exit 1
}

# Create a test input file with MCP commands
$testCommands = @"
{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0"}}}
{"jsonrpc": "2.0", "id": 2, "method": "notifications/initialized"}
{"jsonrpc": "2.0", "id": 3, "method": "tools/list"}
{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "get_database_info", "arguments": {}}}
{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "list_tables", "arguments": {}}}
{"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": {"name": "query_database", "arguments": {"query": "SELECT @@VERSION as server_version, GETDATE() as current_time"}}}
"@

$testCommands | Out-File -FilePath "test-input.txt" -Encoding UTF8

Write-Host "Sending commands to MCP server..." -ForegroundColor Yellow
Write-Host "Commands in test-input.txt:" -ForegroundColor Cyan
Get-Content "test-input.txt" | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
Write-Host ""

Write-Host "Starting MCP server and processing commands..." -ForegroundColor Yellow
Get-Content "test-input.txt" | .\mcp-server-test.exe

Write-Host ""
Write-Host "Cleaning up..." -ForegroundColor Yellow
Remove-Item "test-input.txt" -ErrorAction SilentlyContinue

Write-Host "Test completed!" -ForegroundColor Green
