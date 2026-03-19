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
| `MSSQL_WHITELIST_TABLES` | _(empty)_ | Tables allowed for modification in read-only mode |
| `MSSQL_AUTH` | `sql` | Authentication mode: `sql`, `integrated`, `azure` |
| `MSSQL_ENCRYPT` | _(auto)_ | TLS encryption control. Only effective with `DEVELOPER_MODE=true`. `false` = disable encryption (**required for SQL Server 2008/2012**). If not set: `false` in dev, always `true` in production |
| `MSSQL_CONNECTION_STRING` | _(empty)_ | Custom connection string (overrides other variables) |

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
```

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
