---
title: list_tables
description: List all tables and views in the database
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`explore`](/en/herramientas-mcp/explore/) as part of the API consolidation in version 2.

Use `explore (type=tables)` to get the same result.
:::


Lists all available tables and views in the connected database.

## Parameters

This tool requires no parameters.

## Usage example

```json
{
  "name": "list_tables",
  "arguments": {}
}
```

## Response

Returns a list with the name, schema, and type (TABLE or VIEW) of each object.

```json
[
  {"schema": "dbo", "name": "users", "type": "TABLE"},
  {"schema": "dbo", "name": "orders", "type": "TABLE"},
  {"schema": "dbo", "name": "v_active_users", "type": "VIEW"}
]
```

## Internal query

The tool internally executes:

```sql
SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE
FROM INFORMATION_SCHEMA.TABLES
ORDER BY TABLE_SCHEMA, TABLE_NAME
```

## Notes

- Works in both read and write mode
- Requires no special permissions beyond `SELECT` on `INFORMATION_SCHEMA`
- Results include both user tables and views
