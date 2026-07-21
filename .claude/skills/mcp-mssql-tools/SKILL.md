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

10 tools available via the `mcp-go-mssql` MCP server. Always prefer the most specific tool for the task.

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
| Confirm a modification on a writable dynamic alias | `confirm_operation` | `description` (dynamic mode only) |
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
- **Dynamic aliases**: default to read-only; a writable alias (`_READ_ONLY=false`) still needs `_WHITELIST_TABLES` to modify anything, and modifications require `confirm_operation` first
- **If a modification is blocked by read-only mode**: DO NOT try to bypass it (no `execute_procedure`, alternative tools, or re-framed statements), and DO NOT suggest elevating permissions unless the user asks. Instead, present the complete SQL in a ```sql code block for the user to execute manually (SSMS, sqlcmd, Azure Data Studio), briefly stating the connection is read-only.
- **Prepared statements**: All queries use parameterized execution — SQL injection is blocked at server level
- **Stacked statements**: Multi-statement batches are rejected in read-only/whitelist contexts (except `SET STATISTICS` diagnostic wrappers)
- **Writable-alias confirmation**: Modifications on a writable dynamic alias require `confirm_operation` first (bound to the alias, 90-second expiry)

## Dynamic Multi-Connection Mode

When the server starts in dynamic mode (no classic `MSSQL_SERVER` config, or explicit `MSSQL_DYNAMIC_MODE=true`), it supports multiple database connections:

- `dynamic_available` — Discover configured aliases from `.env` (no credentials shown)
- `dynamic_connect` — Activate a connection by alias
- `dynamic_list` — List active connections (alias, server, database — no passwords)
- `dynamic_disconnect` — Close the active dynamic connection
- `dynamic_connect` switches the active connection; subsequent `query_database` calls run against it

Credentials are stored in `.env` with prefix `MSSQL_DYNAMIC_<ALIAS>_`. The AI only sees aliases.

**Mode selection (isDynamicMode):**
- Explicit `MSSQL_DYNAMIC_MODE=true/false` always wins
- Otherwise: any classic config (`MSSQL_SERVER` / `MSSQL_CONNECTION_STRING` / `MSSQL_DATABASE`) → classic mode, no dynamic tools
- Otherwise: any `MSSQL_DYNAMIC_*` var present → dynamic mode (auto-detect)
