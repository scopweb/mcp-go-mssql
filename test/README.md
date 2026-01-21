# Testing MCP-Go-MSSQL

Quick reference for running tests safely and securely.

## Quick Start

### 1. Create Test Environment File
```bash
cp ../.env.test.example ../.env.test
nano ../.env.test  # or edit with your editor
```

### 2. Configure Test Database Credentials
Edit `../.env.test` with your test database details:
```env
MSSQL_SERVER=your-test-server
MSSQL_DATABASE=test_db
MSSQL_USER=test_user
MSSQL_PASSWORD=test_password
```

### 3. Set Restrictive Permissions
```bash
chmod 600 ../.env.test  # Linux/Mac
```

### 4. Run Tests

**All tests:**
```bash
cd ..
go test -v ./...
```

**Unit tests only (no database required):**
```bash
go test -v -run "^TestSecurity|^TestInput|^TestExtract" ./...
```

**Specific test:**
```bash
go test -v -run TestSecurityLoggerSanitization ./...
```

**With coverage:**
```bash
go test -v -cover ./...
```

## Test Files

- `main_test.go` - Core functionality tests
- `main_permissions_test.go` - Permission and access control tests

## Security Rules

✅ **DO:**
- Use `.env.test` for credentials
- Use a separate test database
- Use test-only credentials
- Keep `.env.test` out of Git

❌ **DON'T:**
- Hardcode passwords
- Use production database
- Use production credentials
- Commit `.env.test` to Git

## Skipping Tests

**Skip if database unavailable:**
```bash
go test -v -short ./...
```

**Skip specific test:**
```bash
go test -v -skip TestDatabaseConnection ./...
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "undefined: NewSecurityLogger" | Run from project root: `cd .. && go test` |
| Database connection failed | Check `.env.test` credentials |
| Permission denied on `.env.test` | `chmod 644 ../.env.test` |
| Tests not found | Ensure in correct directory with `*_test.go` files |

## More Information

See [TESTING_SECURITY.md](../docs/TESTING_SECURITY.md) for detailed security guidelines.
