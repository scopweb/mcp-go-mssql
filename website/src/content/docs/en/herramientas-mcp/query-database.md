---
title: query_database
description: Execute SQL queries securely
---

Executes a SQL query against the MSSQL database using prepared statements.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | SQL query to execute |
| `max_cell_bytes` | integer | No | Cap for a single cell (minimum 16). A longer value is shortened and marked |
| `max_response_bytes` | integer | No | Approximate cap for the whole result (minimum 512). The result stops there |
| `format` | string | No | `map` (default, one object per row) or `columnar` (the column names once and every row an array of values in that order) |

## Usage example

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SELECT TOP 10 * FROM users WHERE active = 1"
  }
}
```

## Output formats

`query_database` renders the rows in two formats:

- `format="map"` (default): one object per row, with the column name on every value.
- `format="columnar"`: the column names are paid once and every row is an array of values in that order.

```json
{
  "format": "columnar",
  "columns": ["id", "name", "city"],
  "rows": [[1, "Acme", "Barcelona"], [2, "Beta", null]],
  "row_count": 2,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {"max_rows": 500, "max_cell_bytes": null, "max_response_bytes": null},
  "stats": {"enabled": false, "messages": []}
}
```

The columnar format needs unique column names: a result that repeats a column name is refused, because a row cannot carry it twice. With ten columns and two hundred rows the columnar form is around a third of the size of the map form; measured on a table of 38 columns and 100 rows, the whole answer went from 182 KB to 91 KB.

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
