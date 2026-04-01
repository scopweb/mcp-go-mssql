---
title: MCP Tools
description: Overview of the 5 tools available in the MCP server
---

The MCP-Go-MSSQL server exposes **5 tools** that Claude Desktop can use to interact with Microsoft SQL Server databases securely.

## Tool list

| Tool | Description | Key parameters |
|------|-------------|----------------|
| [`query_database`](/en/herramientas-mcp/query-database/) | Execute SQL queries | `query` (required) |
| [`get_database_info`](/en/herramientas-mcp/get-database-info/) | Connection info and status | — |
| [`explore`](/en/herramientas-mcp/explore/) | Explore objects: tables, databases, procedures, search | `type`, `filter`, `pattern`, `search_in`, `database` |
| [`inspect`](/en/herramientas-mcp/inspect/) | Inspect table structure: columns, indexes, foreign keys | `table_name` (required), `schema`, `detail` |
| [`execute_procedure`](/en/herramientas-mcp/execute-procedure/) | Execute stored procedure (whitelist required) | `procedure_name` (required), `parameters` |

## MCP protocol

Tools communicate via JSON-RPC through stdin/stdout. Claude Desktop sends `tools/list` requests to discover tools and `tools/call` to execute them.

## Security

All tools:
- Use **prepared statements** to prevent SQL injection
- Respect **read-only mode** when enabled
- Validate **referenced tables** against the whitelist
- Operate with **context timeouts** (30 seconds)
- Sanitize sensitive information in logs
