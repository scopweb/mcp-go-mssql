# Guide: Dynamic Mode for One Application with Multiple Related Databases

**Goal of this guide**: Configure the MCP MSSQL server in dynamic mode in a **secure and practical** way when you have **one logical application** that needs to access several related databases.

---

## The Real-World Use Case

Many enterprise applications do not live in a single database. It is very common to have scenarios like:

- **CRM** + **Identity** + **Orders** + **Billing**
- Main database + Audit/Logs database + Reporting database
- Modules separated for technical or historical reasons

In these cases, you want to:
- Load **only one MCP server** in Claude Desktop / Grok / Claude Code.
- Easily switch between the related databases of that application.
- Keep a high level of security (without exposing all company databases).

**Dynamic Mode** is the right tool for this.

---

## Core Security Principle

> **Only define the aliases that belong to that specific application.**  
> Never expose all company databases just because "you might need them someday".

This is the most common and dangerous mistake.

---

## Recommended Secure Configuration

### 1. `.env` Structure

```env
# ============================================
# DYNAMIC MODE - Single Application Scope
# ============================================
MSSQL_DYNAMIC_MODE=true
DEVELOPER_MODE=false

# --- Main application database ---
MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.company.local
MSSQL_DYNAMIC_APP_MAIN_DATABASE=MyApp_Production
MSSQL_DYNAMIC_APP_MAIN_USER=app_ai_read
MSSQL_DYNAMIC_APP_MAIN_PASSWORD=...
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true

# --- Identity / Users database (related) ---
MSSQL_DYNAMIC_APP_IDENTITY_SERVER=sql01.company.local
MSSQL_DYNAMIC_APP_IDENTITY_DATABASE=IdentityDB
MSSQL_DYNAMIC_APP_IDENTITY_USER=app_ai_read
MSSQL_DYNAMIC_APP_IDENTITY_PASSWORD=...
MSSQL_DYNAMIC_APP_IDENTITY_READ_ONLY=true

# --- Audit / Logs database ---
MSSQL_DYNAMIC_APP_AUDIT_SERVER=sql01.company.local
MSSQL_DYNAMIC_APP_AUDIT_DATABASE=AuditLogs
MSSQL_DYNAMIC_APP_AUDIT_USER=app_ai_read
MSSQL_DYNAMIC_APP_AUDIT_PASSWORD=...
MSSQL_DYNAMIC_APP_AUDIT_READ_ONLY=true

# ============================================
# ONLY if you truly need write access
# ============================================

# Example: Dedicated work area for the AI
MSSQL_DYNAMIC_APP_WORK_SERVER=devsql01.company.local
MSSQL_DYNAMIC_APP_WORK_DATABASE=AI_WorkArea
MSSQL_DYNAMIC_APP_WORK_USER=app_ai_writer
MSSQL_DYNAMIC_APP_WORK_PASSWORD=...

# IMPORTANT: Even when allowing writes, strictly limit the tables
MSSQL_DYNAMIC_APP_WORK_READ_ONLY=false
MSSQL_DYNAMIC_APP_WORK_WHITELIST_TABLES=ai_temp_tasks,ai_temp_results,ai_work_orders
```

### 2. Recommended SQL Server Users

Create specific users with minimal permissions:

| Alias           | Recommended SQL User | Recommended Permissions               |
|-----------------|----------------------|---------------------------------------|
| `APP_MAIN`      | `app_ai_read`        | Only `SELECT` + specific views        |
| `APP_IDENTITY`  | `app_ai_read`        | Only `SELECT`                         |
| `APP_AUDIT`     | `app_ai_read`        | Only `SELECT`                         |
| `APP_WORK`      | `app_ai_writer`      | Only on whitelisted tables            |

**Never use `sa` or a highly privileged user** for dynamic aliases.

---

## How to Use It in Practice (with the AI)

Once configured, in your conversation with Claude or Grok you can say:

