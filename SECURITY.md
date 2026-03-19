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
- SQL injection bypasses
- Authentication/authorization flaws
- TLS/encryption weaknesses
- Connection string credential exposure
- Input validation bypasses
- Dependency vulnerabilities

### Security Features

This project implements multiple security layers:
- Prepared statements (no dynamic SQL)
- TLS encryption for all production connections
- Read-only mode with table whitelist
- Security event logging with credential sanitization
- Input validation and SQL injection protection

For details, see [docs/SECURITY_ANALYSIS.md](docs/SECURITY_ANALYSIS.md).
