# Test MCP Server with time for connection

Write-Host "Testing MCP Server with time for database connection..." -ForegroundColor Green
Write-Host ""

# Set environment variables  
$env:MSSQL_SERVER = "10.203.3.10"
$env:MSSQL_DATABASE = "JJP_TRANSFER"
$env:MSSQL_USER = "userTRANSFER"
$env:MSSQL_PASSWORD = "jl3RN7o02g"
$env:MSSQL_PORT = "1433"
$env:DEVELOPER_MODE = "true"

# Start the server in background and send initialize
Write-Host "Starting server..." -ForegroundColor Yellow
$serverProcess = Start-Process -FilePath ".\mcp-server-test.exe" -PassThru -NoNewWindow -RedirectStandardInput "input.txt" -RedirectStandardOutput "output.txt" -RedirectStandardError "error.txt"

# Send initialize command first
'{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0"}}}' | Out-File -FilePath "input.txt" -Encoding UTF8

Start-Sleep -Seconds 2

# Send notifications/initialized 
'{"jsonrpc": "2.0", "id": 2, "method": "notifications/initialized"}' | Out-File -FilePath "input.txt" -Append -Encoding UTF8

Write-Host "Waiting for database connection to establish (5 seconds)..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Now test database functions
Write-Host "Testing database info..." -ForegroundColor Yellow
'{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_database_info", "arguments": {}}}' | Out-File -FilePath "input.txt" -Append -Encoding UTF8

Start-Sleep -Seconds 2

# Terminate server
$serverProcess.Kill()

# Show results
Write-Host ""
Write-Host "Output:" -ForegroundColor Cyan
if (Test-Path "output.txt") {
    Get-Content "output.txt"
}

Write-Host ""
Write-Host "Errors:" -ForegroundColor Red  
if (Test-Path "error.txt") {
    Get-Content "error.txt"
}

# Cleanup
Remove-Item "input.txt", "output.txt", "error.txt" -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Test completed!" -ForegroundColor Green