---
title: Production
description: Production deployment guide for MCP-Go-MSSQL
---

## Build the binary

```bash
go build -ldflags "-w -s" -o mcp-go-mssql
```

The `-w -s` flags strip debug information and reduce binary size.

## Environment variables

```bash
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=ProductionDB
MSSQL_USER=prod_user
MSSQL_PASSWORD=strong_password
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

## Production checklist

- [ ] `DEVELOPER_MODE=false`
- [ ] `MSSQL_READ_ONLY=true` (recommended for AI)
- [ ] Valid TLS certificates on SQL Server
- [ ] Database user with minimal permissions
- [ ] Restrictive permissions on `.env` files (600)
- [ ] `.env` excluded from version control
- [ ] Binary compiled with stripping flags
- [ ] Security log monitoring configured
- [ ] Firewall configured to restrict SQL port access

## Production security

- TLS encryption is mandatory and cannot be disabled
- Self-signed certificates are rejected (`trustservercertificate=false`)
- Errors show generic messages to the client
- Technical details only appear in internal logs
