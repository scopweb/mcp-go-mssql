# 🔒 Security Summary

## Overview

This document summarizes the security features and best practices for MCP-Go-MSSQL.

## Security Features

### Database Security
- ✅ **TLS Encryption**: Mandatory encryption for all database connections
- ✅ **SQL Injection Protection**: Prepared statements exclusively (no dynamic SQL)
- ✅ **Connection Pooling**: Limited connections prevent resource exhaustion
- ✅ **Connection Timeouts**: Configurable limits on active connections

### Application Security
- ✅ **Secure Logging**: Automatic sanitization of sensitive data from logs
- ✅ **Error Handling**: Generic error messages to clients, detailed logs internally
- ✅ **Input Validation**: Query size limits prevent buffer overflows
- ✅ **Multi-table Validation**: Detects unauthorized access via JOINs/subqueries

### Access Control
- ✅ **Read-Only Mode**: Blocks INSERT/UPDATE/DELETE when enabled
- ✅ **Table Whitelist**: Granular control over modifiable tables
- ✅ **Role-Based Configuration**: Different configs for different environments

### Environment Configuration
- ✅ **Flexible Auth Methods**: SQL, Windows Integrated (SSPI), custom connection strings
- ✅ **Developer vs Production Modes**: Different TLS strictness for dev/prod
- ✅ **Environment Variables**: All credentials loaded from environment
- ✅ **Configuration Templates**: `.env.example` provides secure defaults

## Testing Security

### Secure Test Setup
- ✅ **No Hardcoded Credentials**: Tests load from `.env.test` only
- ✅ **Environment File Support**: `loadEnvFile()` safely loads test credentials
- ✅ **Git Protection**: `.env.test` in `.gitignore` (never committed)
- ✅ **Flexible Configuration**: Override via environment variables

### How to Test Safely

```bash
# 1. Create test configuration
cp .env.test.example .env.test

# 2. Edit with test database credentials (NOT production)
nano .env.test

# 3. Set restrictive permissions
chmod 600 .env.test

# 4. Run tests
go test -v ./...
```

## Best Practices

### DO ✅
- Use environment variables for all credentials
- Create separate test database
- Set restrictive file permissions (600) on `.env` files
- Use `.env.test` for local testing
- Store production secrets in secure vault
- Rotate credentials regularly (90 days)
- Enable read-only mode for AI access
- Use table whitelist for AI operations
- Monitor security logs regularly
- Keep dependencies updated

### DON'T ❌
- Hardcode credentials in source code
- Use production database for testing
- Commit `.env` or `.env.test` to Git
- Share credentials via email or chat
- Use same password for multiple environments
- Deploy with `DEVELOPER_MODE=true` in production
- Use weak passwords (< 20 characters)
- Disable TLS/encryption
- Log sensitive data
- Ignore security warnings

## Production Configuration

### Recommended Setup
```json
{
  "mcpServers": {
    "production-db-ai-safe": {
      "command": "mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "Production",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "from_secure_vault",
        "MSSQL_PORT": "1433",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

### Security Parameters
- `DEVELOPER_MODE`:
  - `"false"` for production: Strict TLS validation
  - `"true"` for development: Allows self-signed certificates
- `MSSQL_READ_ONLY`: Enable read-only mode for AI access
- `MSSQL_WHITELIST_TABLES`: Limit AI modifications to specific tables

## Deployment Checklist

### Development
- [ ] Copy `.env.example` to `.env`
- [ ] Edit with local database credentials
- [ ] Set permissions: `chmod 600 .env`
- [ ] Copy `.env.test.example` to `.env.test`
- [ ] Edit with test database credentials
- [ ] Set permissions: `chmod 600 .env.test`

### Production
- [ ] Use environment variables (not config files)
- [ ] Store passwords in secure vault
- [ ] Enable `MSSQL_READ_ONLY=true`
- [ ] Set `MSSQL_WHITELIST_TABLES` appropriately
- [ ] Set `DEVELOPER_MODE=false`
- [ ] Enable TLS certificate validation

### CI/CD (GitHub Actions)
- [ ] Store credentials in GitHub Secrets
- [ ] Reference via `${{ secrets.DB_PASSWORD }}`
- [ ] Never commit credentials to YAML
- [ ] Run tests in isolated environment

## Documentation

- **[TESTING_SECURITY.md](docs/TESTING_SECURITY.md)** - Detailed testing guidelines
- **[test/README.md](test/README.md)** - Quick start for running tests
- **[CLAUDE.md](CLAUDE.md)** - Project guidelines
- **[README.md](README.md)** - General setup

## Compliance

- ✅ No hardcoded credentials in source code
- ✅ All credentials loaded from environment
- ✅ TLS encryption mandatory for database
- ✅ SQL injection protection implemented
- ✅ Input validation enforced
- ✅ Sensitive data sanitized in logs
- ✅ Access control configured
- ✅ Test credentials not visible in Git
- ✅ Security documentation comprehensive
- ✅ Safe configuration templates provided

## Questions?

For security concerns or questions:
1. Open an issue on GitHub
2. Do NOT include any credentials in the issue
3. Describe the issue and your environment
