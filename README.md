# MCP Go MSSQL

Secure MCP server and CLI in Go for Microsoft SQL Server. It gives Claude Desktop, Claude Code, and other MCP clients a hardened stdio bridge to the database.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Full documentation: [mcp-go-mssql.scopweb.com](https://mcp-go-mssql.scopweb.com)

## What it does

- Stdio MCP server with six tools: `query_database`, `get_database_info`, `explore`, `inspect`, `execute_procedure`, `explain_query`.
- Optional dynamic mode for several related databases in one process (`dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list`, `confirm_operation`).
- Standalone CLI for terminal use (`examples/claude-code/db-connector.go`).

Built for environments where the security boundary matters: absolute read-only mode, a table whitelist only when writes are explicitly enabled, prepared statements, TLS fail-closed in production, and an explicit opt-in for SQL Server 2008/2012 (TLS 1.0).

Agent and harness contract: [docs/AGENT_CONTRACT.md](docs/AGENT_CONTRACT.md).

## Install

Download the [latest release](https://github.com/scopweb/mcp-go-mssql/releases/latest) or build:

```bash
go build -ldflags "-w -s" -o mcp-go-mssql ./cmd/mcp-go-mssql
```

Requires Go 1.26+.

## Claude Desktop

Point `command` at the binary. Do not commit real credentials.

```json
{
  "mcpServers": {
    "mssql": {
      "command": "/absolute/path/to/mcp-go-mssql",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "your_password",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true"
      }
    }
  }
}
```

With `MSSQL_READ_ONLY=true` the agent can only read and run explicitly allowed administrative introspection. No whitelist can enable writes in that mode. For controlled writes, set `MSSQL_READ_ONLY=false` and an explicit `MSSQL_WHITELIST_TABLES` list.

| Variable | Description |
|----------|-------------|
| `MSSQL_SERVER` / `MSSQL_PORT` | Host and port (default 1433) |
| `MSSQL_DATABASE` | Database (optional with integrated auth) |
| `MSSQL_USER` / `MSSQL_PASSWORD` | SQL authentication credentials |
| `MSSQL_AUTH` | `sql` (default), `integrated` / `windows` (SSPI, no password) |
| `MSSQL_CONNECTION_STRING` | Full DSN; overrides the other `MSSQL_*` fields |
| `DEVELOPER_MODE` | `false` = production (strict TLS, generic errors) |
| `MSSQL_READ_ONLY` / `MSSQL_WHITELIST_TABLES` | Absolute read-only / write whitelist (whitelist applies only when `READ_ONLY=false`) |
| `MSSQL_WHITELIST_PROCEDURES` | Procedures `execute_procedure` may run |
| `MSSQL_ALLOW_INSECURE_TLS` | Allow insecure TLS in production (legacy servers only) |

The full list is in [.env.example](.env.example) and in the [environment variable docs](https://mcp-go-mssql.scopweb.com/configuracion/variables-entorno).

## OpenCode

Add this to `opencode.json` in the project, or to `~/.config/opencode/opencode.json`. OpenCode uses `mcp`, not `mcpServers`. `command` is an array and variables go in `environment`.

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "mssql": {
      "type": "local",
      "command": ["/absolute/path/to/mcp-go-mssql"],
      "enabled": true,
      "timeout": 30000,
      "environment": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "{env:MSSQL_PASSWORD}",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true"
      }
    }
  }
}
```

On Windows, `command` is `["C:\\path\\to\\mcp-go-mssql.exe"]`. `{env:MSSQL_PASSWORD}` reads the password from the environment so it is not stored in the config file. `timeout` is milliseconds for the first tool listing; the default is 5000, which is often too short for a cold start. Check the server with `opencode mcp list`.

## Legacy SQL Server (2008/2012, TLS 1.0)

Current Go runtimes do not negotiate TLS 1.0. Use a URL-form connection string:

```ini
MSSQL_CONNECTION_STRING=sqlserver://user:password@legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

In production (`DEVELOPER_MODE=false`) this refuses to start by design. Confirm it with `MSSQL_ALLOW_INSECURE_TLS=true`. Every affected connection then logs an auditable `WARN`.

## Dynamic mode

One process can reach several related databases. Each alias is read-only unless you say otherwise.

```ini
MSSQL_DYNAMIC_MODE=true
MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.local
MSSQL_DYNAMIC_APP_MAIN_DATABASE=AppDB
MSSQL_DYNAMIC_APP_MAIN_USER=ai_reader
MSSQL_DYNAMIC_APP_MAIN_PASSWORD=...
```

- Every alias defaults to `READ_ONLY=true`.
- A writable alias (`_READ_ONLY=false`) with no `_WHITELIST_TABLES` cannot modify anything.
- The whitelist applies only when `_READ_ONLY=false`. Modifications require `confirm_operation` first.
- Do not set a global `MSSQL_READ_ONLY=false` in dynamic mode.
- Run one stdio process per agent or session. The active alias belongs to the process and is not isolation between agents.

Guide: [dynamic mode](https://mcp-go-mssql.scopweb.com/guias/modo-dinamico-una-aplicacion-varias-bases).

## CLI

```bash
go run examples/claude-code/db-connector.go test
go run examples/claude-code/db-connector.go tables
go run examples/claude-code/db-connector.go describe MyTable
go run examples/claude-code/db-connector.go query "SELECT TOP 10 * FROM MyTable"
```

The CLI rejects writes unless an operator sets `MSSQL_CLI_ALLOW_WRITE=true`. Agents should use the MCP server and its whitelist policy.

## Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| `certificate signed by unknown authority` | Self-signed certificate | `DEVELOPER_MODE=true` (development only) |
| `protocol version 301` / TLS handshake | TLS 1.0 server (2008/2012) | URL connection string with `encrypt=disable` (see above) |
| `Refusing to start... MSSQL_ALLOW_INSECURE_TLS` | Insecure TLS in production | Fix encryption, or opt in with `MSSQL_ALLOW_INSECURE_TLS=true` |
| `Database not connected` | Missing variables | Check `.env` against `.env.example` |
| `read-only mode: only SELECT...` | Read-only is on | Run the SQL yourself. A whitelist does not enable writes while `READ_ONLY=true` |

Connection check without starting the server: `go run test/test-connection.go`.

## Layout

```
cmd/mcp-go-mssql/       MCP server
docs/                   agent contract
examples/claude-code/   CLI
test/                   connection check and security tests
scripts/                build and diagnostic scripts
website/                documentation site (Astro Starlight)
config/                 MCP config templates
.env.example            environment template
```

## Security

The whitelist and read-only guard are defense in depth. The control you can rely on is a least-privilege SQL login. Do not commit `.env` or real credentials. Report vulnerabilities via [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).

## Español

Documentación en español: [mcp-go-mssql.scopweb.com](https://mcp-go-mssql.scopweb.com).

Servidor MCP y CLI en Go para Microsoft SQL Server. El modo por defecto es solo lectura absoluto: ninguna whitelist habilita escrituras mientras `MSSQL_READ_ONLY=true`. La instalación, las variables y el modo dinámico están en el sitio de documentación.
