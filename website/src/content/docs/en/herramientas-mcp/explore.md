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

### Search by name, by column or inside the source code

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "Pedido" } }
{ "name": "explore", "arguments": { "type": "search", "pattern": "CustomerId", "search_in": "column" } }
{ "name": "explore", "arguments": { "type": "search", "pattern": "Pedido", "search_in": "definition" } }
```

The three search modes share one answer: the matched objects, the reason of the match, the direct foreign keys of the matched tables and the suggested next step.

```json
{
  "matches": [{
    "object_type": "TABLE",
    "schema": "dbo",
    "name": "Order",
    "qualified_name": "dbo.Order",
    "match": {"field": "name", "kind": "exact", "term": "Order"},
    "columns": [{"name": "CustomerId", "data_type": "int", "max_length": null}],
    "foreign_keys": [{"from_columns": ["CustomerId"], "to_table": "dbo.Customer", "to_columns": ["Id"]}]
  }],
  "returned": 1,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {"max_objects": 50, "max_foreign_key_objects": 30,
             "max_foreign_keys_per_object": 10, "max_cell_bytes": null, "max_response_bytes": null},
  "next_steps": [{"tool": "inspect", "arguments": {"table_name": "dbo.Order", "detail": "all"},
                  "reason": "read the full structure of dbo.Order"}]
}
```

- The order is deterministic: an exact name first, then a prefix, then a substring, and the name as the tie break.
- `match` says which field matched (`name`, `column`, `definition`) and how (`exact`, `prefix`, `contains`).
- `foreign_keys` are the direct foreign keys of the matched tables: the join paths of the schema.
- `next_steps` suggests the call to continue, usually `inspect` with `detail=all`.
- `truncated: true` means more objects matched than the limit returned: narrow the search instead of treating the list as complete.

## Row limit

All results are capped at **500 rows**. If there are more, the last element will include a `_truncated` warning key.

The column search (`search_in=column`) does not use that cap: it returns up to `limit` matches (50 by default, 200 max) and reports `truncated: true` when more existed.
