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

-- Ejecución de código
EXEC sp_help                  -- Bloqueado (excepto procedimientos seguros)
EXEC xp_cmdshell 'dir'       -- Siempre bloqueado
```

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

Consulta la sección [Whitelist de tablas](/seguridad/whitelist-tablas/) para más detalles.
