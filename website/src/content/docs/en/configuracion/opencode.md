---
title: OpenCode
description: Configuring MCP-Go-MSSQL in OpenCode
---

Add the server to `opencode.json` in the project, or to `~/.config/opencode/opencode.json`. OpenCode uses `mcp`, not `mcpServers`. `command` is an array and variables go in `environment`.

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

On Windows, `command` is `["C:\\path\\to\\mcp-go-mssql.exe"]`. `{env:MSSQL_PASSWORD}` reads the password from the environment so it is not stored in the file. `timeout` is milliseconds for the first tool listing; the default (5000) is often too short on a cold start.

Check the server with `opencode mcp list`.

The same variables apply to [Claude Desktop](/en/configuracion/claude-desktop/). The full list is in [Environment variables](/en/configuracion/variables-entorno/).
