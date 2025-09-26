# Issue: Missing `describe` Tool in MCP Server

## Problem Description

When using the MCP Go MSSQL server with Claude Desktop, users encountered an error when trying to use a "describe" tool. The error indicated that the tool was not available, even though the documentation and CLI tool (`claude-code/db-connector.go`) referenced describe functionality.

### Error Symptoms
- Claude Desktop reported that "describe" tool does not exist
- MCP server only exposed 2 tools: `query_database` and `get_database_info`
- Users expected table description and listing functionality similar to the CLI tool

## Root Cause Analysis

The MCP server implementation in `main.go` was missing essential database exploration tools that were available in the CLI tool:

1. **Missing Tools:**
   - `describe_table` - To describe table structure and schema
   - `list_tables` - To list all tables and views in the database

2. **Inconsistency:**
   - CLI tool (`claude-code/db-connector.go`) had `describe` and `tables` commands
   - MCP server (`main.go`) did not expose equivalent functionality
   - Documentation referenced these features but they were not available via MCP

## Solution Implemented

### 1. Added New Tools to MCP Server

Added two new tools to the `tools/list` handler in `main.go`:

```go
{
    Name:        "list_tables",
    Description: "List all tables and views in the database",
    InputSchema: InputSchema{
        Type:       "object",
        Properties: map[string]Property{},
        Required:   []string{},
    },
},
{
    Name:        "describe_table",
    Description: "Get the structure and schema information for a specific table",
    InputSchema: InputSchema{
        Type: "object",
        Properties: map[string]Property{
            "table_name": {
                Type:        "string",
                Description: "Name of the table to describe",
            },
        },
        Required: []string{"table_name"},
    },
},
```

### 2. Implemented Tool Handlers

Added corresponding case handlers in `handleToolCall` method:

#### `list_tables` Implementation
- Uses `INFORMATION_SCHEMA.TABLES` query
- Returns all tables and views with schema information
- Filters for 'BASE TABLE' and 'VIEW' types
- Orders by schema and table name

#### `describe_table` Implementation
- Uses `INFORMATION_SCHEMA.COLUMNS` query with parameterized input
- Returns detailed column information: name, data type, nullability, defaults, max length
- Uses prepared statements for security
- Validates table existence and provides appropriate error messages

### 3. Security Considerations

Both new tools maintain the existing security standards:
- Use prepared statements to prevent SQL injection
- Respect read-only mode restrictions (when enabled)
- Include proper input validation
- Maintain consistent error handling patterns

## Testing and Validation

1. **Compilation Test:** ✅ Code compiles without errors
2. **Tool Registration:** ✅ New tools appear in MCP tool list
3. **Security:** ✅ Uses parameterized queries and input validation
4. **Error Handling:** ✅ Proper error responses for edge cases

## Available Tools After Fix

The MCP server now provides 4 tools for Claude Desktop:

1. `query_database` - Execute custom SQL queries
2. `get_database_info` - Get database connection status and info
3. `list_tables` - List all tables and views in the database
4. `describe_table` - Get detailed table structure information

## Usage Examples

### List Tables
```
Tool: list_tables
Parameters: (none)
```

### Describe Table
```
Tool: describe_table
Parameters: {
  "table_name": "Users"
}
```

## Files Modified

- `main.go` - Added new tool definitions and handlers
- This documentation file created to track the issue and solution

## Compatibility

- ✅ Backward compatible - existing tools unchanged
- ✅ Maintains security standards
- ✅ Follows existing code patterns and conventions
- ✅ Works with both development and production modes

## Resolution Status

**RESOLVED** - The describe functionality is now available in the MCP server, providing feature parity with the CLI tool for database exploration tasks.