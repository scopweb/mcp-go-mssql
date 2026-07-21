---
title: SQL Injection Protection
description: How MCP-Go-MSSQL prevents SQL injection attacks
---

SQL injection protection is built on exclusive prepared statements plus in-depth validation. Even so, the strongest boundary is **least-privilege database permissions**: a login with only the required grants makes the database engine itself reject whatever the validator misses.

## Protection mechanism

```go
// All queries use PrepareContext()
stmt, err := s.db.PrepareContext(ctx, query)
defer stmt.Close()
rows, err := stmt.QueryContext(ctx, args...)
```

### Defense in depth

1. **Mandatory prepared statements** — All queries use `PrepareContext()`
2. **Code and data separation** — Parameters are passed as separate arguments
3. **No SQL string concatenation** — The go-mssqldb driver handles escaping automatically

## Blocked attack example

```sql
-- Injection attempt:
SELECT * FROM users WHERE username = '1' OR '1'='1' --

-- With prepared statements, it's treated as a literal:
SELECT * FROM users WHERE username = '1'' OR ''1''=''1'' --'
```

## Additional protections

### Dangerous command blocking

In read-only mode, these are blocked:
- `EXEC` / `EXECUTE`
- `SP_` / `XP_` (dangerous system procedures)
- `OPENROWSET` / `OPENDATASOURCE`
- `BULK INSERT`
- `RECONFIGURE`

### Input validation

- Query size limit (1 MB by default, configurable with `MSSQL_MAX_QUERY_SIZE`)
- Empty input rejection
- Comment stripping that could hide commands

## Security tests

```bash
# Run the SQL injection test suite
go test -v -run TestSQLInjectionVulnerability ./test/security/...
```

The tests cover 6 different attack vectors, all successfully blocked.
