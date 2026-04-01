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

In development mode (`DEVELOPER_MODE=true`), it changes to `trustservercertificate=true`.

## Custom connection string

Set `MSSQL_CONNECTION_STRING` to use your own string:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true;trustservercertificate=false"
```

This variable overrides `MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_USER`, `MSSQL_PASSWORD`, and `MSSQL_PORT`.

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
| `encrypt` | `true` (always) | `false` by default (configurable with `MSSQL_ENCRYPT`) |
| `trustservercertificate` | `false` | `true` |
