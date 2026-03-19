---
title: TLS & Encryption
description: TLS encryption configuration for database connections
---

All database connections are protected by TLS encryption.

## Behavior by mode

### Production mode (`DEVELOPER_MODE=false`)

- `encrypt=true` — Mandatory encryption
- `trustservercertificate=false` — Requires valid, trusted certificates
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
- **Solution:** Set `MSSQL_ENCRYPT=false` together with `DEVELOPER_MODE=true`

```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

> This disables TLS on the connection. Only use for legacy servers that cannot be upgraded. For SQL Server 2016+ and Azure SQL, always keep encryption enabled.
