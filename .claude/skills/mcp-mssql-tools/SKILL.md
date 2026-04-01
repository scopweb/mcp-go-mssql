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

6 tools available via the `mcp-go-mssql` MCP server. Always prefer the most specific tool for the task.

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
- **Schema validation**: The server validates that referenced tables exist before executing
