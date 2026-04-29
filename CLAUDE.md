# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **secure** Go-based solution that provides MSSQL database connectivity for critical data environments in two ways:

1. **MCP Server** (`main.go`): For Claude Desktop integration via Model Context Protocol
2. **CLI Tool** (`cli/db-connector.go`): For direct database access via command line

Both implementations serve as hardened bridges to Microsoft SQL Server databases, with comprehensive security features including TLS encryption, input validation, and connection pooling.

## Architecture

The codebase implements a security-first architecture with these key components:

- **SecurityLogger**: Dedicated security event logging with sanitization
- **MCPServer struct**: Handles secure server instances with input validation and SQL injection protection
- **Connection security**: TLS support and connection timeouts
- **Database security**: Encrypted connections, connection pooling, and prepared statement support

## Security Features

### Database Security
- **Mandatory encryption**: Database connections FORCE TLS (encrypt=true, trustservercertificate=false)
- **Connection pooling**: Limited connection counts to prevent resource exhaustion
- **SQL Injection Protection**: Uses prepared statements exclusively - NO dynamic SQL
- **Secure error handling**: Generic error messages to clients, detailed logs internally
- **Destructive operation confirmation**: DDL operations (ALTER VIEW, DROP TABLE, etc.) on existing objects require explicit user confirmation via `confirm_operation` tool. Tokens expire after 5 minutes. Controlled by `MSSQL_CONFIRM_DESTRUCTIVE` env var (default: true)

### Network Security
- **Database TLS encryption**: Mandatory TLS for all database connections
- **Connection timeouts**: Prevent hanging connections
- **Resource limits**: Connection pooling with limits

### Logging Security
- **Security event logging**: Dedicated security logger for all security events
- **Data sanitization**: Automatic removal of sensitive data from logs
- **Connection tracking**: Log all database connection attempts

## Development Commands

### Initial Setup
```bash
go mod init mcp-go-mssql
go mod tidy
```

### Environment Configuration
```bash
# Copy environment template and configure
cp .env.example .env
# Edit .env with your database credentials

# Load environment variables (Linux/Mac)
source .env

# Windows PowerShell
Get-Content .env | ForEach-Object { 
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name, $value) 
}
```

### Build
```bash
go build
```

### Development Mode
```bash
# Development with detailed errors
# Set "developer_mode": true in config.json
go run main.go

# Production mode (default)
# Set "developer_mode": false in config.json  
go run main.go
```

### Production Deployment
```bash
# Build for production
go build -ldflags "-w -s" -o mcp-go-mssql-secure

# Run with production config
./mcp-go-mssql-secure
```

## Secure Configuration

### MCP Protocol Implementation
This server now implements the proper MCP (Model Context Protocol) using stdin/stdout JSON-RPC communication, compatible with Claude Desktop.

### For Claude Desktop Integration
1. Use `.mcp.json` as template for Claude Desktop configuration (see examples below)
2. **NEVER commit sensitive credentials to version control**
3. Place the compiled `mcp-go-mssql.exe` in the same directory
4. Configuration uses environment variables passed through Claude Desktop MCP settings
5. Alternatively, use `MSSQL_CONNECTION_STRING` for a complete connection string instead of individual env vars

### Environment Variables
The server reads database connection from these environment variables. See `.env.example` for complete configuration templates.

**Required Variables (choose one):**
- `MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_USER`, `MSSQL_PASSWORD`: Individual connection parameters
- `MSSQL_CONNECTION_STRING`: Complete SQL Server connection string (alternative to individual vars)

**Required for Windows Integrated Auth:**
- `MSSQL_SERVER`: SQL Server hostname/IP address
- `MSSQL_DATABASE`: Database name to connect to
- `MSSQL_AUTH`: Set to `"integrated"` or `"windows"` for Windows Authentication

**Optional Variables:**
- `MSSQL_PORT`: SQL Server port (defaults to 1433)
- `DEVELOPER_MODE`: Controls error verbosity and certificate validation
  - `"true"`: Development mode with detailed errors and relaxed TLS certificate validation
  - `"false"`: Production mode with generic errors and strict certificate validation
- `MSSQL_READ_ONLY`: Enable read-only mode with optional whitelist (security feature)
  - `"true"`: Only SELECT queries allowed (except for whitelisted tables)
  - `"false"`: Full access mode (default)
