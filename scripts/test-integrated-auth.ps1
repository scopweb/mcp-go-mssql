# Test Windows Integrated Authentication
# This script helps diagnose connection issues with integrated authentication

Write-Host "=== MCP-Go-MSSQL Integrated Authentication Test ===" -ForegroundColor Cyan
Write-Host ""

# Set environment variables
$env:MSSQL_SERVER = "localhost"
$env:MSSQL_AUTH = "integrated"
$env:DEVELOPER_MODE = "true"
# Intentionally not setting MSSQL_DATABASE to test database-less connection

Write-Host "Environment Variables:" -ForegroundColor Yellow
Write-Host "  MSSQL_SERVER: $env:MSSQL_SERVER"
Write-Host "  MSSQL_AUTH: $env:MSSQL_AUTH"
Write-Host "  MSSQL_DATABASE: $(if ($env:MSSQL_DATABASE) { $env:MSSQL_DATABASE } else { '(not set)' })"
Write-Host "  DEVELOPER_MODE: $env:DEVELOPER_MODE"
Write-Host ""

# Get the executable path
$exePath = "C:\MCPs\clone\mcp-go-mssql\build\mcp-go-mssql.exe"

if (-not (Test-Path $exePath)) {
    Write-Host "ERROR: Executable not found at: $exePath" -ForegroundColor Red
    Write-Host "Please build the project first using: .\build.bat" -ForegroundColor Yellow
    exit 1
}

Write-Host "Testing connection..." -ForegroundColor Yellow
Write-Host "Press Ctrl+C to stop" -ForegroundColor Gray
Write-Host ""

# Create a test MCP request
$initRequest = @{
    jsonrpc = "2.0"
    id = 1
    method = "initialize"
    params = @{
        protocolVersion = "2025-06-18"
        capabilities = @{}
        clientInfo = @{
            name = "test-script"
            version = "1.0"
        }
    }
} | ConvertTo-Json -Compress

$dbInfoRequest = @{
    jsonrpc = "2.0"
    id = 2
    method = "tools/call"
    params = @{
        name = "get_database_info"
        arguments = @{}
    }
} | ConvertTo-Json -Compress

# Start the process and capture both stdout and stderr
Write-Host "Starting MCP server..." -ForegroundColor Cyan
Write-Host "================== STDERR (Logs) ==================" -ForegroundColor Magenta

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $exePath
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.UseShellExecute = $false
$psi.CreateNoWindow = $true

$process = New-Object System.Diagnostics.Process
$process.StartInfo = $psi

# Event handlers for stderr (logs)
$stderrBuilder = New-Object System.Text.StringBuilder
$process.add_ErrorDataReceived({
    if ($EventArgs.Data) {
        $msg = $EventArgs.Data
        Write-Host $msg -ForegroundColor Gray
        [void]$stderrBuilder.AppendLine($msg)
    }
})

# Start process
$process.Start() | Out-Null
$process.BeginErrorReadLine()

# Wait a moment for server to start
Start-Sleep -Seconds 2

# Send initialize request
Write-Host ""
Write-Host "Sending initialize request..." -ForegroundColor Cyan
$process.StandardInput.WriteLine($initRequest)
$process.StandardInput.Flush()

# Read response
$response = $process.StandardOutput.ReadLine()
Write-Host "Response: $response" -ForegroundColor Green

# Wait for connection attempt
Start-Sleep -Seconds 3

# Send get_database_info request
Write-Host ""
Write-Host "Sending get_database_info request..." -ForegroundColor Cyan
$process.StandardInput.WriteLine($dbInfoRequest)
$process.StandardInput.Flush()

# Read response
$response = $process.StandardOutput.ReadLine()
Write-Host "Response: $response" -ForegroundColor Green

# Wait a bit more to capture all logs
Start-Sleep -Seconds 2

# Clean shutdown
Write-Host ""
Write-Host "Stopping server..." -ForegroundColor Yellow
if (-not $process.HasExited) {
    $process.Kill()
}

Write-Host ""
Write-Host "================== Test Complete ==================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Check the logs above for connection errors." -ForegroundColor Yellow
Write-Host "Common issues:" -ForegroundColor Yellow
Write-Host "  - SQL Server not running on localhost" -ForegroundColor Gray
Write-Host "  - TCP/IP not enabled in SQL Server Configuration" -ForegroundColor Gray
Write-Host "  - Windows user doesn't have permission to access SQL Server" -ForegroundColor Gray
Write-Host "  - Named Pipes not enabled" -ForegroundColor Gray
Write-Host ""
Write-Host "To check SQL Server status, run:" -ForegroundColor Yellow
Write-Host "  Get-Service -Name 'MSSQL*'" -ForegroundColor Cyan
