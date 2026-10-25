---
title: explain_query
description: Estimated execution plan without running the query
---

`explain_query` shows the estimated plan for a `SELECT`. It does not run the query and does not return rows.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `query` | Yes | A read-only `SELECT` |

Any statement that is not a read is rejected.

## When to use it

Use it before a large `query_database` call to see joins, scans, and indexes. For real timings and logical reads, wrap the query with `SET STATISTICS`. See [statistics](/en/herramientas-mcp/statistics/).
