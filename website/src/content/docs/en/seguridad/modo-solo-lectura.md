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

-- Code execution
EXEC sp_help                  -- Blocked (except safe procedures)
EXEC xp_cmdshell 'dir'       -- Always blocked
```

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
# Specific whitelist
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia

# Wildcard: enables modifications on ALL tables
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=*
```

| Configuration | SELECT | Modifications |
|---|---|---|
| `READ_ONLY=true` only | ✅ | ❌ Blocked |
| `READ_ONLY=true` + specific tables | ✅ | ✅ Only listed |
| `READ_ONLY=true` + `*` | ✅ | ✅ All tables |

See the [Table Whitelist](/en/seguridad/whitelist-tablas/) section for more details.
