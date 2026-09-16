---
title: query_database
description: Execute SQL queries securely
---

Executes a SQL query against the MSSQL database using prepared statements.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | SQL query to execute |

## Usage example

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SELECT TOP 10 * FROM users WHERE active = 1"
  }
}
```

## Allowed queries

### In read mode (`MSSQL_READ_ONLY=true`)
- `SELECT` and read-only CTEs — Allowed
- Explicitly allowlisted schema admin procedures (`sp_help`, `sp_columns`, …) — Allowed only as `sys`/`dbo` with no state change
- `INSERT`, `UPDATE`, `DELETE`, DDL, `SELECT INTO`, `execute_procedure` — Blocked even with a whitelist
- Concatenated batches (with or without `;`) — Blocked
- `xp_cmdshell`, `sp_executesql` — Always blocked

### In full mode (`MSSQL_READ_ONLY=false`)
- All standard SQL operations
- `EXEC`, `xp_cmdshell` — Always blocked for security

:::note[Multi-statement batches]
In read-only / whitelist mode, multi-statement batches are rejected, **except** `SET STATISTICS TIME/IO` diagnostic wrappers. See [Execution statistics](/en/herramientas-mcp/statistics/) for details.
:::

## Query examples

```sql
-- Simple query
SELECT * FROM products WHERE price > 100

-- Complex JOIN
SELECT u.name, COUNT(o.id) as total_orders
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.name

-- CTE
WITH recent_orders AS (
    SELECT * FROM orders WHERE order_date > DATEADD(day, -30, GETDATE())
)
SELECT * FROM recent_orders

-- Window functions
SELECT name, salary,
    ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank
FROM employees
```

## Security

- Queries are executed with `PrepareContext()` — no SQL string concatenation
- Maximum query size is configurable via `MSSQL_MAX_QUERY_SIZE`
- A 30-second timeout is applied by default
- In read-only mode, all referenced tables are validated (including JOINs and subqueries)
