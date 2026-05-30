---
title: Read-Only Mode
description: Restricting access to SELECT queries only
---

Read-only mode blocks all modification operations, allowing only SELECT queries.

## Configuration

```bash
# Enable read-only mode
MSSQL_READ_ONLY=true
```

## Behavior

### Allowed operations

```sql
-- All SELECT queries
SELECT * FROM users
SELECT u.*, o.total FROM users u JOIN orders o ON u.id = o.user_id

-- Subqueries
SELECT * FROM (SELECT id, name FROM users) sub

-- CTEs
WITH active AS (SELECT * FROM users WHERE active = 1)
SELECT * FROM active

-- Aggregations and window functions
SELECT department, AVG(salary) FROM employees GROUP BY department
```

### Blocked operations

```sql
-- Data modifications
INSERT INTO users VALUES (1, 'test')        -- Blocked
UPDATE users SET name = 'new' WHERE id = 1   -- Blocked
DELETE FROM users WHERE id = 1               -- Blocked

-- DDL
CREATE TABLE temp (id INT)    -- Blocked
DROP TABLE users              -- Blocked
ALTER TABLE users ADD col INT -- Blocked

-- Dangerous code execution
EXEC xp_cmdshell 'dir'       -- Always blocked
EXEC sp_executesql '...'     -- Always blocked
EXEC sp_configure ...        -- Always blocked
```

### Allowed administrative and schema reads

Even in read-only mode, a small set of **administrative and schema introspection** operations are permitted because they are inherently read-only. This makes database discovery much more practical for tools and AI assistants:

```sql
-- Safe system procedures for schema exploration
EXEC sp_help 'dbo.Users'              -- Table structure
EXEC sp_helptext 'dbo.MyProcedure'    -- Source code of an object
EXEC sp_spaceused 'dbo.Orders'        -- Space usage information
EXEC sp_columns @table_name = 'Customers'
EXEC sp_fkeys 'Orders'
```

These procedures are explicitly allowed because they do not modify data or server configuration. Any other system procedure (or dynamic SQL via EXEC) remains blocked.

For most schema discovery use cases we recommend the dedicated `explore` and `inspect` tools instead of raw queries.

## Query validation

Validation uses regular expressions with word boundaries (`\bINSERT\b`, `\bUPDATE\b`, etc.) to avoid false positives. For example:

```sql
-- Allowed (does not contain INSERT as an operation)
SELECT created_at FROM transactions

-- Allowed (update_count is a column name, not an operation)
SELECT update_count FROM statistics

-- Blocked (contains the UPDATE operation)
UPDATE users SET status = 'active'
```

## Combining with whitelist

To allow modifications only on specific tables, combine with `MSSQL_WHITELIST_TABLES`:

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

See the [Table Whitelist](/en/seguridad/whitelist-tablas/) section for more details.
