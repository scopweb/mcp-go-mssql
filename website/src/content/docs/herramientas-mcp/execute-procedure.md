---
title: execute_procedure
description: Ejecutar un procedimiento almacenado autorizado
---

Ejecuta un procedimiento almacenado que esté incluido en la lista de procedimientos autorizados (whitelist).

## Parámetros

| Nombre | Tipo | Requerido | Descripción |
|--------|------|-----------|-------------|
| `procedure_name` | string | Sí | Nombre del procedimiento a ejecutar |
| `parameters` | string | No | Objeto JSON con nombres y valores de parámetros |

## Configuración requerida

Para usar esta herramienta, debes configurar la variable de entorno:

```bash
MSSQL_WHITELIST_PROCEDURES="sp_GetCustomerOrders,sp_GenerateReport"
```

## Ejemplo de uso

```json
{
  "name": "execute_procedure",
  "arguments": {
    "procedure_name": "sp_GetCustomerOrders",
    "parameters": "{\"customer_id\": 123}"
  }
}
```

## Seguridad

- **Solo ejecuta procedimientos de la whitelist** — Cualquier procedimiento no autorizado es rechazado
- **Validación de nombre** — Los nombres se validan con regex `^[\w.\[\]]+$` para prevenir inyección
- **Procedimientos peligrosos bloqueados** — `xp_cmdshell`, `sp_configure`, `sp_executesql` y otros están explícitamente bloqueados incluso si se añaden a la whitelist
- **Logging de seguridad** — Cada ejecución se registra en los logs de seguridad

## Disponibilidad

Si `MSSQL_WHITELIST_PROCEDURES` no está definida, la herramienta está **completamente deshabilitada** — no se permite ningún procedimiento, incluidos los del sistema. No hay excepciones.

## Restricciones en modo dinámico

- **Alias de solo lectura**: `execute_procedure` está **bloqueado**.
- **Alias escribibles** (`_READ_ONLY=false`): requiere una confirmación previa con `confirm_operation` antes de cada ejecución.
