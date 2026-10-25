---
title: Basic Configuration
description: Initial configuration of the MCP-Go-MSSQL server
---

## Environment variables

The safest way to configure the server is through environment variables. Copy the example template:

```bash
cp .env.example .env
```

Edit `.env` with your database credentials.

### Load environment variables

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

## Required variables

| Variable | Description |
|----------|-------------|
| `MSSQL_SERVER` | SQL Server hostname or IP |
| `MSSQL_DATABASE` | Database name |
| `MSSQL_USER` | SQL Server username |
| `MSSQL_PASSWORD` | SQL Server password |

## Optional variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MSSQL_PORT` | `1433` | SQL Server port |
| `DEVELOPER_MODE` | `false` | `true` for development, `false` for production |
| `MSSQL_READ_ONLY` | `false` | `true` = absolute read-only. A whitelist cannot enable writes |
| `MSSQL_WHITELIST_TABLES` | _(empty)_ | Tables allowed for modification only when `READ_ONLY=false` |
| `MSSQL_AUTH` | `sql` | Authentication mode (`sql`, `integrated`/`windows`; `azure` is not implemented — use `MSSQL_CONNECTION_STRING`) |
| `MSSQL_CONNECTION_STRING` | _(empty)_ | Custom connection string (overrides other variables) |

## Execution modes

### Development mode

```bash
DEVELOPER_MODE=true go run ./cmd/mcp-go-mssql
```

In development mode:
- Self-signed certificates are allowed
- Errors show technical details
- Encryption can be disabled for local SQL Server

### Production mode

```bash
DEVELOPER_MODE=false ./mcp-go-mssql
```

In production mode:
- Valid TLS certificates are required
- Errors are generic (no technical details)
- Encryption is mandatory

## Verify the connection

```bash
cd test
go run test-connection.go
```
