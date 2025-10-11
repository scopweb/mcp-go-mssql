# Whitelist Security Feature - Implementation Guide

## Overview

This document explains the **Granular Table Permissions** security feature implemented in the MCP-Go-MSSQL server.

## Problem Statement

When using AI assistants with production databases, there's a risk of:
- Accidental data deletion/modification
- Data exfiltration through malicious queries like: `DELETE temp_ai FROM temp_ai JOIN production_table`
- Unauthorized access to sensitive tables via JOINs or subqueries

## Solution: Whitelist-Based Permission System

The whitelist security feature validates **ALL tables** referenced in modification queries, preventing access to non-whitelisted tables even through JOINs, subqueries, or CTEs.

---

## Configuration

### Environment Variables

```bash
# Enable read-only mode with whitelist
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia

# Full read-only (no modifications allowed)
MSSQL_READ_ONLY=true
# (no MSSQL_WHITELIST_TABLES)

# Full access mode (development)
MSSQL_READ_ONLY=false
```

### Claude Desktop Example

```json
{
  "mcpServers": {
    "production-db-safe": {
      "command": "C:/path/to/mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "prod-server.database.windows.net",
        "MSSQL_DATABASE": "ProductionDB",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "secure_password",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia"
      }
    }
  }
}
```

---

## How It Works

### Query Validation Flow

```
1. User executes query
   ↓
2. Basic input validation
   ↓
3. Read-only mode check
   ↓
4. Extract operation type (INSERT/UPDATE/DELETE/etc.)
   ↓
5. Extract ALL tables referenced (FROM, JOIN, subqueries, CTEs)
   ↓
6. Validate ALL tables are in whitelist
   ↓
7. Execute query OR block with error
```

### Multi-Table Detection

The parser detects tables in:
- `FROM` clauses
- `JOIN` operations (INNER, LEFT, RIGHT, FULL)
- Subqueries: `SELECT * FROM (SELECT * FROM table)`
- `INSERT INTO ... SELECT ... FROM`
- `UPDATE ... SET col = (SELECT ... FROM)`
- `DELETE ... FROM ... JOIN`
- CTEs: `WITH cte AS (SELECT * FROM table)`

---

## Security Examples

### ✅ Allowed Queries

```sql
-- SELECT always allowed (read-only)
SELECT * FROM production_table
SELECT * FROM production_table JOIN temp_ai ON ...

-- Modifications on whitelisted tables
UPDATE temp_ai SET col = 'value' WHERE id = 1
DELETE FROM temp_ai WHERE id = 1
INSERT INTO temp_ai VALUES (1, 'test')
CREATE VIEW v_temp_ia AS SELECT * FROM temp_ai
```

### ❌ Blocked Queries

```sql
-- Modification on non-whitelisted table
UPDATE users SET password = 'hacked'
-- Error: permission denied: table 'users' is not whitelisted for UPDATE operations

-- JOIN to non-whitelisted table
DELETE temp_ai FROM temp_ai t1
INNER JOIN users t2 ON t1.id = t2.id
-- Error: permission denied: table 'users' is not whitelisted for DELETE operations

-- Subquery accessing sensitive data
UPDATE temp_ai SET password = (SELECT password FROM users WHERE id = 1)
-- Error: permission denied: table 'users' is not whitelisted for UPDATE operations

-- INSERT from non-whitelisted table
INSERT INTO temp_ai SELECT * FROM customers
-- Error: permission denied: table 'customers' is not whitelisted for INSERT operations
```

---

## Testing

### Run Security Tests

```bash
# Test table extraction
go test -v -run TestExtractAllTablesFromQuery

# Test permission validation
go test -v -run TestValidateTablePermissions

# Run all tests
go test -v ./...
```

### Manual Testing

1. **Set environment variables:**
   ```powershell
   $env:MSSQL_READ_ONLY="true"
   $env:MSSQL_WHITELIST_TABLES="temp_ai,v_temp_ia"
   ```

