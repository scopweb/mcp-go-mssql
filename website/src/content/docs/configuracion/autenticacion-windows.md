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
server=your-server;database=YourDatabase;integrated security=sspi;encrypt=true
```

## Claude Desktop

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

## Solución de problemas

- **Login failed**: Verifica que el usuario de Windows tiene un login en SQL Server
- **SSPI handshake failed**: Verifica que SQL Server acepta Windows Authentication en su configuración
- **Cannot generate SSPI context**: Puede indicar problemas de DNS o Kerberos en el dominio
