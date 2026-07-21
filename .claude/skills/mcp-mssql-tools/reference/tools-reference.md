# MCP MSSQL Tools — Complete Reference

## 1. `explore`

Discover database objects. The starting point for any database interaction.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | No | `tables` (default), `views`, `databases`, `procedures`, `search` |
| `filter` | string | No | Name filter (LIKE match, e.g. `Pedido`) |
| `schema` | string | No | Schema filter for procedures |
| `pattern` | string | When `type=search` | Search term |
| `search_in` | string | No | `name` (default) or `definition` (searches inside source code) |

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

# List all views with metadata (check_option, is_updatable, definition preview)
explore(type="views")

# List all databases on the server
explore(type="databases")
```

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

**Returns:** server name, database name, version, read-only status, whitelist tables, connection pool stats.

**When to use:**
- Start of conversation — confirm you're connected
- Before write operations — check if read-only mode is active
- Debugging connection issues

---

## 4. `query_database`

Execute SQL against the database. All queries use prepared statements.

**Aliases:** run_sql, execute_sql, db_query, sql_execute, sql_query, run_query, exec_query

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `query` | string | Yes | SQL query (SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP) |

**Annotations:** not read-only, not destructive, not idempotent

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
- Read-only mode blocks INSERT/UPDATE/DELETE/CREATE/DROP except on whitelisted tables
- Multi-statement batches are rejected in read-only/whitelist contexts (only `SET STATISTICS TIME/IO` diagnostic wrappers are allowed)
- All queries executed as prepared statements

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
get_database_info           → check connection & mode
explore(type="tables")      → see all tables
explore(type="search", pattern="Order") → find relevant tables
inspect(table_name="Orders", detail="all") → understand structure
query_database(query="SELECT TOP 5 * FROM Orders") → sample data
```

### Pattern: Performance investigation
```
explain_query(query="SELECT ...")  → check plan
inspect(table_name="X", detail="indexes") → check available indexes
query_database(query="optimized query") → run with improvements
```

### Pattern: Safe write to temp table
```
get_database_info → confirm whitelist includes temp_ai
query_database(query="CREATE TABLE temp_ai (...)")
query_database(query="INSERT INTO temp_ai SELECT ... FROM SourceTable")
query_database(query="SELECT * FROM temp_ai") → verify results
```

---

## 11. `dynamic_available`

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

## 12. `dynamic_connect`

Activate a pre-configured dynamic database connection.

**Parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `alias` | string | Yes | Connection alias from `.env` (e.g., `ferratge`, `identity`) |

**Annotations:** non-destructive, idempotent (reconnecting overwrites existing)

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

## 13. `dynamic_list`

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

## 14. `dynamic_disconnect`

Close the currently active dynamic connection and return to disconnected state.

**Parameters:** none

**Annotations:** non-destructive, idempotent (disconnecting with no active connection is a no-op)

**Examples:**

```
dynamic_disconnect()
```

---

## 15. Targeting a dynamic connection

There is no `connection` parameter on `query_database`. Use `dynamic_connect(alias="...")` to switch the active connection; all subsequent `query_database` calls run against that alias and inherit its security posture (per-alias READ_ONLY + WHITELIST_TABLES).
