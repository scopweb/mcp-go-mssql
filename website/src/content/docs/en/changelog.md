---
title: Changelog
description: MCP-Go-MSSQL change history
---

All relevant changes to this project are documented here.

## Latest changes

### Dynamic Multi-Connection Mode

When `MSSQL_DYNAMIC_MODE=true` is enabled, the server can connect to multiple databases from a single MCP instance. Connections are pre-configured in `.env` and the AI only sees safe aliases — **no sensitive data exposed**.

**New variables:**
- `MSSQL_DYNAMIC_MODE` (default: `false`) — Enable dynamic connections
- `MSSQL_DYNAMIC_MAX_CONNECTIONS` (default: `10`) — Maximum active connections

**New tools:** `dynamic_available`, `dynamic_connect`, `dynamic_list`, `dynamic_disconnect`

**Connection configuration (`.env`):**
```bash
MSSQL_DYNAMIC_IDENTITY_SERVER=10.203.3.11
MSSQL_DYNAMIC_IDENTITY_DATABASE=JJP_CRM_IDENTITY
MSSQL_DYNAMIC_IDENTITY_USER=ppp
MSSQL_DYNAMIC_IDENTITY_PASSWORD=ppppp
```

**Per-connection security:**
- `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY`
- `MSSQL_DYNAMIC_<ALIAS>_WHITELIST_TABLES`
- `MSSQL_DYNAMIC_<ALIAS>_AUTOPILOT`

**In Claude Desktop** you only need:
```json
{"MSSQL_DYNAMIC_MODE": "true"}
```
(no credentials in the JSON)

**Dual-mode architecture:**
- `MSSQL_SERVER` set → direct connection mode (legacy), `.env` NOT loaded, no dynamic tools
- `MSSQL_SERVER` not set → loads `.env`, enables dynamic tools if `MSSQL_DYNAMIC_MODE=true`

**`dynamic_available`** reads `.env` directly to discover available aliases

---

### Destructive operation confirmation

**New feature:** Confirmation system for DDL operations that modify or destroy existing objects.

**New variables:**
- `MSSQL_CONFIRM_DESTRUCTIVE` (default: `true`) — Require confirmation for `ALTER VIEW`, `DROP TABLE`, etc. on existing objects
- `MSSQL_AUTOPILOT` (default: `false`) — Autonomous mode: skips schema validation (does NOT skip destructive confirmation or READ_ONLY). Whitelist still active

**New tool:** `confirm_operation` — Confirm pending destructive operations with token.

**Operations requiring confirmation:**

| Operation | Target |
|-----------|--------|
| `ALTER VIEW` | Existing view |
| `DROP TABLE` | Existing table |
| `DROP VIEW` | Existing view |
| `DROP PROCEDURE` | Existing procedure |
| `DROP FUNCTION` | Existing function |
| `ALTER TABLE` | Existing table |
| `TRUNCATE TABLE` | Existing table |

**Tokens:**
- Generated with `crypto/rand` (32-char hex)
- Valid for 5 minutes
- Single-use (deleted after execution or expiration)
- Only for objects that **already exist** — `CREATE TABLE new_table` does not require confirmation

---

### Cross-database queries (`MSSQL_ALLOWED_DATABASES`)

**New variable:** `MSSQL_ALLOWED_DATABASES`
- Query multiple databases from a single MCP connector
- Format: comma-separated list, e.g.: `"OtherDB1,OtherDB2"`
- Enables 3-part name queries: `SELECT * FROM OtherDB.dbo.TableName`
- Schema validation checks tables exist in the target database
- Cross-database modifications are **always blocked** (security)

**Tool improvements:**
- `explore` accepts new `database` parameter to list tables in allowed databases
- `get_database_info` shows configured cross-databases
- Clear error messages when referencing a non-allowed database

**Regex fix:**
- Table name parser now supports 3-part names (`database.schema.table`)
- Fixes false "table not found" errors for qualified references like `dbo.TableName`

---

### MCP spec compliance (2025-11-25)

- **Content annotations**: all responses include `audience` and `priority` fields
- **Tool annotations**: `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` on every tool
- **Rate limiter**: 60 tool calls per minute (token bucket)
- **JSON-RPC 2.0**: strict validation, proper error codes (-32600, -32601, -32602, -32700)
- **`logging/setLevel`** handler for dynamic log level control
- **`ping`** handler for health checks
- **Clean shutdown** with graceful connection cleanup

