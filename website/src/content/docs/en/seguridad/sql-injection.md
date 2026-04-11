---
title: SQL Injection Protection
description: How MCP-Go-MSSQL prevents SQL injection attacks, including AI-assisted techniques
---

SQL injection protection is absolute thanks to the exclusive use of prepared statements.

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

## Additional AI-specific protections

Beyond prepared statements, the server implements validations specific to techniques an AI can use to evade detection:

### Dangerous command blocking

In read-only mode, these are blocked:
- `EXEC` / `EXECUTE`
- `SP_` / `XP_` (dangerous system procedures)
- `OPENROWSET` / `OPENDATASOURCE` — prevents data exfiltration to external servers
- `BULK INSERT`
- `RECONFIGURE`

### CHAR() concatenation detection

Prevents an AI from dynamically building SQL keywords:

```sql
-- Blocked:
CHAR(83)+CHAR(69)+CHAR(76)+CHAR(69)+CHAR(67)+CHAR(84) * FROM users
```

### Inline comment detection

Prevents keywords from being hidden inside comments:

```sql
-- Blocked:
SEL/*x*/ECT * FROM users
/*INS*/ INSERT INTO users VALUES (1)
```

### Dangerous table hint detection

Prevents dirty reads and other non-standard behaviors:

```sql
-- Blocked:
SELECT * FROM users WITH (NOLOCK)
SELECT * FROM users WITH (READUNCOMMITTED)
SELECT * FROM users WITH (TABLOCK)
```

### WAITFOR detection

Prevents timing attacks where an AI infers data by measuring delays:

```sql
-- Blocked:
IF (SELECT COUNT(*) FROM users) > 0 WAITFOR DELAY '00:00:05'
```

### Unicode control character detection

Prevents obfuscation via bidirectional characters and zero-width spaces:

```sql
-- Blocked (RTL Override):
SELECT\u202E * FROM users

-- Blocked (Zero-width space):
SEL\u200BECT * FROM users
```

### Unicode homoglyph detection

Prevents Cyrillic/Greek characters from being used to mimic Latin letters:

```sql
-- Blocked (Cyrillic 'е' = Latin 'e'):
SEL\u0435CT * FROM users
```

### Subquery whitelist validation

Prevents access to restricted tables through nested subqueries:

```sql
-- Blocked if "secrets" is not in whitelist:
SELECT * FROM (SELECT secret FROM secrets) AS x
```

## Input validation

- Query size limit (1MB default, configurable)
- Empty input rejection
- String literal preservation — content of `'...'` is excluded from pattern matching to avoid false positives

## Security tests

```bash
# Run SQL injection and AI attack tests
go test -v -run TestSQLInjectionVulnerability ./test/security/...
go test -v -run TestAIAttackVectors ./test/security/...

# Vulnerability verification
govulncheck ./...
```

Tests cover 6+ traditional attack vectors and 20 AI-specific attack vectors, all successfully blocked.
