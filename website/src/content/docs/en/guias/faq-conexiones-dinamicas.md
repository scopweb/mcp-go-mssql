---
title: Dynamic connections FAQ
description: Short rules for one MCP server and several related databases
---

Use dynamic mode when one application needs several databases. The full guide is [Dynamic mode](/en/guias/modo-dinamico-una-aplicacion-varias-bases/).

## Rules

- Every alias starts as `READ_ONLY=true`.
- Do not set a global `MSSQL_READ_ONLY=false` in dynamic mode.
- An alias with `_READ_ONLY=false` and no `_WHITELIST_TABLES` cannot modify anything.
- The whitelist applies only when `_READ_ONLY=false`. Modifications require `confirm_operation` first.
- One stdio process per agent. The active alias does not isolate agents from each other.
- The control you can rely on is still the SQL login. This layer does not replace it.

## Where the configuration goes

| Client | Where | Variables |
|--------|--------|-----------|
| Claude Desktop | `mcpServers.*.env` | the same `MSSQL_DYNAMIC_*` variables |
| OpenCode | `mcp.*.environment` in `opencode.json` | the same ones. See [OpenCode](/en/configuracion/opencode/) |

The server also reads a `.env` next to the executable. Host environment variables win.

## Check

1. Start the server with `MSSQL_DYNAMIC_MODE=true`.
2. Call `dynamic_available`.
3. `dynamic_connect` to the read-only alias.
4. A write on a read-only alias must be rejected.
