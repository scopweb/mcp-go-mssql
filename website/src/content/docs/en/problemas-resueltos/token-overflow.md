---
title: "Token overflow when exploring large databases"
description: list_tables without a filter or row limit caused Claude's response to be too large, failing with "Could not fully generate Claude's response"
---

When trying to explore a database with many tables, or search for references to a SQL object, Claude would fail with:

> **Could not fully generate Claude's response**

## Issue details

| Field | Value |
|---|---|
| **Date** | 2026-03-03 |
| **Severity** | High (blocks exploration of large databases) |
| **Status** | ✅ Resolved |

## Root cause

Three combined problems:

1. **`executeSecureQuery` had no row limit** — it returned all results without a cap, generating JSON blobs of hundreds of KB that exceeded Claude's context window.
2. **`list_tables` had no filter** — on databases with more than 200 tables/views the result was massive.
3. **No direct object search tool** — finding "which procedures reference X" required listing all objects and inspecting them one by one (extremely expensive in tokens).

## Solutions applied

### 1 — Global 500-row limit

All queries are now capped at 500 rows. If the result is truncated, the last element includes a `_truncated` warning so Claude knows to narrow the query with `WHERE` or `TOP`.

### 2 — `filter` parameter for object exploration

Data can now be filtered before fetching (currently via `explore`, which unifies the old `list_tables`):

```
explore  filter="Order"    →  only tables/views containing "Order"
explore  filter="Truck"    →  only tables/views containing "Truck"
```

### 3 — Direct object search

`explore` with `type="search"` (formerly the standalone `search_objects` tool) searches in two modes:

| Mode | Usage | Returns |
|---|---|---|
| By name (default) | `explore type="search" pattern="OrderTruck"` | Tables, views, procs and functions whose **name** matches |
| By definition | `explore type="search" pattern="OrderTruck" search_in="definition"` | Procs, functions and views that **reference** that text in their source code |

## Updated function schema

All connectors expose exactly the same functions:

| Function | Parameters |
|---|---|
| `query_database` | `query: string` |
| `get_database_info` | — |
| `explore` | `type?: string, filter?: string, schema?: string, pattern?: string, search_in?: string` |
| `inspect` | `table_name: string, schema?: string, detail?: string` |
| `execute_procedure` | `procedure_name: string, parameters?: string` |

## Example: original use case resolved

To find references to `PedidoCamioCarregaCamioAdmin` with states 26/27:

```sql
-- 1. Search the object by name
explore  type="search"  pattern="PedidoCamioCarregaCamioAdmin"

-- 2. Find which procs/views reference it in their code
explore  type="search"  pattern="PedidoCamioCarregaCamioAdmin"  search_in="definition"

-- 3. Direct query filtering by state
query_database  query="SELECT * FROM PedidoCamioCarregaCamio WHERE Estado IN (26,27)"
```
