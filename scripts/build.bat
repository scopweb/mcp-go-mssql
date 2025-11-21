@echo off
echo Building MCP Go MSSQL Server...

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo Error: Go is not installed or not in PATH
    pause
    exit /b 1
)

REM Create build directory if it doesn't exist
if not exist build mkdir build

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

REM Build production version
echo Building production executable...
go build -ldflags "-w -s" -o build/mcp-go-mssql.exe
if %errorlevel% neq 0 (
    echo Error: Build failed
    pause
    exit /b 1
)

echo.
echo ✓ Build successful!
echo ✓ Executable: build/mcp-go-mssql.exe
echo.
echo To use with Claude Desktop:
echo 1. Copy build/mcp-go-mssql.exe to your desired location
echo 2. Update your Claude Desktop config with the full path
echo 3. Configure environment variables in the config
echo.
pause