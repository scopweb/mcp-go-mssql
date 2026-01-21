# Actualización de Go requerida

## Estado actual

**Versión actual de Go:** 1.24.7  
**Versión requerida:** 1.24.11 o superior

## Vulnerabilidades detectadas

Se encontraron **7 vulnerabilidades** en la biblioteca estándar de Go 1.24.7:

1. **GO-2025-4155** - Consumo excesivo de recursos en crypto/x509 (Fixed in: go1.24.11)
2. **GO-2025-4013** - Panic con certificados DSA en crypto/x509 (Fixed in: go1.24.8)
3. **GO-2025-4011** - Agotamiento de memoria en encoding/asn1 (Fixed in: go1.24.8)
4. **GO-2025-4010** - Validación insuficiente de hostnames IPv6 en net/url (Fixed in: go1.24.8)
5. **GO-2025-4009** - Complejidad cuadrática en encoding/pem (Fixed in: go1.24.8)
6. **GO-2025-4008** - Error ALPN con información controlada por atacante en crypto/tls (Fixed in: go1.24.8)
7. **GO-2025-4007** - Complejidad cuadrática en name constraints crypto/x509 (Fixed in: go1.24.9)

## Pasos para actualizar Go

### Opción 1: Actualización manual (Recomendado)

1. **Descargar Go 1.24.11+:**
   - Visita: https://go.dev/dl/
   - Descarga la versión más reciente de Go 1.24.x para Windows (amd64)

2. **Ejecutar el instalador:**
   - Cierra todas las ventanas de PowerShell/Terminal
   - Ejecuta el instalador .msi descargado
   - Sigue el asistente de instalación

3. **Verificar la instalación:**
   ```powershell
   go version
   # Debe mostrar: go version go1.24.11 (o superior) windows/amd64
   ```

4. **Recompilar el proyecto:**
   ```powershell
   cd C:\MCPs\clone\mcp-go-mssql
   go clean
   go build -o build\mcp-go-mssql.exe main.go
   ```

5. **Verificar vulnerabilidades:**
   ```powershell
   go run golang.org/x/vuln/cmd/govulncheck@latest ./...
   ```

### Opción 2: Usando chocolatey (si está instalado)

```powershell
# Actualizar Go via chocolatey
choco upgrade golang -y

# Verificar versión
go version

# Recompilar
cd C:\MCPs\clone\mcp-go-mssql
go clean
go build -o build\mcp-go-mssql.exe main.go
```

### Opción 3: Usando winget (Windows 11)

```powershell
# Buscar versiones disponibles
winget search GoLang.Go

# Instalar la última versión
winget install --id GoLang.Go.1.24 -e

# Verificar versión
go version

# Recompilar
cd C:\MCPs\clone\mcp-go-mssql
go clean
go build -o build\mcp-go-mssql.exe main.go
```

## Estado de dependencias

✅ **github.com/microsoft/go-mssqldb** - v1.9.4 (última versión)  
✅ **golang.org/x/crypto** - v0.45.0 (actualizado)  
✅ **golang.org/x/text** - v0.31.0 (actualizado)  
✅ **github.com/stretchr/testify** - v1.11.1 (actualizado)

Todas las dependencias están actualizadas. Solo necesitas actualizar Go.

## Después de actualizar

Una vez actualizado Go, ejecuta:

```powershell
cd C:\MCPs\clone\mcp-go-mssql

# Limpiar cache y recompilar
go clean -cache -modcache
go mod tidy
go build -o build\mcp-go-mssql.exe main.go

# Verificar que no hay vulnerabilidades
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Ejecutar tests de seguridad
go test ./test/security/... -v
```

## Commit después de actualizar

```powershell
# Actualizar go.mod si cambió la versión de go
git add go.mod go.sum
git commit -m "chore: Update Go to 1.24.11+ to fix security vulnerabilities"
git push origin master
```

---

**Prioridad:** 🔴 Alta - Las vulnerabilidades afectan crypto/x509, crypto/tls y encoding/pem, que son usadas en las conexiones TLS a SQL Server.
