# Test MCP Server with real database connection

Write-Host "Testing MCP Server with database connection..." -ForegroundColor Green
Write-Host "Server: 10.203.3.10:1433"
Write-Host "Database: JJP_TRANSFER"
Write-Host ""

# Set environment variables
$env:MSSQL_SERVER = "10.203.3.10"
$env:MSSQL_DATABASE = "JJP_TRANSFER" 
$env:MSSQL_USER = "userTRANSFER"
$env:MSSQL_PASSWORD = "jl3RN7o02g"
$env:MSSQL_PORT = "1433"
$env:DEVELOPER_MODE = "true"

# Test 1: Initialize
Write-Host "Test 1: Initialize MCP Server" -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0"}}}' | .\mcp-server-test.exe
Write-Host ""

# Test 2: List tools  
Write-Host "Test 2: List Available Tools" -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}' | .\mcp-server-test.exe
Write-Host ""

# Wait a bit for database connection
Start-Sleep -Seconds 3

# Test 3: Get database info
Write-Host "Test 3: Get Database Info" -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_database_info", "arguments": {}}}' | .\mcp-server-test.exe
Write-Host ""

# Test 4: List tables
Write-Host "Test 4: List Tables" -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "list_tables", "arguments": {}}}' | .\mcp-server-test.exe
Write-Host ""

# Test 5: Simple query
Write-Host "Test 5: Simple Query (SELECT @@VERSION)" -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "query_database", "arguments": {"query": "SELECT @@VERSION as server_version"}}}' | .\mcp-server-test.exe

Write-Host ""
Write-Host "Testing completed!" -ForegroundColor Green