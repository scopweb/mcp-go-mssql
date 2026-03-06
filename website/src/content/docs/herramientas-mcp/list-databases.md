---
title: list_databases
description: Listar todas las bases de datos del servidor
---

:::caution[Herramienta reemplazada en v2]
Esta herramienta fue **fusionada** en [`explore`](/herramientas-mcp/explore/) como parte de la consolidación de la API en la versión 2.

Usa `explore (type=databases)` para obtener el mismo resultado.
:::


Lista todas las bases de datos de usuario disponibles en la instancia de SQL Server.

## Parámetros

Esta herramienta no requiere parámetros.

## Ejemplo de uso

```json
{
  "name": "list_databases",
  "arguments": {}
}
```

## Respuesta

Devuelve la lista de bases de datos excluyendo las del sistema (`master`, `tempdb`, `model`, `msdb`).

## Notas

- Especialmente útil con autenticación Windows (SSPI) sin base de datos específica
- Permite a Claude explorar qué bases de datos están disponibles
- Requiere permisos de `VIEW ANY DATABASE` o equivalente
