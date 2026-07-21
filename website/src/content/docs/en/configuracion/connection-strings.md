---
title: Connection Strings
description: Connection string reference for MCP-Go-MSSQL
---

MCP-Go-MSSQL automatically builds the connection string from environment variables. You can also provide a custom connection string.

## Automatic connection string

With standard variables, the server generates:

```
server=HOST;database=DB;user id=USER;password=PASS;port=1433;encrypt=true;trustservercertificate=false
```

In development mode (`DEVELOPER_MODE=true`), it changes to `encrypt=false` and `trustservercertificate=true`.

## Custom connection string

Set `MSSQL_CONNECTION_STRING` to use your own string:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true;trustservercertificate=false"
```

This variable overrides `MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_USER`, `MSSQL_PASSWORD`, and `MSSQL_PORT`.

> **Production validation (fail-closed):** a user-supplied `MSSQL_CONNECTION_STRING` is validated at startup in production mode. If it disables encryption (`encrypt=false`/`disable`) or certificate validation (`trustservercertificate=true`), the server **refuses to start** unless explicitly opted in with `MSSQL_ALLOW_INSECURE_TLS=true` (auditable WARN per connection).

### Legacy servers (SQL Server 2000/2008/2012)

These servers only negotiate TLS 1.0, which modern Go runtimes reject. Use the URL form with `encrypt=disable`:

```bash
# Development
DEVELOPER_MODE=true
MSSQL_CONNECTION_STRING="sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"

# Production (requires explicit opt-in)
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING="sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"
```

## Examples by environment

### Local SQL Server

```
server=localhost;database=DevDB;user id=sa;password=DevPass123;encrypt=true;trustservercertificate=true
```

### Azure SQL Database

```
server=myserver.database.windows.net;database=MyDB;user id=myuser@myserver;password=MyPass;encrypt=true;trustservercertificate=false
```

### Windows Authentication

```
server=myserver;database=MyDB;integrated security=sspi;encrypt=true
```

## Encryption parameters

| Parameter | Production | Development |
|-----------|------------|-------------|
| `encrypt` | `true` (mandatory, fail-closed) | `false` (default; overridable with `MSSQL_ENCRYPT=true`) |
| `trustservercertificate` | `false` | `true` |
