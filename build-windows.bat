@echo off
REM Build script for mcp-go-mssql (Windows)
REM Produces two binaries inside build\:
REM   mcp-go-mssql-dev.exe    - development build (debug info included)
REM   mcp-go-mssql-secure.exe - production build (stripped, -ldflags "-w -s")

setlocal

set OUTPUT_DIR=build
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%

echo [1/3] Tidying dependencies...
go mod tidy
if errorlevel 1 goto :error

echo [2/3] Building %OUTPUT_DIR%\mcp-go-mssql-dev.exe ...
go build -o %OUTPUT_DIR%\mcp-go-mssql-dev.exe .
if errorlevel 1 goto :error

echo [3/3] Building %OUTPUT_DIR%\mcp-go-mssql-secure.exe (stripped) ...
go build -ldflags "-w -s" -o %OUTPUT_DIR%\mcp-go-mssql-secure.exe .
if errorlevel 1 goto :error

echo.
echo Build completed successfully:
dir /b %OUTPUT_DIR%\mcp-go-mssql-*.exe
goto :eof

:error
echo.
echo BUILD FAILED
exit /b 1
