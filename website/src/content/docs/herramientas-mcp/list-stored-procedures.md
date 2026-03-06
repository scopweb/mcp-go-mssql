---
title: list_stored_procedures
description: Listar procedimientos almacenados de la base de datos
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`explore`](/herramientas-mcp/explore/) como parte de la consolidación de la API en la versión 2.

Usa `explore (type=procedures)` para obtener el mismo resultado.
:::


Lista todos los procedimientos almacenados disponibles en la base de datos, con opción de filtrar por esquema.

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `schema` | string | No | Filtrar por esquema (opcional) |

## Ejemplo de uso

Sin filtro:
```json
{
  "name": "list_stored_procedures",
  "arguments": {}
}
```

Con filtro de esquema:
```json
{
  "name": "list_stored_procedures",
  "arguments": {
    "schema": "dbo"
  }
}
```

## Notas

- En modo solo lectura, los procedimientos del sistema que son seguros (`sp_help`, `sp_helptext`, `sp_columns`, etc.) están permitidos
- Los procedimientos peligrosos (`xp_cmdshell`, `sp_configure`, `sp_executesql`) están bloqueados siempre
