---
title: explain_query
description: Mostrar el plan de ejecución estimado de una consulta SQL sin ejecutarla
---

Muestra el plan de ejecución estimado de una consulta SQL **sin ejecutarla**. Útil para análisis de rendimiento y optimización de queries.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `query` | string | Sí | Consulta SELECT a analizar |

:::caution[Solo SELECT]
Esta herramienta **solo acepta consultas SELECT**. Esto se aplica siempre, independientemente del modo `MSSQL_READ_ONLY`.
:::

## Ejemplo de uso

```json
{
  "name": "explain_query",
  "arguments": {
    "query": "SELECT u.name, COUNT(o.id) FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.name"
  }
}
```

## Respuesta de ejemplo

```
Execution plan:

  |--Stream Aggregate(GROUP BY:([u].[name]))
       |--Sort(ORDER BY:([u].[name] ASC))
            |--Hash Match(Inner Join, HASH:([u].[id])=([o].[user_id]))
                 |--Table Scan(OBJECT:([MyDB].[dbo].[users] AS [u]))
                 |--Table Scan(OBJECT:([MyDB].[dbo].[orders] AS [o]))
```

## Cómo funciona

1. Adquiere una conexión dedicada del pool
2. Ejecuta `SET SHOWPLAN_TEXT ON` en esa conexión
3. Envía la consulta — SQL Server devuelve el plan estimado **sin ejecutar** la query
4. Desactiva `SET SHOWPLAN_TEXT OFF` y libera la conexión

## Casos de uso

- **Identificar table scans** que podrían beneficiarse de un índice
- **Comparar planes** antes y después de agregar índices
- **Detectar JOINs costosos** en queries complejas
- **Validar que una query usará un índice** antes de ejecutarla en producción

## Seguridad

- Solo consultas SELECT — no hay riesgo de modificar datos
- Usa conexión dedicada para que `SET SHOWPLAN_TEXT` no afecte otras queries
- Timeout de 30 segundos
- Anotaciones MCP: `readOnlyHint=true`, `destructiveHint=false`, `idempotentHint=true`
