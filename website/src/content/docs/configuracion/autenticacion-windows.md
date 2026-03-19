---
title: Autenticación Windows (SSPI)
description: Configuración de autenticación integrada de Windows para MCP-Go-MSSQL
---

La autenticación integrada de Windows (SSPI) permite conectar sin especificar usuario y contraseña, usando las credenciales del usuario de Windows actual.

## Requisitos

- SQL Server configurado para aceptar Windows Authentication
- El usuario de Windows debe tener acceso a la base de datos
- Solo funciona en entornos Windows

## Configuración

```bash
MSSQL_AUTH=integrated
MSSQL_SERVER=your-server
MSSQL_DATABASE=YourDatabase
```

No es necesario definir `MSSQL_USER` ni `MSSQL_PASSWORD`.

## Connection string generado

```
server=your-server;port=1433;database=YourDatabase;integrated security=SSPI;encrypt=false;trustservercertificate=true
```

> El valor de `encrypt` y `trustservercertificate` depende de `DEVELOPER_MODE` y `MSSQL_ENCRYPT`.

## Claude Desktop

### Servidor moderno (SQL Server 2016+)

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

### Servidor legacy (SQL Server 2008/2012)

SQL Server 2008/2012 no soporta TLS 1.2, que es el mínimo requerido por el driver Go. Es necesario desactivar el cifrado.

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

> `MSSQL_ENCRYPT=false` solo funciona con `DEVELOPER_MODE=true`. En producción el cifrado es obligatorio.

## Solución de problemas

- **Login failed**: Verifica que el usuario de Windows tiene un login en SQL Server
- **SSPI handshake failed**: Verifica que SQL Server acepta Windows Authentication en su configuración
- **Cannot generate SSPI context**: Puede indicar problemas de DNS o Kerberos en el dominio
- **TLS handshake failed**: SQL Server 2008/2012 no soporta TLS 1.2. Configura `MSSQL_ENCRYPT=false` con `DEVELOPER_MODE=true`
- **Connection timeout**: Verifica que el puerto está correctamente configurado (`MSSQL_PORT`, default 1433)
