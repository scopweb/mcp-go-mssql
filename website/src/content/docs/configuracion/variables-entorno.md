---
title: Variables de entorno
description: Referencia completa de variables de entorno de MCP-Go-MSSQL
---

Todas las credenciales y opciones de configuración se gestionan mediante variables de entorno. Nunca hardcodees credenciales en el código fuente.

## Variables requeridas

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `MSSQL_SERVER` | Hostname o IP del servidor SQL | `prod-server.database.windows.net` |
| `MSSQL_DATABASE` | Nombre de la base de datos | `ProductionDB` |
| `MSSQL_USER` | Usuario de SQL Server | `app_user` |
| `MSSQL_PASSWORD` | Contraseña de SQL Server | _(secreto)_ |

## Variables opcionales

| Variable | Default | Descripción |
|----------|---------|-------------|
| `MSSQL_PORT` | `1433` | Puerto de SQL Server |
| `DEVELOPER_MODE` | `false` | `true` para desarrollo (TLS relajado, errores detallados) |
| `MSSQL_READ_ONLY` | `false` | Bloquea operaciones de escritura |
| `MSSQL_WHITELIST_TABLES` | _(vacío)_ | Tablas permitidas para modificación en modo read-only |
| `MSSQL_AUTH` | `sql` | Modo de autenticación: `sql`, `integrated`, `azure` |
| `MSSQL_ENCRYPT` | _(auto)_ | Control de cifrado TLS. Solo efectivo con `DEVELOPER_MODE=true`. `false` = desactivar cifrado (**necesario para SQL Server 2008/2012**). Si no se define: `false` en dev, siempre `true` en producción |
| `MSSQL_ALLOWED_DATABASES` | _(vacío)_ | BDs adicionales accesibles para queries cross-database (separadas por comas) |
| `MSSQL_CONNECTION_STRING` | _(vacío)_ | Connection string personalizado (anula otras variables) |
| `MSSQL_MAX_QUERY_SIZE` | `1048576` | Tamaño máximo de consulta en caracteres (1 MB por defecto) |

## Plantilla .env

```bash
# Copiar y editar
cp .env.example .env

# Ejemplo de contenido
MSSQL_SERVER=localhost
MSSQL_DATABASE=MyDB
MSSQL_USER=sa
MSSQL_PASSWORD=YourPassword123
MSSQL_PORT=1433
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false

# Cross-database (opcional)
MSSQL_ALLOWED_DATABASES=OtherDB1,OtherDB2
```

### Acceso cross-database

Permite consultar tablas de otras bases de datos del mismo servidor usando nombres de 3 partes:

```sql
SELECT * FROM OtherDB.dbo.TableName
```

**Comportamiento de seguridad:**
- Solo lectura: las modificaciones (INSERT/UPDATE/DELETE) en BDs cruzadas están **siempre bloqueadas**
- La validación de schema verifica que las tablas existan en la BD destino
- El usuario SQL debe tener permisos en las BDs adicionales

## Cargar variables

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

## Permisos de archivo

```bash
# Linux/macOS
chmod 600 .env

# Windows
icacls .env /inheritance:r /grant:r "%USERNAME%:R"
```
