---
title: Uso con IA
description: Configuración recomendada para usar MCP-Go-MSSQL con asistentes de IA
---

MCP-Go-MSSQL está diseñado para que Claude y otros asistentes de IA trabajen con bases de datos de producción de forma segura.

## Configuración AI-Safe recomendada

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

Esta configuración permite a la IA:
- **Leer** cualquier tabla de la base de datos
- **Escribir** solo en `temp_ai` y `v_temp_ia`
- Todas las demás tablas quedan protegidas contra modificación

## Flujo de trabajo típico

1. La IA consulta datos de producción con `query_database`
2. Procesa y transforma los datos
3. Escribe resultados en tablas temporales del whitelist
4. El usuario revisa y promueve los datos si corresponde

## Tablas temporales para IA

Crea tablas dedicadas para que la IA trabaje:

```sql
CREATE TABLE temp_ai (
    id INT IDENTITY PRIMARY KEY,
    created_at DATETIME DEFAULT GETDATE(),
    data NVARCHAR(MAX)
);
```

Añádelas al whitelist:

```bash
MSSQL_WHITELIST_TABLES=temp_ai
```

## Protección contra errores

El modo read-only + whitelist protege contra:
- Borrado accidental de datos de producción
- Modificación de tablas críticas
- SQL injection que intente acceder a tablas no autorizadas via JOIN
