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

Usa las credenciales de Windows del usuario actual. No requiere usuario ni contraseña en la configuración.

```bash
MSSQL_AUTH=integrated
```

Consulta [Autenticación Windows (SSPI)](/seguridad/autenticacion-windows/) para detalles de configuración.

## Azure Active Directory

Para bases de datos en Azure SQL.

```bash
MSSQL_AUTH=azure
MSSQL_USER=user@tenant.onmicrosoft.com
MSSQL_PASSWORD=your_password
```

## Connection string personalizado

Cuando necesitas control total sobre los parámetros de conexión:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true"
```

Esta variable anula todas las demás variables de conexión.

## Prioridad de autenticación

1. `MSSQL_CONNECTION_STRING` (si está definido, se usa directamente)
2. `MSSQL_AUTH=integrated` (SSPI, ignora user/password)
3. `MSSQL_AUTH=azure` (Azure AD)
4. `MSSQL_AUTH=sql` o sin definir (SQL Server auth por defecto)
