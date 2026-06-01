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
| `MSSQL_DYNAMIC_MODE` | _(auto-detect)_ | `true` = force dynamic mode (multiple aliases). `false` = force classic mode (single connection). When unset, auto-detects based on `MSSQL_DYNAMIC_*` variables. **Critical for isolation when running multiple MCP servers.** |
| `MSSQL_IGNORE_LOCAL_ENV` | `false` | `true` = completely ignore any `.env` file next to the executable. Essential for classic servers configured purely via `.mcp.json` when leftover `.env` files may exist. |

## Dynamic vs Classic Mode Precedence (Important)

The server decides between **classic mode** (single database) and **dynamic mode** (multiple aliases) with this priority:

| Priority | Condition | Result |
|----------|-----------|--------|
| 1 | `MSSQL_DYNAMIC_MODE=false` (or `0`, `no`, `off`) | **Always classic** (ignores everything else) |
| 2 | `MSSQL_DYNAMIC_MODE=true` | **Always dynamic** |
| 3 | `MSSQL_SERVER`, `MSSQL_CONNECTION_STRING` or `MSSQL_DATABASE` is present | **Classic** (protects normal `.mcp.json` configs) |
| 4 | Only `MSSQL_DYNAMIC_*` variables exist | Dynamic (auto-detect) |

**This fixes the most common complaint:**  
If you configure a server as classic in Claude Desktop (using `MSSQL_SERVER` + credentials in the `"env"` block), it should now **stay in classic mode** even if there are `.env` files nearby or inherited dynamic variables.

### Recommended Recipe: Fully Isolated Classic Server

When running multiple MCP servers at the same time (some dynamic, some classic), add these two lines to all your **classic** instances:

```json
{
  "mcpServers": {
    "mssql2": {
      "command": "C:\\MCPs\\MCP-EXE\\mssql2\\sinenv\\mcp-go-mssql-secure.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "10.203.3.10",
        "MSSQL_DATABASE": "JJP_TRANSFER",
        "MSSQL_USER": "userTRANSFER",
        "MSSQL_PASSWORD": "your_password",
        "DEVELOPER_MODE": "true",
        "MSSQL_READ_ONLY": "false",

        "MSSQL_IGNORE_LOCAL_ENV": "true",
        "MSSQL_DYNAMIC_MODE": "false"
      }
    }
  }
}
```

**Why these two lines?**
- `MSSQL_IGNORE_LOCAL_ENV=true` → Ignores any `.env` file in the same folder as the exe.
- `MSSQL_DYNAMIC_MODE=false` → Forces classic mode even if the parent process has dynamic variables.

### Common Issues

**"I still see `dynamic_available` / `dynamic_connect` tools on a server that should be classic"**

Most common causes:
- You didn't fully restart Claude Desktop after editing `.mcp.json`.
- You're still running an old binary (need version from commit `0bf02d5` or newer).
- Missing the two isolation variables above.
- Dynamic variables coming from your PowerShell profile or user environment.

Add the two variables shown in the recipe above. If it still happens, temporarily enable `DEVELOPER_MODE=true` and check the startup logs — it should clearly say `DYNAMIC_MODE=false (classic single-connection mode)`.

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
