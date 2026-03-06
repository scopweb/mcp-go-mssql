---
title: list_tables
description: Listar todas las tablas y vistas de la base de datos
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`explore`](/herramientas-mcp/explore/) como parte de la consolidación de la API en la versión 2.

Usa `explore (type=tables)` para obtener el mismo resultado.
:::


Lista todas las tablas y vistas disponibles en la base de datos conectada.

## Parámetros

Esta herramienta no requiere parámetros.

## Ejemplo de uso

```json
{
  "name": "list_tables",
  "arguments": {}
}
```

## Respuesta

Devuelve una lista con el nombre, esquema y tipo (TABLE o VIEW) de cada objeto.

```json
[
  {"schema": "dbo", "name": "users", "type": "TABLE"},
  {"schema": "dbo", "name": "orders", "type": "TABLE"},
  {"schema": "dbo", "name": "v_active_users", "type": "VIEW"}
]
```

## Consulta interna

La herramienta ejecuta internamente:

```sql
SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE
FROM INFORMATION_SCHEMA.TABLES
ORDER BY TABLE_SCHEMA, TABLE_NAME
```

## Notas

- Funciona en modo lectura y escritura
- No requiere permisos especiales más allá de `SELECT` en `INFORMATION_SCHEMA`
- Los resultados incluyen tanto tablas de usuario como vistas
