---
title: list_stored_procedures
description: List database stored procedures
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`explore`](/en/herramientas-mcp/explore/) as part of the API consolidation in version 2.

Use `explore (type=procedures)` to get the same result.
:::


Lists all available stored procedures in the database, with an option to filter by schema.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `schema` | string | No | Filter by schema (optional) |

## Usage example

Without filter:
```json
{
  "name": "list_stored_procedures",
  "arguments": {}
}
```

With schema filter:
```json
{
  "name": "list_stored_procedures",
  "arguments": {
    "schema": "dbo"
  }
}
```

## Notes

- In read-only mode, safe system procedures (`sp_help`, `sp_helptext`, `sp_columns`, etc.) are allowed
- Dangerous procedures (`xp_cmdshell`, `sp_configure`, `sp_executesql`) are always blocked
