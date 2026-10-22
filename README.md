# MCP Go MSSQL

Secure MCP server and CLI in Go for Microsoft SQL Server. It gives Claude Desktop, Claude Code, and other MCP clients a hardened stdio bridge to the database.

[![Go Version](https://img.shields.io/badge/Go-1.27.2+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Full documentation: [mcp-go-mssql.scopweb.com](https://mcp-go-mssql.scopweb.com)

## What it does

- Stdio MCP server with six tools in classic mode (`query_database`, `get_database_info`, `explore`, `inspect`, `execute_procedure`, `explain_query`).
- Optional dynamic mode for several related databases in one process, which adds five more (`dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list`, `confirm_operation`).
- Standalone CLI for terminal use (`examples/claude-code/db-connector.go`).

Built for environments where the security boundary matters: absolute read-only mode, a table whitelist only when writes are explicitly enabled, prepared statements, TLS fail-closed in production, and an explicit opt-in for SQL Server 2008/2012 (TLS 1.0).

These controls do not replace a least-privilege SQL login. They are an extra layer you can turn on quickly, without changing that database user.

Agent and harness contract: [docs/AGENT_CONTRACT.md](docs/AGENT_CONTRACT.md).

<!-- BEGIN GENERATED TOOL CATALOG -->
<!-- Generated from tools/list by cmd/mcp-go-mssql/tools_docs_test.go. Do not edit by hand. -->
<!-- Regenerate: go test ./cmd/mcp-go-mssql -run TestToolCatalogDocumentation -update -->

| Tool | Parameters | Required | Mode | readOnly | destructive | idempotent |
| --- | --- | --- | --- | --- | --- | --- |
| `query_database` | `format:string`, `max_cell_bytes:integer`, `max_response_bytes:integer`, `query:string` | `query` | classic | no | yes | no |
| `get_database_info` | — | — | classic | yes | no | yes |
| `explore` | `filter:string`, `limit:integer`, `max_cell_bytes:integer`, `max_response_bytes:integer`, `pattern:string`, `schema:string`, `search_in:string`, `table:string`, `type:string` | — | classic | yes | no | yes |
| `inspect` | `detail:string`, `max_cell_bytes:integer`, `max_response_bytes:integer`, `schema:string`, `table_name:string` | `table_name` | classic | yes | no | yes |
| `execute_procedure` | `parameters:string`, `procedure_name:string` | `procedure_name` | classic | no | yes | no |
| `explain_query` | `query:string` | `query` | classic | yes | no | yes |
| `dynamic_available` | — | — | dynamic-only | yes | no | yes |
| `dynamic_connect` | `alias:string` | `alias` | dynamic-only | no | no | no |
| `dynamic_disconnect` | — | — | dynamic-only | yes | no | yes |
| `dynamic_list` | — | — | dynamic-only | yes | no | yes |
| `confirm_operation` | `description:string` | `description` | dynamic-only | no | yes | no |

<!-- END GENERATED TOOL CATALOG -->

## Install

Download the [latest release](https://github.com/scopweb/mcp-go-mssql/releases/latest) or build:

```bash
go build -ldflags "-w -s" -o mcp-go-mssql ./cmd/mcp-go-mssql
```

Requires Go 1.27.2+ (also used by CI via `go.mod`).

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
| `MSSQL_USAGE_LOG` | `false` disables the per-call usage records on stderr (default: enabled) |

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

## Formats

`query_database` can render the rows in two formats:

- `format="map"` (default): the shape of contract v1, one object per row, with the column name on every value.
- `format="columnar"`: the column names are paid once and every row is an array of values in that order. The truncation flags are the same.

```json
{"format": "columnar", "columns": ["id", "name"], "rows": [[1, "Acme"], [2, "Beta"]],
 "row_count": 2, "truncated": false, "rows_truncated": false,
 "cells_truncated": false, "response_truncated": false,
 "limits": {"max_rows": 500, "max_cell_bytes": null, "max_response_bytes": null},
 "stats": {"enabled": false, "messages": []}}
```

The columnar format needs unique column names: a result that repeats a column name is refused, because a row cannot carry it twice. With ten columns and two hundred rows the columnar form is around a third of the size of the map form, and the difference grows with the width of the table. Measured on a table of 38 columns and 100 rows, the whole answer went from 182 KB to 91 KB.

## Response budgets

`query_database`, `explore` and `inspect` accept two optional budgets:

- `max_cell_bytes` (minimum 16): the cap of a single cell. A longer value is shortened, keeps its beginning and is marked. Without it a `nvarchar(max)`, `xml` or `varbinary` cell is returned whole — a single 1 MiB cell can reach 6 MB of JSON.
- `max_response_bytes` (minimum 512): an approximate cap for the whole result. The result stops there.

They are opt-in and the server never rewrites your SQL to fit a budget. Every result reports `rows_truncated`, `cells_truncated` and `response_truncated` separately, plus a `limits` object with the budgets that were applied.

```json
{"rows":[{"id":1,"notes":"<prefix> [truncated: 1048576 bytes total]"}],
 "row_count":1,"truncated":false,"rows_truncated":false,
 "cells_truncated":true,"response_truncated":false,
 "truncated_cells":[{"row":0,"column":"notes","original_bytes":1048576}],
 "limits":{"max_rows":500,"max_cell_bytes":4096,"max_response_bytes":null},
 "stats":{"enabled":false,"messages":[]}}
```

## Usage accounting

Every tool call is counted once and written to **stderr** as JSON with `component="usage"`: the tool, the elapsed time, the bytes on the wire, the bytes of text and of structured content, the rows delivered and the truncation flag when the tool exposes them, and the outcome (`success`, `tool_error`, `rpc_error`, `timeout`).

The records carry no SQL, no results and no credentials. They are not affected by the MCP `logging/setLevel` request, so a harness that lowers the security log level can still count calls. Set `MSSQL_USAGE_LOG=false` to disable the stream.

```json
{"level":"INFO","msg":"tool call","component":"usage","session_id":"573403616c10c8bd","call_id":1,"tool":"query_database","method":"tools/call","status":"success","elapsed_ms":42,"response_bytes":1234,"text_bytes":1000,"structured_bytes":234,"rows":500,"row_truncated":true,"read_only":true}
```

This is what makes "bytes per task" and "calls per task" measurable instead of estimated. No tool result changes.

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
