---
title: Auditoría y logging
description: Sistema de logging de seguridad y auditoría de MCP-Go-MSSQL
---

MCP-Go-MSSQL incluye un sistema dedicado de logging de seguridad que registra eventos relevantes sin exponer datos sensibles.

## SecurityLogger

El componente `SecurityLogger` se encarga de registrar todos los eventos de seguridad con sanitización automática.

### Eventos registrados

- Intentos de conexión a la base de datos (éxito y fallo)
- Queries bloqueadas por modo solo lectura
- Acceso denegado a tablas fuera del whitelist
- Intentos de SQL injection detectados
- Errores de validación de entrada

### Sanitización automática

El logger elimina automáticamente datos sensibles antes de escribir en disco:

- Contraseñas y tokens
- Connection strings completos
- Datos de usuario en queries

## Formato de logs

Los logs de seguridad se escriben en formato estructurado con los siguientes campos:

| Campo | Descripción |
|-------|-------------|
| `timestamp` | Fecha y hora UTC del evento |
| `level` | Nivel: INFO, WARN, ERROR, SECURITY |
| `event` | Tipo de evento de seguridad |
| `source` | Componente que generó el evento |
| `message` | Descripción sanitizada del evento |

## Configuración

El logging de seguridad está habilitado por defecto y no se puede desactivar. Los mensajes de error al cliente son siempre genéricos en modo producción (`DEVELOPER_MODE=false`), mientras que los detalles técnicos se registran internamente.

## Mejores prácticas

1. Revisar logs de seguridad periódicamente
2. Configurar alertas para eventos de nivel SECURITY
3. Rotar y archivar logs según políticas de retención
4. No exponer archivos de log a usuarios no autorizados
