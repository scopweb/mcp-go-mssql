---
title: Security Overview
description: Overview of MCP-Go-MSSQL security features
---

MCP-Go-MSSQL implements multiple security layers to protect databases in production environments.

## Security features

### Database security
- **Mandatory TLS encryption** for all connections in production
- **SQL Injection protection** with exclusive prepared statements
- **Connection pooling** with limits to prevent resource exhaustion
- **Configurable connection timeouts**

### Application security
- **Secure logging** with automatic sanitization of sensitive data
- **Secure error handling** — generic messages to the client, details in internal logs
- **Input validation** with query size limits
- **Multi-table validation** — detects unauthorized access via JOINs/subqueries

### Access control
- **Read-only mode** — blocks INSERT/UPDATE/DELETE
- **Table whitelist** — granular control over modifiable tables
- **Role-based configuration** — different configs for different environments

### AI-assisted attack protection

MCP-Go-MSSQL implements specific defenses against attack techniques that an AI can execute automatically:

- **CHAR()/NCHAR() concatenation** — detects dynamically built keywords
- **Inline comments** — detects keywords hidden inside SQL comments
- **Dangerous table hints** — blocks `WITH (NOLOCK)`, `WITH (READUNCOMMITTED)`, etc.
- **WAITFOR DELAY** — prevents timing attacks to infer data
- **OPENROWSET/OPENDATASOURCE** — prevents exfiltration to external servers
- **Unicode control characters** — blocks RTL override and zero-width spaces
- **Unicode homoglyphs** — detects Cyrillic/Greek letters mimicking ASCII
- **Subqueries against whitelist** — validates that subquery tables are also in whitelist

See [AI-Assisted Attack Protection](./ataques-ia.md) for complete details.

### Authentication
- **Multiple methods** — SQL Server, Windows Integrated (SSPI), custom connection strings
- **Dev/prod modes** — different levels of TLS strictness
- **Environment variables** — credentials never in source code
- **Configuration templates** — `.env.example` with secure defaults

## Compliance

- OWASP Top 10 (2021)
- CWE Top 25 (2024)
- NIST Cybersecurity Framework
- Go database best practices

## Best practices

### Do
- Use environment variables for all credentials
- Create a separate database for testing
- Set restrictive permissions (600) on `.env` files
- Enable read-only mode for AI access
- Monitor security logs regularly
- Keep dependencies up to date

### Don't
- Hardcode credentials in source code
- Use the production database for testing
- Commit `.env` or `config.json` to Git
- Deploy with `DEVELOPER_MODE=true` in production
- Disable TLS/encryption
- Log sensitive data
