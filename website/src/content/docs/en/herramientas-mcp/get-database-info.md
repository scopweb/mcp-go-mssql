---
title: get_database_info
description: Get connection status and database information
---

Returns information about the connection status, access mode, and security configuration.

## Parameters

This tool requires no parameters.

## Usage example

```json
{
  "name": "get_database_info",
  "arguments": {}
}
```

## Response

```json
{
  "status": "connected",
  "server": "my-server.database.windows.net",
  "database": "MyDatabase",
  "read_only": true,
  "whitelist_tables": ["temp_ai", "v_temp_ia"],
  "encryption": "enabled",
  "developer_mode": false
}
```

## Typical usage

Claude usually invokes this tool automatically at the beginning of a conversation to understand:
- Whether the database is connected
- What access mode it has (read/write)
- Which tables it can modify
