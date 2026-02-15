# Issue #1: Add SQL Server 2008/Legacy Version Support

## Problem Description
The MCP server was unable to connect to legacy SQL Server versions (2008, 2012) due to TLS handshake failures. The standard connection string format used for modern SQL Server versions was incompatible with older server instances.

## Error Symptoms
- **Error Message**: `TLS Handshake failed: tls: server selected unsupported protocol version 301`
- **Affected Versions**: SQL Server 2008, SQL Server 2012, and other legacy instances
- **Connection Status**: Failed with standard format, successful with SQL Server Management Studio

## Root Cause
Legacy SQL Server versions require a different connection string format than modern versions. The Go `mssqldb` driver supports multiple formats:

1. **Standard Format** (Modern SQL Server 2014+):
   ```
   server=HOST;database=DB;user id=USER;password=PASS;encrypt=false;trustservercertificate=true
   ```

2. **URL Format** (Legacy SQL Server 2008/2012):
   ```
   sqlserver://USER:PASS@HOST:PORT?database=DB&encrypt=disable&trustservercertificate=true
   ```

## Solution Implemented

### 1. Custom Connection String Support
Added `MSSQL_CONNECTION_STRING` environment variable that allows users to specify any connection string format:

```go
func buildSecureConnectionString() (string, error) {
    // Check for custom connection string first
    if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
        return customConnStr, nil
    }
    // Fall back to standard format building...
}
```

### 2. Configuration Examples
**Legacy SQL Server (URL Format):**
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://sa:YourPassword@legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true",
    "DEVELOPER_MODE": "true"
  }
}
```

**Modern SQL Server (Standard Format):**
```json
{
  "env": {
    "MSSQL_SERVER": "modern-server.database.windows.net",
    "MSSQL_DATABASE": "MyDatabase",
    "MSSQL_USER": "user",
    "MSSQL_PASSWORD": "password",
    "DEVELOPER_MODE": "false"
  }
}
```

### 3. Enhanced Logging
Added detailed logging to help diagnose connection issues:
- Custom connection string detection
- Environment variable validation
- Connection format identification

## Testing
- ✅ **SQL Server 2008**: Successfully connects using URL format
- ✅ **Modern SQL Server**: Continues to work with standard format
- ✅ **Debugging**: Added comprehensive connection testing tool in `debug/debug-connection.go`

## Documentation Updates
- Updated README.md with legacy server examples
- Added troubleshooting section for TLS handshake errors
- Documented both connection string formats

## Files Modified
- `main.go`: Added custom connection string support
- `README.md`: Updated configuration examples and troubleshooting
- `debug/debug-connection.go`: Enhanced debugging tool
- `CLAUDE.md`: Updated project documentation

## Benefits
1. **Backward Compatibility**: Supports SQL Server versions back to 2008
2. **Flexibility**: Users can specify any connection string format
3. **Future-Proof**: Works with both legacy and modern SQL Server versions
4. **Better Diagnostics**: Enhanced logging for connection troubleshooting

## Issue Status: ✅ RESOLVED
**Fixed in commit**: Add comprehensive SQL Server 2008 legacy support with custom connection strings