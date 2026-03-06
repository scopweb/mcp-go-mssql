---
title: describe_table
description: Obtener la estructura y esquema de una tabla
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`inspect`](/herramientas-mcp/inspect/) como parte de la consolidación de la API en la versión 2.

Usa `inspect (detail=columns)` para obtener el mismo resultado.
:::


Obtiene información detallada sobre las columnas de una tabla, incluyendo tipos de datos, nulabilidad y valores por defecto.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `table_name` | string | Sí | Nombre de la tabla (puede incluir esquema: `dbo.TableName`) |
| `schema` | string | No | Nombre del esquema (por defecto `dbo`) |

## Ejemplo de uso

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "users"
  }
}
```

Con esquema explícito:

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "users",
    "schema": "sales"
  }
}
```

O usando notación de esquema en el nombre:

```json
{
  "name": "describe_table",
  "arguments": {
    "table_name": "sales.users"
  }
}
```

## Respuesta

```json
[
  {
    "column_name": "id",
    "data_type": "int",
    "is_nullable": "NO",
    "column_default": null,
    "max_length": null
  },
  {
    "column_name": "name",
    "data_type": "nvarchar",
    "is_nullable": "YES",
    "column_default": null,
    "max_length": 255
  }
]
```

## Notas

- Soporta el formato `schema.table` y `[schema].[table]`
- Filtra correctamente por esquema y nombre de tabla para evitar confusiones entre tablas con el mismo nombre en diferentes esquemas
