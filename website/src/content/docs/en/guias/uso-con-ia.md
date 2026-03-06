---
title: AI Usage
description: Recommended configuration for using MCP-Go-MSSQL with AI assistants
---

MCP-Go-MSSQL is designed so that Claude and other AI assistants can work with production databases safely.

## Recommended AI-Safe configuration

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

This configuration allows the AI to:
- **Read** any table in the database
- **Write** only to `temp_ai` and `v_temp_ia`
- All other tables remain protected from modification

## Typical workflow

1. The AI queries production data with `query_database`
2. Processes and transforms the data
3. Writes results to whitelisted temporary tables
4. The user reviews and promotes the data as appropriate

## Temporary tables for AI

Create dedicated tables for the AI to work with:

```sql
CREATE TABLE temp_ai (
    id INT IDENTITY PRIMARY KEY,
    created_at DATETIME DEFAULT GETDATE(),
    data NVARCHAR(MAX)
);
```

Add them to the whitelist:

```bash
MSSQL_WHITELIST_TABLES=temp_ai
```

## Error protection

The read-only + whitelist mode protects against:
- Accidental deletion of production data
- Modification of critical tables
- SQL injection attempting to access unauthorized tables via JOIN
