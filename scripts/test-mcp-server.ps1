# Test MCP Server with real database connection

Write-Host "Testing MCP Server with database connection..." -ForegroundColor Green

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

Write-Host "Server: $env:MSSQL_SERVER:$env:MSSQL_PORT"
Write-Host "Database: $env:MSSQL_DATABASE"
Write-Host ""

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
