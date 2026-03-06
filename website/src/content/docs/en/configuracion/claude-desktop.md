---
title: Claude Desktop
description: Configuring MCP-Go-MSSQL for Claude Desktop
---

MCP-Go-MSSQL integrates with Claude Desktop through the MCP (Model Context Protocol) over JSON-RPC 2.0 via stdin/stdout.

## Configuration

Edit the Claude Desktop configuration file and add the MCP server:

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

## Recommended profiles

### Production with AI-Safe

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

### Local development

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

## Verify the integration

Once configured, restart Claude Desktop. The MCP tools will automatically appear available in the conversation.
