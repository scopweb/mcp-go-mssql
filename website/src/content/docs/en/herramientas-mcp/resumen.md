---
title: MCP Tools
description: Overview of the 6 base tools available in the MCP server
---

The MCP-Go-MSSQL server exposes **6 base tools** that Claude Desktop can use to interact with Microsoft SQL Server databases securely. In dynamic mode, **5 additional tools** are exposed (11 in total).

## Tool list

| Tool | Description | Key parameters |
|------|-------------|----------------|
| [`query_database`](/en/herramientas-mcp/query-database/) | Execute SQL queries | `query` (required) |
| [`get_database_info`](/en/herramientas-mcp/get-database-info/) | Connection info and status | — |
| [`explore`](/en/herramientas-mcp/explore/) | Explore objects: tables, databases, procedures, search | `type`, `filter`, `schema`, `pattern`, `search_in` |
| [`inspect`](/en/herramientas-mcp/inspect/) | Inspect table structure: columns, indexes, foreign keys | `table_name` (required), `schema`, `detail` |
| [`execute_procedure`](/en/herramientas-mcp/execute-procedure/) | Execute stored procedure (whitelist required) | `procedure_name` (required), `parameters` |
| [`explain_query`](/en/herramientas-mcp/explain-query/) | Estimated execution plan without running the query (SELECT only) | `query` (required) |

## Dynamic mode tools

Only exposed when the server starts in dynamic mode (`MSSQL_DYNAMIC_*`):

| Tool | Description |
|------|-------------|
| `dynamic_available` | List preconfigured connection aliases |
| `dynamic_connect` | Activate a dynamic connection by alias |
| `dynamic_disconnect` | Close the active dynamic connection |
| `dynamic_list` | Show loaded aliases and which one is active |
| `confirm_operation` | Confirm a modification on a writable alias (expires after 90 seconds) |

## MCP protocol

Tools communicate via JSON-RPC through stdin/stdout. Claude Desktop sends `tools/list` requests to discover tools and `tools/call` to execute them.

## Security

All tools:
- Use **prepared statements** to prevent SQL injection
- Respect **read-only mode** when enabled
- Validate **referenced tables** against the whitelist
- Operate with **context timeouts** (30 seconds)
- Sanitize sensitive information in logs
