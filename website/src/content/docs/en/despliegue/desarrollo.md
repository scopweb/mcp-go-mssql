---
title: Development
description: Development environment setup for MCP-Go-MSSQL
---

## Requirements

- Go 1.25.0 or later
- Microsoft SQL Server (local or remote)
- Git

## Initial setup

```bash
git clone https://github.com/DavidSerrano-Rodriguez/mcp-go-mssql.git
cd mcp-go-mssql
go mod tidy
```

## Environment variables

```bash
cp .env.example .env
# Edit .env with development credentials
source .env
```

Example for local development:

```bash
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=sa
MSSQL_PASSWORD=DevPassword123
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false
```

## Run in development mode

```bash
go run main.go
```

In development mode (`DEVELOPER_MODE=true`):
- Self-signed TLS certificates are allowed
- Errors show full technical details
- Encryption is still mandatory

## Test the connection

```bash
cd test
go run test-connection.go
```

## Claude Code CLI

```bash
go run claude-code/db-connector.go test
go run claude-code/db-connector.go tables
go run claude-code/db-connector.go query "SELECT @@VERSION"
```

## Build

```bash
go build
```
