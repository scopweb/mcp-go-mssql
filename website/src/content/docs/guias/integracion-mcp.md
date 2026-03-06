---
title: Integración MCP
description: Detalles de la implementación del protocolo MCP en MCP-Go-MSSQL
---

## Protocolo MCP

MCP (Model Context Protocol) es el protocolo que permite a Claude Desktop comunicarse con servidores externos. MCP-Go-MSSQL implementa MCP sobre JSON-RPC 2.0 usando stdin/stdout.

## Herramientas disponibles

El servidor expone 9 herramientas MCP:

| Herramienta | Descripción |
|-------------|-------------|
| `query_database` | Ejecuta consultas SQL |
| `list_tables` | Lista todas las tablas de la base de datos |
| `describe_table` | Muestra la estructura de una tabla |
| `get_database_info` | Información general de la base de datos |
| `list_databases` | Lista bases de datos disponibles |
| `get_indexes` | Muestra índices de una tabla |
| `get_foreign_keys` | Muestra foreign keys de una tabla |
| `list_stored_procedures` | Lista procedimientos almacenados |
| `execute_procedure` | Ejecuta un procedimiento almacenado |

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
