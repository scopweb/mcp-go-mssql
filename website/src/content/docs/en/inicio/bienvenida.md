---
title: Welcome
description: What is MCP-Go-MSSQL and why you need it
---

**MCP-Go-MSSQL** is a secure bridge between Claude and your Microsoft SQL Server database. Written in Go, it lets you query, analyze, and operate on data directly from the conversation, without leaving Claude Desktop or Claude Code.

## Why this project exists

AI assistants are incredibly useful for working with data, but connecting them to a production database is scary. One mistake and you could lose critical data. MCP-Go-MSSQL solves that problem: it gives Claude full read access and a controlled write space, so you can leverage AI without risking anything.

## Two ways to use it

1. **MCP Server** (`main.go`) — Integrates with Claude Desktop through the MCP protocol. Configure it once and Claude will have database tools available in every conversation.

2. **Claude Code CLI** (`claude-code/db-connector.go`) — Direct command-line access. Ideal for development, scripts, and automation.

Both share the same security layers: TLS, prepared statements, read-only mode, and whitelist.

## What you can do

- **Explore** your database structure: tables, columns, indexes, foreign keys
- **Query** data with full SQL: JOINs, CTEs, window functions
- **Analyze** production information with no risk of accidental modification
- **Operate** on authorized temporary tables so the AI can process and transform data
- **Execute** stored procedures in a controlled manner

## Security built in

It's not an add-on module you enable separately. Security is embedded in every layer:

- **TLS encryption** mandatory in production
- **Exclusive prepared statements** — SQL injection is impossible
- **Read-only mode** that blocks any unauthorized writes
- **Table whitelist** for granular write permissions where you decide
- **Multi-table validation** that detects unauthorized access via JOINs and subqueries
- **Secure logging** that never records credentials or sensitive data

## Requirements

- **Go 1.24+** ([download](https://go.dev/dl/))
- **Microsoft SQL Server** with TLS support (2012 or later recommended)
- Network access to the SQL Server port (1433 by default)

## Project structure

```
mcp-go-mssql/
├── main.go                    # MCP server for Claude Desktop
├── claude-code/
│   └── db-connector.go        # CLI for Claude Code
├── test/
│   └── security/              # Security test suite
├── scripts/                   # Build and utility scripts
└── website/                   # This documentation
```

## Next step

Continue to [Installation](/en/inicio/instalacion/) to get the server running in minutes.
