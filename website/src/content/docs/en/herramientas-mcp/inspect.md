---
title: inspect
description: Inspect a table's structure - columns, indexes, foreign keys or everything at once
---

:::tip[Replaces (v1)]
This tool unifies: `describe_table`, `get_indexes` and `get_foreign_keys` from the previous version.
:::



Unified tool for inspecting a table's structure. Replaces `describe_table`, `get_indexes` and `get_foreign_keys`.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `table_name` | string | **Required.** Table name. Accepts `dbo.Table` or just `Table` |
| `schema` | string | Schema name (default: `dbo`) |
| `detail` | string | What to retrieve: `columns` (default), `indexes`, `foreign_keys`, `dependencies`, `all` |

## Usage modes

### Columns (default)

```json
{ "name": "inspect", "arguments": { "table_name": "Orders" } }
```

### Indexes

```json
{ "name": "inspect", "arguments": { "table_name": "Orders", "detail": "indexes" } }
```

### Foreign keys

```json
{ "name": "inspect", "arguments": { "table_name": "Orders", "detail": "foreign_keys" } }
```

### Dependencies (impact analysis)

```json
{ "name": "inspect", "arguments": { "table_name": "Orders", "detail": "dependencies" } }
```

Shows which SQL objects (views, procedures, functions) **depend on this table**. Uses `sys.sql_expression_dependencies` for impact analysis before schema changes.

Returns: `referencing_schema`, `referencing_object`, `referencing_type`, `is_caller_dependent`, `is_ambiguous`.

### Everything in one call

```json
{ "name": "inspect", "arguments": { "table_name": "Orders", "detail": "all" } }
```

With `detail=all` the result groups sections under the keys `columns`, `indexes`, `foreign_keys` and `dependencies`.

## Example response (detail=all)

```json
{
  "columns": [ {"column_name": "Id", "data_type": "int", ...} ],
  "indexes": [ {"index_name": "PK_Orders", "is_primary_key": true, ...} ],
  "foreign_keys": [ {"constraint_name": "FK_Orders_Customers", ...} ]
}
```
