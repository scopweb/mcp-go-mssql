---
title: describe_table
description: Get the structure and schema of a table
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`inspect`](/en/herramientas-mcp/inspect/) as part of the API consolidation in version 2.

Use `inspect (detail=columns)` to get the same result.
:::


Gets detailed information about a table's columns, including data types, nullability, and default values.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `table_name` | string | Yes | Table name (can include schema: `dbo.TableName`) |
| `schema` | string | No | Schema name (defaults to `dbo`) |

## Usage example

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "users"
  }
}
```

With explicit schema:

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "users",
    "schema": "sales"
  }
}
```

Or using schema notation in the name:

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "sales.users"
  }
}
```

## Response

```json
[
  {
    "column_name": "id",
    "data_type": "int",
    "is_nullable": "NO",
    "column_default": null,
    "max_length": null
  },
  {
    "column_name": "name",
    "data_type": "nvarchar",
    "is_nullable": "YES",
    "column_default": null,
    "max_length": 255
  }
]
```

## Notes

- Supports `schema.table` and `[schema].[table]` formats
- Correctly filters by schema and table name to avoid confusion between tables with the same name in different schemas
