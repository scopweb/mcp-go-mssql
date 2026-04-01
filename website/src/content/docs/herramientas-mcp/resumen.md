---
title: Herramientas MCP
description: Resumen de las 5 herramientas disponibles en el servidor MCP
---

El servidor MCP-Go-MSSQL expone **5 herramientas** que Claude Desktop puede utilizar para interactuar con bases de datos Microsoft SQL Server de forma segura.

## Lista de herramientas

| Herramienta | Descripción | Parámetros clave |
|-------------|-------------|------------------|
| [`query_database`](/herramientas-mcp/query-database/) | Ejecutar consultas SQL | `query` (requerido) |
| [`get_database_info`](/herramientas-mcp/get-database-info/) | Info de conexión y estado | — |
| [`explore`](/herramientas-mcp/explore/) | Explorar objetos: tablas, bases de datos, procedimientos, búsqueda | `type`, `filter`, `pattern`, `search_in`, `database` |
| [`inspect`](/herramientas-mcp/inspect/) | Inspeccionar estructura de una tabla: columnas, índices, claves foráneas | `table_name` (requerido), `schema`, `detail` |
| [`execute_procedure`](/herramientas-mcp/execute-procedure/) | Ejecutar procedimiento almacenado (whitelist requerida) | `procedure_name` (requerido), `parameters` |

## Protocolo MCP

Las herramientas se comunican via JSON-RPC a través de stdin/stdout. Claude Desktop envía solicitudes `tools/list` para descubrir las herramientas y `tools/call` para ejecutarlas.

## Seguridad

Todas las herramientas:
- Usan **prepared statements** para prevenir SQL injection
- Respetan el **modo solo lectura** cuando está activado
- Validan las **tablas referenciadas** contra la whitelist
- Operan con **timeouts de contexto** (30 segundos)
- Sanitizan la información sensible en los logs
