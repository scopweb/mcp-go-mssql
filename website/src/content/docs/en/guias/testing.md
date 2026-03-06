---
title: Testing
description: Testing guide for MCP-Go-MSSQL
---

## Connection test

```bash
cd test
go run test-connection.go
```

This test verifies connectivity, authentication, and TLS encryption.

## Security tests

```bash
go test -v -run TestSQLInjectionVulnerability ./test/security/...
```

The security suite covers 6 SQL injection attack vectors.

## Claude Code CLI

Use the CLI tool to test operations:

```bash
# Connection test
go run claude-code/db-connector.go test

# Database information
go run claude-code/db-connector.go info

# List tables
go run claude-code/db-connector.go tables

# Describe a table
go run claude-code/db-connector.go describe users

# Execute a query
go run claude-code/db-connector.go query "SELECT @@VERSION"
```

## Manual tests

### Verify read-only mode

With `MSSQL_READ_ONLY=true`, confirm that write queries are blocked:

```bash
go run claude-code/db-connector.go query "INSERT INTO some_table VALUES (1)"
# Should return: Query blocked: read-only mode
```

### Verify whitelist

With `MSSQL_WHITELIST_TABLES=temp_ai`, confirm that only that table accepts writes:

```bash
go run claude-code/db-connector.go query "INSERT INTO temp_ai (data) VALUES ('test')"
# Should execute successfully
```

## Test environment

Always use a separate database for testing. Never run tests against production.
