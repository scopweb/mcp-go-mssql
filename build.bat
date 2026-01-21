@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo Building MCP Go MSSQL Server...
echo.

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo Error: Go is not installed or not in PATH
    echo Please download Go from https://go.dev/dl/
    pause
    exit /b 1
)

echo Checking Go version...
for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo Found Go version: !GO_VERSION!
echo.

REM Create build directory if it doesn't exist
if not exist build (
    mkdir build
    echo Created build directory
)

REM Clean previous builds
if exist build\mcp-go-mssql.exe (
    del build\mcp-go-mssql.exe
    echo Cleaned previous build
)

REM Download dependencies
echo Downloading dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo Error: Failed to download dependencies
    pause
    exit /b 1
)

REM Build production version with optimizations
echo Building production executable...
go build -ldflags "-w -s" -o build\mcp-go-mssql.exe -v
if %errorlevel% neq 0 (
    echo Error: Build failed
    pause
    exit /b 1
)

REM Get file size
for /F %%A in ('dir /b build\mcp-go-mssql.exe') do (
    for /F "usebackq" %%B in (`dir /-C "build\%%A" ^| find "mcp-go-mssql.exe"`) do (
        echo.
        echo File size: %%B bytes
    )
)

echo.
echo ========================================
echo ✓ Build successful!
echo ========================================
echo.
echo Executable location: build\mcp-go-mssql.exe
echo.
echo To use with Claude Desktop:
echo 1. Ensure claude_desktop_config.json has the correct path:
echo    "command": "C:\\MCPs\\clone\\mcp-go-mssql\\build\\mcp-go-mssql.exe"
echo 2. Configure environment variables in the config
echo 3. Restart Claude Desktop to reload the MCP server
echo.
pause