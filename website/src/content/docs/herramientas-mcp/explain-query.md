---
title: explain_query
description: Plan de ejecución estimado sin ejecutar la consulta
---

`explain_query` muestra el plan estimado de un `SELECT`. No ejecuta la consulta y no devuelve filas.

## Parámetros

| Parámetro | Requerido | Descripción |
|-----------|-----------|-------------|
| `query` | Sí | Consulta `SELECT` de solo lectura |

Cualquier sentencia que no sea de lectura se rechaza.

## Cuándo usarla

Úsala antes de un `query_database` grande para ver joins, scans e índices. Para tiempos reales y lecturas lógicas, envuelve la consulta con `SET STATISTICS`. Ver [statistics](/herramientas-mcp/statistics/).
