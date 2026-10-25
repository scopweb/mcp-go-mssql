---
title: Welcome
description: What MCP-Go-MSSQL is and how to use it
---

**MCP-Go-MSSQL** is an MCP server and a CLI in Go for Microsoft SQL Server. It serves Claude Desktop, Claude Code, OpenCode, and any stdio MCP client.

## What it solves

Connecting an agent to a production database is risky. This server adds a layer you can turn on quickly: absolute read-only mode, a whitelist only when writes are enabled, prepared statements, and TLS that fails closed in production.

That layer **does not replace** a least-privilege SQL login. You do not have to change that user to turn it on.

## Two ways to use it

1. **MCP server** (`cmd/mcp-go-mssql`) — tools inside the conversation.
2. **CLI** (`examples/claude-code/db-connector.go`) — terminal. Writes stay off unless an operator sets `MSSQL_CLI_ALLOW_WRITE=true`.

## What the agent can do

- Explore tables, columns, indexes, and foreign keys
- Run read queries
- Show an estimated plan (`explain_query`) and execution statistics
- Write only to whitelisted tables, and only when `MSSQL_READ_ONLY=false`
- Run procedures listed in `MSSQL_WHITELIST_PROCEDURES`

## Requirements

- Go 1.27.2+ ([download](https://go.dev/dl/))
- A reachable Microsoft SQL Server (port 1433 by default)
- Valid TLS in production. SQL Server 2008/2012 (TLS 1.0) needs an explicit opt-in

Continue to [Installation](/en/inicio/instalacion/).
