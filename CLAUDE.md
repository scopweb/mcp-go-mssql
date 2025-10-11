# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **secure** Go-based solution that provides MSSQL database connectivity for critical data environments in two ways:

1. **MCP Server** (`main.go`): For Claude Desktop integration via Model Context Protocol
2. **CLI Tool** (`claude-code/db-connector.go`): For Claude Code direct database access

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
1. Use `config.example.json` as template for Claude Desktop configuration
2. **NEVER commit sensitive credentials to version control**
3. Place the compiled `mcp-go-mssql.exe` in the same directory
4. Configuration uses environment variables passed through Claude Desktop MCP settings

### Environment Variables
The server reads database connection from these environment variables. See `.env.example` for complete configuration templates.

**Required Variables:**
- `MSSQL_SERVER`: SQL Server hostname/IP address
- `MSSQL_DATABASE`: Database name to connect to
- `MSSQL_USER`: Username for SQL Server authentication
- `MSSQL_PASSWORD`: Password for SQL Server authentication

**Optional Variables:**
- `MSSQL_PORT`: SQL Server port (defaults to 1433)
- `DEVELOPER_MODE`: Controls error verbosity and certificate validation
  - `"true"`: Development mode with detailed errors and relaxed TLS certificate validation
  - `"false"`: Production mode with generic errors and strict certificate validation
- `MSSQL_READ_ONLY`: Enable read-only mode with optional whitelist (security feature)
  - `"true"`: Only SELECT queries allowed (except for whitelisted tables)
  - `"false"`: Full access mode (default)
- `MSSQL_WHITELIST_TABLES`: Comma-separated list of tables/views allowed for modification in read-only mode
  - Example: `"temp_ai,v_temp_ia"`
  - Only these tables can be modified (INSERT/UPDATE/DELETE/CREATE/DROP) when `MSSQL_READ_ONLY=true`
  - All other tables remain read-only
  - Prevents accidental data modification in production databases while allowing AI to work with temporary tables

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
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia"
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
   - **Claude Code operations**: `go run claude-code/db-connector.go [command]`

### Available Claude Code Database Commands
- `go run claude-code/db-connector.go test` - Test connection
- `go run claude-code/db-connector.go info` - Database information  
- `go run claude-code/db-connector.go tables` - List all tables
- `go run claude-code/db-connector.go describe TABLE_NAME` - Table structure
- `go run claude-code/db-connector.go query "SQL_STATEMENT"` - Execute queries