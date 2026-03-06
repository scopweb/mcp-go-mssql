---
title: Claude Desktop
description: Configuración de MCP-Go-MSSQL para Claude Desktop
---

MCP-Go-MSSQL se integra con Claude Desktop mediante el protocolo MCP (Model Context Protocol) sobre JSON-RPC 2.0 a través de stdin/stdout.

## Configuración

Edita el archivo de configuración de Claude Desktop y añade el servidor MCP:

```json
{
  "mcpServers": {
    "mssql": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "your_user",
        "MSSQL_PASSWORD": "your_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia"
      }
    }
  }
}
```

## Perfiles recomendados

### Producción con AI-Safe

```json
{
  "mssql-prod": {
    "command": "mcp-go-mssql.exe",
    "env": {
      "MSSQL_SERVER": "prod-server.database.windows.net",
      "MSSQL_DATABASE": "ProductionDB",
      "MSSQL_USER": "readonly_user",
      "MSSQL_PASSWORD": "your_password",
      "DEVELOPER_MODE": "false",
      "MSSQL_READ_ONLY": "true",
      "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia"
    }
  }
}
```

### Desarrollo local

```json
{
  "mssql-dev": {
    "command": "mcp-go-mssql.exe",
    "env": {
      "MSSQL_SERVER": "localhost",
      "MSSQL_DATABASE": "DevDB",
      "MSSQL_USER": "sa",
      "MSSQL_PASSWORD": "dev_password",
      "DEVELOPER_MODE": "true",
      "MSSQL_READ_ONLY": "false"
    }
  }
}
```

## Verificar la integración

Una vez configurado, reinicia Claude Desktop. Las herramientas MCP aparecerán disponibles automáticamente en la conversación.
