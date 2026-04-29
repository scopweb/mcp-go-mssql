---
name: mcp-mssql-tools
description: >
  Expose and guide usage of mcp-go-mssql MCP server tools. Use when connected to a
  MSSQL database via MCP, when the user asks about available database tools, or when
  you need to decide which MCP tool to use for a database task. Triggers: "what tools
  do I have", "how do I query", "explore the database", "show me the tables",
  "database tools", "mcp-go-mssql", any SQL Server / MSSQL database interaction.
---

# MCP MSSQL Tools — Quick Reference

11 tools available via the `mcp-go-mssql` MCP server (7 base + 4 dynamic when `MSSQL_DYNAMIC_MODE=true`). Always prefer the most specific tool for the task.

## Tool Selection Guide

| Goal | Tool | Key param |
|------|------|-----------|
| List tables, views, procedures, databases | `explore` | `type` |
| Search objects by name or source code | `explore` | `type=search`, `pattern` |
| See column types, indexes, FKs, dependencies | `inspect` | `table_name`, `detail` |
| Check connection status and server info | `get_database_info` | — |
| Run any SQL (SELECT, INSERT, UPDATE, DELETE) | `query_database` | `query` |
| Run a whitelisted stored procedure | `execute_procedure` | `procedure_name` |
| Analyze query performance without executing | `explain_query` | `query` |
| Confirm a destructive DDL operation | `confirm_operation` | `token` |
| Discover available dynamic connections | `dynamic_available` | — |
| Connect to a dynamic database alias | `dynamic_connect` | `alias` |
| List active dynamic connections | `dynamic_list` | — |
| Close a dynamic connection | `dynamic_disconnect` | `alias` |

## Aliases (alternative names the AI may use)

- `query_database`: run_sql, execute_sql, db_query, sql_execute, sql_query, run_query, exec_query
- `get_database_info`: server_info, db_status, db_info, connection_status
- `explore`: list_tables, list_views, list_procedures, show_tables, show_views, db_explore, find_tables, search_tables
- `inspect`: describe_table, table_structure, schema_info, show_columns, table_info, column_info, index_info
- `explain_query`: show_plan, explain_plan, sql_explain, analyze_query, query_plan, plan_analysis

## Workflow: First Contact with a Database

Follow this sequence when starting work on an unfamiliar database:

1. **`get_database_info`** — Confirm connection, check read-only mode and whitelist
2. **`explore`** (`type=tables`) — Get the full table/view inventory
3. **`explore`** (`type=search`, `pattern=<term>`) — Find relevant objects
4. **`inspect`** (`detail=all`) — Understand structure of target tables
5. **`query_database`** — Query with knowledge, not guesses

> Never guess table or column names. Always explore/inspect first.

## Tools Detail

For complete parameter reference, examples, and usage patterns see [tools-reference.md](reference/tools-reference.md).

## Security Awareness

- **Read-only mode**: When `MSSQL_READ_ONLY=true`, only SELECT is allowed except on whitelisted tables
- **Cross-database**: `MSSQL_ALLOWED_DATABASES` tables are always read-only
- **Prepared statements**: All queries use parameterized execution — SQL injection is blocked at server level
- **Schema validation**: The server validates that referenced tables exist before executing (skipped when `MSSQL_AUTOPILOT=true`)
- **Destructive confirmation**: DDL operations (DROP, ALTER, CREATE TABLE) ALWAYS require `confirm_operation` — AUTOPILOT does NOT skip this

## Dynamic Multi-Connection Mode

When `MSSQL_DYNAMIC_MODE=true` (and `MSSQL_SERVER` is not set), the server supports multiple database connections:

- `dynamic_available` — Discover configured aliases from `.env` (no credentials shown)
- `dynamic_connect` — Activate a connection by alias
- `dynamic_list` — List active connections (alias, server, database — no passwords)
- `dynamic_disconnect` — Close an active connection
- `query_database` — Use `connection` param to target a specific dynamic connection

Credentials are stored in `.env` with prefix `MSSQL_DYNAMIC_<ALIAS>_`. The AI only sees aliases.

**.env search order:** The server first looks for `.env` next to the executable, then falls back to the current working directory.

**Dual-mode architecture:**
- `MSSQL_SERVER` set → direct connection mode, no dynamic tools
- `MSSQL_SERVER` not set → loads `.env`, enables dynamic tools if `MSSQL_DYNAMIC_MODE=true`