---
title: Changelog
description: MCP-Go-MSSQL change history
---

All relevant changes to this project are documented here.

## Latest change

### Corporate vault over HTTPS (2026-10-26)

- Passwords and complete DSNs from the vault API using an application key and `*_VAULT_SECRET` references, in classic and dynamic modes.
- One startup request, a 10-second timeout and a 1 MiB response limit. Private in-memory configuration without exporting secrets to the process environment.
- Trusted HTTPS is mandatory; redirects and ambient HTTP proxies are not used. Vault failures, missing secrets and ambiguous sources stop startup; rotation requires restart.
- IIS proxy path prefixes, host environment precedence and the `.env` next to the executable are covered in the [vault guide](/en/configuracion/vault-corporativo/).
- No new tools or SQL permission changes. The standalone CLI does not gain this integration.

### Recent contract and query changes

- A single read using `;WITH`, nested comments or delimiters inside literals is not mistaken for a batch. Real batches remain rejected.
- The server announces the write policy to the agent in `initialize` and write rejections.
- Optional columnar format, unified `explore` search, response budgets and three truncation indicators.
- Go 1.27.3 minimum; result budgets and read-only defaults are part of agent contract v2.

The complete history is in the repository [CHANGELOG.md](https://github.com/scopweb/mcp-go-mssql/blob/master/CHANGELOG.md).

## Earlier changes

### Absolute read-only and stacked SQL batches

- `MSSQL_READ_ONLY=true` no longer allows whitelist writes. Writing requires `READ_ONLY=false` and an explicit whitelist.
- Concatenated batches with or without `;`, comment-hidden `SELECT INTO`, and non-`sys`/`dbo` or mutating admin procedures are rejected.
- The CLI is read-only by default (`MSSQL_CLI_ALLOW_WRITE=true` for operators).
- Agent contract: `docs/AGENT_CONTRACT.md`.

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
