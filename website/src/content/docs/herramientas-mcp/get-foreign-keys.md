---
title: get_foreign_keys
description: Obtener relaciones de claves foráneas de una tabla
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`inspect`](/herramientas-mcp/inspect/) como parte de la consolidación de la API en la versión 2.

Usa `inspect (detail=foreign_keys)` para obtener el mismo resultado.
:::


Devuelve las relaciones de claves foráneas de una tabla, tanto entrantes (otras tablas que referencian a esta) como salientes (tablas que esta referencia).

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `table_name` | string | Sí | Nombre de la tabla (puede incluir esquema: `dbo.TableName`) |
| `schema` | string | No | Nombre del esquema (por defecto `dbo`) |

## Ejemplo de uso

```json
{
  "name": "get_foreign_keys",
  "arguments": {
    "table_name": "orders"
  }
}
```

## Respuesta

Para cada relación incluye:
- Nombre de la clave foránea
- Tabla y columna padre
- Tabla y columna referenciada
- Dirección (entrante/saliente)

## Uso típico

- Entender las relaciones entre tablas
- Verificar integridad referencial
- Planificar JOINs para consultas complejas
