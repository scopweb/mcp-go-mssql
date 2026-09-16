---
title: Uso con IA
description: Configuración recomendada para usar MCP-Go-MSSQL con asistentes de IA
---

MCP-Go-MSSQL está diseñado para que Claude y otros asistentes de IA trabajen con bases de datos de producción de forma segura.

## Configuración AI-Safe recomendada

```bash
MSSQL_READ_ONLY=true
```

Esta configuración permite a la IA:
- **Leer** cualquier tabla de la base de datos
- **Explorar esquema** con `explore`/`inspect` y procedimientos administrativos de solo lectura
- **No escribir, borrar ni alterar** nada. Una whitelist no cambia esto.

Para un espacio de trabajo temporal escribible, usa un alias o instancia aparte con `READ_ONLY=false` y whitelist explícita.

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

El modo solo lectura absoluto protege contra:
- Borrado accidental de datos de producción
- Modificación de tablas críticas
- SQL injection que intente acceder a tablas no autorizadas via JOIN
