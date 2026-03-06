---
title: MCP Integration
description: Details of the MCP protocol implementation in MCP-Go-MSSQL
---

## MCP Protocol

MCP (Model Context Protocol) is the protocol that allows Claude Desktop to communicate with external servers. MCP-Go-MSSQL implements MCP over JSON-RPC 2.0 using stdin/stdout.

## Available tools

The server exposes 9 MCP tools:

| Tool | Description |
|------|-------------|
| `query_database` | Execute SQL queries |
| `list_tables` | List all database tables |
| `describe_table` | Show table structure |
| `get_database_info` | General database information |
| `list_databases` | List available databases |
| `get_indexes` | Show table indexes |
| `get_foreign_keys` | Show table foreign keys |
| `list_stored_procedures` | List stored procedures |
| `execute_procedure` | Execute a stored procedure |

## Communication flow

1. Claude Desktop starts the `mcp-go-mssql` process
2. The server sends its capabilities (tool list)
3. Claude Desktop sends JSON-RPC requests via stdin
4. The server responds via stdout
5. Security logs are written to stderr

## Lifecycle

- The server connects to the database on startup
- Maintains the connection pool active throughout the session
- Shuts down cleanly when Claude Desktop ends the session
