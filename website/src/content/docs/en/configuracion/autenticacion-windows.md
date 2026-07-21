---
title: Windows Authentication (SSPI)
description: Configuring Windows integrated authentication for MCP-Go-MSSQL
---

Windows integrated authentication (SSPI) allows connecting without specifying a username and password, using the current Windows user's credentials.

## Requirements

- SQL Server configured to accept Windows Authentication
- The Windows user must have database access
- Only works in Windows environments

## Configuration

```bash
MSSQL_AUTH=integrated
# `windows` is also valid as an alias
MSSQL_SERVER=your-server
MSSQL_DATABASE=YourDatabase
```

There is no need to set `MSSQL_USER` or `MSSQL_PASSWORD`. `MSSQL_DATABASE` is optional with integrated authentication.

## Generated connection string

```
server=your-server;port=1433;database=YourDatabase;integrated security=SSPI;encrypt=false;trustservercertificate=true
```

> The values of `encrypt` and `trustservercertificate` depend on `DEVELOPER_MODE` and `MSSQL_ENCRYPT`.

## Claude Desktop

### Modern server (SQL Server 2016+)

```json
{
  "mssql-windows": {
    "command": "mcp-go-mssql.exe",
    "env": {
      "MSSQL_SERVER": "your-server",
      "MSSQL_DATABASE": "YourDatabase",
      "MSSQL_AUTH": "integrated",
      "DEVELOPER_MODE": "false",
      "MSSQL_READ_ONLY": "true"
    }
  }
}
```

### Legacy server (SQL Server 2008/2012)

SQL Server 2008/2012 does not support TLS 1.2, which is the minimum required by the Go driver. Encryption must be disabled.

```json
{
  "mssql-legacy": {
    "command": "mcp-go-mssql.exe",
    "env": {
      "MSSQL_SERVER": "legacy-server",
      "MSSQL_DATABASE": "LegacyDB",
      "MSSQL_AUTH": "integrated",
      "DEVELOPER_MODE": "true",
      "MSSQL_ENCRYPT": "false"
    }
  }
}
```

> `MSSQL_ENCRYPT=false` only works with `DEVELOPER_MODE=true`. In production, insecure TLS **fails closed** (the server refuses to start), but legacy servers remain accessible via `MSSQL_CONNECTION_STRING` in URL form (`encrypt=disable`) plus the explicit opt-in `MSSQL_ALLOW_INSECURE_TLS=true`:
>
> ```json
> {
>   "mssql-legacy-prod": {
>     "command": "mcp-go-mssql.exe",
>     "env": {
>       "MSSQL_AUTH": "integrated",
>       "DEVELOPER_MODE": "false",
>       "MSSQL_ALLOW_INSECURE_TLS": "true",
>       "MSSQL_CONNECTION_STRING": "sqlserver://legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true&integrated security=SSPI"
>     }
>   }
> }
> ```

## Troubleshooting

- **Login failed**: Verify that the Windows user has a login in SQL Server
- **SSPI handshake failed**: Verify that SQL Server accepts Windows Authentication in its configuration
- **Cannot generate SSPI context**: May indicate DNS or Kerberos issues in the domain
- **TLS handshake failed**: SQL Server 2008/2012 doesn't support TLS 1.2. Set `MSSQL_ENCRYPT=false` with `DEVELOPER_MODE=true`
- **Connection timeout**: Verify the port is correctly configured (`MSSQL_PORT`, default 1433)
