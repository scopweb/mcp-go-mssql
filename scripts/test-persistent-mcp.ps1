# Test MCP Server with persistent connection

Write-Host "Testing MCP Server with persistent connection..." -ForegroundColor Green
Write-Host "This test will start the server and send multiple commands" -ForegroundColor Yellow
Write-Host ""

# Set environment variables  
$env:MSSQL_SERVER = "10.203.3.10"
$env:MSSQL_DATABASE = "JJP_TRANSFER"
$env:MSSQL_USER = "userTRANSFER"
$env:MSSQL_PASSWORD = "jl3RN7o02g"
$env:MSSQL_PORT = "1433"
$env:DEVELOPER_MODE = "true"

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