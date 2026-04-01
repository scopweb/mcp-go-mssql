---
title: inspect
description: Inspecciona la estructura de una tabla - columnas, índices, claves foráneas o todo a la vez
---

:::tip[Reemplaza a (v1)]
Esta herramienta unifica: `describe_table`, `get_indexes` y `get_foreign_keys` de la versión anterior.
:::



Herramienta unificada para inspeccionar la estructura de una tabla. Reemplaza a `describe_table`, `get_indexes` y `get_foreign_keys`.

## Parámetros

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `table_name` | string | **Requerido.** Nombre de la tabla. Acepta `dbo.Tabla` o solo `Tabla` |
| `schema` | string | Esquema (por defecto: `dbo`) |
| `detail` | string | Qué recuperar: `columns` (por defecto), `indexes`, `foreign_keys`, `dependencies`, `all` |

## Modos de uso

### Columnas (por defecto)

```json
{ "name": "inspect", "arguments": { "table_name": "Pedidos" } }
```

### Índices

```json
{ "name": "inspect", "arguments": { "table_name": "Pedidos", "detail": "indexes" } }
```

### Claves foráneas

```json
{ "name": "inspect", "arguments": { "table_name": "Pedidos", "detail": "foreign_keys" } }
```

### Dependencias (análisis de impacto)

```json
{ "name": "inspect", "arguments": { "table_name": "Pedidos", "detail": "dependencies" } }
```

Muestra qué objetos SQL (vistas, procedimientos, funciones) **dependen de esta tabla**. Usa `sys.sql_expression_dependencies` para análisis de impacto antes de modificar el esquema.

Devuelve: `referencing_schema`, `referencing_object`, `referencing_type`, `is_caller_dependent`, `is_ambiguous`.

### Todo en una sola llamada

```json
{ "name": "inspect", "arguments": { "table_name": "Pedidos", "detail": "all" } }
```

Con `detail=all` el resultado agrupa las secciones bajo las claves `columns`, `indexes`, `foreign_keys` y `dependencies`.

## Respuesta de ejemplo (detail=all)

```json
{
  "columns": [ {"column_name": "Id", "data_type": "int", ...} ],
  "indexes": [ {"index_name": "PK_Pedidos", "is_primary_key": true, ...} ],
  "foreign_keys": [ {"constraint_name": "FK_Pedidos_Clientes", ...} ]
}
```
