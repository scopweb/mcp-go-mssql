# MCP-Go-MSSQL

A secure Go-based solution for Microsoft SQL Server connectivity supporting both **Claude Desktop** (via MCP server) and **Claude Code** (via CLI tools).

## Features

- **Security-first design** with TLS encryption for database connections
- **SQL injection protection** using prepared statements
- **Connection timeouts** and resource limits
- **Configurable security parameters** for production and development
- **Secure logging** with sensitive data sanitization

## Quick Start

1. **Setup**
   ```bash
   go mod tidy
   ```

2. **Configuration**
   ```bash
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
- `MSSQL_SERVER`: SQL Server hostname
- `MSSQL_DATABASE`: Database name  
- `MSSQL_USER`: Database username
- `MSSQL_PASSWORD`: Database password
- `MSSQL_PORT`: Port (default: 1433)
- `DEVELOPER_MODE`: 
  - `"true"`: Detailed errors + allows self-signed TLS certificates
  - `"false"`: Production mode with strict certificate validation

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
cd test
go run test-connection.go
```

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
├── config.example.json       # Claude Desktop configuration template
└── README.md                 # This file
```

## License

This project is designed for secure database connectivity in critical environments.