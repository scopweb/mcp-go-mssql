---
title: TLS & Encryption
description: TLS encryption configuration for database connections
---

TLS encryption is **mandatory in production**. In development mode encryption is disabled by default (`encrypt=false`) to ease local instances, and legacy servers (SQL Server 2000/2008/2012) are supported through an explicit opt-in (`MSSQL_ALLOW_INSECURE_TLS`).

## Behavior by mode

### Production mode (`DEVELOPER_MODE=false`)

- `encrypt=true` — Mandatory encryption
- `trustservercertificate=false` — Requires valid, trusted certificates
- **Fail-closed**: if a connection string (global `MSSQL_CONNECTION_STRING`, per-alias `_CONNECTION_STRING` or per-alias `_ENCRYPT`) disables encryption (`encrypt=false`/`disable`) or certificate validation (`trustservercertificate=true`), the server **refuses to start** instead of silently degrading TLS
- Generic errors without technical information

### Development mode (`DEVELOPER_MODE=true`)

- `encrypt=false` — Encryption disabled for local SQL Server
- `trustservercertificate=true` — Allows self-signed certificates
- Detailed errors for debugging

## Force encryption in development

If you need encryption in development:

```bash
MSSQL_ENCRYPT=true
DEVELOPER_MODE=true
```

This enables encryption but allows self-signed certificates.

## TLS connection strings

### Production (Azure SQL)
```
server=prod.database.windows.net;database=ProdDB;encrypt=true;trustservercertificate=false
```

### Local development
```
server=localhost;database=DevDB;encrypt=false;trustservercertificate=true
```

### Development with encryption
```
server=localhost;database=DevDB;encrypt=true;trustservercertificate=true
```

## Explicit opt-in for insecure TLS (`MSSQL_ALLOW_INSECURE_TLS`)

In production (`DEVELOPER_MODE=false`), any configuration that disables encryption or certificate validation **aborts server startup** (fail-closed). For legacy servers that cannot be upgraded (SQL Server 2000/2008/2012, which only negotiate TLS 1.0), there is an explicit escape hatch:

```bash
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING=sqlserver://USER:PASS@legacy-sql2000.local:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

- With `MSSQL_ALLOW_INSECURE_TLS=true` the connection proceeds and an **auditable WARN is logged on every affected connection**.
- If unset, insecure TLS in production **aborts startup**.
- This is the sanctioned way to keep legacy servers working: the exception becomes conscious and greppable instead of accidental.

## TLS troubleshooting

### "certificate signed by unknown authority"
- **Cause:** Self-signed certificate or unrecognized CA
- **Development:** Set `DEVELOPER_MODE=true`
- **Production:** Install valid SSL certificates on SQL Server

### "SSL Provider: No credentials are available"
- **Cause:** Local SQL Server without TLS configuration
- **Solution:** Set `DEVELOPER_MODE=true` to disable local encryption

### "TLS Handshake failed"
- **Cause:** Legacy SQL Server (2008/2012) doesn't support TLS 1.2, which is the minimum required by the Go driver
- **Solution (development):** Set `MSSQL_ENCRYPT=false` together with `DEVELOPER_MODE=true`
- **Solution (production):** Use `MSSQL_CONNECTION_STRING` in URL form with `encrypt=disable` plus the explicit opt-in `MSSQL_ALLOW_INSECURE_TLS=true`

**Development:**
```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

**Production:**
```bash
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING=sqlserver://USER:PASS@legacy-sql2000.local:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

> This disables TLS on the connection. Only use for legacy servers that cannot be upgraded. For SQL Server 2016+ and Azure SQL, always keep encryption enabled.
