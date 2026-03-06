---
title: CLI Commands
description: Complete list of available commands in the Claude Code CLI
---

## Available Commands

The Claude Code CLI provides several commands for interacting with MSSQL databases.

### Basic Usage

```bash
go run claude-code/db-connector.go [command] [arguments]
```

## Commands

### test

Tests the database connection.

```bash
go run claude-code/db-connector.go test
```

**Output**: Confirms if the connection was successful or displays connection errors.

### info

Gets general information about the connected database.

```bash
go run claude-code/db-connector.go info
```

**Output**: Server information, version, and configuration.

### tables

Lists all available tables in the database.

```bash
go run claude-code/db-connector.go tables
```

**Output**: List of table names with their schemas.

### describe

Describes the structure of a specific table.

```bash
go run claude-code/db-connector.go describe TABLE_NAME
```

**Arguments**:
- `TABLE_NAME`: Name of the table to describe

**Output**: Columns, data types, constraints, and indexes of the table.

### query

Executes a custom SQL query.

```bash
go run claude-code/db-connector.go query "SELECT * FROM table WHERE condition"
```

**Arguments**:
- SQL query in quotes

**Output**: Query results in tabular format.

**Security note**: The CLI uses prepared statements to prevent SQL injection.

## Required Environment Variables

Before running any command, make sure to configure the environment variables:

```bash
# Copy template
cp .env.example .env

# Edit .env with your credentials
# Load variables (Linux/Mac)
source .env

# Windows PowerShell
Get-Content .env | ForEach-Object {
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name, $value)
}
```

Required variables:
- `MSSQL_SERVER`: SQL Server
- `MSSQL_DATABASE`: Database
- `MSSQL_USER`: User
- `MSSQL_PASSWORD`: Password

Optional variables:
- `MSSQL_PORT`: Port (default: 1433)
- `DEVELOPER_MODE`: Development mode (true/false)
- `MSSQL_READ_ONLY`: Read-only mode (true/false)
- `MSSQL_WHITELIST_TABLES`: Allowed tables in read-only mode

## Usage Examples

### Test connection

```bash
go run claude-code/db-connector.go test
```

### List tables

```bash
go run claude-code/db-connector.go tables
```

### View table structure

```bash
go run claude-code/db-connector.go describe Users
```

### Execute SELECT query

```bash
go run claude-code/db-connector.go query "SELECT TOP 10 * FROM Users WHERE Active = 1"
```

### Query with JOIN

```bash
go run claude-code/db-connector.go query "SELECT u.Name, o.OrderDate FROM Users u JOIN Orders o ON u.Id = o.UserId"
```

## Security

The CLI implements the same security features as the MCP server:

- Encrypted TLS connections
- Prepared statements to prevent SQL injection
- Input validation
- Security logging
- Read-only mode support

See [Security](/en/seguridad/resumen) for more details.

## Troubleshooting

### Error: "Database not connected"

Verify that environment variables are configured:

```bash
echo "Server: $MSSQL_SERVER"
echo "Database: $MSSQL_DATABASE"
echo "User: $MSSQL_USER"
```

### TLS certificate error

If you see certificate errors, set `DEVELOPER_MODE=true` for development:

```bash
export DEVELOPER_MODE=true
```

**Warning**: Do not use `DEVELOPER_MODE=true` in production.

### Network error

Check firewall and verify that SQL Server port (1433) is open.

## Next Steps

- [Environment variables](/en/configuracion/variables-entorno)
- [Security](/en/seguridad/resumen)
- [Troubleshooting](/en/despliegue/solucion-problemas)
