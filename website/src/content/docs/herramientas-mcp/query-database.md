---
title: query_database
description: Ejecutar consultas SQL de forma segura
---

Ejecuta una consulta SQL contra la base de datos MSSQL usando prepared statements.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `query` | string | Sí | Consulta SQL a ejecutar |
| `max_cell_bytes` | integer | No | Tope del tamaño de una celda (mínimo 16). Un valor mayor se acorta y queda marcado |
| `max_response_bytes` | integer | No | Tope aproximado del tamaño de toda la respuesta (mínimo 512). El resultado se detiene ahí |
| `format` | string | No | `map` (por defecto, un objeto por fila) o `columnar` (los nombres de columna una vez y cada fila un array de valores en ese orden) |

## Ejemplo de uso

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SELECT TOP 10 * FROM users WHERE active = 1"
  }
}
```

## Formatos de salida

`query_database` admite dos formatos:

- `format="map"` (por defecto): un objeto por fila, con el nombre de la columna en cada valor.
- `format="columnar"`: los nombres de columna se pagan una vez y cada fila es un array de valores en ese orden.

```json
{
  "format": "columnar",
  "columns": ["id", "name", "city"],
  "rows": [[1, "Acme", "Barcelona"], [2, "Beta", null]],
  "row_count": 2,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {"max_rows": 500, "max_cell_bytes": null, "max_response_bytes": null},
  "stats": {"enabled": false, "messages": []}
}
```

El formato columnar necesita nombres de columna únicos: si la consulta repite un nombre, el servidor lo rechaza porque una fila no puede llevarlo dos veces. Con diez columnas y doscientas filas, la forma columnar ocupa alrededor de un tercio que la forma mapa; medida en una tabla de 38 columnas y 100 filas, la respuesta completa pasó de 182 KB a 91 KB.

## Consultas permitidas

### En modo lectura (`MSSQL_READ_ONLY=true`)
- `SELECT` y CTEs de solo lectura — Permitidos
- Procedimientos administrativos de esquema explícitamente permitidos (`sp_help`, `sp_columns`, …) — Permitidos si son `sys`/`dbo` y no cambian estado
- `INSERT`, `UPDATE`, `DELETE`, DDL, `SELECT INTO`, `execute_procedure` — Bloqueados, aunque haya whitelist
- Lotes concatenados (con o sin `;`) — Bloqueados
- `xp_cmdshell`, `sp_executesql` — Siempre bloqueado

### En modo completo (`MSSQL_READ_ONLY=false`)
- Todas las operaciones SQL estándar
- `EXEC`, `xp_cmdshell` — Siempre bloqueado por seguridad

:::note[Lotes multi-sentencia]
En modo solo lectura / whitelist se rechazan los lotes con varias sentencias, **excepto** los wrappers de diagnóstico `SET STATISTICS TIME/IO`. Consulta [Estadísticas de ejecución](/herramientas-mcp/statistics/) para más detalles.
:::

## Ejemplos de consultas

```sql
-- Consulta simple
SELECT * FROM products WHERE price > 100

-- JOIN complejo
SELECT u.name, COUNT(o.id) as total_orders
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.name

-- CTE
WITH recent_orders AS (
    SELECT * FROM orders WHERE order_date > DATEADD(day, -30, GETDATE())
)
SELECT * FROM recent_orders

-- Funciones de ventana
SELECT name, salary,
    ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank
FROM employees
```

## Seguridad

- Las consultas se ejecutan con `PrepareContext()` — no hay concatenación de strings SQL
- El tamaño máximo de consulta es configurable via `MSSQL_MAX_QUERY_SIZE`
- Se aplica un timeout de 30 segundos por defecto
- En modo read-only, se validan todas las tablas referenciadas (incluyendo JOINs y subqueries)
