---
title: Herramientas MCP
description: Resumen de las 6 herramientas disponibles en el servidor MCP
---

El servidor MCP-Go-MSSQL expone **6 herramientas** que Claude Desktop puede utilizar para interactuar con bases de datos Microsoft SQL Server de forma segura.

## Lista de herramientas

| Herramienta | Descripción | Parámetros clave |
|-------------|-------------|------------------|
| [`query_database`](/herramientas-mcp/query-database/) | Ejecutar consultas SQL | `query` (requerido) |
| [`get_database_info`](/herramientas-mcp/get-database-info/) | Info de conexión y estado | — |
| [`explore`](/herramientas-mcp/explore/) | Explorar objetos: tablas, vistas, bases de datos, procedimientos, búsqueda | `type`, `filter`, `pattern`, `search_in`, `database` |
| [`inspect`](/herramientas-mcp/inspect/) | Inspeccionar estructura: columnas, índices, claves foráneas, dependencias | `table_name` (requerido), `schema`, `detail` |
| [`explain_query`](/herramientas-mcp/explain-query/) | Plan de ejecución estimado sin ejecutar la query | `query` (requerido) |
| [`execute_procedure`](/herramientas-mcp/execute-procedure/) | Ejecutar procedimiento almacenado (whitelist requerida) | `procedure_name` (requerido), `parameters` |

## Protocolo MCP

Las herramientas se comunican via JSON-RPC a través de stdin/stdout. Claude Desktop envía solicitudes `tools/list` para descubrir las herramientas y `tools/call` para ejecutarlas.

Todas las respuestas incluyen **content annotations** según la spec MCP 2025-11-25:
- `audience`: indica si el contenido es para `user`, `assistant` o ambos
- `priority`: de 0.0 (menor) a 1.0 (mayor importancia)

## Rate limiting

El servidor implementa un rate limiter de **60 llamadas por minuto** (token bucket). Si se excede el límite, la herramienta devuelve un error y hay que esperar antes de reintentar.

## Seguridad

Todas las herramientas:
- Usan **prepared statements** para prevenir SQL injection
- Respetan el **modo solo lectura** cuando está activado
- Validan las **tablas referenciadas** contra la whitelist
- **Validan que las tablas existan** antes de ejecutar (schema validation con sugerencias "Did you mean?")
- Operan con **timeouts de contexto** (30 segundos)
- Sanitizan la información sensible en los logs
- Incluyen **anotaciones MCP** (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`)
