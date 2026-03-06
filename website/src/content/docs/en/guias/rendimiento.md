---
title: Performance
description: Performance optimization for MCP-Go-MSSQL
---

## Connection pooling

MCP-Go-MSSQL uses the built-in connection pool from the `go-mssqldb` driver. Connections are automatically reused.

### Pool configuration

The pool is configured with limits to prevent resource exhaustion:

- **Maximum open connections**: Limited to avoid saturating the SQL Server
- **Idle connections**: Kept open for quick reuse
- **Timeouts**: Connections exceeding the timeout are automatically closed

## Prepared statements

All queries use `PrepareContext()`, which allows SQL Server to cache execution plans and improve performance on repeated queries.

## Recommendations

### Efficient queries

- Use `SELECT` with specific columns instead of `SELECT *`
- Limit results with `TOP` or `OFFSET/FETCH`
- Leverage existing indexes in `WHERE` clauses

### Monitoring

- Watch query response times in the logs
- Use `get_indexes` to verify that tables have adequate indexes
- Check `get_database_info` for general statistics

### Timeouts

Connection timeouts prevent queries from running indefinitely. If a legitimate query exceeds the timeout, consider optimizing it or increasing the limit.
