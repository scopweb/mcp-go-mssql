---
title: query_database
description: Ejecutar consultas SQL de forma segura
---

Ejecuta una consulta SQL contra la base de datos MSSQL usando prepared statements.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `query` | string | Sí | Consulta SQL a ejecutar |

## Ejemplo de uso

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SELECT TOP 10 * FROM users WHERE active = 1"
  }
}
```

## Consultas permitidas

### En modo lectura (`MSSQL_READ_ONLY=true`)
- `SELECT` — Siempre permitido
- `INSERT`, `UPDATE`, `DELETE` — Solo en tablas de la whitelist
- `EXEC`, `xp_cmdshell` — Siempre bloqueado

### En modo completo (`MSSQL_READ_ONLY=false`)
- Todas las operaciones SQL estándar
- `EXEC`, `xp_cmdshell` — Siempre bloqueado por seguridad

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
