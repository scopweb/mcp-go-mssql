---
title: FAQ de conexiones dinámicas
description: Reglas cortas para usar un solo servidor MCP con varias bases relacionadas
---

Usa el modo dinámico cuando una aplicación necesita varias bases. La guía completa está en [Modo dinámico](/guias/modo-dinamico-una-aplicacion-varias-bases/).

## Reglas

- Cada alias nace en `READ_ONLY=true`.
- No pongas `MSSQL_READ_ONLY=false` global en modo dinámico.
- Un alias con `_READ_ONLY=false` y sin `_WHITELIST_TABLES` no puede modificar nada.
- La whitelist solo aplica con `_READ_ONLY=false`. Las modificaciones piden `confirm_operation` antes.
- Un proceso stdio por agente. El alias activo no aísla agentes entre sí.
- El control real sigue siendo el login de SQL Server. Esta capa no lo sustituye.

## Dónde va la configuración

| Cliente | Dónde | Variables |
|---------|--------|-----------|
| Claude Desktop | `mcpServers.*.env` | las mismas `MSSQL_DYNAMIC_*` |
| OpenCode | `mcp.*.environment` en `opencode.json` | las mismas. Ver [OpenCode](/configuracion/opencode/) |

El servidor también lee un `.env` junto al ejecutable. Las variables del host tienen prioridad.

## Comprobar

1. Arranca el servidor con `MSSQL_DYNAMIC_MODE=true`.
2. Llama a `dynamic_available`.
3. `dynamic_connect` al alias de lectura.
4. Una escritura en un alias de solo lectura debe rechazarse.
