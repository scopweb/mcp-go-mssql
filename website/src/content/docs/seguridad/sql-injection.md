---
title: Protección contra SQL Injection
description: Cómo MCP-Go-MSSQL previene ataques de inyección SQL, incluyendo técnicas assistidas por IA
---

La protección contra SQL injection es absoluta gracias al uso exclusivo de prepared statements.

## Mecanismo de protección

```go
// Todas las queries usan PrepareContext()
stmt, err := s.db.PrepareContext(ctx, query)
defer stmt.Close()
rows, err := stmt.QueryContext(ctx, args...)
```

### Defensa en capas

1. **Prepared statements obligatorios** — Todas las queries usan `PrepareContext()`
2. **Separación de código y datos** — Los parámetros se pasan como argumentos separados
3. **Sin concatenación de strings SQL** — El driver go-mssqldb maneja el escaping automáticamente

## Ejemplo de ataque bloqueado

```sql
-- Intento de inyección:
SELECT * FROM users WHERE username = '1' OR '1'='1' --

-- Con prepared statements, se trata como literal:
SELECT * FROM users WHERE username = '1'' OR ''1''=''1'' --'
```

## Protecciones adicionales contra IA

Además de los prepared statements, el servidor implementa validaciones específicas para técnicas que una IA puede usar para evadir detección:

### Bloqueo de comandos peligrosos

En modo solo lectura, se bloquean:
- `EXEC` / `EXECUTE`
- `SP_` / `XP_` (procedimientos del sistema peligrosos)
- `OPENROWSET` / `OPENDATASOURCE` — previene exfiltración de datos a servidores externos
- `BULK INSERT`
- `RECONFIGURE`

### Detección de concatenación CHAR()

Previene que una IA construya keywords SQL dinámicamente:

```sql
-- Bloqueado:
CHAR(83)+CHAR(69)+CHAR(76)+CHAR(69)+CHAR(67)+CHAR(84) * FROM users
```

### Detección de comentarios inline

Previene que keywords sean ocultados dentro de comentarios:

```sql
-- Bloqueado:
SEL/*x*/ECT * FROM users
/*INS*/ INSERT INTO users VALUES (1)
```

### Detección de table hints peligrosos

Previene dirty reads y otros comportamientos no estándar:

```sql
-- Bloqueado:
SELECT * FROM users WITH (NOLOCK)
SELECT * FROM users WITH (READUNCOMMITTED)
SELECT * FROM users WITH (TABLOCK)
```

### Detección de WAITFOR

Previene timing attacks donde una IA infiere datos midiendo delays:

```sql
-- Bloqueado:
IF (SELECT COUNT(*) FROM users) > 0 WAITFOR DELAY '00:00:05'
```

### Detección de caracteres Unicode de control

Previene obfuscación mediante caracteres bidireccionales y zero-width spaces:

```sql
-- Bloqueado (RTL Override):
SELECT\u202E * FROM users

-- Bloqueado (Zero-width space):
SEL\u200BECT * FROM users
```

### Detección de homoglyphs Unicode

Previene que caracteres Cyrillic/Greek sean usados para imitar letters Latinas:

```sql
-- Bloqueado (Cyrillic 'е' = Latin 'e'):
SEL\u0435CT * FROM users
```

### Validación de subqueries contra whitelist

Previene acceso a tablas restringidas a través de subqueries anidadas:

```sql
-- Bloqueado si "secretos" no está en whitelist:
SELECT * FROM (SELECT secret FROM secretos) AS x
```

## Validación de entrada

- Límite de tamaño de consulta (1MB por defecto, configurable)
- Rechazo de entrada vacía
- Preservación de strings literales — el contenido de `'...'` se excluye del pattern matching para evitar falsos positivos

## Tests de seguridad

```bash
# Ejecutar suite de tests de SQL injection y ataques IA
go test -v -run TestSQLInjectionVulnerability ./test/security/...
go test -v -run TestAIAttackVectors ./test/security/...

# Verificación de vulnerabilidades
govulncheck ./...
```

Los tests cubren 6+ vectores de ataque tradicionales y 20 vectores específicos de IA, todos bloqueados exitosamente.