- `MSSQL_ENCRYPT`: Override TLS encryption setting (only effective when `DEVELOPER_MODE=true`)
  - `"false"`: Disable encryption — **required for SQL Server 2008/2012** which don't support TLS 1.2
  - `"true"`: Force encryption even in dev mode
  - If not set: defaults to `"false"` in dev mode, always `"true"` in production mode
- `MSSQL_WHITELIST_TABLES`: Comma-separated list of tables/views allowed for modification in read-only mode
  - Example: `"temp_ai,v_temp_ia"`
  - Only these tables can be modified (INSERT/UPDATE/DELETE/CREATE/DROP) when `MSSQL_READ_ONLY=true`
  - All other tables remain read-only
  - Prevents accidental data modification in production databases while allowing AI to work with temporary tables
- `MSSQL_ALLOWED_DATABASES`: Comma-separated list of additional databases this connector can query (cross-database)
  - Example: `"JJP_Carregues,JJP_Ferratge_PROD"`
  - Enables queries using 3-part names: `SELECT * FROM JJP_Carregues.dbo.TableName`
  - The SQL user must have permissions on those databases (same server, same credentials)
  - Schema validation checks tables exist in the target database before executing
  - Cross-database tables are **read-only** — modifications are blocked even if MSSQL_WHITELIST_TABLES is set
  - Use `explore` tool with `database` parameter to list tables in allowed databases
- `MSSQL_CONFIRM_DESTRUCTIVE`: Require explicit confirmation for destructive DDL operations (default: true)
  - `"true"`: DDL operations on existing objects (ALTER VIEW, DROP TABLE, etc.) require confirmation via `confirm_operation` tool
  - `"false"`: No confirmation required (for CI/CD automation)
- `MSSQL_AUTOPILOT`: Enable autonomous AI mode (default: false)
  - `"true"`: Skip schema validation — AI can run queries against tables that don't exist without being interrupted
  - Does NOT skip destructive confirmation: DDL on existing objects still requires `confirm_operation`
  - Does NOT skip whitelist protection: only whitelisted tables can be modified
  - Ideal for development with AI assistants that need flexibility around schema checks
  - Combines with `MSSQL_WHITELIST_TABLES` to delimit the AI's operational scope
- `MSSQL_SKIP_SCHEMA_VALIDATION`: Skip table existence validation (default: false)
  - `"true"`: Disables validation that tables referenced in queries actually exist
  - Independent flag — effective skip is `AUTOPILOT OR SKIP_SCHEMA_VALIDATION`
  - Useful when you want to disable schema checks without enabling other AUTOPILOT semantics (currently AUTOPILOT only governs schema validation, but this flag stays decoupled in case AUTOPILOT grows)
  - Does NOT skip whitelist protection or destructive operation confirmation
- `MSSQL_DYNAMIC_MODE`: Enable dynamic multi-database connections (default: false)
  - `"true"`: Enables runtime database connections via `dynamic_connect` tool
  - Allows connecting to multiple databases from a single MCP server instance
  - Tools: `dynamic_available`, `dynamic_connect`, `dynamic_list`, `dynamic_disconnect`
  - In `query_database`, use parameter `connection=<alias>` to query a specific dynamic connection
  - If not specified, queries use the default database connection from environment variables
  - `MSSQL_DYNAMIC_MAX_CONNECTIONS`: Maximum number of dynamic connections (default: 10)
  - **Dual-mode architecture**: If `MSSQL_SERVER` is set (from Claude Desktop JSON config), the server works in direct mode without loading `.env` or registering dynamic tools. If `MSSQL_SERVER` is not set, it loads `.env` and enables dynamic tools if `MSSQL_DYNAMIC_MODE=true`

