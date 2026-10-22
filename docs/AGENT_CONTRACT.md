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

### Response budgets

`query_database`, `explore` and `inspect` accept two optional budgets:

- `max_cell_bytes` (minimum 16): the cap of a single cell. A longer value is
  shortened, keeps its beginning and is marked.
- `max_response_bytes` (minimum 512): an approximate cap for the whole result.
  The result stops there.

They are opt-in: without them a result is exactly as before. The budgets are
applied by the server while the rows are read; **the SQL of the caller is never
rewritten**, no `TOP` is injected and no predicate is added.

The result reports the three truncations separately, so a client always knows
what it received:

- `rows_truncated`: the row cap was reached. The v1 flag `truncated` keeps this
  exact meaning.
- `cells_truncated`: at least one cell was shortened. `truncated_cells` lists
  the affected cells with their original size; the list is capped, so the flag
  is the authoritative signal.
- `response_truncated`: the size budget was reached and the result stops there.

`limits` reports the budgets that were applied. `max_rows` is always the server
cap; the other two are `null` when the caller did not ask for a budget.

These fields are additive: nothing was removed and `truncated` was not
reinterpreted, so the contract stays at version 1. The catalog tools keep their
text shape and add a `[truncated: ...]` notice when the answer is not complete;
they also return the same fields in `structuredContent`.

Tool errors use MCP `isError=true`. Harnesses must not infer success from text.
They must not automatically retry a modification after a timeout or transport
failure because its database outcome can be unknown.

## Usage accounting

The server writes one counted record per tool call to stderr as JSON with `component="usage"`: the tool, the elapsed time, the bytes on the wire, the bytes of text and of structured content, the rows delivered and the truncation flag when the tool exposes them, and the outcome (`success`, `tool_error`, `rpc_error`, `timeout`).

The records carry no SQL, no results, no object definitions and no credentials, and they are not affected by the MCP `logging/setLevel` request: a harness that lowers the security log level can still count calls. They are observability, not part of the result contract — no tool result changes. `MSSQL_USAGE_LOG=false` disables the stream.

A harness that measures its own cost can read these records instead of estimating it. The server counts calls per session, not per task: grouping calls into tasks belongs to the harness. A `timeout` record means the request exceeded the server deadline; it does not prove that a modification was not applied.

## Examples

Real shapes, with neutral names. The CI unmarshals them into the types of the
server, so they cannot become fiction: when a result field changes, the
example has to change with it or the build fails.

### A query that fits in the budget

<!-- example:query-result -->
```json
{
  "rows": [
    {"id": 1, "name": "Acme", "active": true}
  ],
  "row_count": 1,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {
    "max_rows": 500,
    "max_cell_bytes": null,
    "max_response_bytes": null
  },
  "stats": {
    "enabled": false,
    "messages": []
  }
}
```

### A result that was cut

<!-- example:truncated-result -->
```json
{
  "rows": [
    {"id": 1, "notes": "the first bytes of the value [truncated: 1048576 bytes total]"}
  ],
  "row_count": 1,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": true,
  "response_truncated": false,
  "truncated_cells": [
    {"row": 0, "column": "notes", "original_bytes": 1048576}
  ],
  "limits": {
    "max_rows": 500,
    "max_cell_bytes": 4096,
    "max_response_bytes": null
  },
  "stats": {
    "enabled": false,
    "messages": []
  }
}
```

### A search

<!-- example:search-result -->
```json
{
  "matches": [
    {
      "object_type": "TABLE",
      "schema": "dbo",
      "name": "Order",
      "qualified_name": "dbo.Order",
      "match": {"field": "name", "kind": "exact", "term": "Order"},
      "foreign_keys": [
        {
          "from_columns": ["CustomerId"],
          "to_table": "dbo.Customer",
          "to_columns": ["Id"]
        }
      ]
    }
  ],
  "returned": 1,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {
    "max_objects": 50,
    "max_foreign_key_objects": 30,
    "max_foreign_keys_per_object": 10,
    "max_cell_bytes": null,
    "max_response_bytes": null
  },
  "next_steps": [
    {
      "tool": "inspect",
      "arguments": {"table_name": "dbo.Order", "detail": "all"},
      "reason": "read the full structure of dbo.Order"
    }
  ]
}
```

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
