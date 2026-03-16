# Bug #4: Token overflow — "No se pudo generar completamente la respuesta de Claude"

**Date:** 2026-03-03
**Reporter:** Community
**Severity:** High (blocks any exhaustive DB exploration on large databases)
**Status:** ✅ RESUELTO

---

## Descripción

Al intentar buscar referencias a un objeto SQL (`PedidoCamioCarregaCamioAdmin` con estados 26/27) en la base de datos `MyDatabase`, Claude fallaba con el error:

```
No se pudo generar completamente la respuesta de Claude
```

La sesión se interrumpía después de llamar a `list_tables` (o al planificar una búsqueda exhaustiva en vistas, procedimientos y funciones).

## Causa Raíz

Tres problemas combinados:

### 1. `executeSecureQuery` sin límite de filas
```go
// ANTES — sin límite, traía TODAS las filas
var results []map[string]interface{}
for rows.Next() {
    // ...
    results = append(results, row)
}
return results, nil
```
En una base de datos con cientos de tablas/vistas/procedimientos, el resultado JSON era masivo y superaba el límite de tokens del contexto de Claude.

### 2. `list_tables` devolvía todos los objetos sin filtro
```go
// ANTES — sin parámetros de filtrado
query := `
    SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE
    FROM INFORMATION_SCHEMA.TABLES
    WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
    ORDER BY TABLE_SCHEMA, TABLE_NAME
`
```
En MyDatabase con >200 tablas/vistas, el resultado JSON podía superar 50 KB.

### 3. No existía herramienta de búsqueda directa en objetos SQL
No había forma de buscar "¿qué procedimientos almacenados referencian X?" sin primero listar todos y luego inspeccionarlos uno a uno — un enfoque O(n) muy costoso en tokens.

## Síntomas

- Error `No se pudo generar completamente la respuesta de Claude` al llamar `list_tables` en BDs grandes
- Claude "planificaba" búsquedas exhaustivas (listar todo → inspeccionar cada objeto) que agotaban el contexto antes de terminar
- El fallo era silencioso desde el servidor MCP; el problema ocurría en el lado del LLM al procesar la respuesta

## Solución Aplicada

### Fix 1 — Límite global de 500 filas en `executeSecureQuery` ([main.go:619](../../main.go#L619))

```go
const maxQueryRows = 500

// Dentro del loop de rows.Next():
if rowCount >= maxQueryRows {
    truncated = true
    break
}
// ...
if truncated {
    results = append(results, map[string]interface{}{
        "_truncated": fmt.Sprintf("Results limited to %d rows. Use WHERE or TOP to narrow the query.", maxQueryRows),
    })
}
```

Toda consulta queda acotada a 500 filas. Si se trunca, el último elemento del array advierte al LLM para que refine la query.

### Fix 2 — Parámetro `filter` en `list_tables` ([main.go:897](../../main.go#L897))

```go
if filterVal, ok := params.Arguments["filter"].(string); ok && filterVal != "" {
    filterPattern := "%" + filterVal + "%"
    query := `... WHERE TABLE_NAME LIKE @p1 ...`
    results, err = s.executeSecureQuery(ctx, query, filterPattern)
}
```

Uso correcto:
```
list_tables  filter="Pedido"   → solo tablas/vistas con "Pedido" en el nombre
list_tables  filter="Camio"    → solo tablas/vistas con "Camio" en el nombre
```

### Fix 3 — Nuevo tool `search_objects` ([main.go:1693](../../main.go#L1693))

Nuevo tool con dos modos:

**Búsqueda por nombre** (default):
```
search_objects  pattern="PedidoCamioCarregaCamioAdmin"
```
→ Devuelve todas las tablas, vistas, procedimientos y funciones cuyo *nombre* contenga el patrón. Una sola query en `sys.objects`, resultado acotado.

**Búsqueda en definición**:
```
search_objects  pattern="PedidoCamioCarregaCamioAdmin"  search_in="definition"
```
→ Busca dentro del código fuente de procedimientos almacenados, funciones y vistas usando `sys.sql_modules`. Ideal para encontrar referencias a un objeto específico (ej: estado 26 o 27) sin recorrer todos los objetos manualmente.

SQL usado:
```sql
-- search_in=definition
SELECT o.type_desc, SCHEMA_NAME(o.schema_id), o.name, m.definition
FROM sys.sql_modules m
JOIN sys.objects o ON o.object_id = m.object_id
WHERE m.definition LIKE @p1
ORDER BY o.type_desc, o.name
```

## Impacto de la Solución

| Situación | Antes | Después |
|---|---|---|
| `list_tables` en BD con 300 objetos | ~80 KB JSON, fallo de tokens | ≤500 filas + advertencia si trunca |
| Buscar objetos por nombre | No existía → list_tables + parseo manual | `search_objects pattern="X"` directo |
| Buscar referencias en código SQL | No existía → inspección manual O(n) | `search_objects pattern="X" search_in="definition"` |
| Cualquier query sin TOP/WHERE | Sin límite, podía traer miles de filas | Cortado en 500 con aviso |

## Caso de Uso Original Resuelto

Para encontrar referencias a `PedidoCamioCarregaCamioAdmin` con estados 26/27 en MyDatabase:

```
1. search_objects  pattern="PedidoCamioCarregaCamioAdmin"
   → Lista qué tablas/vistas/procs tienen ese nombre

2. search_objects  pattern="PedidoCamioCarregaCamioAdmin"  search_in="definition"
   → Lista qué procedimientos/vistas lo referencian en su código

3. query_database  query="SELECT * FROM PedidoCamioCarregaCamio WHERE Estado IN (26,27)"
   → Consulta directa si la tabla existe
```

## Notas

- El límite de 500 filas aplica a **todas** las herramientas (list_tables, describe_table, query_database, etc.) porque todas usan `executeSecureQuery`
- Para queries que devuelvan más de 500 filas, usar `TOP N` en la propia query SQL
- El campo `_truncated` en el resultado avisa al LLM para que refine la búsqueda automáticamente
