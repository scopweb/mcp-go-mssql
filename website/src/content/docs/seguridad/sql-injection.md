---
title: Protección contra SQL Injection
description: Cómo MCP-Go-MSSQL previene ataques de inyección SQL
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

## Protecciones adicionales

### Bloqueo de comandos peligrosos

En modo solo lectura, se bloquean:
- `EXEC` / `EXECUTE`
- `SP_` / `XP_` (procedimientos del sistema peligrosos)
- `OPENROWSET` / `OPENDATASOURCE`
- `BULK INSERT`
- `RECONFIGURE`

### Validación de entrada

- Límite de tamaño de consulta (1MB por defecto, configurable)
- Rechazo de entrada vacía
- Eliminación de comentarios que podrían ocultar comandos

## Tests de seguridad

```bash
# Ejecutar suite de tests de SQL injection
go test -v -run TestSQLInjectionVulnerability ./test/security/...
```

Los tests cubren 6 vectores de ataque diferentes, todos bloqueados exitosamente.
