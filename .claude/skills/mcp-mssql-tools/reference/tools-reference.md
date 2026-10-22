# MCP MSSQL Tools — Complete Reference

<!-- BEGIN GENERATED TOOL CATALOG -->
<!-- Generated from tools/list by cmd/mcp-go-mssql/tools_docs_test.go. Do not edit by hand. -->
<!-- Regenerate: go test ./cmd/mcp-go-mssql -run TestToolCatalogDocumentation -update -->

| Tool | Parameters | Required | Mode | readOnly | destructive | idempotent |
| --- | --- | --- | --- | --- | --- | --- |
| `query_database` | `query:string` | `query` | classic | no | yes | no |
| `get_database_info` | — | — | classic | yes | no | yes |
| `explore` | `filter:string`, `limit:integer`, `pattern:string`, `schema:string`, `search_in:string`, `table:string`, `type:string` | — | classic | yes | no | yes |
| `inspect` | `detail:string`, `schema:string`, `table_name:string` | `table_name` | classic | yes | no | yes |
| `execute_procedure` | `parameters:string`, `procedure_name:string` | `procedure_name` | classic | no | yes | no |
| `explain_query` | `query:string` | `query` | classic | yes | no | yes |
| `dynamic_available` | — | — | dynamic-only | yes | no | yes |
| `dynamic_connect` | `alias:string` | `alias` | dynamic-only | no | no | no |
| `dynamic_disconnect` | — | — | dynamic-only | yes | no | yes |
| `dynamic_list` | — | — | dynamic-only | yes | no | yes |
| `confirm_operation` | `description:string` | `description` | dynamic-only | no | yes | no |

<!-- END GENERATED TOOL CATALOG -->

## 1. `explore`

Discover database objects. The starting point for any database interaction.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | No | `tables` (default), `views`, `databases`, `procedures`, `search` |
| `filter` | string | No | Name filter (LIKE match, e.g. `Pedido`) |
| `schema` | string | No | Schema filter for procedures; exact schema filter for a column search |
| `table` | string | No | Table filter for a column search (literal substring) |
| `pattern` | string | When `type=search` | Search term |
| `search_in` | string | No | `name` (default), `definition` (searches inside source code), `column` (columns whose name contains the term) |
| `limit` | integer | No | Maximum results for `type=search` (default 50, max 200) |

**Annotations:** read-only, non-destructive, idempotent

**Examples:**

```
# List all tables and views
explore()

# Filter tables by name
explore(filter="Cliente")

# List all stored procedures
explore(type="procedures")

# Search objects by name
explore(type="search", pattern="Pedido")

# Search inside procedure/view source code
explore(type="search", pattern="INSERT INTO temp_ai", search_in="definition")

# Find which table owns a column
explore(type="search", pattern="ClienteId", search_in="column")

# Narrow a column search
explore(type="search", pattern="Cliente", search_in="column", schema="dbo", table="Pedido", limit=20)

# List all views with metadata (check_option, is_updatable, definition preview)
explore(type="views")

# List all databases on the server
explore(type="databases")
```

**Cautions:**
- `search_in="definition"` returns the **full source** of every match: keep the pattern specific, and expect a large response when many modules match.
- Definitions may be hidden without `VIEW DEFINITION` permission; encrypted modules expose nothing.
- Reading a definition is not permission to execute it.

### Column search (`search_in="column"`)

Finds the tables that own a column. The term is a **literal substring** of the column name: wildcards are escaped, so `100%` matches the column `100%` and not every column ending in `100`. Matching follows the database collation (case-insensitive by default).

This mode answers with an **object**, not the array the other modes return:

```json
{
  "matches": [
    {"schema_name": "dbo", "table_name": "Pedido", "column_name": "ClienteId",
     "data_type": "int", "max_length": null, "is_nullable": "NO"}
  ],
  "returned": 1,
  "truncated": false,
  "limit": 50
}
```

`truncated: true` means more columns matched than `limit` returned: narrow the search with `schema`, `table` or a more specific term instead of treating the list as complete. `max_length` is null for non-character types.

The query reads `INFORMATION_SCHEMA.COLUMNS` only, with a deterministic order (schema, table, column position). It asks for one row more than the limit, so it detects further matches without running a second count over the catalog.

---

## 2. `inspect`