```
Load the dynamic mode for the application.

First connect to the main database using dynamic_connect with alias "APP_MAIN".

Then tell me how many active customers are in the Customers table.

Now switch to the Identity database (alias APP_IDENTITY) and find the user with email juan.perez@company.com.

Go back to the main database and give me the last 10 orders from that customer.
```

The AI can switch databases during the same conversation using `dynamic_connect`.

---

## Cross-Database Queries

SQL Server allows querying across databases using 3 or 4-part names:

```sql
-- From APP_MAIN, query data from Identity
SELECT 
    c.Name,
    c.Email,
    u.LastLogin
FROM Customers c
INNER JOIN IdentityDB.dbo.Users u 
    ON c.UserId = u.Id
WHERE c.Status = 'Active';
```

**Advantages**:
- You can mostly stay in one alias (the main one).
- You reduce the number of connection switches.

**Requirements**:
- The connection user must have permissions on the other databases.
- The databases must be on the same SQL Server instance (or use Linked Servers).

---

## Advanced Security Strategies (Recommended)

### Secure by Default Behavior

The server enforces strong safe defaults:

- If you do not set `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY`, the alias loads as **read-only** by default.
- If a writable alias has no `WHITELIST_TABLES` defined, **no modifications are allowed**.
- Any modification on a writable alias requires explicit confirmation via the `confirm_operation` tool.

### Option 1: Everything read-only + one controlled work database (Safest)

This is the most balanced recommendation:

- All "real" databases → `READ_ONLY=true`
- One dedicated database for AI work → `READ_ONLY=false` + very strict whitelist + mandatory `confirm_operation`

Example of allowed tables in the work database:
- `ai_temp`
- `ai_results`
- `ai_tasks`
- `ai_audit_log`

### Option 2: Use a database user with very limited permissions even for writes

Even if you set `READ_ONLY=false`, the database user should only have permissions on the whitelisted tables (not `db_owner`).

---

## Summary of Best Practices

| Practice                                   | Recommended     | Comment |
|--------------------------------------------|-----------------|---------|
| Number of aliases per application          | 3 to 6 maximum  | Less is more secure |
| `READ_ONLY=true` by default                | Yes             | Golden rule |
| Use `WHITELIST_TABLES` when writing        | Always          | Limits blast radius |
| Different DB user per alias type           | Yes             | `app_ai_read` vs `app_ai_writer` |
| Expose all company databases               | Never           | Only define aliases for this application |

---

## Complete Recommended Example for an Application

```env
MSSQL_DYNAMIC_MODE=true
DEVELOPER_MODE=false

# === Read-only only (the majority) ===
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true
MSSQL_DYNAMIC_APP_IDENTITY_READ_ONLY=true
MSSQL_DYNAMIC_APP_AUDIT_READ_ONLY=true
MSSQL_DYNAMIC_APP_REPORTING_READ_ONLY=true

# === Controlled writes (AI work only) ===
MSSQL_DYNAMIC_APP_AIWORK_READ_ONLY=false
MSSQL_DYNAMIC_APP_AIWORK_WHITELIST_TABLES=ai_temp,ai_results,ai_tasks,ai_logs
```

With this setup you load **only one MCP server** and can safely work with all related databases of your application.

---

## Need Help With Your Specific Case?

If you tell me:
- How many databases your application has
- Which ones really need write access
- What kind of operations you want the AI to perform (insert into temp tables, update statuses, generate reports, etc.)

I can give you the exact `.env` configuration and the recommended SQL Server permissions.

---

**Related**:
- [FAQ: Dynamic Connections](../guias/faq-conexiones-dinamicas.md) (Spanish)
- [Environment Variables](../configuracion/variables-entorno.md)
- [Read-Only Mode and Whitelist](../seguridad/modo-solo-lectura.md) (Spanish)
- [Table Whitelist](../seguridad/whitelist-tablas.md) (Spanish)
