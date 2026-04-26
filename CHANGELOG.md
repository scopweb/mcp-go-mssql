# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security — Destructive Operation Confirmation

- 🛡️ **Confirmation required for destructive DDL**: Operations that modify or destroy existing database objects (`ALTER VIEW`, `DROP TABLE`, `DROP VIEW`, `DROP PROCEDURE`, `DROP FUNCTION`, `ALTER TABLE`, `TRUNCATE TABLE`) now require explicit user confirmation before execution
- 🔐 **New `confirm_operation` tool**: Dedicated MCP tool to confirm pending destructive operations using a token received from the warning response
- ⏱️ **Token-based confirmation**: Each destructive operation generates a unique confirmation token (32-char hex, crypto/rand) valid for 5 minutes
- 📋 **Object existence check**: Confirmation is only required when the target object already exists — `CREATE TABLE new_table` proceeds without confirmation
- 🔒 **One-time use tokens**: Tokens are deleted after execution or expiration to prevent replay attacks
- 🌐 **`MSSQL_CONFIRM_DESTRUCTIVE` env var**: Set to `"false"` to disable confirmation (for CI/CD automation)
- 📊 **Security logging**: All destructive operation warnings and confirmations are logged via `secLogger.Printf`
- 🚀 **`MSSQL_AUTOPILOT` mode**: New autonomous AI mode for development workflows
  - Skips destructive DDL confirmation (ALTER VIEW, DROP TABLE, etc.)
  - Skips schema validation (tables/views don't need to exist)
  - Whitelist protection still active: only whitelisted tables can be modified
  - Ideal when AI needs full autonomy within a limited, controlled scope
  - Example: `MSSQL_AUTOPILOT=true` + `MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia`

### Security — AI-Powered Attack Hardening

- 🛡️ **Inline comment keyword obfuscation detection**: New `stripAllComments()` + `stripLineComments()` functions remove ALL SQL comments (not just leading ones), preventing attacks like `SEL/*x*/ECT` or `/*INS*/ INSERT` that hide keywords inside comments to evade keyword detection
- 🛡️ **CHAR()/NCHAR() concatenation blocking**: Detects `CHAR(83)+CHAR(69)+...` patterns used to dynamically build SQL keywords like `SELECT` or `INSERT` that bypass simple regex keyword matching. Now correctly ignores string literals (`'CHAR(83)'`) to avoid false positives
- 🛡️ **Forbidden table hints blocking**: `WITH (NOLOCK)`, `WITH (READUNCOMMITTED)`, `WITH (TABLOCK)`, `WITH (UPDLOCK)`, `WITH (HOLDLOCK)` are now blocked — NOLOCK enables dirty reads on production data
- 🛡️ **WAITFOR DELAY blocking**: Prevents timing attacks where an AI infers data existence by measuring query response delays (`IF (condition) WAITFOR DELAY '00:00:10'`)
- 🛡️ **OPENROWSET/OPENDATASOURCE blocking**: Prevents data exfiltration to external servers via linked server queries
- 🛡️ **Unicode bidirectional control character detection**: Blocks RTL override (`\u202E`), zero-width spaces (`\u200B`), and other invisible Unicode characters used to visually obscure keywords (e.g. `SEL\u202ECT` renders as `SELECt`)
- 🛡️ **Unicode homoglyph detection**: Detects non-Latin letters (Cyrillic `е`, Greek `ε`, etc.) that visually resemble ASCII letters and can be used to obfuscate keywords (`SEL\u0435CT` = `SELECT`). Includes `normalizeToASCII()` transliteration
- 🛡️ **Subquery exfiltration protection**: `validateSubqueriesForRestrictedTables()` validates that tables referenced inside subqueries `(SELECT secret FROM restricted)` are also whitelisted, preventing nested data extraction
- 🛡️ **execute_procedure now validates SQL content**: Stored procedure execution now routes through `executeSecureQuery()` which applies all security validations (keyword detection, hints, timing attacks, etc.), preventing malicious procedures from bypassing security through parameter manipulation
- 🛡️ **String literal preservation**: All pattern-matching functions now strip string literals (`'...'`, `"..."`) before analysis to avoid false positives on user data that happens to contain SQL keywords

### Security — Known Issues

- ✅ **Go stdlib vulnerabilities fixed**: Go 1.26.2 addresses all four vulnerabilities (GO-2026-4947, GO-2026-4946, GO-2026-4870, GO-2026-4866) in `crypto/x509` and `crypto/tls`. Project now requires **Go 1.26.2+**. Run `go install golang.org/dl/go1.26.2@latest && go1.26.2 download` to update

### Dependencies

- 📦 **Go 1.26.2** (released 2026-04-07): Security release fixing 4 stdlib vulnerabilities in `crypto/x509` and `crypto/tls`. Update via `go install golang.org/dl/go1.26.2@latest && go1.26.2 download`
- 📦 **Updated**: `golang.org/x/crypto` v0.49.0 → **v0.50.0**
- 📦 **Updated**: `golang.org/x/mod` v0.34.0 → **v0.35.0**
- 📦 **Updated**: `golang.org/x/text` v0.35.0 → **v0.36.0**

### Tests

- 🧪 **AI attack vector tests**: `TestAIAttackVectors` — 20 test cases covering CHAR concatenation, NOLOCK hints, WAITFOR timing attacks, OPENROWSET exfiltration, Unicode bidirectional control characters, and false-positive prevention with string literals
- 🧪 **Unicode obfuscation tests**: Tests for RTL override, zero-width space, and homoglyph detection

### Added

- 🔌 **Dynamic Multi-Connection Mode (`MSSQL_DYNAMIC_MODE`)**:
  - New optional mode allows connecting to multiple databases from a single MCP server instance
  - Environment variables: `MSSQL_DYNAMIC_MODE=true` (default: false), `MSSQL_DYNAMIC_MAX_CONNECTIONS=10`
  - Three new tools: `dynamic_connect`, `dynamic_list`, `dynamic_disconnect`
  - `query_database` gains optional `connection` parameter to specify which dynamic connection to use
  - **Security-first design**: All credentials stored in `.env` with prefix `MSSQL_DYNAMIC_<ALIAS>_`
  - AI only sees connection aliases (server/database names) — NO passwords or users exposed
  - Per-connection security config: `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY`, `MSSQL_DYNAMIC_<ALIAS>_WHITELIST_TABLES`, `MSSQL_DYNAMIC_<ALIAS>_AUTOPILOT`
  - Example `.env`:
    ```
    MSSQL_DYNAMIC_IDENTITY_SERVER=10.203.3.11
    MSSQL_DYNAMIC_IDENTITY_DATABASE=JJP_CRM_IDENTITY
    MSSQL_DYNAMIC_IDENTITY_USER=ppp
    MSSQL_DYNAMIC_IDENTITY_PASSWORD=ppppp
    ```
  - Claude Desktop config needs only `MSSQL_DYNAMIC_MODE=true` — no credentials in JSON

### Added
- 🌐 **Cross-database queries (`MSSQL_ALLOWED_DATABASES`)**:
  - New environment variable `MSSQL_ALLOWED_DATABASES` (comma-separated) allows querying multiple databases from a single MCP connector
  - Enables 3-part name queries: `SELECT * FROM OtherDB.dbo.TableName`
  - Schema validation checks tables exist in the target database (queries `[OtherDB].INFORMATION_SCHEMA.TABLES`)
  - Cross-database modifications are **always blocked** (security: read-only across databases, even with whitelist)
  - `explore` tool gains new `database` parameter to list tables/views in allowed cross-databases
  - `get_database_info` shows configured cross-database access
  - Clear error messages when referencing non-allowed databases, with list of allowed ones
  - Regex-based table extraction updated to parse 3-part names (`database.schema.table`) correctly — fixes false "table not found" errors for qualified references like `dbo.TableName`

### Added
- 🛡️ **Best-effort schema validation for `query_database`**:
  - Before executing a query, the server validates that all referenced tables/views actually exist in the database
  - If a table doesn't exist, returns an error with "Did you mean?" suggestions using Levenshtein distance + prefix/substring matching
  - **Graceful degradation**: if the connection lacks `INFORMATION_SCHEMA` permissions, validation is silently skipped and the query executes normally
  - Prevents AI clients from inventing table/column names — a common issue where LLMs fabricate plausible-sounding names instead of checking the real schema
  - Zero overhead for AI clients: validation is server-side within the same tool call, no extra tokens consumed
  - Error messages guide the AI to use `explore`/`inspect` tools for schema discovery

### Added
- 📋 **Content annotations** (MCP spec 2025-11-25 SHOULD):
  - All `ContentItem` responses now include `annotations` with `audience` and `priority` fields
  - `audience` controls who sees content: `["assistant"]` for LLM-only diagnostics, `["user", "assistant"]` for shared results
  - `priority` differentiated per tool: `execute_procedure` (0.8) > `query_database` (0.7) > `inspect` (0.5) > `explore` (0.4) > `explain_query`/`get_database_info` (0.3) > errors (1.0)
  - Enables Claude Desktop to filter content visibility between user and LLM
  - 7 reusable annotation presets: `annAssistantLow`, `annAssistantHigh`, `annBothExplore`, `annBothInspect`, `annBothQuery`, `annBothProcedure`, `annBothExplain`, `annBothHigh`

### Fixed
- 🐛 **JSON-RPC 2.0: notifications with ID now get a response**: `notifications/cancelled` and `notifications/initialized` with an `id` field (technically a request per JSON-RPC 2.0) now return a response instead of being silently dropped
- 🐛 **DB connection closed on shutdown**: `server.db` is now explicitly closed when the stdio loop ends, preventing resource leaks
- 🐛 **`ServerInfo.Name` is now stable**: Changed from dynamic `"mcp-go-mssql (connected)"` / `"mcp-go-mssql (disconnected)"` to fixed `"mcp-go-mssql"` — clients can now reliably use the name for server identification
- 🐛 **Removed top-level `_meta` from JSON-RPC messages**: `_meta` was incorrectly placed at the message level in `MCPRequest`/`MCPResponse` structs; per spec it belongs inside `params`/`result`. Removed from top level (was unused). `CallToolResult._meta` remains correct

### Changed
- 🧪 **New tests**: `TestMCPCancelledWithIDReturnsResponse`, `TestMCPInitializedWithIDReturnsResponse`, `TestMCPContentAnnotations` (3 sub-tests verifying audience and priority per tool)

---

### Added
- 📋 **MCP spec compliance** (spec 2025-11-25):
  - **`ping` handler** (MUST): Server now responds to `ping` with empty `{}` result as required by spec.
  - **Tool annotations**: All 6 tools now declare `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` annotations to help clients decide when to prompt for confirmation.
  - **`logging` capability**: Server declares `logging: {}` capability in initialize response.
  - **`logging/setLevel` handler**: Accepts client log level requests and applies them to the slog logger via dynamic `slog.LevelVar`.
  - **`notifications/cancelled` handler**: Gracefully handles cancellation notifications from clients.
  - **`instructions` field**: Initialize response now includes usage instructions for LLMs.
  - **`ServerInfo.title`**: Human-readable display name ("MSSQL Database Connector") for client UIs.
  - **`ToolsListResult.nextCursor`**: Pagination support for `tools/list` responses.
  - **`tools/call` error handling**: Invalid params now return `-32602 Invalid params` instead of silently proceeding with empty params.
  - **`-32600 Invalid Request` validation**: Messages with missing or incorrect `jsonrpc` field now return proper JSON-RPC error.
  - **Tool `title` field**: All 6 tools now include human-readable titles for client UI display.
  - **MCP spec compliance tests**: 14 new tests covering ping, capabilities, annotations, logging level changes, tool titles, cancellation, and error handling.
- 🛡️ **Rate limiter for MCP tool calls**: Token-bucket rate limiter (60 calls/minute) prevents resource exhaustion. Returns a tool execution error with `isError: true` when limit exceeded, as recommended by MCP spec 2025-11-25.
- 🧪 **Rate limiter tests**: Basic, reset, concurrent (atomic + goroutines), and tool-call integration tests for the rate limiter.
- 🧪 **Fuzzing tests**: 7 fuzz functions covering `validateBasicInput`, `validateReadOnlyQuery`, `sanitizeForLogging`, `stripLeadingComments`, `extractOperation`, `extractAllTablesFromQuery`, `validateTablePermissions`.
- 🔧 **`MSSQL_ENCRYPT` environment variable**: New option to control TLS encryption independently in development mode. When `DEVELOPER_MODE=true`, encryption now defaults to `false` (previously defaulted to `true`). This is **required for SQL Server 2008/2012** which don't support TLS 1.2 — without this the Go driver fails the TLS handshake. In production mode (`DEVELOPER_MODE=false`), encryption is always enforced regardless of this setting.

### Changed
- 🏗️ **Migrated `SecurityLogger` to `log/slog`**: Structured JSON logging via stdlib `log/slog` (Go 1.21+) replaces the old `log.Logger` wrapper. Logs now include `component`, `success` fields for machine-parseable audit trails.
- ⚡ **Cached server config at startup**: `MSSQL_READ_ONLY`, `MSSQL_WHITELIST_TABLES`, `MSSQL_WHITELIST_PROCEDURES` are now read once at startup and cached in `serverConfig` struct — eliminates `os.Getenv()` calls on every request.
- 🔒 **Goroutine lifecycle management**: DB connection goroutine now uses `context.Context` for cancellation and `sync.WaitGroup` for clean shutdown.
- 📏 **Scanner buffer limit**: stdin scanner now has an explicit 4MB buffer limit to prevent DoS via oversized lines.
- 🔧 **Extracted `stripLeadingComments` helper**: Deduplicated SQL comment-stripping logic from 3 locations into a single reusable function.
- 🔧 **Error wrapping with `%w`**: All `fmt.Errorf("...: %v", err)` in claude-code and pkg connectors now use `%w` for proper error chain propagation.
- 🔧 **Custom connection string timeout enforcement**: `claude-code/` and `pkg/connector/` now append `connection timeout=30;command timeout=30` to custom connection strings if missing (matching main.go behavior).
- 🧪 **Test improvements**: Migrated `os.Setenv` to `t.Setenv()` (Go 1.17+), tests use cached `serverConfig` instead of env vars, cleaned up informational-only security tests into real assertions.
- 🤖 **Improved error messages for AI/LLM interpretation**: All error responses are now designed to help Claude (and other LLMs) diagnose and resolve issues autonomously:
  - **`get_database_info` when disconnected**: Now shows full configuration dump (server, database, auth mode, encrypt, port, Windows user) plus a "Possible Causes" diagnostic section with specific fixes (missing env vars, TLS incompatibility, auth mode issues, firewall)
  - **"Database not connected" errors**: All tools now instruct Claude to call `get_database_info` for diagnosis instead of showing a generic message
  - **Production query errors**: Instead of bare "query preparation failed", now include actionable hints ("check SQL syntax, table/column names, and permissions. Use explore tool to verify table exists")

### Fixed
- 🐛 **Missing `port` in Windows Integrated Auth connection string** (`main.go`): The `buildSecureConnectionString()` function omitted the `port` parameter when building connection strings for `MSSQL_AUTH=integrated`, causing connections to fail if the server uses a non-default port.
- 🐛 **Hardcoded `encrypt=true` in `pkg/connector/db-connector.go`**: Windows Integrated Auth connection string had `encrypt=true` hardcoded, ignoring `DEVELOPER_MODE` and `MSSQL_ENCRYPT` settings. Now respects both variables consistently across all three connector files.
- 🐛 **Wrong `encrypt` value in `claude-code/db-connector.go`**: The integrated auth branch used `trustCert` value for the `encrypt` parameter instead of a separate encrypt variable. Now correctly uses `MSSQL_ENCRYPT` override.
- 🐛 **`MSSQL_DATABASE` required for integrated auth in `pkg/connector`**: Unlike `main.go` and `claude-code`, the pkg connector required `MSSQL_DATABASE` even for Windows Auth. Now optional, consistent with the other connectors.
- 🐛 **JSON-RPC `-32700` Parse error**: Invalid JSON now returns a proper `-32700` Parse error response instead of silently ignoring malformed input, per JSON-RPC 2.0 spec.
- 🐛 **`_meta` field support**: Added `_meta` field to `MCPRequest`, `MCPResponse`, and `CallToolResult` structs per MCP spec 2025-11-25. Allows clients and servers to pass protocol metadata (progress tokens, etc.).

### Changed
- ⚡ **Consistent encryption defaults across all connectors**: All three files (`main.go`, `claude-code/db-connector.go`, `pkg/connector/db-connector.go`) now share the same logic: dev mode defaults `encrypt=false` and `trustservercertificate=true`, with `MSSQL_ENCRYPT` override available.

### Documentation
- 📚 **`MSSQL_ENCRYPT`** documented in `CLAUDE.md` optional variables section
- 📚 **Legacy SQL Server example** added to `CLAUDE.md` configuration examples (SQL 2008/2012 + Windows Auth)
- 📚 **`.env.example`** updated with legacy SQL Server example and `MSSQL_ENCRYPT` documentation

---

### Added
- 👁️ **`explore` tool: new `type=views`**: Lists only database views with rich metadata — `schema_name`, `view_name`, `check_option`, `is_updatable`, and a 300-char `definition_preview`. Supports optional `filter` parameter (LIKE match on name). Complements `type=tables` which lists both tables and views.
- 🔗 **`inspect` tool: new `detail=dependencies`**: Shows which SQL objects (views, procedures, functions) reference a given table using `sys.sql_expression_dependencies`. Returns `referencing_schema`, `referencing_object`, `referencing_type`, `is_caller_dependent`, `is_ambiguous`. Also included in `detail=all` output. Useful for impact analysis before schema changes.
- 🔍 **New `explain_query` tool**: Shows the estimated SQL Server execution plan for a SELECT query **without executing it**. Uses `SET SHOWPLAN_TEXT ON` on a dedicated connection to isolate the session. Always enforces SELECT-only validation (`extractOperation`) regardless of `MSSQL_READ_ONLY` mode. Useful for query performance analysis with Claude.

### Changed
- 📦 **Dependency update** (2026-03-06):
  - `github.com/microsoft/go-mssqldb` v1.9.4 → **v1.9.8** (bugfixes and driver improvements)
  - `golang.org/x/crypto` v0.45.0 → **v0.48.0** (security patches)
  - `golang.org/x/text` v0.31.0 → **v0.34.0**
  - `github.com/golang-jwt/jwt/v5` v5.3.0 → **v5.3.1**
  - Added `github.com/shopspring/decimal v1.4.0` (new transitive dep from go-mssqldb v1.9.8 for decimal precision)
  - `govulncheck ./...` → **No vulnerabilities found** after update

---

### Changed
- ♻️ **Tool API consolidated: 10 → 5 tools** (breaking change for MCP clients):
  - New `explore` tool replaces `list_tables`, `list_databases`, `list_stored_procedures`, and `search_objects`. Uses `type` parameter: `tables` (default), `databases`, `procedures`, `search`
  - New `inspect` tool replaces `describe_table`, `get_indexes`, and `get_foreign_keys`. Uses `detail` parameter: `columns` (default), `indexes`, `foreign_keys`, `all`
  - Kept unchanged: `query_database`, `get_database_info`, `execute_procedure`
  - Reduces cognitive overhead for LLMs — fewer tool choices, same coverage
- 🔧 **Go 1.26 upgrade**: Updated `go.mod` from `go 1.24.0 / toolchain go1.24.7` to `go 1.26.0`

### Documentation
- 📚 **Versioned website docs**: Added collapsed "Versión anterior (v1)" sidebar section preserving old tool pages (`list_tables`, `describe_table`, `list_databases`, `get_indexes`, `get_foreign_keys`, `list_stored_procedures`)
- Added deprecation banners on all v1 tool pages pointing to the new unified tools
- New `explore` and `inspect` pages (ES + EN) with "Reemplaza a (v1)" tip notices
- Updated tool overview/resumen pages (ES + EN) with 5-tool schema table

---

### Added
- 🔍 **New `search_objects` tool**: Search SQL objects (tables, views, stored procedures, functions) by name pattern OR by text inside their definition body. Two modes:
  - `search_in=name` (default): searches `sys.objects` by name using a LIKE pattern — fast single-query alternative to `list_tables` + manual inspection
  - `search_in=definition`: searches `sys.sql_modules` definition text — finds all procedures/functions/views that reference a specific table, column, or keyword in their source code
- 🔎 **`filter` parameter for `list_tables`**: Optional name filter (case-insensitive LIKE) to return only tables/views whose name contains the given string (e.g. `filter="Pedido"`)

### Fixed
- 🐛 **Bug #4: Token overflow — "No se pudo generar completamente la respuesta de Claude"**: `executeSecureQuery` had no row limit, causing `list_tables` (and other tools) to return hundreds of rows as a massive JSON blob that exceeded Claude's context token limit on large databases. Fixed by adding a global `maxQueryRows = 500` constant: all queries are now capped at 500 rows. If truncated, the last result element contains a `_truncated` warning key instructing the LLM to narrow the query with `WHERE` or `TOP`. Documented in `docs/bugs/bug4.md`.

### Changed
- ⚡ **Global row limit on all queries**: `executeSecureQuery` now stops reading at 500 rows and appends a `_truncated` sentinel row if more were available, preventing token overflows on large result sets
- 🔧 **New tools for database exploration**:
  - `list_databases`: List all user databases on the SQL Server instance
  - `get_indexes`: Get indexes for a specific table (with schema support)
  - `get_foreign_keys`: Get foreign key relationships for a table (incoming and outgoing)
  - `list_stored_procedures`: List all stored procedures (with optional schema filter)
  - `execute_procedure`: Execute whitelisted stored procedures (requires `MSSQL_WHITELIST_PROCEDURES` env var)
- 🔐 **Schema support for describe_table**: Now supports `schema.table` format and optional `schema` parameter (defaults to `dbo`)
- ✅ **Safe system procedures in read-only mode**: Added whitelist of safe read-only system procedures (`sp_help`, `sp_helptext`, `sp_helpindex`, `sp_columns`, `sp_tables`, `sp_fkeys`, `sp_pkeys`, `sp_databases`, etc.) that are now allowed in read-only mode
- 🧪 **New tests**: `TestProcedureNameValidation` for stored procedure name sanitization, `TestPerformanceOptimizations` now validates pre-compiled table extraction patterns, and read-only false positive regression tests (`created_at`, `update_count`, `deleted` columns)

### Fixed
- 🐛 **Bug #1: Connection fails with DEVELOPER_MODE=false**: Fixed TLS certificate requirement issue when `DEVELOPER_MODE=false` (production mode). The connection string was forcing `encrypt=true` and `trustservercertificate=false`, which required valid TLS certificates that internal servers typically don't have. Documented in `docs/bugs/bug1.md`. Workaround: Use `DEVELOPER_MODE=true` for internal servers without TLS certificates.
- 🐛 **Bug #2: READ_ONLY mode blocks whitelisted tables**: Fixed integration conflict between `MSSQL_READ_ONLY=true` and `MSSQL_WHITELIST_TABLES`. Previously, `validateReadOnlyQuery()` would block ALL modifications before `validateTablePermissions()` could check the whitelist. Now when whitelist is configured, modifications are allowed to pass through for whitelist validation. Configuration `READ_ONLY=true` + `WHITELIST=table1` now works correctly, allowing modifications only on whitelisted tables while blocking all others. Enhanced `get_database_info` messages to clearly show "READ-ONLY with whitelist exceptions" when both are configured. Documented in `docs/bugs/bug2.md`.
- 🐛 **Schema detection in table extraction**: Fixed regex patterns to correctly detect `schema.table` and `[schema].[table]` formats in SQL queries for whitelist validation
- 🐛 **describe_table schema filtering**: Now properly filters by both schema and table name to avoid returning columns from tables with same name in different schemas
- 🐛 **Test files in wrong directory**: Moved `test/main_test.go` to root package; deleted duplicate `test/main_permissions_test.go`. Tests now compile and run correctly
- 🐛 **Wrong expected tools count in tests**: `TestMCPToolsList` now expects all 9 tools instead of the outdated count of 4
- 🐛 **Read-only validation false positives**: `validateReadOnlyQuery` now uses word-boundary regex (`\bINSERT\b`, `\bUPDATE\b`, etc.) instead of `strings.Contains`. Queries like `SELECT created_at FROM t` or `SELECT update_count FROM t` are no longer incorrectly blocked

### Changed
- 🔒 **Improved SP security filtering**: Instead of blocking all `sp_` and `xp_` prefixes, now uses a more granular approach with explicit dangerous/safe lists for system procedures
- ⚡ **Pre-compiled table extraction regexes**: Moved 9 regex patterns from `extractAllTablesFromQuery` to package-level `tableExtractionPatterns` var, avoiding recompilation on every call
- 🔒 **Reduced env var logging exposure**: Replaced blanket logging of all `MSSQL_*` environment variables with an explicit safe-list (`MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_PORT`, `MSSQL_AUTH`, `MSSQL_READ_ONLY`, `MSSQL_WHITELIST_TABLES`, `DEVELOPER_MODE`). `MSSQL_PASSWORD` and `MSSQL_CONNECTION_STRING` are no longer logged

### Security
- Added `MSSQL_WHITELIST_PROCEDURES` environment variable for granular control over which stored procedures can be executed via `execute_procedure` tool
- Dangerous system procedures (`xp_cmdshell`, `sp_configure`, `sp_executesql`, etc.) are explicitly blocked even if they bypass other checks
- 🔒 **Race condition on `server.db` fixed**: Added `sync.RWMutex` to `MCPMSSQLServer` with `getDB()`/`setDB()` accessors. The database connection is now set from the background goroutine and read from request handlers without data races
- 🔒 **Custom connection string validation**: `MSSQL_CONNECTION_STRING` is now validated — warns in production if `encrypt=false`, `encrypt` is missing, or `trustservercertificate=true`. Default timeouts (`connection timeout=30;command timeout=30`) are appended if absent
- 🔒 **Procedure name sanitization**: `execute_procedure` now validates procedure names with regex `^[\w.\[\]]+$` before string concatenation into `EXEC` statement, rejecting names with semicolons, spaces, quotes, or other injection vectors
- 🔧 **Go 1.24.11 recommended**: Addresses GO-2025-4175 and GO-2025-4155 (`crypto/x509` vulnerabilities in wildcard DNS name constraints and host certificate error printing)

---

## [Previous Unreleased]

### Added
- 🔐 **Windows Integrated Authentication (SSPI) support**: Added `MSSQL_AUTH` environment variable to allow selection of authentication mode; supports `sql` (default) and `integrated`/`windows` (SSPI) for Windows-based integrated authentication. When `MSSQL_AUTH=integrated` the server will build a connection string with `integrated security=SSPI` and will not require `MSSQL_USER` or `MSSQL_PASSWORD`.
  - `MSSQL_DATABASE` is now **optional** with integrated authentication - if omitted, connects to the Windows user's default database
  - Supports local servers (`localhost`, `.`, `(local)`) and remote domain servers
  - Uses Windows credentials automatically - perfect for Active Directory environments
  - No passwords in configuration files - more secure credential management
- 📝 **Enhanced logging for integrated auth**: Added detailed diagnostic logs showing:
  - Current Windows user running the process
  - Authentication mode being used (SQL vs Integrated)
  - Database connection status with specific troubleshooting tips for Windows auth failures
- 📝 **Documentation & examples updated**: `.env.example` and `README.md` updated to document `MSSQL_AUTH` with multiple configuration examples for integrated authentication scenarios.
- 🧪 **Tools & tests**: `tools/debug/debug-connection.go` and `tools/test/test-connection.go` updated for `MSSQL_AUTH`; added a unit test case for `integrated` auth in `test/main_test.go`.
- 🔧 **Diagnostic scripts**: Added `scripts/test-integrated-auth.ps1` and `scripts/view-logs.ps1` to help troubleshoot Windows authentication issues.
- 📚 **Windows Auth Guide**: Added `WINDOWS_AUTH_GUIDE.md` with comprehensive Named Pipes configuration and troubleshooting.

### Changed
- 🔄 **Named Pipes for Windows Auth**: Windows Integrated Authentication now uses Named Pipes protocol instead of TCP/IP. This allows authentication to work without requiring TCP to be enabled in SQL Server Configuration Manager. Works with both local (`.`) and remote server names.
  - `main.go`: Updated `buildSecureConnectionString()` to use Named Pipes for SSPI
  - `claude-code/db-connector.go`: Updated `connectDatabase()` to use Named Pipes for Windows Auth
  - Eliminates the need to enable TCP/IP protocol for Windows Auth scenarios
  - Tested successfully with SQL Server 2022 on Windows 10
- 🗄️ **Optional Database with Windows Auth**: Made `MSSQL_DATABASE` optional for Windows Integrated Authentication
  - When `MSSQL_DATABASE` is not specified with Windows Auth, users can access all databases they have permissions for
  - Connection string is built dynamically: with database parameter when specified, without when omitted
  - Enables multi-database exploration while maintaining single-database focus option
  - Allows queries across databases using fully qualified names: `SELECT * FROM DatabaseName.schema.table`
  - Useful for development and analysis scenarios with Windows credentials


## [1.2.0] - 2025-11-21

### Added
- 🤖 **AI Usage Guide**: Comprehensive documentation for using with Claude Desktop and AI assistants
  - Added `docs/AI_USAGE_GUIDE.md` with detailed examples
  - Explains what AI can and cannot do with security restrictions
  - Includes real conversation examples with Claude
  - Three configuration scenarios: Analytics, AI-safe, Development
- 🔒 **Security Analysis Report**: Complete security threat assessment
  - Added `docs/SECURITY_ANALYSIS.md` with detailed analysis
  - Covers all 5 major security threats (SQL Injection, Auth Bypass, etc.)
  - Risk matrix and mitigation strategies
  - Production-ready certification
- 🛡️ **Automated Security Validation**: PowerShell script for continuous security checks
  - Added `scripts/security-check.ps1` with 12 automated tests
  - Validates prepared statements, TLS encryption, log sanitization
  - Checks for hardcoded credentials and dangerous patterns
  - Exit codes for CI/CD integration
- 🧪 **Comprehensive Security Test Suite**: Unit tests for security vulnerabilities
  - Added `test/security/` directory with CVE and security tests
  - 16 security tests covering SQL injection, path traversal, command injection
  - Tests for known CVEs in dependencies
  - Cryptography and memory safety checks

### Changed
- 📦 **Updated Dependencies**: All dependencies to latest secure versions
  - `golang.org/x/crypto` → v0.45.0 (from v0.43.0)
  - `golang.org/x/text` → v0.31.0 (from v0.30.0)
  - Added `github.com/stretchr/testify` v1.11.1 for testing
- 📚 **Enhanced Documentation**: README with AI-first messaging
  - Added prominent section highlighting AI assistant support
  - Added documentation index for easy navigation
  - Updated project structure documentation
  - Improved quick-start guides
- 🔧 **Merged Security Branch**: Integrated `claude/add-readonly-database-mode` improvements
  - Security enhancements from dedicated security branch
  - Dependency updates and fixes

### Security
- ✅ **SQL Injection Protection**: 100% mitigated with prepared statements
- ✅ **Authentication Bypass Protection**: TLS encryption + credential validation
- ✅ **Connection String Exposure Protection**: Automatic log sanitization
- ✅ **Command Injection Protection**: Blacklist for dangerous SQL commands
- ✅ **Path Traversal**: Not applicable (no file system operations)
- ✅ **All Security Tests Passing**: 16/16 tests pass, 12/12 validation checks pass
- ✅ **Production Ready**: Security score EXCELLENT, ready for production use

### Documentation
- Added AI usage guide with 9 detailed sections
- Added security analysis with threat assessment
- Added automated security validation script
- Updated README with AI-centric positioning
- Improved navigation and documentation structure

## [1.1.1] - 2024-11-01

### Security
- ✅ **Vulnerability Audit Complete**: Passed comprehensive security scans
  - `govulncheck ./...` - No vulnerabilities detected in dependencies
  - `gosec ./...` - Security analysis completed, all critical findings addressed
- Code reorganization: Moved build artifacts, scripts, and documentation to dedicated directories
- Updated `.gitignore` to reflect new project structure

### Changed
- **Project Structure Refactor**: Organized repository for better maintainability
  - `/docs/` - All documentation files
  - `/scripts/` - Build and test scripts with unified interface
  - `/build/` - Compiled binaries
  - `/config/` - Configuration templates
  - `/test/` - Test files and utilities
- Root directory now contains only essential files: Go modules, main source, and documentation
- **Build Scripts Updated**:
  - `scripts/build.bat` - Windows build (outputs to `build/mcp-go-mssql.exe`)
  - `scripts/build.sh` - Linux/macOS build (outputs to `build/mcp-go-mssql`)
- **Added**: `scripts/README.md` - Complete guide for all build and test scripts

## [1.1.0] - 2024-11-01

### Added
- Granular table permissions with whitelist system for read-only mode
- Security audit logging with data sanitization
- `MSSQL_WHITELIST_TABLES` environment variable for selective table modification in read-only mode
- Comprehensive security scanning in CI pipeline (govulncheck, gosec)

### Changed
- Updated Go version to 1.24.9 to address stdlib security fixes
- Updated Microsoft official SQL Server driver (github.com/microsoft/go-mssqldb)
- Improved error handling to address security findings
- Enhanced connection security with mandatory TLS encryption

### Fixed
- SQL syntax error in describe_table tool
- Missing describe_table and list_tables tools in MCP server
- Unused imports in db-connector.go

### Security
- Implemented mandatory TLS encryption for all database connections
- Added granular permission validation for modification queries
- Enhanced security logging for audit purposes
- Strict input validation and SQL injection protection

## [1.0.0] - 2024-10-02

### Added
- Initial release of secure Go-based MCP server for Microsoft SQL Server
- Model Context Protocol (MCP) implementation for Claude Desktop integration
- Claude Code CLI tool for direct database access
- Database connection pooling and resource management
- Comprehensive security features:
  - TLS encryption support
  - Connection timeouts
  - Input validation
  - SQL injection protection with prepared statements
- Read-only mode support with environmental configuration
- Development and production modes with appropriate certificate validation
- Extensive documentation and security guidelines
- Test utilities and examples
- GitHub community features and discoverability
- Support for legacy SQL Server versions

### Features
- Secure database connectivity to Microsoft SQL Server
- Support for multiple connection profiles
- Query execution with prepared statements
- Table structure inspection
- Connection testing utilities
- Comprehensive logging and monitoring

---

For a detailed list of changes, commits, and authors, see the [Git commit history](https://github.com/your-repo/mcp-go-mssql/commits/master).
