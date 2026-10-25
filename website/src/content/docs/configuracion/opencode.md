---
title: OpenCode
description: Configuración de MCP-Go-MSSQL en OpenCode
---

Añade el servidor en `opencode.json` del proyecto, o en `~/.config/opencode/opencode.json`. OpenCode usa `mcp`, no `mcpServers`. `command` es un array y las variables van en `environment`.

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "mssql": {
      "type": "local",
      "command": ["/absolute/path/to/mcp-go-mssql"],
      "enabled": true,
      "timeout": 30000,
      "environment": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "{env:MSSQL_PASSWORD}",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true"
      }
    }
  }
}
```

En Windows, `command` es `["C:\\path\\to\\mcp-go-mssql.exe"]`. `{env:MSSQL_PASSWORD}` lee la contraseña del entorno para no guardarla en el fichero. `timeout` son milisegundos para el primer listado de herramientas; el valor por defecto (5000) suele quedarse corto en un arranque en frío.

Comprueba el servidor con `opencode mcp list`.

Las mismas variables valen para [Claude Desktop](/configuracion/claude-desktop/). La lista completa está en [Variables de entorno](/configuracion/variables-entorno/).
