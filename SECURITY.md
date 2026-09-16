# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 1.x.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

### How to Report

1. **GitHub Private Vulnerability Reporting** (preferred):
   - Go to [Security Advisories](https://github.com/scopweb/mcp-go-mssql/security/advisories/new)
   - Fill in the vulnerability details

2. **Email**: If private reporting is not available, contact the maintainer through their GitHub profile.

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 7 days
- **Fix or mitigation**: Depends on severity, typically within 30 days

### Scope

The following are in scope:

- SQL injection or stacked-statement bypasses
- Authorization flaws (`READ_ONLY`, whitelist, `confirm_operation`)
- TLS/encryption weaknesses
- Connection string or credential exposure in logs or tool results
- Input validation bypasses
- Dependency vulnerabilities

### Security Features

- Prepared statements (no string-concatenated SQL)
- TLS fail-closed in production unless `MSSQL_ALLOW_INSECURE_TLS=true`
- Absolute read-only mode: a table whitelist cannot enable writes while `READ_ONLY=true`
- Whitelist writes only in an explicitly writable context, with schema-qualified matching
- Stacked batches rejected in restricted contexts, with or without `;`
- Administrative procedures limited to `sys`/`dbo` read-only forms
- Security event logging with credential sanitization
- CLI read-only by default (`MSSQL_CLI_ALLOW_WRITE` is operator-only)

The Go validators are defense in depth. The real write boundary is a least-privilege SQL login.

Details: [docs/AGENT_CONTRACT.md](docs/AGENT_CONTRACT.md) and [security docs](https://mcp-go-mssql.scopweb.com/seguridad/resumen/).