Deep-dive into a single table's structure.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `table_name` | string | Yes | Table name (can include schema: `dbo.TableName`) |
| `schema` | string | No | Schema name (defaults to `dbo`) |
| `detail` | string | No | `columns` (default), `indexes`, `foreign_keys`, `dependencies`, `all` |

**Annotations:** read-only, non-destructive, idempotent

**Examples:**

```
# Column info (types, nullability, defaults)
inspect(table_name="Clientes")

# Everything at once: columns + indexes + FKs + dependencies
inspect(table_name="Pedidos", detail="all")

# Just foreign key relationships
inspect(table_name="PedidoLineas", detail="foreign_keys")

# What views/procedures reference this table?
inspect(table_name="Clientes", detail="dependencies")

# Table in a specific schema
inspect(table_name="staging.ImportData", detail="columns")
```

---

## 3. `get_database_info`

Check connection status and server configuration. No parameters.

**Returns:** plain text with the connection status, the server and database, the encryption mode, the access mode (read-only / whitelist-protected / full access) and, in dynamic mode, the active alias and its effective posture.

It does not report the server version or connection-pool statistics.

**When to use:**
- Start of conversation — confirm you're connected
- Before write operations — check if read-only mode is active
- Debugging connection issues

---

## 4. `query_database`

Execute SQL against the database. All queries use prepared statements.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `query` | string | Yes | SQL query (SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP) |

**Annotations:** not read-only, destructive, not idempotent

**Examples:**

```
# Simple SELECT
query_database(query="SELECT TOP 10 * FROM Clientes")

# Aggregation
query_database(query="SELECT COUNT(*) AS total FROM Pedidos WHERE Fecha >= '2024-01-01'")

# JOIN across tables
query_database(query="SELECT p.Id, c.Nombre FROM Pedidos p JOIN Clientes c ON p.ClienteId = c.Id")

# INSERT into whitelisted table
query_database(query="INSERT INTO temp_ai (clave, valor) VALUES ('result_1', 'processed')")

# DDL on whitelisted table
query_database(query="CREATE TABLE temp_ai (id INT IDENTITY PRIMARY KEY, clave NVARCHAR(100), valor NVARCHAR(MAX))")
```

**Security constraints:**
- Read-only mode blocks INSERT/UPDATE/DELETE/CREATE/DROP and every other modification, `EXEC` and `execute_procedure`. There is no whitelist exception while read-only is active.
- A whitelist only limits modifications in an explicitly writable context (`MSSQL_READ_ONLY=false` or a dynamic alias with `_READ_ONLY=false`); a writable alias without a whitelist permits no modifications.
- Multi-statement batches are rejected in read-only/whitelist contexts (only `SET STATISTICS TIME/IO` diagnostic wrappers are allowed)
- All queries executed as prepared statements
- Results are capped at 500 rows; the `truncated` flag marks a partial result that must be presented as partial

**When a modification is blocked by read-only mode:**
1. DO NOT try to bypass it — no `execute_procedure`, alternative tools, `SET` tricks, or re-framed statements. Read-only is a deliberate security boundary.
2. DO NOT suggest elevating permissions (changing `MSSQL_READ_ONLY`, widening whitelists) unless the user explicitly asks.
3. Instead, present the complete, ready-to-run SQL in a ```sql code block and briefly state the connection is read-only, so the user can execute it manually (SSMS, sqlcmd, Azure Data Studio, etc.).

---

## 5. `execute_procedure`

Run a stored procedure from the whitelist (`MSSQL_WHITELIST_PROCEDURES` env var).

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `procedure_name` | string | Yes | Procedure name (must be whitelisted) |
| `parameters` | string | No | JSON object with param names and values |

**Annotations:** not read-only, destructive, not idempotent

**Examples:**

```
# No parameters
execute_procedure(procedure_name="sp_RefreshCache")

# With parameters
execute_procedure(procedure_name="sp_ProcessOrder", parameters='{"OrderId": 123, "Status": "Approved"}')
```

**Constraints:**
- Only procedures listed in `MSSQL_WHITELIST_PROCEDURES` can be executed
- If the env var is not set, this tool returns an error for any procedure

---

## 6. `explain_query`

Show the estimated execution plan without running the query. SELECT only.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `query` | string | Yes | SELECT query to analyze |

**Annotations:** read-only, non-destructive, idempotent

**Examples:**

```
# Simple plan
explain_query(query="SELECT * FROM Pedidos WHERE ClienteId = 5")

