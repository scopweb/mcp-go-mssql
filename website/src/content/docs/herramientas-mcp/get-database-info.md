---
title: get_database_info
description: Obtener estado de conexión e información de la base de datos
---

Devuelve información sobre el estado de la conexión, modo de acceso y configuración de seguridad.

## Parámetros

Esta herramienta no requiere parámetros.

## Ejemplo de uso

```json
{
  "name": "get_database_info",
  "arguments": {}
}
```

## Respuesta

```json
{
  "status": "connected",
  "server": "mi-servidor.database.windows.net",
  "database": "MiBaseDeDatos",
  "read_only": true,
  "whitelist_tables": ["temp_ai", "v_temp_ia"],
  "encryption": "enabled",
  "developer_mode": false
}
```

## Uso típico

Claude suele invocar esta herramienta automáticamente al inicio de una conversación para entender:
- Si la base de datos está conectada
- Qué modo de acceso tiene (lectura/escritura)
- Qué tablas puede modificar
- Qué bases de datos cruzadas tiene acceso (si `MSSQL_ALLOWED_DATABASES` está configurado)
