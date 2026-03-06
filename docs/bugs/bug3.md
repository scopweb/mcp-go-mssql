# Bug: mssql MCP connectors require tool_search on every new session

**Date:** 2026-03-02  
**Reporter:** jotajotape  
**Severity:** Low (UX / token cost)  
**Status:** Workaround applied

---

## Descripción

Los conectores MCP de tipo `mssql-*` están marcados como **deferred tools** en el sistema de Claude. Esto significa que en cada nueva sesión/conversación, Claude no conoce sus parámetros y llama a `tool_search` antes de poder usarlos — aunque los haya usado miles de veces antes.

## Conectores afectados

- `mssql-JJP_CRM`
- `mssql-JJP_CRM_LOCAL`
- `mssql-SERVER-GDP`
- `mssql-SQL01`
- `mssql-JJP_IDENTITY`

## Impacto

- **Coste de tokens innecesario** en cada sesión que toca BD
- **Latencia extra** — una llamada adicional antes de la consulta real
- Todos los conectores `mssql-*` comparten el mismo schema de funciones, por lo que el `tool_search` es redundante

## Funciones disponibles (schema común)

Todos los conectores exponen exactamente las mismas funciones:

| Función | Parámetros |
|---|---|
| `query_database` | `query: string` |
| `list_tables` | `filter?: string` |
| `describe_table` | `table_name: string, schema?: string` |
| `get_foreign_keys` | `table_name: string, schema?: string` |
| `get_indexes` | `table_name: string, schema?: string` |
| `list_stored_procedures` | `schema?: string` |
| `get_database_info` | — |
| `search_objects` | `pattern: string, search_in?: string` |

## Workaround aplicado

Añadida nota en memoria de usuario de Claude con el schema completo:
```
Conectores mssql (...): NO hacer tool_search, usar directamente:
query_database(query), list_tables(), describe_table(table_name, schema?), ...
```

Con esto Claude salta el `tool_search` y llama directamente al conector.

## Causa raíz (probable)

El sistema de deferred tools no distingue entre tools "conocidos por training" y tools que requieren descubrimiento real. Todos los MCP pasan por el mismo mecanismo aunque su schema sea estático y conocido.

## Solución ideal

Que los conectores `mssql-*` (u otros con schema estático y bien definido) puedan marcarse como **pre-loaded** o que Claude los reconozca por prefijo sin necesidad de `tool_search`.