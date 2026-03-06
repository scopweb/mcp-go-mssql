---
title: "Overflow de tokens al explorar bases de datos grandes"
description: list_tables sin filtro ni límite de filas provocaba que la respuesta de Claude fuera demasiado grande y fallara con "No se pudo generar completamente la respuesta"
---

Al intentar explorar una base de datos con muchas tablas o buscar referencias a un objeto SQL, Claude fallaba con el error:

> **No se pudo generar completamente la respuesta de Claude**

## Detalles del problema

| Campo | Valor |
|---|---|
| **Fecha** | 2026-03-03 |
| **Severidad** | Alta (bloquea la exploración de BDs grandes) |
| **Estado** | ✅ Resuelto |

## Causa raíz

Tres problemas combinados:

1. **`executeSecureQuery` sin límite de filas** — devolvía todos los resultados sin corte, generando JSONs de cientos de KB que superaban el contexto de Claude.
2. **`list_tables` sin filtro** — en bases de datos con más de 200 tablas/vistas el resultado era masivo.
3. **Sin herramienta de búsqueda directa** — para buscar "qué procedimientos referencian X" era necesario listar todos los objetos e inspeccionarlos uno a uno (costosísimo en tokens).

## Soluciones aplicadas

### 1 — Límite global de 500 filas

Todas las consultas quedan acotadas a 500 filas. Si se trunca el resultado, el último elemento incluye una advertencia `_truncated` para que Claude sepa que debe refinar la query con `WHERE` o `TOP`.

### 2 — Parámetro `filter` en `list_tables`

Ahora se puede filtrar antes de traer datos:

```
list_tables  filter="Pedido"   →  solo tablas/vistas que contengan "Pedido"
list_tables  filter="Camio"    →  solo tablas/vistas que contengan "Camio"
```

### 3 — Nuevo tool `search_objects`

Búsqueda directa en dos modos:

| Modo | Uso | Qué devuelve |
|---|---|---|
| Por nombre (default) | `search_objects pattern="PedidoCamio"` | Tablas, vistas, procs y funciones cuyo **nombre** coincide |
| Por definición | `search_objects pattern="PedidoCamio" search_in="definition"` | Procs, funciones y vistas que **referencian** ese texto en su código fuente |

## Schema actualizado de funciones

Todos los conectores exponen exactamente las mismas funciones:

| Función | Parámetros |
|---|---|
| `query_database` | `query: string` |
| `get_database_info` | — |
| `explore` | `type?: string, filter?: string, schema?: string, pattern?: string, search_in?: string` |
| `inspect` | `table_name: string, schema?: string, detail?: string` |
| `execute_procedure` | `procedure_name: string, parameters?: string` |

## Ejemplo: caso de uso original resuelto

Para encontrar referencias a `PedidoCamioCarregaCamioAdmin` con estados 26/27:

```sql
-- 1. Buscar el objeto por nombre
search_objects  pattern="PedidoCamioCarregaCamioAdmin"

-- 2. Buscar qué procs/vistas lo referencian en su código
search_objects  pattern="PedidoCamioCarregaCamioAdmin"  search_in="definition"

-- 3. Consulta directa filtrando por estado
query_database  query="SELECT * FROM PedidoCamioCarregaCamio WHERE Estado IN (26,27)"
```
