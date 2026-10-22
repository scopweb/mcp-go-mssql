---
title: explore
description: Explore database objects - tables, views, databases, stored procedures or search by name, definition or column
---

:::tip[Replaces (v1)]
This tool unifies: `list_tables`, `list_databases`, `list_stored_procedures` and `search_objects` from the previous version.
:::



Unified tool for exploring database objects. Replaces `list_tables`, `list_databases`, `list_stored_procedures` and `search_objects`.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `type` | string | What to explore: `tables` (default), `views`, `databases`, `procedures`, `search` |
| `filter` | string | Name filter (LIKE). Valid for `tables` and `procedures` |
| `schema` | string | Schema filter. For `procedures` and for a column search (exact match) |
| `table` | string | Table filter for a column search (literal substring) |
| `pattern` | string | Search pattern. **Required** when `type=search` |
| `search_in` | string | Where to search: `name` (default), `definition` (source code) or `column` (columns whose name contains the term) |
| `limit` | integer | Maximum results for `type=search` (default 50, max 200) |

## Usage modes

### List tables and views (default)

```json
{ "name": "explore", "arguments": {} }
```

With filter:
```json
{ "name": "explore", "arguments": { "filter": "Order" } }
```

### List views

```json
{ "name": "explore", "arguments": { "type": "views" } }
```

Includes metadata for each view: `check_option`, `is_updatable` and a definition preview.

### List databases

```json
{ "name": "explore", "arguments": { "type": "databases" } }
```

### List stored procedures

```json
{ "name": "explore", "arguments": { "type": "procedures" } }
```

With name and schema filter:
```json
{ "name": "explore", "arguments": { "type": "procedures", "schema": "dbo", "filter": "Truck" } }
```

### Search objects by name

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "OrderTruck" } }
```

### Search inside procedure/view source code

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "OrderTruck", "search_in": "definition" } }
```

### Find which table owns a column

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "CustomerId", "search_in": "column" } }
```

Answers "which table has this column?" in a single call, instead of listing the catalog and inspecting table by table. The term is compared as a **literal substring**: the `%`, `_` and `[` wildcards are escaped, so `100%` finds the column `100%` and not every column ending in `100`.

Optional filters and limit:

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "Customer", "search_in": "column", "schema": "dbo", "table": "Order", "limit": 20 } }
```

This mode returns an **object**, not the plain array of the other modes:

```json
{
  "matches": [
    { "schema_name": "dbo", "table_name": "Order", "column_name": "CustomerId", "data_type": "int", "max_length": null, "is_nullable": "NO" }
  ],
  "returned": 1,
  "truncated": false,
  "limit": 50
}
```

`truncated: true` means more columns matched than the limit returned: narrow it with `schema`, `table` or a more specific term instead of treating the list as complete. `max_length` is `null` for non-character types.

## Row limit

All results are capped at **500 rows**. If there are more, the last element will include a `_truncated` warning key.

The column search (`search_in=column`) does not use that cap: it returns up to `limit` matches (50 by default, 200 max) and reports `truncated: true` when more existed.
