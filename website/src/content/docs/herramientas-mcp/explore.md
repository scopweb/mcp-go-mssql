---
title: explore
description: Explora objetos de la base de datos - tablas, vistas, bases de datos, procedimientos almacenados o búsqueda por nombre, definición o columna
---

:::tip[Reemplaza a (v1)]
Esta herramienta unifica: `list_tables`, `list_databases`, `list_stored_procedures` y `search_objects` de la versión anterior.
:::



Herramienta unificada para explorar objetos de la base de datos. Reemplaza a `list_tables`, `list_databases`, `list_stored_procedures` y `search_objects`.

## Parámetros

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `type` | string | Qué explorar: `tables` (por defecto), `views`, `databases`, `procedures`, `search` |
| `filter` | string | Filtro por nombre (LIKE). Válido para `tables` y `procedures` |
| `schema` | string | Filtro por esquema. Para `procedures` y para la búsqueda por columna (coincidencia exacta) |
| `table` | string | Filtro por tabla en la búsqueda por columna (subcadena literal) |
| `pattern` | string | Patrón de búsqueda. **Requerido** cuando `type=search` |
| `search_in` | string | Dónde buscar: `name` (por defecto), `definition` (código fuente) o `column` (columnas cuyo nombre contiene el término) |
| `limit` | integer | Máximo de resultados en `type=search` (por defecto 50, máximo 200) |

## Modos de uso

### Listar tablas y vistas (por defecto)

```json
{ "name": "explore", "arguments": {} }
```

Con filtro:
```json
{ "name": "explore", "arguments": { "filter": "Pedido" } }
```

### Listar vistas

```json
{ "name": "explore", "arguments": { "type": "views" } }
```

Incluye metadatos de cada vista: `check_option`, `is_updatable` y una vista previa de la definición.

### Listar bases de datos

```json
{ "name": "explore", "arguments": { "type": "databases" } }
```

### Listar procedimientos almacenados

```json
{ "name": "explore", "arguments": { "type": "procedures" } }
```

Con filtro de nombre y esquema:
```json
{ "name": "explore", "arguments": { "type": "procedures", "schema": "dbo", "filter": "Camio" } }
```

### Buscar por nombre, por columna o dentro del código fuente

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "Pedido" } }
{ "name": "explore", "arguments": { "type": "search", "pattern": "ClienteId", "search_in": "column" } }
{ "name": "explore", "arguments": { "type": "search", "pattern": "Pedido", "search_in": "definition" } }
```

Los tres modos de búsqueda comparten la misma respuesta: los objetos encontrados, el motivo de la coincidencia, las claves foráneas directas de las tablas que coinciden y el siguiente paso sugerido.

```json
{
  "matches": [{
    "object_type": "TABLE",
    "schema": "dbo",
    "name": "Pedido",
    "qualified_name": "dbo.Pedido",
    "match": {"field": "name", "kind": "exact", "term": "Pedido"},
    "columns": [{"name": "ClienteId", "data_type": "int", "max_length": null}],
    "foreign_keys": [{"from_columns": ["ClienteId"], "to_table": "dbo.Cliente", "to_columns": ["Id"]}]
  }],
  "returned": 1,
  "truncated": false,
  "rows_truncated": false,
  "cells_truncated": false,
  "response_truncated": false,
  "limits": {"max_objects": 50, "max_foreign_key_objects": 30,
             "max_foreign_keys_per_object": 10, "max_cell_bytes": null, "max_response_bytes": null},
  "next_steps": [{"tool": "inspect", "arguments": {"table_name": "dbo.Pedido", "detail": "all"},
                  "reason": "read the full structure of dbo.Pedido"}]
}
```

- El orden es determinista: primero el nombre exacto, luego el prefijo, luego el contenido, y el nombre como desempate.
- `match` indica el campo (`name`, `column`, `definition`) y el tipo de coincidencia (`exact`, `prefix`, `contains`).
- `foreign_keys` son las claves foráneas directas de las tablas que coinciden: los caminos de join del esquema.
- `next_steps` sugiere la llamada siguiente, normalmente `inspect` con `detail=all`.
- `truncated: true` significa que había más objetos que los que devuelve el límite: acota la búsqueda en lugar de dar la lista por completa.

## Límite de resultados

Todos los resultados están limitados a **500 filas**. Si hay más, el último elemento incluirá la clave `_truncated` como advertencia.

La búsqueda por columna (`search_in=column`) no usa ese límite: devuelve hasta `limit` coincidencias (50 por defecto, 200 máximo) y señala con `truncated: true` que existían más.
