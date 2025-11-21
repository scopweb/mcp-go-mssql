@echo off
REM Script para actualizar dependencias de manera segura
echo Actualizando dependencias del proyecto mcp-go-mssql...

REM Backup del go.mod actual
copy go.mod go.mod.backup
echo Backup creado: go.mod.backup

REM Actualizar dependencias menores (patch versions)
echo Actualizando dependencias patch/minor...
go get -u=patch ./...

REM Actualizar dependencias específicas que son seguras
echo Actualizando dependencias específicas...
go get github.com/golang-jwt/jwt/v5@v5.3.0
go get github.com/stretchr/testify@v1.11.1

REM Limpiar y reorganizar módulos
echo Limpiando módulos...
go mod tidy

REM Verificar que todo compile
echo Verificando compilación...
go build -o test-build.exe
if errorlevel 1 (
    echo ERROR: La compilación falló. Restaurando go.mod...
    copy go.mod.backup go.mod
    go mod tidy
    del test-build.exe
    exit /b 1
)

REM Ejecutar tests si existen
echo Ejecutando tests...
go test ./... 2>nul
if errorlevel 1 (
    echo ADVERTENCIA: Algunos tests fallaron. Verifica manualmente.
) else (
    echo Tests pasaron correctamente.
)

REM Limpiar archivos temporales
del test-build.exe
del go.mod.backup

echo ✅ Actualización completada exitosamente!
echo.
echo Cambios realizados:
go list -u -m all | findstr "\["