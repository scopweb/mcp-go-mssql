---
title: Changelog
description: MCP-Go-MSSQL change history
---

All relevant changes to this project are documented here.

## Latest changes

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
