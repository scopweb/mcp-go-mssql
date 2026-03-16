---
title: "Conectores MCP requieren tool_search en cada sesión"
description: Los conectores mssql-* están marcados como deferred tools, requiriendo tool_search antes de cada uso en nuevas sesiones
---

Los conectores MCP de tipo `mssql-*` están marcados como **deferred tools** en el sistema de Claude. Esto significa que en cada nueva sesión, Claude no conoce sus parámetros y llama a `tool_search` antes de poder usarlos — aunque los haya usado miles de veces antes.

## Detalles del problema

| Campo | Valor |
|---|---|
| **Fecha** | 2026-03-02 |
| **Severidad** | Baja (UX / coste de tokens) |
| **Estado** | Workaround aplicado |

## Conectores afectados

- `mssql-MyDatabase`
- `mssql-MyDatabase_LOCAL`
- `mssql-PROD-SERVER`
- `mssql-SQL01`
- `mssql-MyIdentityDB`

## Impacto

- **Coste de tokens innecesario** en cada sesión que toca BD
- **Latencia extra** — una llamada adicional antes de la consulta real
- Todos los conectores `mssql-*` comparten el mismo schema de funciones, por lo que el `tool_search` es redundante

## Schema común de funciones

Todos los conectores exponen exactamente las mismas funciones:

| Función | Parámetros |
|---|---|
| `query_database` | `query: string` |
| `get_database_info` | — |
| `explore` | `type?: string, filter?: string, schema?: string, pattern?: string, search_in?: string` |
| `inspect` | `table_name: string, schema?: string, detail?: string` |
| `execute_procedure` | `procedure_name: string, parameters?: string` |

## Causa raíz

El sistema de deferred tools no distingue entre tools "conocidos por training" y tools que requieren descubrimiento real. Todos los MCP pasan por el mismo mecanismo aunque su schema sea estático y conocido.

## Workaround aplicado

Se añadió una nota en la memoria de usuario de Claude con el schema completo de los conectores:

```
Conectores mssql (...): NO hacer tool_search, usar directamente:
query_database(query), list_tables(), describe_table(table_name, schema?), ...
```

Con esto Claude salta el `tool_search` y llama directamente al conector.

## Solución ideal

Que los conectores `mssql-*` (u otros con schema estático y bien definido) puedan marcarse como **pre-loaded** o que Claude los reconozca por prefijo sin necesidad de `tool_search`.
