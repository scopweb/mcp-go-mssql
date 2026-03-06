---
title: explore
description: Explora objetos de la base de datos - tablas, vistas, bases de datos, procedimientos almacenados o búsqueda por nombre/definición
---

:::tip[Reemplaza a (v1)]
Esta herramienta unifica: `list_tables`, `list_databases`, `list_stored_procedures` y `search_objects` de la versión anterior.
:::



Herramienta unificada para explorar objetos de la base de datos. Reemplaza a `list_tables`, `list_databases`, `list_stored_procedures` y `search_objects`.

## Parámetros

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `type` | string | Qué explorar: `tables` (por defecto), `databases`, `procedures`, `search` |
| `filter` | string | Filtro por nombre (LIKE). Válido para `tables` y `procedures` |
| `schema` | string | Filtro por esquema. Solo para `procedures` (opcional) |
| `pattern` | string | Patrón de búsqueda. **Requerido** cuando `type=search` |
| `search_in` | string | Dónde buscar: `name` (por defecto) o `definition` (código fuente) |

## Modos de uso

### Listar tablas y vistas (por defecto)

```json
{ "name": "explore", "arguments": {} }
```

Con filtro:
```json
{ "name": "explore", "arguments": { "filter": "Pedido" } }
```

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

## Límite de resultados

Todos los resultados están limitados a **500 filas**. Si hay más, el último elemento incluirá la clave `_truncated` como advertencia.
