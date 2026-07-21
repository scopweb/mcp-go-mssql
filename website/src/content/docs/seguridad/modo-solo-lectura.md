---
title: Modo solo lectura
description: Restricción de acceso a solo consultas SELECT
---

El modo solo lectura bloquea todas las operaciones de modificación, permitiendo únicamente consultas SELECT.

## Configuración

```bash
# Activar modo solo lectura
MSSQL_READ_ONLY=true
```

## Comportamiento

### Operaciones permitidas

```sql
-- Todas las consultas SELECT
SELECT * FROM users
SELECT u.*, o.total FROM users u JOIN orders o ON u.id = o.user_id

-- Subconsultas
SELECT * FROM (SELECT id, name FROM users) sub

-- CTEs
WITH active AS (SELECT * FROM users WHERE active = 1)
SELECT * FROM active

-- Agregaciones y funciones de ventana
SELECT department, AVG(salary) FROM employees GROUP BY department
```

### Operaciones bloqueadas

```sql
-- Modificaciones de datos
INSERT INTO users VALUES (1, 'test')        -- Bloqueado
UPDATE users SET name = 'nuevo' WHERE id = 1 -- Bloqueado
DELETE FROM users WHERE id = 1               -- Bloqueado

-- DDL
CREATE TABLE temp (id INT)    -- Bloqueado
DROP TABLE users              -- Bloqueado
ALTER TABLE users ADD col INT -- Bloqueado

-- Ejecución de código peligrosa
EXEC xp_cmdshell 'dir'       -- Siempre bloqueado
EXEC sp_executesql '...'     -- Siempre bloqueado
EXEC sp_configure ...        -- Siempre bloqueado
```

### Lecturas administrativas y de esquema permitidas

Aunque el modo es de solo lectura, se permiten ciertas operaciones de **administración e introspección de esquema** que son inherentemente de solo lectura. Esto facilita el descubrimiento de la base de datos por parte de herramientas y asistentes de IA:

```sql
-- Procedimientos de sistema seguros para exploración de esquema
EXEC sp_help 'dbo.Usuarios'           -- Estructura de una tabla
EXEC sp_helptext 'dbo.MiProcedimiento' -- Código fuente de un objeto
EXEC sp_spaceused 'dbo.Pedidos'       -- Tamaño y uso de espacio
EXEC sp_columns @table_name = 'Clientes'
EXEC sp_fkeys 'Pedidos'
```

Estos procedimientos están explícitamente permitidos porque no modifican datos ni configuración. Cualquier otro procedimiento del sistema (o EXEC de SQL dinámico) sigue estando bloqueado. 

Se recomienda preferentemente el uso de las herramientas dedicadas `explore` e `inspect` para la mayoría de los casos de descubrimiento de esquema.

## ¿Qué pasa con las consultas multi-statement?

Por defecto, **cualquier consulta con más de una sentencia** sigue siendo rechazada en modo solo lectura con el error `multiple SQL statements are not allowed in a single request`. La única excepción permitida es el envoltorio `SET STATISTICS TIME/IO ON/OFF` alrededor de una única sentencia base, pensado para capturar tiempos de ejecución y lecturas lógicas/físicas sin salir del modo solo lectura. Ver la guía [SET STATISTICS en modo solo lectura](../herramientas-mcp/statistics.md) para los detalles, el formato de respuesta y los casos rechazados.

## Validación de consultas

La validación usa expresiones regulares con word boundaries (`\bINSERT\b`, `\bUPDATE\b`, etc.) para evitar falsos positivos. Por ejemplo:

```sql
-- Permitido (no contiene la palabra INSERT como operación)
SELECT created_at FROM transactions

-- Permitido (update_count es un nombre de columna, no una operación)
SELECT update_count FROM statistics

-- Bloqueado (contiene la operación UPDATE)
UPDATE users SET status = 'active'
```

## Combinación con whitelist

Para permitir modificaciones solo en tablas específicas, combina con `MSSQL_WHITELIST_TABLES`:

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

**Coincidencia cualificada por esquema:** las entradas de la whitelist se comparan cualificadas por esquema. Un nombre sin esquema (`temp_ai`) se asume en `dbo`, por lo que coincide con `temp_ai` y `dbo.temp_ai`, pero **no** con `otro_schema.temp_ai` ni con `otradb.dbo.temp_ai`. Para permitir un objeto fuera de `dbo`, cualifícalo explícitamente (p. ej. `app.temp_ai`).

**Defensa en profundidad:** la validación con regex es una capa adicional, no la frontera de seguridad real. El control de escritura más fuerte debe vivir en la base de datos: usa un login SQL de mínimo privilegio con `SELECT` amplio y `INSERT`/`UPDATE`/`DELETE` solo en las tablas que la IA pueda modificar. Así, aunque el validador falle, el propio motor rechaza la escritura.

Consulta la sección [Whitelist de tablas](/seguridad/whitelist-tablas/) para más detalles.
