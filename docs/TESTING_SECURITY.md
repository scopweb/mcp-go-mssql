# Testing Security Guide

## Overview

This document provides security guidelines for testing the MCP-Go-MSSQL project safely and securely.

## ⚠️ Critical Security Rules

### 1. **NEVER Hardcode Credentials**
- ❌ **Bad**: `os.Setenv("MSSQL_PASSWORD", "my_password")`
- ✅ **Good**: Load from `.env.test` file

### 2. **Use Dedicated Test Database**
- Use a **separate test database**, NOT production
- Use **test-only credentials** with limited permissions
- Consider deleting the test data after each run

### 3. **Protect `.env.test` File**
- **Already in `.gitignore`** - will not be committed
- Set restrictive file permissions:
  ```bash
  # Linux/Mac
  chmod 600 .env.test

  # Windows PowerShell
  icacls ".env.test" /inheritance:r /grant:r "%USERNAME%:RX"
  ```

## Setup Instructions

### Step 1: Create `.env.test` from Template
```bash
cp .env.test.example .env.test
```

### Step 2: Edit `.env.test` with Test Credentials
```bash
# Linux/Mac
nano .env.test

# Windows
notepad .env.test
```

**Important**: Only use test database credentials, never production!

### Step 3: Secure the File
```bash
# Set restrictive permissions (read/write for owner only)
chmod 600 .env.test
```

### Step 4: Run Tests
```bash
# From project root
cd /path/to/mcp-go-mssql

# Run all tests
go test -v ./...

# Run specific test
go test -v -run TestSecurityLoggerSanitization ./...

# Run only non-database tests
go test -v -short ./...

# From test directory
cd test
go test -v
```

## Environment Variable Precedence

The test setup loads variables in this order (first match wins):

1. **System environment variables** (highest priority)
2. **`.env.test` file** in test directory
3. **`.env.test` file** in parent directory
4. **Built-in defaults** (non-sensitive values only)

This means:
- You can override `.env.test` by setting environment variables: `MSSQL_SERVER=myhost go test`
- System variables take precedence over `.env.test`

## Test Categories

### Unit Tests (No Database Required)
These tests don't need database credentials and run fast:
- `TestSecurityLoggerSanitization`
- `TestInputValidation`
- `TestExtractAllTablesFromQuery`
- `TestGetWhitelistedTables`
- `TestExtractOperation`

**Run without `.env.test`:**
```bash
go test -v -run "^TestSecurity|^TestInput|^TestExtract" ./...
```

### Integration Tests (Requires Database)
These tests need valid database credentials:
- `TestBuildSecureConnectionString`
- `TestDatabaseConnection`
- `TestMCPServerInitialization`
- `TestMCPToolsList`

**Skip if `.env.test` not configured:**
```bash
# Mark tests to skip if DB not available
go test -v -short ./...
```

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      mssql:
        image: mcr.microsoft.com/mssql/server:latest
        env:
          SA_PASSWORD: TestPassword123!
          ACCEPT_EULA: "Y"
        options: --health-cmd "/opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -Q \"SELECT 1\"" --health-interval 10s --health-timeout 5s --health-retries 3

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Set test environment
        run: |
          echo "MSSQL_SERVER=localhost" >> $GITHUB_ENV
          echo "MSSQL_DATABASE=TestDB" >> $GITHUB_ENV
          echo "MSSQL_USER=sa" >> $GITHUB_ENV
          echo "MSSQL_PASSWORD=TestPassword123!" >> $GITHUB_ENV
          echo "DEVELOPER_MODE=true" >> $GITHUB_ENV

      - name: Run tests
        run: go test -v ./...
```

## Common Issues

### Issue: "database connection failed"
**Cause**: `.env.test` not configured or database not running
**Solution**:
```bash
cp .env.test.example .env.test
# Edit .env.test with correct database credentials
```

### Issue: "undefined: NewSecurityLogger"
**Cause**: Tests need to be in same package as main.go
**Solution**: Ensure you're running tests from correct directory:
```bash
cd /path/to/mcp-go-mssql
go test ./...  # This will find all *_test.go files
```

### Issue: "permission denied .env.test"
**Cause**: File permissions are too restrictive
**Solution**: Fix permissions:
```bash
chmod 644 .env.test  # readable by all, writable by owner
```

## Best Practices

### 1. Use Meaningful Test Database Name
```
MSSQL_DATABASE=mcp_test_db  # Good - clearly identifies it as test
MSSQL_DATABASE=TestDB       # OK
MSSQL_DATABASE=Production   # BAD - never use production DB
```

### 2. Use Limited Test User
```bash
# Good - test user with minimal permissions
MSSQL_USER=test_user
MSSQL_PASSWORD=test_password_12345

# Bad - using production service account
MSSQL_USER=prod_service
MSSQL_PASSWORD=prod_password
```

### 3. Clean Up After Tests
If tests create temporary data:
```go
defer func() {
    // Clean up test data
    db.Exec("DROP TABLE IF EXISTS test_table")
}()
```

### 4. Use `testing.T.Cleanup()` for Resource Management
```go
func TestWithCleanup(t *testing.T) {
    db := setupTestDB()
    t.Cleanup(func() {
        db.Close()
    })
    // Test code here
}
```

### 5. Validate Tests Don't Leak Sensitive Data
```bash
# Check test output for passwords
go test -v 2>&1 | grep -i password
# Should output: nothing (silent is good)
```

## Security Checklist

- [ ] `.env.test` file created from `.env.test.example`
- [ ] `.env.test` has restrictive permissions (600)
- [ ] `.env.test` is in `.gitignore`
- [ ] Using test database, not production
- [ ] Test credentials are different from production
- [ ] No hardcoded credentials in test files
- [ ] No sensitive data in test output
- [ ] Database tests can be skipped if credentials unavailable
- [ ] `.env.test` cleaned up before committing changes

## References

- [CLAUDE.md](../CLAUDE.md) - Project guidelines
- [README.md](../README.md) - General setup
- [Go testing documentation](https://golang.org/pkg/testing/)

## Questions?

If you have security concerns about testing setup, please:
1. Open an issue on GitHub
2. Do NOT include any credentials in the issue
3. Describe the issue and your environment
