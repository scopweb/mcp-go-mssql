---
title: explain_query
description: Show the estimated execution plan for a SQL query without executing it
---

Shows the estimated execution plan for a SQL query **without executing it**. Useful for performance analysis and query optimization.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | SELECT query to analyze |

:::caution[SELECT only]
This tool **only accepts SELECT queries**. This is always enforced, regardless of `MSSQL_READ_ONLY` mode.
:::

## Usage example

```json
{
  "name": "explain_query",
  "arguments": {
    "query": "SELECT u.name, COUNT(o.id) FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.name"
  }
}
```

## Example response

```
Execution plan:

  |--Stream Aggregate(GROUP BY:([u].[name]))
       |--Sort(ORDER BY:([u].[name] ASC))
            |--Hash Match(Inner Join, HASH:([u].[id])=([o].[user_id]))
                 |--Table Scan(OBJECT:([MyDB].[dbo].[users] AS [u]))
                 |--Table Scan(OBJECT:([MyDB].[dbo].[orders] AS [o]))
```

## How it works

1. Acquires a dedicated connection from the pool
2. Executes `SET SHOWPLAN_TEXT ON` on that connection
3. Sends the query — SQL Server returns the estimated plan **without executing** it
4. Disables `SET SHOWPLAN_TEXT OFF` and releases the connection

## Use cases

- **Identify table scans** that could benefit from an index
- **Compare plans** before and after adding indexes
- **Detect expensive JOINs** in complex queries
- **Validate that a query will use an index** before running it in production

## Security

- SELECT queries only — no risk of data modification
- Uses a dedicated connection so `SET SHOWPLAN_TEXT` doesn't affect other queries
- 30-second timeout
- MCP annotations: `readOnlyHint=true`, `destructiveHint=false`, `idempotentHint=true`
