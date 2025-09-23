# MCP-Go-MSSQL

A secure Go-based solution for Microsoft SQL Server connectivity supporting both **Claude Desktop** (via MCP server) and **Claude Code** (via CLI tools).

## Features

- **Security-first design** with configurable TLS encryption for database connections
- **SQL injection protection** using prepared statements exclusively
- **Connection timeouts** and resource limits with pooling
- **Flexible connection support** for modern and legacy SQL Server versions
- **Custom connection strings** for special configurations (SQL Server 2008+)
- **Configurable security parameters** for production and development environments
- **Secure logging** with automatic sensitive data sanitization

## Quick Start

### 1. **Setup Dependencies**
   ```bash
   go mod tidy
   ```

### 2. **Configure Database Connection**
   
   **Option A: Environment Variables (Recommended)**
   ```bash
   # Copy the example environment file
   cp .env.example .env
   
   # Edit .env with your database credentials
   # Then load the environment variables:
   source .env  # Linux/Mac
   # or for Windows PowerShell:
   # Get-Content .env | ForEach-Object { $name, $value = $_ -split '=', 2; [Environment]::SetEnvironmentVariable($name, $value) }
   ```
   
   **Option B: Direct Export (Linux/Mac)**
   ```bash
   export MSSQL_SERVER="your-server.database.windows.net"
   export MSSQL_DATABASE="YourDatabase"
   export MSSQL_USER="your_user"
   export MSSQL_PASSWORD="your_password"
   export DEVELOPER_MODE="false"
   ```
   
   **Option C: Claude Desktop Integration**
   ```bash
   # Use config.example.json as template for Claude Desktop
   cp config.example.json config.json
   # Edit config.json with your database credentials
   ```

3. **Build and Run**
   ```bash
   # Quick build (Windows)
   build.bat

   # Manual build
   go build -o mcp-go-mssql.exe

   # Development mode (detailed errors)
   go run main.go

   # Production build (optimized)
   go build -ldflags "-w -s" -o mcp-go-mssql-secure.exe
   ```

## Configuration

### Claude Desktop Configuration Examples

