# Contributing

Thanks for your interest in contributing to `mcp-go-mssql`. This file
covers the day-to-day rules that keep the project consistent and safe.
For background, architecture, and security posture see
[`README.md`](README.md) and [`CLAUDE.md`](CLAUDE.md).

## Working with the database

This server is a **bridge** to Microsoft SQL Server. Everything that
touches the database should assume the live schema is unknown and
possibly large. Two rules follow from that.

### 1. Never guess table or column names

Field and table names are easy to invent and hard to verify by reading
code, so don't try. Before writing or accepting any SQL:

- **Discover** tables and views with the `explore` MCP tool
  (`type=tables` or `type=search`).
- **Confirm** columns with the `inspect` MCP tool
  (`detail=columns`).
- Only then run the actual query with `query_database`.

The same rule applies inside this repository: when adding SQL examples
to docs, examples, or tests, copy the names from
`INFORMATION_SCHEMA`, never from memory or from analogous projects.
The full workflow and tool list live in
[`.claude/skills/mcp-mssql-tools/SKILL.md`](.claude/skills/mcp-mssql-tools/SKILL.md).

### 2. Never commit database credentials

The server reads everything sensitive from environment variables.
Before opening a pull request, check:

- `.env` is **not** staged (it is git-ignored; keep it that way).
- Only `.env.example` and `config.example.json` are tracked.
- No real server, database, user, or password is present in new files.

If you need to add a new connection setting, document the env var name
and an obviously-fake value, never a real one.

## Legacy SQL Server (2000 / 2008 / 2012)

These versions only negotiate TLS 1.0, which the modern Go runtime
rejects. The supported workaround is a **per-alias connection string
override** in URL form with `encrypt=disable`:

```bash
# Direct mode
MSSQL_CONNECTION_STRING='sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true'

# Dynamic mode (one alias per legacy server)
MSSQL_DYNAMIC_<ALIAS>_CONNECTION_STRING='sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true'
```

The global `MSSQL_SERVER` / `MSSQL_USER` / `MSSQL_PASSWORD` flow **does
not** work for these versions; only the `_CONNECTION_STRING` override
does. If a user reports `TLS Handshake failed: tls: server selected
unsupported protocol version 301`, point them here.

For the full multi-database / dynamic-mode pattern, see the guide
[`website/src/content/docs/guias/modo-dinamico-una-aplicacion-varias-bases.md`](website/src/content/docs/guias/modo-dinamico-una-aplicacion-varias-bases.md).

## Pull requests

- Run `go build ./...` and `go test ./...` before pushing.
- Add or update tests for any new MCP tool, security check, or query
  validation rule.
- Keep changes focused. If a fix is large, split it into a series of
  small, self-contained PRs.
- Use the [pull request template](.github/PULL_REQUEST_TEMPLATE.md).

## Reporting security issues

Please **do not** open a public issue for suspected SQL-injection
bypasses, credential exposure, or other security defects. Follow the
process described in
[`.github/ISSUES/README.md`](.github/ISSUES/README.md) instead.
