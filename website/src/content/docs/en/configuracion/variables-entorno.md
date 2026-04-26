---
title: Environment Variables
description: Complete reference of MCP-Go-MSSQL environment variables
---

All credentials and configuration options are managed through environment variables. Never hardcode credentials in source code.

## Required variables

| Variable | Description | Example |
|----------|-------------|---------|
| `MSSQL_SERVER` | SQL Server hostname or IP | `prod-server.database.windows.net` |
| `MSSQL_DATABASE` | Database name | `ProductionDB` |
| `MSSQL_USER` | SQL Server username | `app_user` |
| `MSSQL_PASSWORD` | SQL Server password | _(secret)_ |

## Optional variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MSSQL_PORT` | `1433` | SQL Server port |
| `DEVELOPER_MODE` | `false` | `true` for development (relaxed TLS, detailed errors) |
| `MSSQL_READ_ONLY` | `false` | Blocks write operations |
| `MSSQL_WHITELIST_TABLES` | _(empty)_ | Tables allowed for modification in read-only mode. Use `*` to allow all tables |
| `MSSQL_AUTH` | `sql` | Authentication mode: `sql`, `integrated`, `azure` |
| `MSSQL_ENCRYPT` | _(auto)_ | TLS encryption control. Only effective with `DEVELOPER_MODE=true`. `false` = disable encryption (**required for SQL Server 2008/2012**). If not set: `false` in dev, always `true` in production |
| `MSSQL_ALLOWED_DATABASES` | _(empty)_ | Additional databases accessible for cross-database queries (comma-separated) |
| `MSSQL_CONNECTION_STRING` | _(empty)_ | Custom connection string (overrides other variables) |
| `MSSQL_MAX_QUERY_SIZE` | `1048576` | Maximum query size in characters (1 MB default) |
| `MSSQL_CONFIRM_DESTRUCTIVE` | `true` | Require confirmation for destructive DDL operations (ALTER VIEW, DROP TABLE, etc.) — always enforced, AUTOPILOT does not skip it |
| `MSSQL_AUTOPILOT` | `false` | Autonomous mode: skips schema validation (can query non-existent tables). Destructive confirmation and READ_ONLY still enforced |

## .env template

```bash
# Copy and edit
cp .env.example .env

# Example content
MSSQL_SERVER=localhost
MSSQL_DATABASE=MyDB
MSSQL_USER=sa
MSSQL_PASSWORD=YourPassword123
MSSQL_PORT=1433
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false

# Cross-database (optional)
MSSQL_ALLOWED_DATABASES=OtherDB1,OtherDB2
```

### Cross-database access

Allows querying tables in other databases on the same server using 3-part names:

```sql
SELECT * FROM OtherDB.dbo.TableName
```

**Security behavior:**
- Read-only: modifications (INSERT/UPDATE/DELETE) on cross-databases are **always blocked**
- Schema validation checks tables exist in the target database
- The SQL user must have permissions on the additional databases

## Loading variables

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

## File permissions

```bash
# Linux/macOS
chmod 600 .env

# Windows
icacls .env /inheritance:r /grant:r "%USERNAME%:R"
```

## Dynamic Multi-Connection Mode

When `MSSQL_DYNAMIC_MODE=true` is enabled, the server can connect to multiple databases from a single MCP instance. Connections are pre-configured in `.env` and the AI only sees safe aliases — **no sensitive data exposed**.

### Dynamic mode variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MSSQL_DYNAMIC_MODE` | `false` | `true` to enable dynamic connections |
| `MSSQL_DYNAMIC_MAX_CONNECTIONS` | `10` | Maximum number of active dynamic connections |

### Dynamic connection configuration

Connections are defined with prefix `MSSQL_DYNAMIC_<ALIAS>_`:

```bash
# Default connection (always available)
MSSQL_SERVER=10.203.3.10
MSSQL_DATABASE=JJP_CRM
MSSQL_USER=sa
MSSQL_PASSWORD=secret123

# Dynamic connections (AI only sees aliases)
MSSQL_DYNAMIC_IDENTITY_SERVER=10.203.3.11
MSSQL_DYNAMIC_IDENTITY_DATABASE=JJP_CRM_IDENTITY
MSSQL_DYNAMIC_IDENTITY_USER=ppp
MSSQL_DYNAMIC_IDENTITY_PASSWORD=ppppp

MSSQL_DYNAMIC_FERRATGE_SERVER=10.203.3.12
MSSQL_DYNAMIC_FERRATGE_DATABASE=JJP_Ferratge_PROD
MSSQL_DYNAMIC_FERRATGE_USER=ferratge_user
MSSQL_DYNAMIC_FERRATGE_PASSWORD=otra_password
```

### Per-connection security

Each dynamic connection can have its own security configuration:

| Variable | Description |
|----------|-------------|
| `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY` | `true` = read-only |
| `MSSQL_DYNAMIC_<ALIAS>_WHITELIST_TABLES` | Tables allowed for modification |
| `MSSQL_DYNAMIC_<ALIAS>_AUTOPILOT` | `true` = skip schema validation |

### Available tools

- `dynamic_connect` — Activate a connection by alias (no credentials in params)
- `dynamic_list` — List active connections (shows alias, server, DB — no passwords)
- `dynamic_disconnect` — Close a dynamic connection

### Usage example

```json
// 1. List available connections (AI sees aliases, not passwords)
tool: dynamic_list

// 2. Activate connection by alias
tool: dynamic_connect
params: {"alias": "identity"}

// 3. Query using the connection
tool: query_database
params: {"sql": "SELECT * FROM customers", "connection": "identity"}

// 4. Disconnect
tool: dynamic_disconnect
params: {"alias": "identity"}
```

### Claude Desktop configuration

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

**Note:** Credentials go in `.env`, NOT in Claude Desktop config. The JSON only needs `MSSQL_DYNAMIC_MODE=true`.
