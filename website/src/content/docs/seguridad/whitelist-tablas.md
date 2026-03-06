---
title: Whitelist de tablas
description: Sistema de permisos granulares para control de acceso a tablas
---

El sistema de whitelist permite modificar tablas específicas incluso en modo solo lectura, ideal para dar a asistentes de IA un espacio de trabajo temporal.

## Problema que resuelve

Cuando se usan asistentes de IA con bases de datos de producción, existe riesgo de:
- Eliminación accidental de datos
- Exfiltración de datos con queries maliciosas como `DELETE temp_ai FROM temp_ai JOIN production_table`
- Acceso no autorizado a tablas sensibles via JOINs o subqueries

## Configuración

```bash
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

## Flujo de validación

1. El usuario ejecuta una consulta
2. Validación básica de entrada
3. Verificación de modo solo lectura
4. Extracción del tipo de operación (INSERT/UPDATE/DELETE/etc.)
5. Extracción de **todas** las tablas referenciadas (FROM, JOIN, subqueries, CTEs)
6. Validación de que **todas** las tablas estén en la whitelist
7. Ejecución o bloqueo con error

## Detección multi-tabla

El parser detecta tablas en:
- Cláusulas `FROM`
- Operaciones `JOIN` (INNER, LEFT, RIGHT, FULL)
- Subqueries: `SELECT * FROM (SELECT * FROM tabla)`
- `INSERT INTO ... SELECT ... FROM`
- `UPDATE ... SET col = (SELECT ... FROM)`
- `DELETE ... FROM ... JOIN`
- CTEs: `WITH cte AS (SELECT * FROM tabla)`

## Ejemplos

### Consultas permitidas

```sql
-- SELECT siempre permitido (solo lectura)
SELECT * FROM production_table
SELECT * FROM production_table JOIN temp_ai ON ...

-- Modificaciones en tablas de la whitelist
UPDATE temp_ai SET col = 'value' WHERE id = 1
DELETE FROM temp_ai WHERE id = 1
INSERT INTO temp_ai VALUES (1, 'test')
```

### Consultas bloqueadas

```sql
-- Modificación de tabla no autorizada
UPDATE users SET password = 'hacked'
-- Error: permission denied: table 'users' is not whitelisted

-- JOIN con tabla no autorizada en modificación
DELETE temp_ai FROM temp_ai JOIN users ON temp_ai.id = users.id
-- Error: permission denied: table 'users' is not whitelisted

-- Subquery a datos sensibles
UPDATE temp_ai SET data = (SELECT password FROM users WHERE id = 1)
-- Error: permission denied: table 'users' is not whitelisted

-- INSERT desde tabla no autorizada
INSERT INTO temp_ai SELECT * FROM customers
-- Error: permission denied: table 'customers' is not whitelisted
```

## Logs de seguridad

Cada verificación de permisos se registra:

```
[SECURITY] Permission check - Operation: DELETE, Tables found: [temp_ai users], Whitelist: [temp_ai]
[SECURITY] SECURITY VIOLATION: Attempted DELETE on non-whitelisted table 'users'
```

## Recomendaciones para IA

### Crear tablas temporales dedicadas

```sql
CREATE TABLE temp_ai (
    id INT IDENTITY(1,1) PRIMARY KEY,
    operation_type VARCHAR(50),
    data NVARCHAR(MAX),
    created_at DATETIME DEFAULT GETDATE(),
    result NVARCHAR(MAX)
);
```

### Automatizar limpieza

```sql
CREATE PROCEDURE CleanupTempAI
AS
BEGIN
    DELETE FROM temp_ai
    WHERE created_at < DATEADD(day, -7, GETDATE());
END;
```

## Limitaciones

El parser basado en regex puede no detectar tablas en:
- Queries altamente ofuscadas con comentarios anidados
- SQL dinámico dentro de procedimientos almacenados
- CTEs con múltiples niveles de anidamiento

**Mitigación:** Para máxima seguridad, combina con permisos a nivel de base de datos (GRANT/DENY).
