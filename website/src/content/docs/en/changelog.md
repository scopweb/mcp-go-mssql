---
title: Changelog
description: MCP-Go-MSSQL change history
---

All relevant changes to this project are documented here.

## Latest changes

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
