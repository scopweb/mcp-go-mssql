---
title: get_indexes
description: Get indexes for a specific table
---

:::caution[Tool replaced in v2]
This tool was **merged** into [`inspect`](/en/herramientas-mcp/inspect/) as part of the API consolidation in version 2.

Use `inspect (detail=indexes)` to get the same result.
:::


Returns information about the indexes defined for a table, including type, uniqueness, and indexed columns.

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `table_name` | string | Yes | Table name (can include schema: `dbo.TableName`) |
| `schema` | string | No | Schema name (defaults to `dbo`) |

## Usage example

```json
{
  "name": "get_indexes",
  "arguments": {
    "table_name": "orders"
  }
}
```

## Response

Includes for each index:
- Index name
- Type (CLUSTERED, NONCLUSTERED, etc.)
- Whether it is unique
- Included columns

## Typical usage

- Query performance analysis
- Verify that adequate indexes exist for frequent JOINs
- Identify duplicate or unnecessary indexes
