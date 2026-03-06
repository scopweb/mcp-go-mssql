---
title: list_databases
description: List all databases on the server
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`explore`](/en/herramientas-mcp/explore/) as part of the API consolidation in version 2.

Use `explore (type=databases)` to get the same result.
:::


Lists all available user databases on the SQL Server instance.

## Parameters

This tool requires no parameters.

## Usage example

```json
{
  "name": "list_databases",
  "arguments": {}
}
```

## Response

Returns the list of databases excluding system databases (`master`, `tempdb`, `model`, `msdb`).

## Notes

- Especially useful with Windows authentication (SSPI) without a specific database
- Allows Claude to explore which databases are available
- Requires `VIEW ANY DATABASE` permissions or equivalent
