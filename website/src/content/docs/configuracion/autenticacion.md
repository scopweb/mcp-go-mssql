---
title: Modos de autenticación
description: Métodos de autenticación soportados por MCP-Go-MSSQL
---

MCP-Go-MSSQL soporta múltiples métodos de autenticación para adaptarse a diferentes entornos.

## SQL Server Authentication

El método por defecto. Usa usuario y contraseña de SQL Server.

```bash
MSSQL_AUTH=sql
MSSQL_USER=app_user
MSSQL_PASSWORD=your_password
```

## Windows Integrated (SSPI)

Usa las credenciales de Windows del usuario actual. No requiere usuario ni contraseña en la configuración, y `MSSQL_DATABASE` es opcional.

```bash
MSSQL_AUTH=integrated
# `windows` también es válido como alias:
# MSSQL_AUTH=windows
```

Consulta [Autenticación Windows (SSPI)](/seguridad/autenticacion-windows/) para detalles de configuración.

## Azure Active Directory

`MSSQL_AUTH=azure` **no está implementado**: el servidor devuelve un error si se usa ese valor. Para Azure SQL con Azure AD, usa `MSSQL_CONNECTION_STRING` con los parámetros de autenticación adecuados del driver:

```bash
MSSQL_CONNECTION_STRING="server=myserver.database.windows.net;database=mydb;user id=user@tenant.onmicrosoft.com;password=your_password;encrypt=true"
```

## Connection string personalizado

Cuando necesitas control total sobre los parámetros de conexión:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true"
```

Esta variable anula todas las demás variables de conexión.

## Prioridad de autenticación

1. `MSSQL_CONNECTION_STRING` (si está definido, se usa directamente — incluidos los escenarios Azure AD)
2. `MSSQL_AUTH=integrated` o `MSSQL_AUTH=windows` (SSPI, ignora user/password)
3. `MSSQL_AUTH=azure` → **error** (no implementado; usa `MSSQL_CONNECTION_STRING`)
4. `MSSQL_AUTH=sql` o sin definir (SQL Server auth por defecto)
