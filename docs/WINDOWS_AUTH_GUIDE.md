# Windows Integrated Authentication (SSPI) Guide

This guide explains how to use Windows Integrated Authentication (also known as Windows Authentication or SSPI) with the MCP-Go-MSSQL server.

## What is Windows Integrated Authentication?

Windows Integrated Authentication allows SQL Server to use Windows credentials (Active Directory or local Windows accounts) to authenticate users. This means:

- ✅ **No passwords in configuration files** - More secure credential management
- ✅ **Single Sign-On (SSO)** - Users authenticate once with Windows
- ✅ **Active Directory integration** - Centralized user management
- ✅ **Simplified security** - Leverage existing Windows/AD security groups
- ✅ **Audit trail** - SQL Server logs the actual Windows user, not a shared account

## Requirements

1. **Windows Operating System** - SSPI is only supported on Windows
2. **SQL Server configured for Windows Authentication** - The SQL Server instance must allow Windows Authentication mode
3. **User permissions** - The Windows user running the MCP server must have permissions in SQL Server
4. **Network connectivity** - For remote servers, the Windows user must be able to authenticate via the network (typically requires Active Directory)

## Configuration

### Basic Configuration

**Minimal configuration (connects to default database):**
```json
{
  "mcpServers": {
    "local-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

**With specific database:**
```json
{
  "mcpServers": {
    "local-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_DATABASE": "MyDatabase",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

### Environment Variables

**Required:**
- `MSSQL_SERVER` - Server name (see Server Name Options below)
- `MSSQL_AUTH` - Set to `"integrated"` or `"windows"`

**Optional:**
- `MSSQL_DATABASE` - Database name. If omitted, connects to the Windows user's default database
- `DEVELOPER_MODE` - Set to `"true"` for development (disables encryption by default)

**Not needed with integrated auth:**
- `MSSQL_USER` - ❌ Not used
- `MSSQL_PASSWORD` - ❌ Not used
- `MSSQL_PORT` - ❌ Not needed (SSPI can use Named Pipes)

### Server Name Options

**For local SQL Server:**
- `"localhost"` - Standard localhost
- `"."` - Shorthand for local server (recommended)
- `"(local)"` - Alternative local server notation
- `".\\SQLEXPRESS"` - Local SQL Server Express instance
- `"localhost\\INSTANCENAME"` - Specific named instance

**For remote SQL Server:**
- `"SERVERNAME"` - Server hostname
- `"SERVER.domain.local"` - Fully qualified domain name
- `"SERVERNAME\\INSTANCE"` - Named instance on remote server
- `"10.0.0.100"` - IP address (requires proper DNS/AD setup)

## Setup Steps

### Step 1: Verify SQL Server Authentication Mode

SQL Server must be configured to allow Windows Authentication:

1. Open **SQL Server Management Studio (SSMS)**
2. Connect to your SQL Server instance
3. Right-click the server → **Properties**
4. Go to **Security** page
5. Ensure **Server authentication** is set to:
   - "Windows Authentication mode" or
   - "SQL Server and Windows Authentication mode" (mixed mode)

### Step 2: Grant Windows User Permissions

The Windows user running the MCP server needs SQL Server permissions:

**Option A: Using SSMS GUI**
1. Expand **Security** → **Logins**
2. Right-click **Logins** → **New Login**
3. Click **Search** and find your Windows user (format: `DOMAIN\Username` or `COMPUTERNAME\Username`)
4. Grant appropriate database access and roles

**Option B: Using T-SQL**
```sql
-- Add Windows user as SQL Server login
CREATE LOGIN [DOMAIN\Username] FROM WINDOWS;

-- Grant access to specific database
USE YourDatabase;
CREATE USER [DOMAIN\Username] FOR LOGIN [DOMAIN\Username];

-- Grant roles (example: read-write access)
ALTER ROLE db_datareader ADD MEMBER [DOMAIN\Username];
ALTER ROLE db_datawriter ADD MEMBER [DOMAIN\Username];
```

### Step 3: Test Connection

Use the provided diagnostic script:

```powershell
cd C:\MCPs\clone\mcp-go-mssql
.\scripts\test-integrated-auth.ps1
```

Or test manually:

```powershell
# Set environment variables
$env:MSSQL_SERVER = "localhost"
$env:MSSQL_DATABASE = "master"
$env:MSSQL_AUTH = "integrated"
$env:DEVELOPER_MODE = "true"

# Run test tool
cd tools\test
go run test-connection.go
```

## Troubleshooting

### Error: "Login failed for user 'NT AUTHORITY\ANONYMOUS LOGON'"

**Cause:** SQL Server received an anonymous login instead of your Windows credentials.

**Solutions:**
1. Ensure you're using a domain account or local Windows account that exists in SQL Server
2. Check if SQL Server is running under the correct service account
3. For remote servers, verify the service principal name (SPN) is registered correctly

### Error: "Cannot open database requested by the login"

**Cause:** The Windows user doesn't have permission to access the specified database.

**Solutions:**
1. Omit `MSSQL_DATABASE` to connect to the default database
2. Grant the Windows user access to the target database (see Step 2 above)
3. Check if the database exists: `SELECT name FROM sys.databases`

### Error: "A network-related or instance-specific error occurred"

**Cause:** Cannot connect to the SQL Server instance.

**Solutions:**
1. Verify SQL Server is running: `Get-Service -Name 'MSSQL*'`
2. Check server name is correct (try `"."` for local server)
3. For named instances, include the instance name: `"localhost\\SQLEXPRESS"`
4. Verify Windows Firewall isn't blocking the connection
5. Check SQL Server Configuration Manager:
   - Ensure TCP/IP or Named Pipes protocols are enabled
   - Check that SQL Server Browser service is running (for named instances)

### Error: "Login failed for user 'DOMAIN\Username'"

**Cause:** Windows user doesn't have login permissions in SQL Server.

**Solution:**
Add the Windows user to SQL Server logins (see Step 2 above)

### Viewing Diagnostic Logs

Check the logs for detailed error information:

```powershell
# View Claude Desktop logs in real-time
.\scripts\view-logs.ps1

# Or manually check:
Get-Content "$env:APPDATA\Claude\logs\mcp*.log" -Wait -Tail 50
```

The enhanced logs will show:
- Current Windows user running the process
- Authentication mode being used
- Specific troubleshooting tips for your error

## Security Best Practices

### ✅ Recommended

1. **Use Windows Authentication in production** - More secure than storing passwords
2. **Use least-privilege accounts** - Grant only necessary database permissions
3. **Enable `MSSQL_READ_ONLY=true`** - For AI assistants accessing production data
4. **Use `MSSQL_WHITELIST_TABLES`** - Limit which tables can be modified
5. **Audit access** - SQL Server logs show the actual Windows user making changes

### ⚠️ Best Practices

1. **Avoid running as Administrator** - Use a dedicated service account
2. **Use domain accounts for servers** - Better security and manageability
3. **Document required permissions** - Make it easy to grant access to new users
4. **Test with minimal permissions** - Verify the MCP server works with least-privilege

## Examples

### Example 1: Local SQL Server Express (Development)

```json
{
  "mcpServers": {
    "local-sqlexpress": {
      "command": "C:\\MCPs\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".\\SQLEXPRESS",
        "MSSQL_DATABASE": "TestDB",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

### Example 2: Remote Domain Server (Production)

```json
{
  "mcpServers": {
    "prod-sql": {
      "command": "C:\\MCPs\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "SQL-PROD.company.local",
        "MSSQL_DATABASE": "ProductionDB",
        "MSSQL_AUTH": "integrated",
        "MSSQL_READ_ONLY": "true",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

### Example 3: AI-Safe with Read-Only + Whitelist

```json
{
  "mcpServers": {
    "ai-safe-db": {
      "command": "C:\\MCPs\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".",
        "MSSQL_DATABASE": "AnalyticsDB",
        "MSSQL_AUTH": "integrated",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai_workspace,staging_data",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

### Example 4: Multiple Databases with Same Credentials

```json
{
  "mcpServers": {
    "db-development": {
      "command": "C:\\MCPs\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_DATABASE": "DevDB",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true"
      }
    },
    "db-testing": {
      "command": "C:\\MCPs\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_DATABASE": "TestDB",
        "MSSQL_AUTH": "integrated",
        "MSSQL_READ_ONLY": "true",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

## Additional Resources

- [SQL Server Windows Authentication Documentation](https://learn.microsoft.com/en-us/sql/relational-databases/security/choose-an-authentication-mode)
- [Configuring Windows Service Accounts and Permissions](https://learn.microsoft.com/en-us/sql/database-engine/configure-windows/configure-windows-service-accounts-and-permissions)
- [MCP-Go-MSSQL Main README](../README.md)
- [Security Features and Whitelist Guide](WHITELIST_SECURITY.md)
- [AI Usage Guide](AI_USAGE_GUIDE.md)

---

**Need help?** Check the diagnostic logs with `.\scripts\view-logs.ps1` or run `.\scripts\test-integrated-auth.ps1` for detailed troubleshooting information.
