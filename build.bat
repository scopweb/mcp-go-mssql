@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

:menu
cls
echo ========================================
echo   MCP-Go-MSSQL Build Script
echo ========================================
echo.
echo Selecciona el tipo de build:
echo.
echo 1 - Produccion (SEGURO)
echo    Genera: build\mcp-go-mssql.exe (~7MB)
echo    Flags: -ldflags "-w -s" (stripped)
echo    Uso: Despliegue en servidores de produccion
echo    Stack traces: Solo direcciones hex
echo.
echo 2 - Desarrollo (DEBUG)
echo    Genera: build\mcp-go-mssql-dev.exe (~10MB)
echo    Flags: Ninguno (sin strip)
echo    Uso: Debugging local, errores visibles en terminal
echo    Stack traces: Nombres completos de funciones y lineas
echo.
echo 3 - Con log a archivo (DEBUG + CRASH LOG)
echo    Genera: build\mcp-go-mssql-dev.exe (~10MB)
echo    Flags: Ninguno (sin strip)
echo    Uso: Guardar output en crash.log para analisis posterior
echo    Execution: build\mcp-go-mssql-dev.exe 2^>^&1 ^| tee crash.log
echo.
echo 4 - Salir
echo.
set /p choice="Opcion (1-4): "

if "!choice!"=="1" goto :build-prod
if "!choice!"=="2" goto :build-dev
if "!choice!"=="3" goto :build-log
if "!choice!"=="4" goto :exit
goto :menu

:build-prod
cls
echo.
echo ========================================
echo   Build Produccion (SEGURO)
echo ========================================
echo.
echo Generando executables para produccion...
echo.

REM Build normal
echo Building mcp-go-mssql.exe...
go build -ldflags "-w -s" -o build\mcp-go-mssql.exe -v
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build mcp-go-mssql.exe fallido
    pause
    exit /b 1
)

REM Build secure
echo Building mcp-go-mssql-secure.exe...
go build -ldflags "-w -s" -o build\mcp-go-mssql-secure.exe -v
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build mcp-go-mssql-secure.exe fallido
    pause
    exit /b 1
)

echo.
echo ========================================
echo   Build Produccion exitoso
echo ========================================
echo.
echo Ubicaciones:
echo   - build\mcp-go-mssql.exe
echo   - build\mcp-go-mssql-secure.exe
echo.
echo Caracteristicas:
echo   - Binaryes pequenos (~7MB cada uno)
echo   - Sin symbol table ni debug info (stripped)
echo   - OPTIMIZADO PARA PRODUCCION
echo.
pause
goto :menu

:build-dev
cls
echo.
echo ========================================
echo   Build Desarrollo (DEBUG)
echo ========================================
echo.
echo Generando executable con debug symbols...
echo.
go build -o build\mcp-go-mssql-dev.exe -v
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build fallido
    pause
    exit /b 1
)
for /F %%A in ('dir /b build\mcp-go-mssql-dev.exe') do (
    for /F "usebackq" %%B in (`dir /-C "build\%%A" ^| find "mcp-go-mssql-dev.exe"`) do (
        echo.
        echo Tamanio: %%B bytes (~10MB)
    )
)
echo.
echo ========================================
echo   Build Desarrollo exitoso
echo ========================================
echo.
echo Ubicacion: build\mcp-go-mssql-dev.exe
echo.
echo Caracteristicas:
echo   - Binary completo (~10MB)
echo   - Symbol table y debug info incluidos
echo   - Stack traces completos con nombres de funciones
echo.
echo Uso:
echo   1. Ejecuta: build\mcp-go-mssql-dev.exe
echo   2. Los errores apareceran en esta terminal
echo   3. Copia los stack traces para analizar
echo.
pause
goto :menu

:build-log
cls
echo.
echo ========================================
echo   Build Desarrollo + Log (DEBUG + CRASH LOG)
echo ========================================
echo.
echo Generando executable con debug symbols...
REM Limpiar log anterior si existe
if exist crash.log (
    echo Limpiando crash.log anterior...
    del crash.log
)
echo.
go build -o build\mcp-go-mssql-dev.exe -v
if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Build fallido
    pause
    exit /b 1
)
for /F %%A in ('dir /b build\mcp-go-mssql-dev.exe') do (
    for /F "usebackq" %%B in (`dir /-C "build\%%A" ^| find "mcp-go-mssql-dev.exe"`) do (
        echo.
        echo Tamanio: %%B bytes (~10MB)
    )
)
echo.
echo ========================================
echo   Build exitoso
echo ========================================
echo.
echo Ubicacion: build\mcp-go-mssql-dev.exe
echo Log file: crash.log (se crea al ejecutar)
echo.
echo Caracteristicas:
echo   - Binary completo (~10MB)
echo   - Symbol table y debug info incluidos
echo   - Output completo guardado en crash.log
echo.
echo Ejecucion (abre otra terminal):
echo   build\mcp-go-mssql-dev.exe 2^>^&1 ^| tee crash.log
echo.
echo Esto guardara todo el output en crash.log
echo INCLUDING: stderr (errores) + stdout (logs JSON)
echo.
pause
goto :menu

:exit
exit /b 0
