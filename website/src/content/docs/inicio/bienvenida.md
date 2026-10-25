---
title: Bienvenida
description: Qué es MCP-Go-MSSQL y cómo se usa
---

**MCP-Go-MSSQL** es un servidor MCP y un CLI en Go para Microsoft SQL Server. Sirve a Claude Desktop, Claude Code, OpenCode y cualquier cliente MCP por stdio.

## Qué resuelve

Conectar un agente a una base de producción es arriesgado. Este servidor añade una capa fácil de activar: solo lectura absoluto, whitelist solo si las escrituras están habilitadas, prepared statements y TLS que falla cerrado en producción.

Esa capa **no sustituye** un login de SQL Server con mínimos privilegios. No hace falta cambiar ese usuario para activarla.

## Dos formas de usarlo

1. **Servidor MCP** (`cmd/mcp-go-mssql`) — herramientas en la conversación.
2. **CLI** (`examples/claude-code/db-connector.go`) — terminal. Rechaza escrituras salvo que un operador ponga `MSSQL_CLI_ALLOW_WRITE=true`.

## Qué puede hacer el agente

- Explorar tablas, columnas, índices y claves foráneas
- Consultar con SQL de lectura
- Ver el plan estimado (`explain_query`) y estadísticas de ejecución
- Escribir solo en tablas de una whitelist, y solo si `MSSQL_READ_ONLY=false`
- Ejecutar procedimientos que estén en `MSSQL_WHITELIST_PROCEDURES`

## Requisitos

- Go 1.27.2+ ([descargar](https://go.dev/dl/))
- Microsoft SQL Server accesible (puerto 1433 por defecto)
- En producción, TLS válido. SQL Server 2008/2012 (TLS 1.0) exige un opt-in explícito

Sigue con la [Instalación](/inicio/instalacion/).
