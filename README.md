# MCP-Go-MSSQL

A secure Go-based solution for Microsoft SQL Server connectivity supporting both **Claude Desktop** (via MCP server) and **Claude Code** (via CLI tools).

## Features

- **Security-first design** with TLS encryption for database connections
- **SQL injection protection** using prepared statements
- **Connection timeouts** and resource limits
- **Configurable security parameters** for production and development
- **Secure logging** with sensitive data sanitization

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

3. **Run**
   ```bash
   # Development mode (detailed errors)
   go run main.go

   # Production build
   go build -ldflags "-w -s" -o mcp-go-mssql-secure
   ./mcp-go-mssql-secure
   ```

## Configuration

### Claude Desktop Configuration
```json
{
  "mcpServers": {
    "production-db": {
      "command": "mcp-go-mssql.exe",
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

### Environment Variables

All database connections use environment variables for security. See `.env.example` for complete configuration examples.

**Required Variables:**
- `MSSQL_SERVER`: SQL Server hostname or IP address
- `MSSQL_DATABASE`: Database name to connect to
- `MSSQL_USER`: Username for SQL Server authentication
- `MSSQL_PASSWORD`: Password for SQL Server authentication

**Optional Variables:**
- `MSSQL_PORT`: SQL Server port (default: 1433)
- `DEVELOPER_MODE`: 
  - `"true"`: Development mode (detailed errors, allows self-signed certificates)
  - `"false"`: Production mode (generic errors, strict certificate validation)

**Environment Setup Examples:**
```bash
# Azure SQL Database
MSSQL_SERVER=your-server.database.windows.net
MSSQL_DATABASE=YourAzureDB
MSSQL_USER=your_user@your-server
DEVELOPER_MODE=false

# Local Development
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
DEVELOPER_MODE=true
```

## Security Features

- **Forced TLS encryption** for all database connections
- **Flexible certificate validation**:
  - Production: Strict certificate validation
  - Development: Allows self-signed certificates
- **Prepared statements** to prevent SQL injection
- **Secure error handling** with production/development modes
- **Connection pooling** with resource limits
- **Input validation** and sanitization

## Requirements

- Go 1.24+
- Microsoft SQL Server with TLS support
- Network access to SQL Server (port 1433)

## Troubleshooting

### TLS Certificate Issues
```
Error: "certificate signed by unknown authority"
Solution: Set DEVELOPER_MODE=true for self-signed certificates
```

### Connection Test
```bash
# Make sure environment variables are set first
cd test
go run test-connection.go
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
├── claude-code/               # Claude Code integration
│   ├── db-connector.go       # CLI database tool
│   └── README.md             # Claude Code documentation
├── test/
│   └── test-connection.go    # Connection testing utility
├── .env.example              # Environment variables template
├── config.example.json       # Claude Desktop configuration template
├── CLAUDE.md                 # Claude Code project documentation
└── README.md                 # This file
```

## License

This project is designed for secure database connectivity in critical environments.