**Configuration Examples:**
```bash
# Production Azure SQL with AI Whitelist (RECOMMENDED for AI assistants)
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=ProductionDB
MSSQL_USER=prod_user@prod-server
MSSQL_PASSWORD=your_password
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
# This allows AI to read all data but only modify temp_ai and v_temp_ia

# Production with Full Read-Only (No modifications)
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=ProductionDB
MSSQL_USER=prod_user@prod-server
MSSQL_PASSWORD=your_password
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
# No MSSQL_WHITELIST_TABLES means ALL modifications are blocked

# Local Development (Full Access)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false
# Full access for development

# Legacy SQL Server 2008/2012 with Windows Integrated Auth
MSSQL_SERVER=legacy-server
MSSQL_DATABASE=LegacyDB
MSSQL_AUTH=integrated
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
# No user/password needed — uses Windows credentials
# MSSQL_ENCRYPT=false is required because SQL 2008 doesn't support TLS 1.2

# Using Connection String (alternative to individual vars)
MSSQL_CONNECTION_STRING=Data Source=myServer;Database=myDB;Integrated Security=True;Encrypt=False;TrustServerCertificate=True
DEVELOPER_MODE=true
# Connection string includes server, database, and all connection options
# Useful for complex connection strings or integrated Windows authentication

# Cross-Database Access (query multiple databases from one connector)
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=JJP_Ferratge_DEV
MSSQL_USER=prod_user@prod-server
MSSQL_PASSWORD=your_password
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
MSSQL_ALLOWED_DATABASES=JJP_Carregues,JJP_Ferratge_PROD
# Primary DB is JJP_Ferratge_DEV — AI can also read from JJP_Carregues and JJP_Ferratge_PROD
# Cross-database queries: SELECT * FROM JJP_Carregues.dbo.TableName
# Modifications only allowed on whitelisted tables in the primary database

# Autonomous AI Development (AI works without interruptions within whitelist scope)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
DEVELOPER_MODE=true
MSSQL_AUTOPILOT=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia,mi_vista
# AI can modify/created/drop only whitelisted objects without confirmation
# Schema validation is skipped — AI can query non-existent tables without error
# Useful for development where AI needs full autonomy within a limited scope
```

### TLS Certificate Handling
- **Production Mode** (`DEVELOPER_MODE=false`): Requires valid, trusted TLS certificates
- **Development Mode** (`DEVELOPER_MODE=true`): Allows self-signed or untrusted certificates
- **Always Encrypted**: All database connections use TLS encryption regardless of mode

### Claude Desktop Configuration Example:
```json
{
  "mcpServers": {
    "production-db-ai-safe": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "your_user",
        "MSSQL_PASSWORD": "your_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia",
        "MSSQL_ALLOWED_DATABASES": "OtherDB1,OtherDB2"
      }
    },
    "production-db-readonly": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "readonly_user",
        "MSSQL_PASSWORD": "readonly_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true"
      }
    },
    "dev-db": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "dev-server.local",
        "MSSQL_DATABASE": "DevDatabase",
        "MSSQL_USER": "dev_user",
        "MSSQL_PASSWORD": "dev_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "true",
        "MSSQL_READ_ONLY": "false"
      }
    },
    "dev-db-autopilot": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "dev-server.local",
        "MSSQL_DATABASE": "DevDatabase",
        "MSSQL_USER": "dev_user",
        "MSSQL_PASSWORD": "dev_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "true",
        "MSSQL_AUTOPILOT": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia"
      }
    },
    "dev-db-fullaccess": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_CONNECTION_STRING": "Data Source=dev-server.local;Database=DevDatabase;User Id=dev_user;Password=dev_password;Encrypt=False;TrustServerCertificate=True",
        "DEVELOPER_MODE": "true",
        "MSSQL_AUTOPILOT": "true",
        "MSSQL_WHITELIST_TABLES": "*"
      }
    },
    "dev-windows-auth": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "dev-server.local",
        "MSSQL_DATABASE": "DevDatabase",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "true",
        "MSSQL_AUTOPILOT": "true",
        "MSSQL_WHITELIST_TABLES": "*"
      }
    },
    "dev-db-dynamic": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_DATABASE": "DevDB",
        "MSSQL_USER": "dev_user",
        "MSSQL_PASSWORD": "dev_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "true",
        "MSSQL_DYNAMIC_MODE": "true"
      }
    }
  }
}
```

### Critical Security Parameters:
- `DEVELOPER_MODE`:
  - `"false"` for production: Strict TLS certificate validation, generic error messages
  - `"true"` for development: Allows untrusted certificates, detailed error messages
- **Database Encryption**: Always uses TLS encryption (`encrypt=true`)
- **Certificate Validation**:
  - Production: `trustservercertificate=false` (requires valid certificates)
  - Development: `trustservercertificate=true` (allows self-signed certificates)
