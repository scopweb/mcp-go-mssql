# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 🔐 **Windows Integrated Authentication (SSPI) support**: Added `MSSQL_AUTH` environment variable to allow selection of authentication mode; supports `sql` (default) and `integrated`/`windows` (SSPI) for Windows-based integrated authentication. When `MSSQL_AUTH=integrated` the server will build a connection string with `integrated security=SSPI` and will not require `MSSQL_USER` or `MSSQL_PASSWORD`.
- 📝 **Documentation & examples updated**: `.env.example` and `README.md` updated to document `MSSQL_AUTH` and to show a sample configuration using integrated authentication.
- 🧪 **Tools & tests**: `tools/debug/debug-connection.go` and `tools/test/test-connection.go` updated for `MSSQL_AUTH`; added a unit test case for `integrated` auth in `test/main_test.go`.
- 📚 **Windows Auth Guide**: Added `WINDOWS_AUTH_GUIDE.md` with comprehensive Named Pipes configuration and troubleshooting.

### Changed
- 🔄 **Named Pipes for Windows Auth**: Windows Integrated Authentication now uses Named Pipes protocol instead of TCP/IP. This allows authentication to work without requiring TCP to be enabled in SQL Server Configuration Manager. Works with both local (`.`) and remote server names.
  - `main.go`: Updated `buildSecureConnectionString()` to use Named Pipes for SSPI
  - `claude-code/db-connector.go`: Updated `connectDatabase()` to use Named Pipes for Windows Auth
  - Eliminates the need to enable TCP/IP protocol for Windows Auth scenarios
  - Tested successfully with SQL Server 2022 on Windows 10


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
