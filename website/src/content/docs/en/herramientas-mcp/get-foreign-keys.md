---
title: get_foreign_keys
description: Get foreign key relationships for a table
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`inspect`](/en/herramientas-mcp/inspect/) as part of the API consolidation in version 2.

Use `inspect (detail=foreign_keys)` to get the same result.
:::


Returns the foreign key relationships for a table, both incoming (other tables that reference this one) and outgoing (tables that this one references).

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `table_name` | string | Yes | Table name (can include schema: `dbo.TableName`) |
| `schema` | string | No | Schema name (defaults to `dbo`) |

## Usage example

```json
{
  "name": "get_foreign_keys",
  "arguments": {
    "table_name": "orders"
  }
}
```

## Response

For each relationship includes:
- Foreign key name
- Parent table and column
- Referenced table and column
- Direction (incoming/outgoing)

## Typical usage

- Understanding relationships between tables
- Verifying referential integrity
- Planning JOINs for complex queries