**Modern SQL Server (Standard Configuration):**
```json
{
  "mcpServers": {
    "production-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "user",
        "MSSQL_PASSWORD": "password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

**Legacy SQL Server (Custom Connection String):**
```json
{
  "mcpServers": {
    "legacy-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_CONNECTION_STRING": "sqlserver://sa:password@SERVER-GDP:1433?database=GDPA&encrypt=disable&trustservercertificate=true",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

### Environment Variables

All database connections use environment variables for security. See `.env.example` for complete configuration examples.

**Required Variables (when not using custom connection string):**
- `MSSQL_SERVER`: SQL Server hostname or IP address
- `MSSQL_DATABASE`: Database name to connect to
- `MSSQL_USER`: Username for SQL Server authentication
- `MSSQL_PASSWORD`: Password for SQL Server authentication

**Optional Variables:**
- `MSSQL_PORT`: SQL Server port (default: 1433)
- `MSSQL_ENCRYPT`: Override encryption setting (`"true"` or `"false"`)
- `MSSQL_CONNECTION_STRING`: **Complete custom connection string** (overrides all other MSSQL_* settings)
- `MSSQL_READ_ONLY`: **Security restriction** (`"true"` allows only SELECT queries, `"false"` allows all operations)
- `DEVELOPER_MODE`:
  - `"true"`: Development mode (detailed errors, allows self-signed certificates, disables encryption by default)
  - `"false"`: Production mode (generic errors, strict certificate validation, forces encryption)

**🔧 Custom Connection String Priority:**
When `MSSQL_CONNECTION_STRING` is set, all other `MSSQL_*` variables are ignored except `DEVELOPER_MODE`.

**Environment Setup Examples:**
```bash
# Azure SQL Database (Production)
MSSQL_SERVER=your-server.database.windows.net
MSSQL_DATABASE=YourAzureDB
MSSQL_USER=your_user@your-server
MSSQL_PASSWORD=your_secure_password
DEVELOPER_MODE=false

# Local Development (No Encryption)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
DEVELOPER_MODE=true

# Local Development (Force Encryption)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
MSSQL_ENCRYPT=true
DEVELOPER_MODE=true

# Legacy SQL Server (e.g., SQL Server 2008) - Custom Connection String
MSSQL_CONNECTION_STRING=sqlserver://sa:password@SERVER-GDP:1433?database=GDPA&encrypt=disable&trustservercertificate=true
DEVELOPER_MODE=true

# Read-Only Mode (Security Restricted)
MSSQL_SERVER=server.example.com
MSSQL_DATABASE=MyDatabase
MSSQL_USER=readonly_user
MSSQL_PASSWORD=readonly_password
MSSQL_READ_ONLY=true
MSSQL_MAX_QUERY_SIZE=2097152
DEVELOPER_MODE=false
```

## Security Features

- **Configurable TLS encryption** for database connections:
  - Production: Forces encryption (`encrypt=true`)
  - Development: Allows disabling encryption for local SQL Server instances
- **Flexible certificate validation**:
  - Production: Strict certificate validation (`trustservercertificate=false`)
  - Development: Allows self-signed certificates (`trustservercertificate=true`)
- **Prepared statements** to prevent SQL injection
- **Secure error handling** with production/development modes
- **Connection pooling** with resource limits
- **Input validation** and sanitization

## Requirements

- Go 1.24+
- Microsoft SQL Server with TLS support
- Network access to SQL Server (port 1433)

## Troubleshooting

### Connection Issues

**TLS Certificate Issues:**
```
Error: "certificate signed by unknown authority"
Solution: Set DEVELOPER_MODE=true for self-signed certificates
```

**Encryption Issues:**
```
Error: "SSL Provider: No credentials are available in the security package"
Solution: Set DEVELOPER_MODE=true to disable encryption for local SQL Server
```

**Force No Encryption (Development):**
```bash
# For local SQL Server without TLS
DEVELOPER_MODE=true
# This automatically sets encrypt=false for development
```

**TLS Handshake Issues (Legacy SQL Server):**
```
Error: "TLS Handshake failed: tls: server selected unsupported protocol version"
Solution: Use custom connection string with URL format for SQL Server 2008/2012
```

**Connection String Formats:**

**Standard Format (Modern SQL Server 2014+):**
```bash
# Automatically used when individual variables are set
MSSQL_SERVER=server.example.com
MSSQL_DATABASE=MyDatabase
MSSQL_USER=username
MSSQL_PASSWORD=password
DEVELOPER_MODE=true
```

**URL Format (Legacy SQL Server 2008-2012):**
```bash
# Use this for older SQL Server versions
MSSQL_CONNECTION_STRING=sqlserver://username:password@server:1433?database=dbname&encrypt=disable&trustservercertificate=true
DEVELOPER_MODE=true
```

**No Encryption (Development):**
```bash
# For local SQL Server without TLS
DEVELOPER_MODE=true
# This automatically sets encrypt=false for development
```

### Connection Test
```bash
# Make sure environment variables are set first
cd test
go run test-connection.go

# For debugging connection issues
cd debug
go run debug-connection.go
```

### Security Notes
- ⚠️ **Never commit `.env` or `config.json` files** with real credentials
- ✅ **Always use environment variables** for sensitive data
- 🔒 **Use strong passwords** and enable TLS encryption
- 🏢 **For production**: Set `DEVELOPER_MODE=false` and use valid certificates

### Usage Options

### 🖥️ Claude Desktop (MCP Server)
Use `main.go` as an MCP server with Claude Desktop:

**Available Tools:**
- `get_database_info`: Check connection status
- `query_database`: Execute SQL queries securely

### 💻 Claude Code (CLI Tool)  
Use `claude-code/db-connector.go` directly with Claude Code:

```bash
cd claude-code
go run db-connector.go test                    # Test connection
go run db-connector.go tables                  # List tables
go run db-connector.go query "SELECT ..."      # Execute queries
```

See [claude-code/README.md](claude-code/README.md) for detailed Claude Code integration.

## Project Structure

```
mcp-go-mssql/
├── main.go                    # MCP server for Claude Desktop
├── build.bat                  # Windows build script
├── claude-code/               # Claude Code integration
│   ├── db-connector.go       # CLI database tool
│   └── README.md             # Claude Code documentation
├── debug/
│   └── debug-connection.go   # Advanced connection debugging tool
├── test/
│   └── test-connection.go    # Basic connection testing utility
├── .env.example              # Environment variables template
├── config.example.json       # Claude Desktop configuration template
├── CLAUDE.md                 # Claude Code project documentation
└── README.md                 # This file
```

## License

This project is designed for secure database connectivity in critical environments.