---

### Best-effort schema validation for `query_database`

- Before executing a query, validates that all referenced tables/views exist in the database
- Parses table references from JOINs, subqueries, CTEs, and 3-part names
- "Did you mean?" suggestions using Levenshtein distance when a table is not found
- Silently skips validation if `INFORMATION_SCHEMA` is not accessible
- System schema objects (`INFORMATION_SCHEMA`, `sys`) are automatically excluded

---

### SQL Server 2008/2012 support and improved diagnostics

**New variable:** `MSSQL_ENCRYPT`
- Controls TLS encryption independently in development mode
- `MSSQL_ENCRYPT=false` is **required for SQL Server 2008/2012** which don't support TLS 1.2
- Only effective with `DEVELOPER_MODE=true`. In production, encryption is always enforced

**Connection fixes:**
- Added `port` to integrated auth connection string (was previously omitted)
- Fixed hardcoded `encrypt=true` in CLI and pkg connectors
- `MSSQL_DATABASE` is now optional for integrated auth across all connectors

**Improved diagnostics for Claude:**
- `get_database_info` when disconnected now shows: full configuration + possible causes + specific solutions
- All "Database not connected" errors guide Claude to use `get_database_info` for diagnosis
- Production query errors include actionable hints (check syntax, permissions, use `explore`)

---

### `inspect` — new `detail=dependencies`

- Shows which SQL objects (views, procedures, functions) **depend on a given table**
- Uses `sys.sql_expression_dependencies` for impact analysis
- Returns: `referencing_schema`, `referencing_object`, `referencing_type`
- Also included in `detail=all`
- Useful for assessing impact before schema changes

---

### `explore` — new `type=views`

- Lists only database **views** with rich metadata: `schema_name`, `view_name`, `check_option`, `is_updatable`, `definition_preview` (300 chars)
- Supports `filter` parameter for name filtering (LIKE)
- Complements `type=tables` which continues returning both tables and views

---

### New tool: `explain_query`

- Shows the **estimated execution plan** for a SELECT query without running it
- Uses `SET SHOWPLAN_TEXT ON` on a dedicated connection (isolated from pool)
- Only accepts SELECT — validation always enforced, regardless of `MSSQL_READ_ONLY`
- Useful for query performance analysis with Claude

---

### Dependency update (2026-03-06)

**Updated dependencies:**
- `github.com/microsoft/go-mssqldb` v1.9.4 → **v1.9.8** (driver bugfixes)
- `golang.org/x/crypto` v0.45.0 → **v0.48.0** (security patches)
- `golang.org/x/text` v0.31.0 → **v0.34.0**
- `github.com/golang-jwt/jwt/v5` v5.3.0 → **v5.3.1**
- New transitive dep: `github.com/shopspring/decimal v1.4.0` (decimal precision in go-mssqldb v1.9.8)

**Audit:** `govulncheck ./...` → No vulnerabilities found

---

### Documentation and stability

**New features:**
- Complete documentation site with Starlight (ES + EN)
- Go upgrade guide
- MCP integration roadmap
- scopweb visual theme with dark/light mode

**Fixes:**
- Resolved race condition in the connection pool
- Eliminated false positives in read-only mode validation
- Fixed compilation errors in the test suite

**Security:**
- Mandatory TLS encryption on all production connections
- SQL injection protection with exclusive prepared statements
- Read-only mode with granular table whitelist
- Multi-table validation covering JOINs, subqueries, and CTEs
- Security logging with automatic credential sanitization

**Infrastructure:**
- MIT license added
- Build scripts with consistent output to `build/` directory
- Internal references sanitized for publication

## Previous versions

### First release

- 9 MCP tools: query_database, list_tables, describe_table, get_database_info, list_databases, get_indexes, get_foreign_keys, list_stored_procedures, execute_procedure
- MCP server compatible with Claude Desktop via JSON-RPC 2.0
- CLI for Claude Code with test, info, tables, describe, query commands
- Support for SQL Server, Windows Integrated (SSPI), and Azure AD authentication
- Custom connection strings for special configurations
- Development mode with self-signed certificates and detailed errors
