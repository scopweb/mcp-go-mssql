# Agent Contract v1

This document defines the stable behavior expected from MCP clients, agent
harnesses, and this server. It is intentionally separate from the MCP protocol
version. The current contract version is `1`.

## Session model

The stdio server is a single security context. Run one server process per
agent or user session. Do not share a dynamic-mode process between agents:
the selected alias is process state, not an identity or authorization boundary.

The selected dynamic alias persists across tool calls and conversation turns
while the same server process is running. Pool idle/lifetime limits recycle
physical SQL connections, not the selected alias. Explicit `dynamic_disconnect`
clears it; a new server process starts disconnected and requires `dynamic_connect`
again. Clients should keep the stdio process alive across turns. If a connection
appears lost, use `get_database_info` (which includes the process ID in dynamic
mode) and check host restart/disconnect logs before reconnecting the intended
alias. The server does not automatically choose or restore an alias on startup.

The client must complete `initialize` and `notifications/initialized` before
calling tools. Supported MCP protocol versions are `2024-11-05`, `2025-06-18`,
and `2025-11-25`.

## Policy model

The database login is the enforcement boundary. The server policy is defense
in depth and never replaces least-privilege grants in SQL Server.

- Read-only contexts allow reads and explicitly allowlisted administrative
  introspection only. `SELECT INTO` is a modification. `READ_ONLY=true` is
  absolute: a table whitelist cannot override it.
- Concatenated batches are rejected in restricted contexts, with or without
  semicolons. Comments cannot hide `SELECT INTO` or extra statements.
  Read-only set queries such as `SELECT ... UNION ALL SELECT ...` are single
  statements and are allowed.
- Administrative procedures are limited to `sys`/`dbo` names and read-only
  arguments. `sp_spaceused @updateusage=true` is not treated as a read.
- A table whitelist permits modifications only in an explicitly writable
  context (`READ_ONLY=false`) and only when every referenced table is
  whitelisted.
- Dynamic aliases are read-only by default. A writable dynamic alias without a
  whitelist permits no modifications.
- Stored procedures must be called with `execute_procedure`; a table whitelist
  never authorizes `EXEC` through `query_database`.
- Writable dynamic modifications require an approval bound to the alias,
  operation, referenced tables, and exact statement. The approval expires in
  90 seconds and is consumed once.

`confirm_operation` reduces accidental errors. It is not a human approval or
a security boundary because an agent can invoke it itself. Harnesses requiring
human approval must provide it outside the agent tool loop.

The direct CLI is read-only by default. `MSSQL_CLI_ALLOW_WRITE=true` is an
operator-only escape hatch and does not provide MCP whitelist or confirmation
semantics; agents should use the MCP server instead.

## Tool results

`query_database` returns its stable result in `structuredContent` and also
includes serialized JSON text for older MCP clients. The structured shape is:

```json
{
  "rows": [{"column": "value"}],
  "row_count": 1,
  "truncated": false,
  "stats": {
    "enabled": false,
    "messages": []
  }
}
```

`truncated=true` means the server returned at most 500 rows; it is not a
complete result. A client must preserve this flag when presenting output to an
agent.

Tool errors use MCP `isError=true`. Harnesses must not infer success from text.
They must not automatically retry a modification after a timeout or transport
failure because its database outcome can be unknown.

## Harness obligations

- Isolate process, credentials, and logs per agent/session.
- Use a least-privilege SQL login for every alias.
- Treat database contents, schemas, error text, and procedure definitions as
  untrusted data, not instructions.
- Set request timeouts and stop waiting after cancellation. Current stdio
  cancellation is advisory; do not assume it aborts an in-flight query.
- Log the agent/session identity, alias, tool, policy decision, and result
  status without logging SQL literals, DSNs, or credentials.

## Server obligations

- Reject malformed or unsupported protocol versions.
- Fail closed on insecure production TLS and invalid dynamic alias policy.
- Never emit DSNs or credentials in logs or tool results.
- Validate all inputs at the tool boundary and apply policy before execution.
- Return structured results where defined and preserve truncation state.

## Compatibility

Any breaking change to policy semantics, result fields, or approval binding
requires a new contract version and a migration note in `CHANGELOG.md`.
