---
title: SET STATISTICS in read-only mode
description: How to capture per-statement execution timings inside a read-only query
---

When the server is running with `MSSQL_READ_ONLY=true`, you can prepend
`SET STATISTICS TIME ON;` (and/or `SET STATISTICS IO ON;`) to a single
SELECT to ask SQL Server to report per-statement execution timings or
logical / physical reads. This is the **only** multi-statement form the
read-only mode accepts — every other multi-statement pattern
(`SELECT 1; DELETE FROM prod`, `PRINT 'x'; SELECT 1`, …) is rejected.

## Usage

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SET STATISTICS TIME ON; SET STATISTICS IO ON; SELECT TOP 10 id, name FROM orders ORDER BY id; SET STATISTICS TIME OFF;"
  }
}
```

The wrapper is parsed by a strict regular-language matcher. Allowed:

- Leading `SET STATISTICS TIME/IO (ON|OFF)` (zero or one of each kind)
- One base statement (single statement, no internal `;`)
- Trailing `SET STATISTICS TIME/IO (ON|OFF)` (zero or one of each kind)

Rejected:

- A wrapper around a non-read-only statement (`SET STATISTICS TIME ON; DELETE FROM prod` → blocked by the existing read-only check on the base statement).
- More than one base statement (`SET STATISTICS TIME ON; SELECT 1; SELECT 2` → blocked as multi-statement).
- A non-`SET STATISTICS` token inside the wrapper (`SET STATISTICS TIME ON; PRINT 'x'; SELECT 1` → blocked as multi-statement).
- Empty statements between the wrapper tokens (`SET STATISTICS TIME ON; ; SELECT 1` → blocked).
- A wrapper around a write is rejected when `READ_ONLY=true`, even if the table is whitelisted.

## Response

The response payload shape changes from a bare JSON array of row maps
to a structured object with a `stats` block:

```json
{
  "rows": [{ "id": 1, "name": "x" }, { "id": 2, "name": "y" }],
  "row_count": 2,
  "truncated": false,
  "stats": {
    "enabled": true,
    "time_on": true,
    "io_on": true,
    "messages": [
      " SQL Server Execution Times:",
      "   CPU time = 16 ms,  elapsed time = 22 ms.",
      "Table 'orders'. Scan count 1, logical reads 4, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0."
    ],
    "parsed": {
      "cpu_time_ms": 16,
      "elapsed_time_ms": 22,
      "logical_reads": 4,
      "physical_reads": 0,
      "scan_count_orders": 1,
      "logical_reads_orders": 4,
      "physical_reads_orders": 0
    }
  }
}
```

The `stats` block is always present (with `enabled:false` when no
wrapper was used) so clients have a stable shape. The previous
`_truncated` sentinel row is replaced by the `truncated` boolean.

## Notes

- The wrapper is captured via the `sqlexp.ReturnMessage` queue exposed
  by `go-mssqldb` v1.11.2; the connection layer was refactored from
  `sql.Open("sqlserver", dsn)` to `sql.OpenDB(mssql.NewConnector(dsn))`
  to make this work.
- The CLI tool (`examples/claude-code/db-connector.go`) is not affected —
  wrappers are an MCP-tool concern only.
- For multi-statement batches the `stats` block reports timings for
  each statement individually (the base statement's timings; the
  leading/trailing SETs themselves do not produce timings).