---
title: Security Analysis
description: Detailed analysis of the MCP-Go-MSSQL security model
---

MCP-Go-MSSQL has been designed following recognized security standards and applies defense in depth across all layers.

## Threat model

### Covered attack vectors

| Vector | Mitigation |
|--------|------------|
| SQL Injection | Exclusive prepared statements, no dynamic concatenation |
| Unauthorized access | Read-only mode + table whitelist |
| Data interception | Mandatory TLS on all connections |
| Resource exhaustion | Connection pooling with configurable limits |
| Information leakage | Generic errors to client, details only in internal logs |
| Privilege escalation | Multi-table validation on JOINs and subqueries |

### Standards compliance

- **OWASP Top 10 (2021)**: A01-Broken Access Control, A03-Injection, A02-Cryptographic Failures
- **CWE Top 25 (2024)**: CWE-89 (SQL Injection), CWE-306 (Missing Auth), CWE-798 (Hardcoded Credentials)
- **NIST Cybersecurity Framework**: Identify, Protect, Detect, Respond

## Layer-by-layer analysis

### Transport layer

- Mandatory TLS encryption (`encrypt=true`)
- Certificate validation in production (`trustservercertificate=false`)
- Self-signed certificates only allowed in development mode

### Application layer

- Automatic sanitization of sensitive data in logs
- Query size limit (1 MB)
- Empty input rejection
- System command blocking (`xp_cmdshell`, `OPENROWSET`, etc.)

### Data layer

- Prepared statements for all queries without exception
- Validation of all tables referenced in modifications
- Connection pooling with active connection limits
- Configurable timeouts to prevent hanging connections

## Recommendations

1. Run with `MSSQL_READ_ONLY=true` in production
2. Set `MSSQL_WHITELIST_TABLES` only for AI temporary tables
3. Use a database user with minimal permissions
4. Monitor security logs periodically
5. Rotate credentials regularly
