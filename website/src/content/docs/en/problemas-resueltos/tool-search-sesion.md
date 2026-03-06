---
title: "MCP Connectors require tool_search on every session"
description: mssql-* connectors are marked as deferred tools, requiring tool_search before each use in new sessions
---

MCP connectors of type `mssql-*` are marked as **deferred tools** in Claude's system. This means that on every new session, Claude doesn't know their parameters and calls `tool_search` before it can use them — even though it has used them thousands of times before.

## Issue details

| Field | Value |
|---|---|
| **Date** | 2026-03-02 |
| **Severity** | Low (UX / token cost) |
| **Status** | Workaround applied |

## Affected connectors

- `mssql-MyDatabase`
- `mssql-MyDatabase_LOCAL`
- `mssql-SERVER-GDP`
- `mssql-SQL01`
- `mssql-MyIdentityDB`

## Impact

- **Unnecessary token cost** on every session that touches the database
- **Extra latency** — one additional call before the actual query
- All `mssql-*` connectors share the same function schema, making the `tool_search` redundant

## Common function schema

All connectors expose exactly the same functions:

| Function | Parameters |
|---|---|
| `query_database` | `query: string` |
| `get_database_info` | — |
| `explore` | `type?: string, filter?: string, schema?: string, pattern?: string, search_in?: string` |
| `inspect` | `table_name: string, schema?: string, detail?: string` |
| `execute_procedure` | `procedure_name: string, parameters?: string` |

## Root cause

The deferred tools system doesn't distinguish between tools "known from training" and tools that require real discovery. All MCPs go through the same mechanism even though their schema is static and well-known.

## Workaround applied

A note was added to Claude's user memory with the complete connector schema:

```
mssql connectors (...): Do NOT use tool_search, call directly:
query_database(query), list_tables(), describe_table(table_name, schema?), ...
```

With this, Claude skips `tool_search` and calls the connector directly.

## Ideal solution

Allow `mssql-*` connectors (or others with static, well-defined schemas) to be marked as **pre-loaded**, or have Claude recognize them by prefix without needing `tool_search`.
