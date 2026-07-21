---
title: MCP Integration
description: Details of the MCP protocol implementation in MCP-Go-MSSQL
---

## MCP Protocol

MCP (Model Context Protocol) is the protocol that allows Claude Desktop to communicate with external servers. MCP-Go-MSSQL implements MCP over JSON-RPC 2.0 using stdin/stdout.

## Available tools

The server exposes 6 base MCP tools (11 in dynamic mode, which adds 5 more):

| Tool | Description |
|------|-------------|
| `query_database` | Execute SQL queries |
| `get_database_info` | General connection and database information |
| `explore` | Explore objects: tables, views, databases, procedures and search |
| `inspect` | Inspect a table: columns, indexes, foreign keys and dependencies |
| `execute_procedure` | Execute a whitelisted stored procedure |
| `explain_query` | Estimated execution plan of a SELECT query without running it |

In dynamic mode, the following are also exposed: `dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list` and `confirm_operation`.

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
