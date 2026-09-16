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
| Data interception | Mandatory TLS in production with fail-closed startup (the server refuses to start on insecure TLS); explicit escape `MSSQL_ALLOW_INSECURE_TLS=true` for legacy servers |
| Resource exhaustion | Connection pooling with configurable limits |
| Information leakage | Generic errors to client, details only in internal logs |
| Privilege escalation | Multi-table validation on JOINs and subqueries |

### Standards compliance

- **OWASP Top 10 (2021)**: A01-Broken Access Control, A03-Injection, A02-Cryptographic Failures
- **CWE Top 25 (2024)**: CWE-89 (SQL Injection), CWE-306 (Missing Auth), CWE-798 (Hardcoded Credentials)
- **NIST Cybersecurity Framework**: Identify, Protect, Detect, Respond

## Layer-by-layer analysis

### Transport layer

- Mandatory TLS encryption in production (`encrypt=true`), with **fail-closed** startup on insecure TLS
- Certificate validation in production (`trustservercertificate=false`); explicit opt-in `MSSQL_ALLOW_INSECURE_TLS=true` with auditable WARN for legacy servers
- Self-signed certificates and disabled encryption only allowed by default in development mode

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

1. Use a **least-privilege** SQL login: broad `SELECT` and `INSERT`/`UPDATE`/`DELETE` only on dedicated tables — this is the primary write boundary (the Go validation is defense in depth)
2. Run with `MSSQL_READ_ONLY=true` in production
3. If a writable workspace is needed, use `READ_ONLY=false` and `MSSQL_WHITELIST_TABLES` only for AI temporary tables. With `READ_ONLY=true` the whitelist does not grant writes.
4. Monitor security logs periodically
5. Rotate credentials regularly
