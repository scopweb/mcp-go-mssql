## Bug: SQL Syntax Error in `describe_table` Tool

### Problem
The `describe_table` tool was failing with the error:
```
mssql: Incorrect syntax near '?'
```

### Root Cause
Both `main.go` and `claude-code/db-connector.go` were using `?` as a parameter placeholder in SQL queries, which is the MySQL/ODBC style. The `go-mssqldb` driver requires MSSQL-style parameters using `@p1, @p2, ...` format.

### Files Affected
1. **main.go:592** - `describe_table` tool in MCP server
2. **claude-code/db-connector.go:287** - `describeTable` function in CLI tool

### Incorrect Code
```sql
WHERE TABLE_NAME = ?
```

### Fixed Code
```sql
WHERE TABLE_NAME = @p1
```

### Changes Made
- Replaced `?` with `@p1` in both SQL queries
- Simplified parameter passing from `sql.Named()` to direct parameter in `QueryContext`
- Added proper error handling for column retrieval in db-connector.go

### Testing
The fix was verified to work correctly with:
- Claude Desktop MCP integration
- Direct CLI tool usage

### Impact
- **Before**: `describe_table` tool completely non-functional
- **After**: Tool works correctly, returning proper table structure information

### Related Code
```go
// Before
query := `... WHERE TABLE_NAME = ?`
stmt, err := db.PrepareContext(ctx, query)
rows, err := stmt.QueryContext(ctx, sql.Named("tableName", tableName))

// After
query := `... WHERE TABLE_NAME = @p1`
rows, err := db.QueryContext(ctx, query, tableName)
```
