---
title: Herramientas MCP
description: Resumen de las 6 herramientas base disponibles en el servidor MCP
---

El servidor MCP-Go-MSSQL expone **6 herramientas base** que Claude Desktop puede utilizar para interactuar con bases de datos Microsoft SQL Server de forma segura. En modo dinámico se exponen **5 herramientas adicionales** (11 en total).

## Lista de herramientas

| Herramienta | Descripción | Parámetros clave |
|-------------|-------------|------------------|
| [`query_database`](/herramientas-mcp/query-database/) | Ejecutar consultas SQL | `query` (requerido) |
| [`get_database_info`](/herramientas-mcp/get-database-info/) | Info de conexión y estado | — |
| [`explore`](/herramientas-mcp/explore/) | Explorar objetos: tablas, bases de datos, procedimientos, búsqueda | `type`, `filter`, `schema`, `pattern`, `search_in` |
| [`inspect`](/herramientas-mcp/inspect/) | Inspeccionar estructura de una tabla: columnas, índices, claves foráneas | `table_name` (requerido), `schema`, `detail` |
| [`execute_procedure`](/herramientas-mcp/execute-procedure/) | Ejecutar procedimiento almacenado (whitelist requerida) | `procedure_name` (requerido), `parameters` |
| [`explain_query`](/herramientas-mcp/explain-query/) | Plan de ejecución estimado sin ejecutar la consulta (solo SELECT) | `query` (requerido) |

## Herramientas del modo dinámico

Solo se exponen cuando el servidor arranca en modo dinámico (`MSSQL_DYNAMIC_*`):

| Herramienta | Descripción |
|-------------|-------------|
| `dynamic_available` | Listar alias de conexión preconfigurados |
| `dynamic_connect` | Activar una conexión dinámica por alias |
| `dynamic_disconnect` | Cerrar la conexión dinámica activa |
| `dynamic_list` | Mostrar alias cargados y cuál está activo |
| `confirm_operation` | Confirmar una modificación en un alias escribible (expira en 90 segundos) |

## Protocolo MCP

Las herramientas se comunican via JSON-RPC a través de stdin/stdout. Claude Desktop envía solicitudes `tools/list` para descubrir las herramientas y `tools/call` para ejecutarlas.

## Seguridad

Todas las herramientas:
- Usan **prepared statements** para prevenir SQL injection
- Respetan el **modo solo lectura** cuando está activado
- Validan las **tablas referenciadas** contra la whitelist
- Operan con **timeouts de contexto** (30 segundos)
- Sanitizan la información sensible en los logs
