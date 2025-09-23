# Issues Documentation

This directory contains detailed documentation of issues that have been identified and resolved in the MCP-Go-MSSQL project.

## Resolved Issues

### #1 - SQL Server 2008/Legacy Version Support
**File**: [01-sql-server-2008-support.md](01-sql-server-2008-support.md)
**Status**: ✅ Resolved
**Priority**: High
**Summary**: Added support for legacy SQL Server versions (2008, 2012) through custom connection string configuration. Resolves TLS handshake failures with older database versions.

**Key Features**:
- Custom connection string support via `MSSQL_CONNECTION_STRING`
- URL format compatibility for legacy SQL Server
- Enhanced debugging tools
- Backward compatibility maintained

---

### #2 - Increase Query Size Limit
**File**: [02-increase-query-size-limit.md](02-increase-query-size-limit.md)
**Status**: ✅ Resolved
**Priority**: Medium
**Summary**: Increased default query size limit from 4KB to 1MB to support complex SQL operations. Added configurable limit option.

**Key Features**:
- Default limit increased to 1,048,576 characters (1MB)
- Configurable via `MSSQL_MAX_QUERY_SIZE` environment variable
- Better error messages showing current limits
- Supports complex queries, CTEs, and bulk operations

---

### #3 - Read-Only Security Mode
**File**: [03-read-only-security-mode.md](03-read-only-security-mode.md)
**Status**: ✅ Resolved
**Priority**: High
**Summary**: Implemented comprehensive read-only mode for enhanced security. Restricts database access to SELECT and read operations only.

**Key Features**:
- Read-only mode via `MSSQL_READ_ONLY=true`
- Comprehensive query validation
- Multi-layer security protection
- Security audit logging
- Clear error messages and status reporting

---

## Issue Statistics

- **Total Issues**: 3
- **Resolved**: 3 ✅
- **In Progress**: 0
- **Open**: 0

## Security Enhancements

The following security improvements have been implemented:

1. **Legacy Database Support**: Secure connection to older SQL Server versions
2. **Query Size Protection**: Prevents memory exhaustion while allowing legitimate operations
3. **Access Control**: Read-only mode for restricted environments
4. **Audit Logging**: Security event tracking and violation logging

## Configuration Examples

### Production Read-Only Setup
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://readonly_user:pass@prod-server:1433?database=analytics&encrypt=enable",
    "MSSQL_READ_ONLY": "true",
    "MSSQL_MAX_QUERY_SIZE": "2097152",
    "DEVELOPER_MODE": "false"
  }
}
```

### Legacy SQL Server Development Setup
```json
{
  "env": {
    "MSSQL_CONNECTION_STRING": "sqlserver://sa:password@legacy-server:1433?database=devdb&encrypt=disable&trustservercertificate=true",
    "MSSQL_READ_ONLY": "false",
    "MSSQL_MAX_QUERY_SIZE": "1048576",
    "DEVELOPER_MODE": "true"
  }
}
```

## Contributing

When documenting new issues:

1. Create a new file following the naming convention: `##-descriptive-name.md`
2. Use the established template structure
3. Include problem description, solution, testing, and benefits
4. Update this README.md with the new issue entry
5. Add configuration examples when applicable

## Testing

All resolved issues have been tested with:
- ✅ SQL Server 2008 (Legacy)
- ✅ SQL Server 2019 (Modern)
- ✅ Azure SQL Database
- ✅ Various query sizes and complexities
- ✅ Read-only and full access modes

## Documentation

For implementation details, see:
- [Main README](../../README.md)
- [Claude Code Documentation](../../CLAUDE.md)
- [Project Structure](../../README.md#project-structure)