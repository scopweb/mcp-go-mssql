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

### Buscar objetos por nombre

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "PedidoCamio" } }
```

### Buscar en el código fuente de procedimientos/vistas

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "PedidoCamio", "search_in": "definition" } }
```

### Buscar qué tabla tiene una columna

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "ClienteId", "search_in": "column" } }
```

Responde en una sola llamada a "¿qué tabla tiene esta columna?", sin listar el catálogo e inspeccionar tabla por tabla. El término se compara como **subcadena literal**: los comodines `%`, `_` y `[` se escapan, así que `100%` busca la columna `100%` y no cualquier columna que termine en `100`.

Filtros opcionales y límite:

```json
{ "name": "explore", "arguments": { "type": "search", "pattern": "Cliente", "search_in": "column", "schema": "dbo", "table": "Pedido", "limit": 20 } }
```

Este modo devuelve un **objeto**, no el array plano de los demás modos:

```json
{
  "matches": [
    { "schema_name": "dbo", "table_name": "Pedido", "column_name": "ClienteId", "data_type": "int", "max_length": null, "is_nullable": "NO" }
  ],
  "returned": 1,
  "truncated": false,
  "limit": 50
}
```

`truncated: true` significa que había más columnas que las que devuelve el límite: acota con `schema`, `table` o un término más específico en lugar de dar la lista por completa. `max_length` es `null` en los tipos que no son de texto.

## Límite de resultados

Todos los resultados están limitados a **500 filas**. Si hay más, el último elemento incluirá la clave `_truncated` como advertencia.

La búsqueda por columna (`search_in=column`) no usa ese límite: devuelve hasta `limit` coincidencias (50 por defecto, 200 máximo) y señala con `truncated: true` que existían más.
