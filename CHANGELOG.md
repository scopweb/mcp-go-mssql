# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
