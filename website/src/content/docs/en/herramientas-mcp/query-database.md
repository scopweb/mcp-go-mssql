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
- `SELECT` — Always allowed
- `INSERT`, `UPDATE`, `DELETE` — Only on whitelisted tables
- `EXEC`, `xp_cmdshell` — Always blocked

### In full mode (`MSSQL_READ_ONLY=false`)
- All standard SQL operations
- `EXEC`, `xp_cmdshell` — Always blocked for security

## Cross-database queries

If `MSSQL_ALLOWED_DATABASES` is configured, you can use 3-part names:

```sql
-- Query another database
SELECT * FROM OtherDB.dbo.Clients WHERE active = 1

-- JOIN across databases
SELECT a.name, b.total
FROM local_table a
JOIN OtherDB.dbo.remote_table b ON a.id = b.id
```

:::caution[Read-only]
Modifications (INSERT/UPDATE/DELETE) on cross-databases are **always blocked**, even if the table is in the whitelist.
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