2. **Start the server:**
   ```bash
   go run main.go
   ```

3. **Test with queries:**
   ```sql
   -- Should succeed
   UPDATE temp_ai SET col = 'value'

   -- Should fail
   UPDATE users SET col = 'value'
   ```

---

## Security Logs

All permission checks are logged with detail:

```
[SECURITY] Permission check - Operation: DELETE, Tables found: [temp_ai users], Whitelist: [temp_ai]
[SECURITY] SECURITY VIOLATION: Attempted DELETE operation on non-whitelisted table 'users'
```

```
[SECURITY] Permission check - Operation: UPDATE, Tables found: [temp_ai], Whitelist: [temp_ai v_temp_ia]
[SECURITY] Permission granted: UPDATE operation on whitelisted table(s) [temp_ai]
```

---

## Database Setup Recommendations

### Create Dedicated Temp Tables

```sql
-- Create temporary work table for AI
CREATE TABLE temp_ai (
    id INT IDENTITY(1,1) PRIMARY KEY,
    operation_type VARCHAR(50),
    data NVARCHAR(MAX),
    created_at DATETIME DEFAULT GETDATE(),
    result NVARCHAR(MAX)
);

-- Create view for AI queries
CREATE VIEW v_temp_ia AS
SELECT
    id,
    operation_type,
    data,
    created_at
FROM temp_ai
WHERE created_at >= DATEADD(hour, -24, GETDATE());

-- Grant permissions (optional, for defense in depth)
GRANT SELECT, INSERT, UPDATE, DELETE ON temp_ai TO ai_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON v_temp_ia TO ai_user;
DENY SELECT, INSERT, UPDATE, DELETE ON ALL OTHER TABLES TO ai_user;
```

### Cleanup Automation

```sql
-- Scheduled job to clean old temp data
CREATE PROCEDURE CleanupTempAI
AS
BEGIN
    DELETE FROM temp_ai
    WHERE created_at < DATEADD(day, -7, GETDATE());
END;
```

---

## Limitations

### Parser Limitations

The regex-based parser may not detect tables in:
- Highly obfuscated queries with nested comments
- Dynamic SQL within stored procedures (not recommended anyway)
- Very complex CTEs with multiple levels

**Mitigation**: For maximum security, also use database-level permissions (GRANT/DENY).

### Performance Impact

- Minimal: Regex patterns are pre-compiled
- Average overhead: < 1ms per query
- No impact on query execution time

---

## Defense in Depth Strategy

For production environments, use multiple layers:

1. **Application Level** (this feature)
   - Whitelist validation
   - Query parsing
   - Security logging

2. **Database Level**
   - SQL Server user permissions (GRANT/DENY)
   - Row-level security
   - Audit logging

3. **Network Level**
   - Firewall rules
   - VPN/Private endpoints
   - IP whitelisting

---

## Troubleshooting

### Issue: All queries blocked

**Cause**: Empty whitelist or whitelist not set

**Solution**:
```bash
# Check configuration
echo $MSSQL_WHITELIST_TABLES

# Set whitelist
export MSSQL_WHITELIST_TABLES="temp_ai,v_temp_ia"
```

### Issue: Legitimate query blocked

**Cause**: Query references non-whitelisted table

**Solution**: Check security logs to see which table triggered the block:
```
[SECURITY] Permission check - Operation: UPDATE, Tables found: [temp_ai information_schema], Whitelist: [temp_ai]
```

Add the table to whitelist if appropriate, or modify query to avoid referencing it.

### Issue: Parser not detecting table

**Cause**: Complex SQL syntax not covered by regex patterns

**Solution**:
1. Simplify query structure
2. Report issue for parser improvement
3. Use database-level permissions as backup

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/your-repo/mcp-go-mssql/issues
- Security concerns: Report privately via GitHub Security

---

**Version**: 1.0.0
**Last Updated**: 2025-10-11
**Author**: MCP-Go-MSSQL Security Team
