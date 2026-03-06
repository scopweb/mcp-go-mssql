---
title: Quick Start
description: Get MCP-Go-MSSQL up and running fast
---

## In 3 steps

### 1. Configure credentials

```bash
cp .env.example .env
# Edit .env with your connection details
```

### 2. Build

```bash
go build -o mcp-go-mssql.exe
```

### 3. Run

**As an MCP server (for Claude Desktop):**
```bash
./mcp-go-mssql.exe
```

**As a CLI tool (for Claude Code):**
```bash
cd claude-code
go run db-connector.go test
go run db-connector.go tables
go run db-connector.go query "SELECT TOP 10 * FROM my_table"
```

## Claude Desktop configuration example

Add this to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "my-database": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "my-server.database.windows.net",
        "MSSQL_DATABASE": "MyDatabase",
        "MSSQL_USER": "my_user",
        "MSSQL_PASSWORD": "my_password",
        "MSSQL_PORT": "1433",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

## Available tools

Once connected, Claude Desktop will have access to these tools:

| Tool | Description |
|------|-------------|
| `get_database_info` | Connection status, encryption, and access mode |
| `query_database` | Execute SQL queries securely |
| `list_tables` | List tables and views |
| `describe_table` | Column structure (supports `schema.table`) |
| `list_databases` | List server databases |
| `get_indexes` | Table indexes |
| `get_foreign_keys` | Foreign key relationships |
| `list_stored_procedures` | List stored procedures |
| `execute_procedure` | Execute authorized stored procedures |

## Next step

Check the [MCP Tools](/en/herramientas-mcp/resumen/) section to learn about each tool in detail.
