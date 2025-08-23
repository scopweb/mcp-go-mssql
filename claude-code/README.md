# Claude Code Database Integration

This directory contains tools specifically designed for **Claude Code** to interact with Microsoft SQL Server databases securely.

## 🎯 Purpose

While the main MCP server (`../main.go`) is designed for Claude Desktop, this `db-connector.go` script provides Claude Code with direct database access capabilities.

## ⚡ Quick Start

### 1. Set Environment Variables
```bash
export MSSQL_SERVER="your-server.database.windows.net"
export MSSQL_DATABASE="YourDatabase"
export MSSQL_USER="your_user"
export MSSQL_PASSWORD="your_password"
export MSSQL_PORT="1433"                    # Optional, defaults to 1433
export DEVELOPER_MODE="false"               # "true" for dev, "false" for prod
```

### 2. Test Connection
```bash
cd claude-code
go run db-connector.go test
```

## 🔧 Available Commands

### Connection Testing
```bash
go run db-connector.go test                    # Test connection + show server info
go run db-connector.go info                    # Show detailed database info
```

### Schema Exploration
```bash
go run db-connector.go tables                  # List all tables and views
go run db-connector.go describe users          # Show table structure
go run db-connector.go describe orders         # Show specific table columns
```

### Query Execution
```bash
# SELECT queries (returns JSON data)
go run db-connector.go query "SELECT TOP 10 * FROM users"
go run db-connector.go query "SELECT COUNT(*) as total FROM orders"

# UPDATE/INSERT/DELETE queries (returns affected rows)
go run db-connector.go query "UPDATE users SET status = 'active' WHERE id = 1"
go run db-connector.go query "INSERT INTO logs (message, created_at) VALUES ('Test', GETDATE())"
```

## 📋 Output Format

All commands return structured JSON:

### Success Response
```json
{
  "success": true,
  "data": [
    {"column1": "value1", "column2": "value2"}
  ],
  "info": "Query executed successfully. Rows returned: 1"
}
```

### Error Response
```json
{
  "success": false,
  "error": "Connection failed: login error"
}
```

## 🔒 Security Features

- **Environment Variables**: No hardcoded credentials
- **TLS Encryption**: All database connections use TLS
- **Prepared Statements**: SQL injection protection for parameterized queries
- **Connection Pooling**: Resource management with limits
- **Certificate Validation**: 
  - Production: Strict certificate validation
  - Development: Allows self-signed certificates with `DEVELOPER_MODE=true`

## 🎨 Claude Code Usage Examples

### Tell Claude Code to:

```
"Set up environment variables for the database connection and test it"
"List all tables in the database using the db-connector"
"Show me the structure of the users table"
"Execute a query to get the top 10 recent orders"
"Check the database connection status"
```

### Claude Code will execute:
```bash
go run claude-code/db-connector.go test
go run claude-code/db-connector.go tables  
go run claude-code/db-connector.go describe users
go run claude-code/db-connector.go query "SELECT TOP 10 * FROM orders ORDER BY created_at DESC"
```

## 🔍 Troubleshooting

### Connection Issues
```bash
# Check if environment variables are set
echo $MSSQL_SERVER $MSSQL_DATABASE $MSSQL_USER

# Test basic connectivity
go run db-connector.go test
```

### TLS Certificate Problems
```bash
# For development with self-signed certificates
export DEVELOPER_MODE="true"
go run db-connector.go test

# For production (strict certificate validation)
export DEVELOPER_MODE="false"
go run db-connector.go test
```

### Common Errors

| Error | Solution |
|-------|----------|
| `missing required environment variables` | Set all required MSSQL_* variables |
| `certificate signed by unknown authority` | Set `DEVELOPER_MODE=true` for dev environments |
| `login failed` | Check username/password and SQL Server authentication |
| `network error` | Check firewall rules and server accessibility |

## 🔄 Integration with Main MCP Server

This tool complements the main MCP server:

- **Claude Desktop** → Uses `../main.go` (MCP Server)
- **Claude Code** → Uses `./db-connector.go` (Direct CLI tool)
- **Other IDEs** → Can use either approach

## 📦 Dependencies

Same as main project:
- `github.com/denisenkom/go-mssqldb` - SQL Server driver
- Go 1.20+ with standard library

## 🚀 Advanced Usage

### Custom Queries with Parameters
For complex queries, modify the script to accept parameters:
```bash
go run db-connector.go query "SELECT * FROM users WHERE status = 'active' AND created_at > DATEADD(day, -7, GETDATE())"
```

### Batch Operations
Chain multiple commands:
```bash
go run db-connector.go tables > tables.json
go run db-connector.go query "SELECT COUNT(*) FROM users" > user_count.json
```

This tool gives Claude Code the same secure database access capabilities as the MCP server, but in a format that Claude Code can use directly.