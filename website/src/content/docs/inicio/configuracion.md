---
title: Configuración básica
description: Configuración inicial del servidor MCP-Go-MSSQL
---

## Variables de entorno

La forma más segura de configurar el servidor es mediante variables de entorno. Copia la plantilla de ejemplo:

```bash
cp .env.example .env
```

Edita `.env` con tus credenciales de base de datos.

### Cargar variables de entorno

**Linux/macOS:**
```bash
source .env
```

**Windows PowerShell:**
```powershell
Get-Content .env | ForEach-Object {
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name, $value)
}
```

## Variables requeridas

| Variable | Descripción |
|----------|-------------|
| `MSSQL_SERVER` | Hostname o IP del servidor SQL |
| `MSSQL_DATABASE` | Nombre de la base de datos |
| `MSSQL_USER` | Usuario de SQL Server |
| `MSSQL_PASSWORD` | Contraseña de SQL Server |

## Variables opcionales

| Variable | Valor por defecto | Descripción |
|----------|-------------------|-------------|
| `MSSQL_PORT` | `1433` | Puerto de SQL Server |
| `DEVELOPER_MODE` | `false` | `true` para desarrollo, `false` para producción |
| `MSSQL_READ_ONLY` | `false` | Modo solo lectura |
| `MSSQL_WHITELIST_TABLES` | _(vacío)_ | Tablas permitidas para modificación en modo read-only |
| `MSSQL_AUTH` | `sql` | Modo de autenticación (`sql`, `integrated`, `azure`) |
| `MSSQL_CONNECTION_STRING` | _(vacío)_ | Connection string personalizado (anula otras variables) |

## Modos de ejecución

### Modo desarrollo

```bash
DEVELOPER_MODE=true go run main.go
```

En modo desarrollo:
- Se permiten certificados autofirmados
- Los errores muestran detalles técnicos
- El cifrado se puede desactivar para SQL Server local

### Modo producción

```bash
DEVELOPER_MODE=false ./mcp-go-mssql
```

En modo producción:
- Se requieren certificados TLS válidos
- Los errores son genéricos (sin detalles técnicos)
- El cifrado es obligatorio

## Verificar la conexión

```bash
cd test
go run test-connection.go
```
