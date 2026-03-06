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

En modo desarrollo (`DEVELOPER_MODE=true`), cambia a `trustservercertificate=true`.

## Connection string personalizado

Define `MSSQL_CONNECTION_STRING` para usar un string propio:

```bash
MSSQL_CONNECTION_STRING="server=myserver;database=mydb;user id=myuser;password=mypass;encrypt=true;trustservercertificate=false"
```

Esta variable anula `MSSQL_SERVER`, `MSSQL_DATABASE`, `MSSQL_USER`, `MSSQL_PASSWORD` y `MSSQL_PORT`.

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
| `encrypt` | `true` (siempre) | `true` (siempre) |
| `trustservercertificate` | `false` | `true` |
