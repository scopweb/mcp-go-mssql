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
| `MSSQL_WHITELIST_TABLES` | _(vacío)_ | Tablas permitidas para modificación en modo read-only. Usa `*` para permitir todas las tablas |
| `MSSQL_AUTH` | `sql` | Modo de autenticación: `sql`, `integrated`, `azure` |
| `MSSQL_ENCRYPT` | _(auto)_ | Control de cifrado TLS. Solo efectivo con `DEVELOPER_MODE=true`. `false` = desactivar cifrado (**necesario para SQL Server 2008/2012**). Si no se define: `false` en dev, siempre `true` en producción |
| `MSSQL_ALLOWED_DATABASES` | _(vacío)_ | BDs adicionales accesibles para queries cross-database (separadas por comas) |
| `MSSQL_CONNECTION_STRING` | _(vacío)_ | Connection string personalizado (anula otras variables) |
| `MSSQL_MAX_QUERY_SIZE` | `1048576` | Tamaño máximo de consulta en caracteres (1 MB por defecto) |
| `MSSQL_CONFIRM_DESTRUCTIVE` | `true` | Require confirmación para operaciones DDL destructivas (ALTER VIEW, DROP TABLE, etc.) — siempre activo, AUTOPILOT no lo skipea |
| `MSSQL_AUTOPILOT` | `false` | Modo autónomo: skipea validación de schema (puede consultar tablas inexistentes). Confirmación destructiva y READ_ONLY siguen activos |

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

## Modo Dynamic Multi-Connection

Cuando `MSSQL_DYNAMIC_MODE=true` está habilitado, el servidor puede conectar a múltiples bases de datos desde una única instancia MCP. Las conexiones se pre-configuran en `.env` y la IA solo ve alias seguros — **sin datos sensibles expuestos**.

### Variables de modo dinámico

| Variable | Default | Descripción |
|----------|---------|-------------|
| `MSSQL_DYNAMIC_MODE` | `false` | `true` para habilitar conexiones dinámicas |
| `MSSQL_DYNAMIC_MAX_CONNECTIONS` | `10` | Número máximo de conexiones dinámicas activas |

### Configuración de conexiones dinámicas

Las conexiones se definen con prefijo `MSSQL_DYNAMIC_<ALIAS>_`:

```bash
# Conexión por defecto (siempre disponible)
MSSQL_SERVER=10.203.3.10
MSSQL_DATABASE=JJP_CRM
MSSQL_USER=sa
MSSQL_PASSWORD=secret123

# Conexiones dinámicas (la IA solo ve los alias)
MSSQL_DYNAMIC_IDENTITY_SERVER=10.203.3.11
MSSQL_DYNAMIC_IDENTITY_DATABASE=JJP_CRM_IDENTITY
MSSQL_DYNAMIC_IDENTITY_USER=ppp
MSSQL_DYNAMIC_IDENTITY_PASSWORD=ppppp

MSSQL_DYNAMIC_FERRATGE_SERVER=10.203.3.12
MSSQL_DYNAMIC_FERRATGE_DATABASE=JJP_Ferratge_PROD
MSSQL_DYNAMIC_FERRATGE_USER=ferratge_user
MSSQL_DYNAMIC_FERRATGE_PASSWORD=otra_password
```

### Seguridad por conexión

Cada conexión dinámica puede tener su propia configuración de seguridad:

| Variable | Descripción |
|----------|-------------|
| `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY` | `true` = solo lectura |
| `MSSQL_DYNAMIC_<ALIAS>_WHITELIST_TABLES` | Tablas permitidas para modificación |
| `MSSQL_DYNAMIC_<ALIAS>_AUTOPILOT` | `true` =跳过 validación de schema |

### Herramientas disponibles

- `dynamic_connect` — Activar una conexión por alias (sin credenciales en params)
- `dynamic_list` — Listar conexiones activas (muestra alias, server, BD — sin passwords)
- `dynamic_disconnect` — Cerrar una conexión dinámica

### Ejemplo de uso

```json
// 1. Listar conexiones disponibles (la IA ve alias, no passwords)
tool: dynamic_list

// 2. Activar conexión por alias
tool: dynamic_connect
params: {"alias": "identity"}

// 3. Query usando la conexión
tool: query_database
params: {"sql": "SELECT * FROM customers", "connection": "identity"}

// 4. Desconectar
tool: dynamic_disconnect
params: {"alias": "identity"}
```

### Configuración en Claude Desktop

```json
{
  "mcpServers": {
    "mssql-multi": {
      "command": "C:\\MCPs\\clone\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "DEVELOPER_MODE": "true",
        "MSSQL_DYNAMIC_MODE": "true"
      }
    }
  }
}
```

**Nota:** Las credenciales van en `.env`, NO en la configuración de Claude Desktop. El JSON solo necesita `MSSQL_DYNAMIC_MODE=true`.
