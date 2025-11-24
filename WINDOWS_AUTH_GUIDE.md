# Windows Integrated Authentication (SSPI) Guide

## Overview

This project now supports **Windows Integrated Authentication (SSPI)** for SQL Server without requiring TCP/IP to be enabled. It uses **Named Pipes** protocol which is built into Windows.

## Requirements

- Windows operating system (SSPI only works on Windows)
- SQL Server with Named Pipes enabled (default)
- User running the application must have SQL Server login permissions

## Configuration

### For Claude Code

Set these environment variables:

```bash
$env:MSSQL_SERVER = "."                    # "." for local server or server name
$env:MSSQL_AUTH = "integrated"             # or "windows"
$env:DEVELOPER_MODE = "true"               # For self-signed certificates

# Optional: specify a database (omit to access all databases)
$env:MSSQL_DATABASE = "YourDatabase"       # Database name (optional)

# Then run:
go run claude-code/db-connector.go test
```

**Note**: When using Windows Auth, `MSSQL_DATABASE` is **optional**. If not specified, you can access all databases that your Windows user has permissions to.

### For Claude Desktop

Update your `claude_desktop_config.json`:

**Option 1: Access all databases (no database specified):**
```json
{
  "mcpServers": {
    "mssql-windows-auth-all-dbs": {
      "command": "C:\\path\\to\\mcp-go-mssql-windows-amd64.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

**Option 2: Access a specific database:**
```json
{
  "mcpServers": {
    "mssql-windows-auth": {
      "command": "C:\\path\\to\\mcp-go-mssql-windows-amd64.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

## Server Naming Convention

| Server Value | Type | Example |
|------------|------|---------|
| `.` | Local default instance | `.` |
| `HOSTNAME` | Remote server | `sql-prod.company.local` |
| `.\\INSTANCENAME` | Local named instance | `.\\SQLEXPRESS` |
| `HOSTNAME\\INSTANCENAME` | Remote named instance | `sql-server.company.local\\PROD` |

## How It Works

1. **Named Pipes Protocol**: Uses `\\.\pipe\sql\query` instead of TCP port 1433
2. **SSPI Authentication**: Automatically uses Windows user credentials
3. **No TCP Required**: Works even if TCP/IP is disabled in SQL Server
4. **Encryption**: Supports TLS encryption over Named Pipes

## Connection String Format

When `MSSQL_AUTH=integrated` or `MSSQL_AUTH=windows`:

**With database specified (default):**
```
server=SERVER;database=DATABASE;encrypt=true|false;trustservercertificate=true|false;integrated security=SSPI;connection timeout=30;command timeout=30
```

**Without database specified (access all databases):**
```
server=SERVER;encrypt=true|false;trustservercertificate=true|false;integrated security=SSPI;connection timeout=30;command timeout=30
```

**Note**: No `user id` or `password` parameters are used with Windows Auth.

## Accessing Multiple Databases

When `MSSQL_DATABASE` is not specified with Windows Auth, you can:

1. **List all databases:**
   ```bash
   go run claude-code/db-connector.go query "SELECT name FROM sys.databases ORDER BY name"
   ```

2. **Query across databases:**
   ```bash
   go run claude-code/db-connector.go query "SELECT * FROM DatabaseName.schema.TableName"
   ```

3. **Switch databases in operations:**
   ```bash
   go run claude-code/db-connector.go query "USE DatabaseName; SELECT * FROM TableName"
   ```

This is particularly useful when you have permissions to multiple databases and want a single connection to access all of them.

## Testing the Connection

### Using the db-connector tool:

**Test with specific database:**
```bash
$env:MSSQL_SERVER = "."
$env:MSSQL_DATABASE = "JJP_CRM_LOCAL"
$env:MSSQL_AUTH = "integrated"
$env:DEVELOPER_MODE = "true"

go run claude-code/db-connector.go test
```

**Test with access to all databases:**
```bash
$env:MSSQL_SERVER = "."
$env:MSSQL_AUTH = "integrated"
$env:DEVELOPER_MODE = "true"
# Note: MSSQL_DATABASE is not set

go run claude-code/db-connector.go test
go run claude-code/db-connector.go query "SELECT name FROM sys.databases"
```

### Using sqlcmd (for verification):

```bash
sqlcmd -S . -d JJP_CRM_LOCAL -E -Q "SELECT @@VERSION"
```

The `-E` flag enables Windows Authentication.

## Security Considerations

✅ **Advantages:**
- No passwords stored in configuration files
- Uses Windows credentials already cached by the OS
- Works seamlessly in domain environments
- Supports Windows credential delegation

⚠️ **Important:**
- The application must run under a Windows user with SQL Server login
- Named Pipes requires Windows Server infrastructure
- Cannot be used in Linux/macOS environments

## Troubleshooting

### "Connection denied" or "Named Pipes provider error"

1. Verify SQL Server is running:
   ```bash
   Get-Service MSSQLSERVER | Select Status
   ```

2. Verify user has SQL Server login:
   ```bash
   sqlcmd -S . -E -Q "SELECT SYSTEM_USER"
   ```

3. Check Named Pipes is enabled in SQL Server Configuration Manager:
   - Open SQL Server Configuration Manager
   - SQL Server Network Configuration → Protocols for MSSQLSERVER
   - Named Pipes should be "Enabled"

### "integrated security=SSPI" not working

- Ensure you're on Windows
- Check that `MSSQL_AUTH=integrated` or `MSSQL_AUTH=windows` is set
- Verify database permissions with `sqlcmd -S . -E -Q "USE database; SELECT 1"`

## Fallback to SQL Authentication

If Windows Auth is not available, you can still use SQL Server authentication:

```json
{
  "env": {
    "MSSQL_SERVER": ".",
    "MSSQL_DATABASE": "YourDatabase",
    "MSSQL_USER": "sqluser",
    "MSSQL_PASSWORD": "password",
    "DEVELOPER_MODE": "true"
  }
}
```

## Advanced: Custom Connection String

For more control, use `MSSQL_CONNECTION_STRING`:

```bash
$env:MSSQL_CONNECTION_STRING = "server=.;database=JJP_CRM_LOCAL;integrated security=SSPI;encrypt=true;trustservercertificate=true"
```

This bypasses the environment variable parsing and uses your exact connection string.

## References

- [go-mssqldb Documentation](https://github.com/microsoft/go-mssqldb)
- [SQL Server Named Pipes](https://docs.microsoft.com/en-us/sql/tools/configuration-manager/protocols-for-mssqlserver)
- [Windows Integrated Authentication](https://docs.microsoft.com/en-us/sql/relational-databases/security/choose-an-authentication-mode)
