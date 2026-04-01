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
| `database` | string | No | Target a cross-database from `MSSQL_ALLOWED_DATABASES` |

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

# List tables in a cross-database
explore(type="tables", database="JJP_Carregues")

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

**Returns:** server name, database name, version, read-only status, whitelist tables, allowed databases, connection pool stats.

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
| `query` | string | Yes | SQL query (prepared statement execution) |

**Annotations:** not read-only, not destructive, not idempotent

**Examples:**

```
# Simple SELECT
query_database(query="SELECT TOP 10 * FROM Clientes")

# Aggregation
query_database(query="SELECT COUNT(*) AS total FROM Pedidos WHERE Fecha >= '2024-01-01'")

# JOIN across tables
query_database(query="SELECT p.Id, c.Nombre FROM Pedidos p JOIN Clientes c ON p.ClienteId = c.Id")

# Cross-database query (if allowed)
query_database(query="SELECT * FROM JJP_Carregues.dbo.Carregues WHERE Id = 1")

# INSERT into whitelisted table
query_database(query="INSERT INTO temp_ai (clave, valor) VALUES ('result_1', 'processed')")

# DDL on whitelisted table
query_database(query="CREATE TABLE temp_ai (id INT IDENTITY PRIMARY KEY, clave NVARCHAR(100), valor NVARCHAR(MAX))")
```

**Security constraints:**
- Read-only mode blocks INSERT/UPDATE/DELETE/CREATE/DROP except on whitelisted tables
- Cross-database tables are always read-only
- Server validates table names exist before execution
- All queries executed as prepared statements

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

### Pattern: Cross-database comparison
```
get_database_info → check allowed databases
explore(database="DB_A") → tables in DB_A
explore(database="DB_B") → tables in DB_B
query_database(query="SELECT ... FROM DB_A.dbo.T UNION ALL SELECT ... FROM DB_B.dbo.T")
```

### Pattern: Safe write to temp table
```
get_database_info → confirm whitelist includes temp_ai
query_database(query="CREATE TABLE temp_ai (...)")
query_database(query="INSERT INTO temp_ai SELECT ... FROM SourceTable")
query_database(query="SELECT * FROM temp_ai") → verify results
```
