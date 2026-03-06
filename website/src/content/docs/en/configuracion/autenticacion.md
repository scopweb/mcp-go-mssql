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

Uses the current Windows user's credentials. No username or password required in configuration.

```bash
MSSQL_AUTH=integrated
```

See [Windows Authentication (SSPI)](/en/configuracion/autenticacion-windows/) for configuration details.

## Azure Active Directory

For Azure SQL databases.

```bash
MSSQL_AUTH=azure
MSSQL_USER=user@tenant.onmicrosoft.com
MSSQL_PASSWORD=your_password
```

## Custom connection string

When you need full control over connection parameters:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true"
```

This variable overrides all other connection variables.

## Authentication priority

1. `MSSQL_CONNECTION_STRING` (if defined, used directly)
2. `MSSQL_AUTH=integrated` (SSPI, ignores user/password)
3. `MSSQL_AUTH=azure` (Azure AD)
4. `MSSQL_AUTH=sql` or undefined (SQL Server auth by default)
