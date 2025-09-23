# Issue #2: Increase Query Size Limit for Large SQL Queries

## Problem Description
The MCP server had a restrictive query size limit of 4,096 characters, which was insufficient for complex SQL queries, large stored procedure calls, or bulk operations. Users encountered "input too large" errors when attempting to execute legitimate queries.

## Error Symptoms
- **Error Message**: `Query Error: input too large`
- **Limitation**: 4,096 character maximum for SQL queries
- **Impact**: Unable to execute complex queries, CTEs, or large SELECT statements
- **Use Cases Affected**:
  - Complex reporting queries with multiple JOINs
  - Common Table Expressions (CTEs)
  - Large INSERT statements with multiple VALUES
  - Stored procedure calls with extensive parameters

## Root Cause
The original implementation used a hardcoded limit for security purposes:

```go
func (s *MCPMSSQLServer) validateBasicInput(input string) error {
    if len(input) > 4096 {  // Too restrictive
        return fmt.Errorf("input too large")
    }
    // ...
}
```

This limit was too conservative for real-world database operations while still providing reasonable protection against abuse.

## Solution Implemented

### 1. Increased Default Limit
Raised the default query size limit from 4KB to 1MB (1,048,576 characters):

```go
func (s *MCPMSSQLServer) validateBasicInput(input string) error {
    // Allow larger queries - up to 1MB (1048576 characters)
    maxSize := 1048576
    if customMax := os.Getenv("MSSQL_MAX_QUERY_SIZE"); customMax != "" {
        if size, err := strconv.Atoi(customMax); err == nil && size > 0 {
            maxSize = size
        }
    }

    if len(input) > maxSize {
        return fmt.Errorf("input too large (max %d characters)", maxSize)
    }
    // ...
}
```

### 2. Configurable Limit
Added `MSSQL_MAX_QUERY_SIZE` environment variable for custom limits:

**Configuration Example:**
```json
{
  "env": {
    "MSSQL_SERVER": "server.example.com",
    "MSSQL_DATABASE": "MyDatabase",
    "MSSQL_USER": "user",
    "MSSQL_PASSWORD": "password",
    "MSSQL_MAX_QUERY_SIZE": "500000",
    "DEVELOPER_MODE": "true"
  }
}
```

### 3. Better Error Messages
Enhanced error reporting to show the current limit:
- **Before**: `input too large`
- **After**: `input too large (max 1048576 characters)`

## Size Comparison

| Scenario | Old Limit | New Default | Custom Option |
|----------|-----------|-------------|---------------|
| Simple SELECT | ✅ 4KB | ✅ 1MB | ✅ Configurable |
| Complex JOIN | ❌ 4KB | ✅ 1MB | ✅ Configurable |
| Large CTE | ❌ 4KB | ✅ 1MB | ✅ Configurable |
| Bulk INSERT | ❌ 4KB | ✅ 1MB | ✅ Configurable |
| Stored Proc Call | ❌ 4KB | ✅ 1MB | ✅ Configurable |

## Security Considerations

### Maintained Security
- **Still Protected**: Prevents extremely large queries that could cause memory issues
- **Reasonable Limit**: 1MB allows complex queries while preventing abuse
- **Configurable**: Administrators can set appropriate limits for their environment

### Memory Usage
- **Typical Query**: 1-10KB (well within limits)
- **Complex Query**: 50-200KB (now supported)
- **Maximum Default**: 1MB (prevents memory exhaustion)

## Testing Results

### Before Fix
```sql
-- This query failed at ~5KB
SELECT
  col1, col2, col3, col4, col5, col6, col7, col8, col9, col10,
  -- ... many more columns and complex logic
FROM table1 t1
JOIN table2 t2 ON t1.id = t2.foreign_id
-- ... additional JOINs and WHERE clauses
-- ERROR: input too large
```

### After Fix
```sql
-- Same query now succeeds
-- ✅ Works with queries up to 1MB
-- ✅ Configurable for specific needs
-- ✅ Better error messages
```

## Configuration Options

### Default Configuration (1MB limit)
No additional configuration needed - works out of the box.

### Custom Limit Configuration
```json
{
  "env": {
    "MSSQL_MAX_QUERY_SIZE": "2097152",  // 2MB limit
    "MSSQL_CONNECTION_STRING": "...",
    "DEVELOPER_MODE": "true"
  }
}
```

### Conservative Limit (if needed)
```json
{
  "env": {
    "MSSQL_MAX_QUERY_SIZE": "65536",   // 64KB limit
    "MSSQL_CONNECTION_STRING": "...",
    "DEVELOPER_MODE": "false"
  }
}
```

## Files Modified
- `main.go`: Updated `validateBasicInput()` function
- `README.md`: Documented new `MSSQL_MAX_QUERY_SIZE` variable
- Added `strconv` import for integer parsing

## Benefits
1. **Supports Complex Queries**: Can handle large, real-world SQL operations
2. **Maintains Security**: Still prevents abuse with reasonable limits
3. **Flexible Configuration**: Administrators can set appropriate limits
4. **Better User Experience**: Clear error messages with actual limits
5. **Backward Compatible**: Existing configurations continue to work

## Issue Status: ✅ RESOLVED
**Fixed in commit**: Increase query size limit to 1MB with configurable option