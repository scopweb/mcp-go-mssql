````markdown
# Security Enhancement: Granular Table Permissions with Whitelist

## Problem Statement

When using AI assistants (like Claude Desktop) with production databases through MCP, there's a significant security risk:

### Current Limitation

The existing `MSSQL_READ_ONLY=true` mode blocks **all** modification operations, which is too restrictive for AI workflows that need temporary workspace tables.

### Security Risks Without Whitelist

1. **Accidental Data Modification**: AI could accidentally modify production tables
2. **Malicious JOIN Queries**: Even with a "safe" target table, queries can access sensitive data:
	```sql
	-- Target table is "safe" but joins to sensitive data
	DELETE temp_ai FROM temp_ai
	INNER JOIN users ON temp_ai.id = users.id
	WHERE users.password = 'leaked'
	```

3. **Data Exfiltration via Subqueries**:
	```sql
	-- Appears to only modify temp table
	UPDATE temp_ai
	SET exported_data = (SELECT password FROM users WHERE id = 1)
	```

4. **INSERT...SELECT from Production Tables**:
	```sql
	-- Copies sensitive data to "safe" table
	INSERT INTO temp_ai
	SELECT * FROM customers WHERE credit_card IS NOT NULL
	```

## Proposed Solution

Implement a **whitelist-based permission system** that:

### Features Required

1. **Multi-table validation**: Parse and validate ALL tables referenced in a query (FROM, JOIN, subqueries, CTEs)
2. **Flexible whitelist**: Allow specific tables/views for modification via environment variable
3. **Comprehensive logging**: Log all permission checks and violations
4. **Zero false positives**: Don't break legitimate queries on whitelisted tables

### Configuration Example

```bash
# Enable read-only with whitelist
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia

# Result:
# ✅ SELECT * FROM any_table          → Allowed (read-only)
# ✅ UPDATE temp_ai SET col = 'val'   → Allowed (whitelisted)
# ❌ UPDATE users SET col = 'val'     → BLOCKED (not whitelisted)
# ❌ DELETE temp_ai FROM temp_ai JOIN users → BLOCKED (users not whitelisted)
```

## Use Case

**AI Assistant with Production Database Access**:
- AI needs to read all tables for analysis
- AI needs temporary workspace (`temp_ai`, `v_temp_ia`) for computations
- AI must NOT modify production data
- Protection against malicious or buggy queries

## Expected Behavior

### Allowed Operations
```sql
-- Read operations (always allowed in read-only mode)
SELECT * FROM production_table
SELECT * FROM production_table JOIN temp_ai ON ...

-- Modifications on whitelisted tables only
UPDATE temp_ai SET col = 'value'
INSERT INTO temp_ai VALUES (1, 'test')
DELETE FROM temp_ai WHERE id = 1
CREATE VIEW v_temp_ia AS SELECT * FROM temp_ai
```

### Blocked Operations
```sql
-- Modification on non-whitelisted table
UPDATE users SET password = 'hacked'
-- Error: permission denied: table 'users' is not whitelisted

-- JOIN to non-whitelisted table in modification query
DELETE temp_ai FROM temp_ai t1 INNER JOIN users t2 ON t1.id = t2.id
-- Error: permission denied: table 'users' is not whitelisted for DELETE operations

-- Subquery accessing non-whitelisted table
UPDATE temp_ai SET col = (SELECT password FROM users WHERE id = 1)
-- Error: permission denied: table 'users' is not whitelisted for UPDATE operations

-- INSERT...SELECT from non-whitelisted table
INSERT INTO temp_ai SELECT * FROM customers
-- Error: permission denied: table 'customers' is not whitelisted for INSERT operations
```

## Implementation Requirements

### Core Functionality
- [ ] SQL parser to extract ALL table names from queries
- [ ] Support for: FROM, JOIN, subqueries, CTEs, INSERT...SELECT
- [ ] Whitelist validation for modification operations (INSERT/UPDATE/DELETE/CREATE/DROP)
- [ ] Leave SELECT operations unrestricted (read-only access)

### Configuration
- [ ] Environment variable: `MSSQL_WHITELIST_TABLES` (comma-separated)
- [ ] Works in conjunction with `MSSQL_READ_ONLY=true`
- [ ] Empty whitelist = no modifications allowed

### Security Logging
- [ ] Log all permission checks
- [ ] Log denied operations with details
- [ ] Include operation type and all tables found

### Testing
- [ ] Unit tests for table extraction
- [ ] Unit tests for permission validation
- [ ] Integration tests for malicious query patterns
- [ ] Performance tests (minimal overhead)

## Additional Context

This feature implements **defense in depth** - it complements (not replaces) database-level permissions:

1. **Application Layer** (this feature): Whitelist validation
2. **Database Layer**: SQL Server GRANT/DENY permissions
3. **Network Layer**: Firewall rules, VPN

## Priority

**High** - This addresses a critical security gap when using AI assistants with production databases.

## Related

- Similar to PostgreSQL row-level security
- Inspired by AWS IAM resource-level permissions
- Complements existing `MSSQL_READ_ONLY` feature

---

**Environment**:
- SQL Server 2019+
- Go 1.21+
- MCP Protocol 2024-11-05

**Labels**: `enhancement`, `security`, `ai-safety`

``` 

`