- `MSSQL_READ_ONLY` with `MSSQL_WHITELIST_TABLES`: Granular Permission Control
  - **How it works**: When enabled, the server validates ALL tables referenced in modification queries
  - **Example**: `DELETE temp_ai FROM temp_ai JOIN users` → BLOCKED (users not whitelisted)
  - **Protection against**:
    - Accidental data deletion in production tables
    - SQL injection via JOIN/subquery to non-whitelisted tables
    - Unauthorized data exfiltration through INSERT...SELECT
  - **Recommended setup**: Create dedicated temp tables for AI operations

## Security Best Practices

### Configuration Security
1. **Environment Variables**: Use environment variables for sensitive data (never hardcode credentials)
2. **File Permissions**: 
   - **`.env` files**: Set restrictive permissions (600 on Unix, equivalent on Windows)
   - **Windows**: `icacls .env /inheritance:r /grant:r "%USERNAME%:R"`
   - **Linux/Unix**: `chmod 600 .env`
3. **Git Security**: 
   - ✅ `.env` files are in `.gitignore` 
   - ❌ **NEVER** commit `.env` or `config.json` with real credentials
   - ✅ Only commit `.env.example` and `config.example.json` templates
4. **Credential Rotation**: Regularly rotate database passwords
5. **Network Isolation**: Deploy in secure network segments

### Database Security
1. **Least Privilege**: Use database users with minimal required permissions
2. **Connection Limits**: Set appropriate connection pool limits
3. **Audit Logging**: Enable database audit logs
4. **SSL/TLS**: Always use encrypted database connections

### Deployment Security
1. **Binary Security**: Use stripped binaries (`-ldflags "-w -s"`)
2. **Container Security**: Run in non-root containers when containerized
3. **Network Security**: Use firewalls and network segmentation
4. **Monitoring**: Implement security monitoring and alerting

## Dependencies

- `github.com/denisenkom/go-mssqldb`: Microsoft SQL Server driver with TLS support
- Go standard library: crypto/tls, context, regexp for security features

## IMPORTANT SECURITY NOTES

⚠️  **This application handles critical database data. Always:**
1. Use TLS for all connections (both client and database)
2. Implement proper firewall rules
3. Monitor security logs regularly
4. Keep dependencies updated
5. Test security configurations before production deployment
6. Use strong authentication credentials
7. Implement network segmentation

## Troubleshooting

### TLS Certificate Issues
If you see errors like "certificate signed by unknown authority":

1. **For Development**: Set `DEVELOPER_MODE=true` to allow self-signed certificates
2. **For Production**: 
   - Install proper SSL certificates on your SQL Server
   - Or configure your certificate authority in the client system
   - Never use `DEVELOPER_MODE=true` in production

### Connection Testing
Use the included test utility:
```bash
cd test
go run test-connection.go
```

### Common Issues
- **"Database not connected"**: Check if environment variables are set correctly
- **TLS handshake failed**: Use `DEVELOPER_MODE=true` for self-signed certificates
- **Login failed**: Verify username/password and SQL Server authentication mode
- **Network error**: Check firewall rules and SQL Server port (default 1433)

## Important Claude Code Instructions

### Environment Setup for Database Operations
When Claude Code needs to connect to the database:

1. **Check if environment variables are set:**
   ```bash
   echo "Server: $MSSQL_SERVER"
   echo "Database: $MSSQL_DATABASE" 
   echo "User: $MSSQL_USER"
   echo "Password: ${MSSQL_PASSWORD:+***SET***}"
   ```

2. **If not set, guide user to configure `.env`:**
   ```bash
   # Copy template and edit
   cp .env.example .env
   # User should edit .env with their credentials
   source .env  # Load variables
   ```

3. **Use the appropriate tool:**
   - **Connection testing**: `go run test/test-connection.go`
   - **Claude Desktop operations**: Use the MCP tools directly (query_database, explore, inspect, etc.)

### Available Claude Code Database Commands
- `go run cli/db-connector.go test` - Test connection
- `go run cli/db-connector.go info` - Database information  
- `go run cli/db-connector.go tables` - List all tables
- `go run cli/db-connector.go describe TABLE_NAME` - Table structure
- `go run cli/db-connector.go query "SQL_STATEMENT"` - Execute queries