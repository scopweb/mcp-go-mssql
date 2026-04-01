---
title: explore
description: Explore database objects - tables, views, databases, stored procedures or search by name/definition
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
| `schema` | string | Schema filter. Only for `procedures` (optional) |
| `pattern` | string | Search pattern. **Required** when `type=search` |
| `search_in` | string | Where to search: `name` (default) or `definition` (source code) |
| `database` | string | Explore tables in an allowed cross-database (requires `MSSQL_ALLOWED_DATABASES`) |

## Usage modes

### List tables and views (default)

```json
{ "name": "explore", "arguments": {} }
```

With filter:
```json
{ "name": "explore", "arguments": { "filter": "Order" } }
```

### List views only (with metadata)

```json
{ "name": "explore", "arguments": { "type": "views" } }
```

With filter:
```json
{ "name": "explore", "arguments": { "type": "views", "filter": "v_Order" } }
```

Returns: `schema_name`, `view_name`, `check_option`, `is_updatable`, `definition_preview` (300 chars of source code).

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

### Explore tables in another database

Requires `MSSQL_ALLOWED_DATABASES` to be configured:

```json
{ "name": "explore", "arguments": { "database": "OtherDB" } }
```

With filter:
```json
{ "name": "explore", "arguments": { "database": "OtherDB", "filter": "Order" } }
```

:::note
Only databases listed in `MSSQL_ALLOWED_DATABASES` can be explored. Attempting to access a non-allowed database returns an error with the list of allowed databases.
:::

## Row limit

All results are capped at **500 rows**. If there are more, the last element will include a `_truncated` warning key.
