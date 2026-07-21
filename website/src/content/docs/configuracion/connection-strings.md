---
title: Connection strings
description: Referencia de connection strings para MCP-Go-MSSQL
---

MCP-Go-MSSQL construye el connection string automáticamente a partir de las variables de entorno. También puedes proporcionar un connection string personalizado.

## Connection string automático

Con las variables estándar, el servidor genera:

```
server=HOST;database=DB;user id=USER;password=PASS;port=1433;encrypt=true;trustservercertificate=false
```

En modo desarrollo (`DEVELOPER_MODE=true`), cambia a `encrypt=false` y `trustservercertificate=true`.

## Connection string personalizado

Define `MSSQL_CONNECTION_STRING` para usar un string propio:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true;trustservercertificate=false"
```

Esta variable anula `MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_USER`, `MSSQL_PASSWORD` y `MSSQL_PORT`.

> **Validación en producción (fail-closed):** un `MSSQL_CONNECTION_STRING` proporcionado por el usuario se valida al arrancar en modo producción. Si desactiva el cifrado (`encrypt=false`/`disable`) o la validación de certificados (`trustservercertificate=true`), el servidor **se niega a arrancar** salvo que se opte explícitamente con `MSSQL_ALLOW_INSECURE_TLS=true` (WARN auditable por conexión).

### Servidores legacy (SQL Server 2000/2008/2012)

Estos servidores solo negocian TLS 1.0, que el runtime Go moderno rechaza. Usa la forma URL con `encrypt=disable`:

```bash
# Desarrollo
DEVELOPER_MODE=true
MSSQL_CONNECTION_STRING="sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"

# Producción (requiere opt-in explícito)
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING="sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"
```

## Ejemplos por entorno

### SQL Server local

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

## Parámetros de cifrado

| Parámetro | Producción | Desarrollo |
|-----------|------------|------------|
| `encrypt` | `true` (obligatorio, fail-closed) | `false` (por defecto; sobreescribible con `MSSQL_ENCRYPT=true`) |
| `trustservercertificate` | `false` | `true` |
