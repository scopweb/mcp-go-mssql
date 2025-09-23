# Issue #3: Implement Read-Only Security Mode for Query Restriction

## Problem Description
The MCP server allowed all SQL operations by default, which posed security risks in environments where users should only be able to read data. There was no built-in mechanism to restrict database access to read-only operations, making it unsuitable for environments requiring strict data protection.

## Security Concerns
- **Data Modification Risk**: Users could accidentally or intentionally modify data
- **Schema Changes**: Potential for structural database changes
- **Stored Procedure Execution**: Risk of executing administrative procedures
- **System Commands**: Possibility of running system-level operations
- **Compliance Requirements**: Many environments require read-only access for reporting users

## Use Cases Requiring Read-Only Access
1. **Reporting and Analytics**: Business users generating reports
2. **Data Exploration**: Analysts exploring datasets
3. **Development Testing**: Developers testing queries against production schemas
4. **Compliance Auditing**: Auditors reviewing data without modification rights
5. **External Integrations**: Third-party systems requiring read access only

## Solution Implemented

### 1. Read-Only Mode Flag
Added `MSSQL_READ_ONLY` environment variable to enable strict query filtering:

```go
func (s *MCPMSSQLServer) validateReadOnlyQuery(query string) error {
    // Check if read-only mode is enabled
    if strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) != "true" {
        return nil // Read-only mode disabled, allow all queries
    }
    // ... validation logic
}
```

### 2. Comprehensive Query Validation
Implemented multi-layer validation that checks:

#### Allowed Operations
- ✅ `SELECT` statements
- ✅ `WITH` (Common Table Expressions)
- ✅ `SHOW` commands
- ✅ `DESCRIBE` / `DESC` commands
- ✅ `EXPLAIN` commands

#### Blocked Operations
- ❌ `INSERT` statements
- ❌ `UPDATE` statements
- ❌ `DELETE` statements
- ❌ `DROP` commands
- ❌ `CREATE` commands
- ❌ `ALTER` commands
- ❌ `TRUNCATE` commands
- ❌ `MERGE` statements
- ❌ `EXEC` / `EXECUTE` commands
- ❌ `CALL` procedures
- ❌ `BULK` operations
- ❌ `BCP` commands
- ❌ `xp_` system procedures
- ❌ `sp_` system procedures

### 3. Advanced Security Features

#### Comment Handling
The validator properly handles SQL comments to prevent bypassing:
```sql
-- This would be blocked:
/* comment */ DELETE FROM table;

-- This would be allowed:
/* comment */ SELECT * FROM table;
```

#### Nested Query Protection
Prevents dangerous operations hidden within SELECT statements:
```sql
-- This would be blocked even though it starts with SELECT:
SELECT * FROM table; DROP TABLE users;
```

#### Case-Insensitive Detection
Works regardless of query casing:
```sql
-- All of these are properly detected and blocked:
delete from table;
DELETE FROM table;
Delete From Table;
DeLeTe FrOm TaBlE;
```

## Configuration Examples

### Enable Read-Only Mode
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://readonly_user:pass@server:1433?database=mydb&encrypt=disable",
    "MSSQL_READ_ONLY": "true",
    "DEVELOPER_MODE": "false"
  }
}
```

### Full Access Mode (Default)
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://admin_user:pass@server:1433?database=mydb&encrypt=disable",
    "MSSQL_READ_ONLY": "false",
    "DEVELOPER_MODE": "false"
  }
}
```

## Security Validation Examples

### ✅ Allowed Queries
```sql
-- Simple SELECT
SELECT * FROM customers;

-- Complex JOIN
SELECT c.name, o.total
FROM customers c
JOIN orders o ON c.id = o.customer_id;

-- Common Table Expression
WITH sales_summary AS (
  SELECT region, SUM(amount) as total
  FROM sales
  GROUP BY region
)
SELECT * FROM sales_summary;

-- System information
SHOW TABLES;
DESCRIBE customers;
EXPLAIN SELECT * FROM orders;
```

### ❌ Blocked Queries
```sql
-- Data modification
INSERT INTO customers (name) VALUES ('New Customer');
UPDATE customers SET name = 'Updated' WHERE id = 1;
DELETE FROM customers WHERE id = 1;

-- Schema changes
CREATE TABLE new_table (id INT);
ALTER TABLE customers ADD COLUMN email VARCHAR(255);
DROP TABLE old_table;

-- Administrative operations
EXEC sp_configure;
EXECUTE master.dbo.xp_cmdshell 'dir';
TRUNCATE TABLE logs;
```

## Enhanced Database Info Display
The `get_database_info` tool now shows the current access mode:

**Read-Only Mode:**
```
Database Status: Connected
Connection: Custom connection string
Mode: Development
Access Mode: READ-ONLY (SELECT queries only)
```

**Full Access Mode:**
```
Database Status: Connected
Connection: Custom connection string
Mode: Production
Access Mode: Full access
```

## Security Logging
All blocked queries are logged for security auditing:
```
[SECURITY] Read-only violation blocked: read-only mode: query contains forbidden operation 'DELETE'
```

## Error Messages
Clear, informative error messages help users understand restrictions:

```
Error: read-only mode: only SELECT and read operations are allowed
Error: read-only mode: query contains forbidden operation 'UPDATE'
Error: read-only mode: only SELECT queries are allowed
```

## Testing Matrix

| Query Type | Full Access | Read-Only Mode |
|------------|-------------|----------------|
| `SELECT *` | ✅ Allowed | ✅ Allowed |
| `INSERT INTO` | ✅ Allowed | ❌ Blocked |
| `UPDATE SET` | ✅ Allowed | ❌ Blocked |
| `DELETE FROM` | ✅ Allowed | ❌ Blocked |
| `WITH ... SELECT` | ✅ Allowed | ✅ Allowed |
| `EXEC proc` | ✅ Allowed | ❌ Blocked |
| `SHOW TABLES` | ✅ Allowed | ✅ Allowed |

## Files Modified
- `main.go`: Added `validateReadOnlyQuery()` function
- `main.go`: Updated query execution flow with read-only validation
- `main.go`: Enhanced database info display
- `main.go`: Added security logging for violations
- `README.md`: Documented `MSSQL_READ_ONLY` configuration option

## Benefits
1. **Enhanced Security**: Prevents accidental or malicious data modification
2. **Compliance Ready**: Meets requirements for read-only database access
3. **Flexible Configuration**: Can be enabled/disabled per environment
4. **Comprehensive Protection**: Multiple layers of validation
5. **Clear Feedback**: Informative error messages and logging
6. **Zero Performance Impact**: Validation only occurs when enabled

## Deployment Recommendations

### Production Reporting Environment
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://report_user:readonly_pass@prod-db:1433?database=analytics",
    "MSSQL_READ_ONLY": "true",
    "DEVELOPER_MODE": "false"
  }
}
```

### Development Environment
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://dev_user:dev_pass@dev-db:1433?database=development",
    "MSSQL_READ_ONLY": "false",
    "DEVELOPER_MODE": "true"
  }
}
```

## Issue Status: ✅ RESOLVED
**Fixed in commit**: Implement comprehensive read-only security mode with query validation