# Complex join plan
explain_query(query="SELECT p.*, c.Nombre FROM Pedidos p JOIN Clientes c ON p.ClienteId = c.Id WHERE p.Fecha > '2024-01-01'")
```

**When to use:**
- Query is slow and you need to understand why
- Before running a heavy query on production
- To check if indexes are being used

---

## 7. `confirm_operation`

Confirm a pending modification (INSERT/UPDATE/DELETE/DDL) on a **writable dynamic alias**. Only available in dynamic mode; not exposed on classic connections.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | Yes | The exact description returned in the CONFIRMATION REQUIRED error |

**When to use:**
- When `query_database` returns a "CONFIRMATION REQUIRED" error on a writable dynamic alias
- The confirmation expires after 90 seconds and is bound to the alias it was issued for (cleared on alias switch/disconnect)

**Example:**
```
confirm_operation(description="DELETE on tables: temp_ai (alias AIWORK)")
```

---

## Common Patterns

### Pattern: Data exploration workflow
```
get_database_info           → check connection & access mode
explore(type="search", pattern="Order") → find the relevant objects first
inspect(table_name="Orders", detail="columns") → understand the structure you need
query_database(query="SELECT TOP 5 * FROM Orders") → sample data
```

Prefer a targeted search over a full catalog listing, and `detail="columns"` over `detail="all"` (which runs four catalog queries).

### Pattern: Performance investigation
```
explain_query(query="SELECT ...")  → check plan
inspect(table_name="X", detail="indexes") → check available indexes
query_database(query="optimized query") → run with improvements
```

### Pattern: Safe write to temp table
```
get_database_info → confirm the whitelist includes temp_ai
query_database(query="CREATE TABLE temp_ai (...)")
query_database(query="INSERT INTO temp_ai SELECT ... FROM SourceTable")
query_database(query="SELECT * FROM temp_ai") → verify results
```

Requires an explicitly writable context: with `MSSQL_READ_ONLY=true` the server rejects the modifications and you must hand the SQL to the user.

---

## 8. `dynamic_available`

Discover pre-configured dynamic connections from `.env` (without connecting).

**Parameters:** none

**Annotations:** read-only, non-destructive, idempotent

**Examples:**

```
# List all configured aliases (no credentials shown)
dynamic_available()
```

**Notes:**
- Reads the environment at startup (including the `.env` file next to the executable, unless `MSSQL_IGNORE_LOCAL_ENV=true`)
- Shows: alias, server, database — NO passwords or users
- Only available in dynamic mode (see mode selection in SKILL.md)

---

## 9. `dynamic_connect`

Activate a pre-configured dynamic database connection.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `alias` | string | Yes | Connection alias from `.env` (e.g., `ferratge`, `identity`) |

**Annotations:** non-destructive, not idempotent (reconnecting replaces the active connection)

**Examples:**

```
# Connect to a dynamic database
dynamic_connect(alias="ferratge")
```

**Notes:**
- Credentials come from env (`MSSQL_DYNAMIC_<ALIAS>_*`)
- Connection inherits security config: READ_ONLY, WHITELIST_TABLES
- If already connected, replaces the existing connection
- A failed connect keeps the previous working connection alive

---

## 10. `dynamic_list`

List all active (connected) dynamic database connections.

**Parameters:** none

**Annotations:** read-only, non-destructive, idempotent

**Examples:**

```
# List active connections
dynamic_list()
```

**Response example:**
```
Active dynamic connections:
- ferratge (SQL01/JJP_Ferratge_DEV) - connected 5m ago
- identity (10.203.3.11/JJP_CRM_IDENTITY) - connected 1h ago
```

---

## 11. `dynamic_disconnect`

Close the currently active dynamic connection and return to disconnected state.

**Parameters:** none

**Annotations:** non-destructive, idempotent (disconnecting with no active connection is a no-op)

**Examples:**

```
dynamic_disconnect()
```

---

## 12. Targeting a dynamic connection

There is no `connection` parameter on `query_database`. Use `dynamic_connect(alias="...")` to switch the active connection; all subsequent `query_database` calls run against that alias and inherit its security posture (per-alias READ_ONLY + WHITELIST_TABLES).
