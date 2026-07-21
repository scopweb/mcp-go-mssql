---
title: Integración MCP
description: Detalles de la implementación del protocolo MCP en MCP-Go-MSSQL
---

## Protocolo MCP

MCP (Model Context Protocol) es el protocolo que permite a Claude Desktop comunicarse con servidores externos. MCP-Go-MSSQL implementa MCP sobre JSON-RPC 2.0 usando stdin/stdout.

## Herramientas disponibles

El servidor expone 6 herramientas MCP base (11 en modo dinámico, que añade 5 más):

| Herramienta | Descripción |
|-------------|-------------|
| `query_database` | Ejecuta consultas SQL |
| `get_database_info` | Información general de la conexión y la base de datos |
| `explore` | Explora objetos: tablas, vistas, bases de datos, procedimientos y búsqueda |
| `inspect` | Inspecciona una tabla: columnas, índices, claves foráneas y dependencias |
| `execute_procedure` | Ejecuta un procedimiento almacenado de la whitelist |
| `explain_query` | Plan de ejecución estimado de una consulta SELECT sin ejecutarla |

En modo dinámico se exponen además: `dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list` y `confirm_operation`.

## Flujo de comunicación

1. Claude Desktop inicia el proceso `mcp-go-mssql`
2. El servidor envía sus capacidades (lista de herramientas)
3. Claude Desktop envía requests JSON-RPC por stdin
4. El servidor responde por stdout
5. Los logs de seguridad se escriben por stderr

## Ciclo de vida

- El servidor se conecta a la base de datos al iniciar
- Mantiene el connection pool activo durante toda la sesión
- Se cierra limpiamente cuando Claude Desktop termina la sesión
