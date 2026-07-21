---
title: Authentication Modes
description: Authentication methods supported by MCP-Go-MSSQL
---

MCP-Go-MSSQL supports multiple authentication methods to adapt to different environments.

## SQL Server Authentication

The default method. Uses SQL Server username and password.

```bash
MSSQL_AUTH=sql
MSSQL_USER=app_user
MSSQL_PASSWORD=your_password
```

## Windows Integrated (SSPI)

Uses the current Windows user's credentials. No username or password required in configuration, and `MSSQL_DATABASE` is optional.

```bash
MSSQL_AUTH=integrated
# `windows` is also valid as an alias:
# MSSQL_AUTH=windows
```

See [Windows Authentication (SSPI)](/en/configuracion/autenticacion-windows/) for configuration details.

## Azure Active Directory

`MSSQL_AUTH=azure` is **not implemented**: the server returns an error if that value is used. For Azure SQL with Azure AD, use `MSSQL_CONNECTION_STRING` with the appropriate driver authentication parameters:

```bash
MSSQL_CONNECTION_STRING="server=myserver.database.windows.net;database=mydb;user id=user@tenant.onmicrosoft.com;password=your_password;encrypt=true"
```

## Custom connection string

When you need full control over connection parameters:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true"
```

This variable overrides all other connection variables.

## Authentication priority

1. `MSSQL_CONNECTION_STRING` (if defined, used directly — including Azure AD scenarios)
2. `MSSQL_AUTH=integrated` or `MSSQL_AUTH=windows` (SSPI, ignores user/password)
3. `MSSQL_AUTH=azure` → **error** (not implemented; use `MSSQL_CONNECTION_STRING`)
4. `MSSQL_AUTH=sql` or undefined (SQL Server auth by default)
