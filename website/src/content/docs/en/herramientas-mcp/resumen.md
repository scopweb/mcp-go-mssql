---
title: MCP Tools
description: Overview of the 6 tools available in the MCP server
---

The MCP-Go-MSSQL server exposes **6 tools** that Claude Desktop can use to interact with Microsoft SQL Server databases securely.

## Tool list

| Tool | Description | Key parameters |
|------|-------------|----------------|
| [`query_database`](/en/herramientas-mcp/query-database/) | Execute SQL queries | `query` (required) |
| [`get_database_info`](/en/herramientas-mcp/get-database-info/) | Connection info and status | — |
| [`explore`](/en/herramientas-mcp/explore/) | Explore objects: tables, views, databases, procedures, search | `type`, `filter`, `pattern`, `search_in`, `database` |
| [`inspect`](/en/herramientas-mcp/inspect/) | Inspect table structure: columns, indexes, foreign keys, dependencies | `table_name` (required), `schema`, `detail` |
| [`explain_query`](/en/herramientas-mcp/explain-query/) | Estimated execution plan without running the query | `query` (required) |
| [`execute_procedure`](/en/herramientas-mcp/execute-procedure/) | Execute stored procedure (whitelist required) | `procedure_name` (required), `parameters` |

## MCP protocol

Tools communicate via JSON-RPC through stdin/stdout. Claude Desktop sends `tools/list` requests to discover tools and `tools/call` to execute them.

All responses include **content annotations** per MCP spec 2025-11-25:
- `audience`: indicates if content is for `user`, `assistant`, or both
- `priority`: from 0.0 (lowest) to 1.0 (highest importance)

## Rate limiting

The server implements a **60 calls per minute** rate limiter (token bucket). If the limit is exceeded, the tool returns an error and you must wait before retrying.

## Security

All tools:
- Use **prepared statements** to prevent SQL injection
- Respect **read-only mode** when enabled
- Validate **referenced tables** against the whitelist
- **Validate that tables exist** before executing (schema validation with "Did you mean?" suggestions)
- Operate with **context timeouts** (30 seconds)
- Sanitize sensitive information in logs
- Include **MCP annotations** (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`)
