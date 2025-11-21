```markdown
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
cd pkg/connector
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
```

## 🔒 Security Features

- **Environment Variables**: No hardcoded credentials
- **TLS Encryption**: All database connections use TLS
- **Prepared Statements**: SQL injection protection for parameterized queries

``` 
