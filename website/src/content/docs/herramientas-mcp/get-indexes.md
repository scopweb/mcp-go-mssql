---
title: get_indexes
description: Obtener índices de una tabla específica
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`inspect`](/herramientas-mcp/inspect/) como parte de la consolidación de la API en la versión 2.

Usa `inspect (detail=indexes)` para obtener el mismo resultado.
:::


Devuelve información sobre los índices definidos para una tabla, incluyendo tipo, unicidad y columnas indexadas.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `table_name` | string | Sí | Nombre de la tabla (puede incluir esquema: `dbo.TableName`) |
| `schema` | string | No | Nombre del esquema (por defecto `dbo`) |

## Ejemplo de uso

```json
{
  "name": "get_indexes",
  "arguments": {
    "table_name": "orders"
  }
}
```

## Respuesta

Incluye para cada índice:
- Nombre del índice
- Tipo (CLUSTERED, NONCLUSTERED, etc.)
- Si es único
- Columnas incluidas

## Uso típico

- Análisis de rendimiento de consultas
- Verificar que existen índices adecuados para JOINs frecuentes
- Identificar índices duplicados o innecesarios
