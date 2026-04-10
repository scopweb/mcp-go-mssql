---
title: Table Whitelist
description: Granular permission system for table access control
---

The whitelist system allows modifying specific tables even in read-only mode — ideal for giving AI assistants a temporary workspace.

## Problem it solves

When using AI assistants with production databases, there's a risk of:
- Accidental data deletion
- Data exfiltration with malicious queries like `DELETE temp_ai FROM temp_ai JOIN production_table`
- Unauthorized access to sensitive tables via JOINs or subqueries

## Configuration

```bash
# Specific whitelist: only listed tables can be modified
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia

# Wildcard: allows modifying ALL tables (while keeping security protections)
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=*
```

## Behavior by configuration

| `MSSQL_WHITELIST_TABLES` | SELECT | Modifications (INSERT/UPDATE/DELETE...) |
|---|---|---|
| Not set | ✅ Yes | ❌ All blocked |
| `table1,table2` | ✅ Yes | ✅ Only listed tables |
| `*` (wildcard) | ✅ Yes | ✅ **All** tables |

> **Note:** With `MSSQL_WHITELIST_TABLES=*`, modifications are allowed on all tables but dangerous system operations (`XP_CMDSHELL`, `SP_CONFIGURE`, etc.) **remain blocked**.

## Validation flow

1. User executes a query
2. Basic input validation
3. Read-only mode check
4. Operation type extraction (INSERT/UPDATE/DELETE/etc.)
5. Extraction of **all** referenced tables (FROM, JOIN, subqueries, CTEs)
6. Validation that **all** tables are in the whitelist
7. Execution or block with error

## Multi-table detection

The parser detects tables in:
- `FROM` clauses
- `JOIN` operations (INNER, LEFT, RIGHT, FULL)
- Subqueries: `SELECT * FROM (SELECT * FROM table)`
- `INSERT INTO ... SELECT ... FROM`
- `UPDATE ... SET col = (SELECT ... FROM)`
- `DELETE ... FROM ... JOIN`
- CTEs: `WITH cte AS (SELECT * FROM table)`

## Examples

### Allowed queries

```sql
-- SELECT always allowed (read-only)
SELECT * FROM production_table
SELECT * FROM production_table JOIN temp_ai ON ...

-- Modifications on whitelisted tables
UPDATE temp_ai SET col = 'value' WHERE id = 1
DELETE FROM temp_ai WHERE id = 1
INSERT INTO temp_ai VALUES (1, 'test')
```

### Blocked queries

```sql
-- Modification of unauthorized table
UPDATE users SET password = 'hacked'
-- Error: permission denied: table 'users' is not whitelisted

-- JOIN with unauthorized table in modification
DELETE temp_ai FROM temp_ai JOIN users ON temp_ai.id = users.id
-- Error: permission denied: table 'users' is not whitelisted

-- Subquery to sensitive data
UPDATE temp_ai SET data = (SELECT password FROM users WHERE id = 1)
-- Error: permission denied: table 'users' is not whitelisted

-- INSERT from unauthorized table
INSERT INTO temp_ai SELECT * FROM customers
-- Error: permission denied: table 'customers' is not whitelisted
```

## Security logs

Each permission check is logged:

```
[SECURITY] Permission check - Operation: DELETE, Tables found: [temp_ai users], Whitelist: [temp_ai]
[SECURITY] SECURITY VIOLATION: Attempted DELETE on non-whitelisted table 'users'
```

## Recommendations for AI

### Create dedicated temporary tables

```sql
CREATE TABLE temp_ai (
    id INT IDENTITY(1,1) PRIMARY KEY,
    operation_type VARCHAR(50),
    data NVARCHAR(MAX),
    created_at DATETIME DEFAULT GETDATE(),
    result NVARCHAR(MAX)
);
```

### Automate cleanup

```sql
CREATE PROCEDURE CleanupTempAI
AS
BEGIN
    DELETE FROM temp_ai
    WHERE created_at < DATEADD(day, -7, GETDATE());
END;
```

## Wildcard `*` — full access with protections

Use `MSSQL_WHITELIST_TABLES=*` when you want the AI assistant to write to any table while keeping read-only security protections:

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=*
```

**Protections that remain active with wildcard:**
- `XP_CMDSHELL`, `XP_REGREAD`, `XP_DIRTREE` → always blocked
- `SP_CONFIGURE`, `SP_ADDLOGIN`, `SP_DROPLOGIN` → always blocked
- Modifications on cross-databases (`MSSQL_ALLOWED_DATABASES`) → always blocked

## Limitations

The regex-based parser may not detect tables in:
- Highly obfuscated queries with nested comments
- Dynamic SQL within stored procedures
- CTEs with multiple levels of nesting

**Mitigation:** For maximum security, combine with database-level permissions (GRANT/DENY).
