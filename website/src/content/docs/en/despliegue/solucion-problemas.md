---
title: Troubleshooting
description: Common problems and solutions for MCP-Go-MSSQL
---

## Connection errors

### "Database not connected"

Environment variables are not configured or are incorrect.

```bash
echo "Server: $MSSQL_SERVER"
echo "Database: $MSSQL_DATABASE"
echo "User: $MSSQL_USER"
```

### "TLS handshake failed" / "Certificate signed by unknown authority"

**Development:** Set `DEVELOPER_MODE=true` to accept self-signed certificates.

**SQL Server 2008/2012:** These servers don't support TLS 1.2. Add `MSSQL_ENCRYPT=false` together with `DEVELOPER_MODE=true`.

```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

**Production:** Install valid SSL certificates on SQL Server or configure the CA on the system.

### "Login failed for user"

- Verify username and password
- Confirm that SQL Server is in mixed authentication mode (SQL + Windows)
- Verify that the user has access to the specified database

### "Network error" / "Connection refused"

- Confirm that SQL Server is listening on the correct port (default 1433)
- Check firewall rules
- Verify that the SQL Server service is running

## Runtime errors

### "Query blocked: read-only mode"

The query is trying to modify data and `MSSQL_READ_ONLY=true` is active. If you need to modify specific tables, add them to `MSSQL_WHITELIST_TABLES`.

### "Table not in whitelist"

The table referenced in a write operation is not in `MSSQL_WHITELIST_TABLES`. Check the exact table name.

### "SSPI handshake failed"

Verify that SQL Server accepts Windows authentication and that the Windows user has a configured login.

## Improved diagnostics with Claude

When Claude receives a "Database not connected" error, it can now automatically call `get_database_info` to obtain:

- **Current configuration**: server, database, authentication mode, encryption, port
- **Possible causes**: missing variables, TLS incompatibility, permission issues
- **Specific solutions**: based on the detected scenario (SQL 2008, integrated auth, etc.)

This allows Claude to diagnose and resolve connection issues without manual intervention.

## Verify the connection

```bash
cd test
go run test-connection.go
```
