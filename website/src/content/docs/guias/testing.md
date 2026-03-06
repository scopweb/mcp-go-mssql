---
title: Testing
description: Guía de testing para MCP-Go-MSSQL
---

## Test de conexión

```bash
cd test
go run test-connection.go
```

Este test verifica conectividad, autenticación y cifrado TLS.

## Tests de seguridad

```bash
go test -v -run TestSQLInjectionVulnerability ./test/security/...
```

La suite de seguridad cubre 6 vectores de ataque de SQL injection.

## CLI de Claude Code

Usa la herramienta CLI para probar operaciones:

```bash
# Test de conexión
go run claude-code/db-connector.go test

# Información de la base de datos
go run claude-code/db-connector.go info

# Listar tablas
go run claude-code/db-connector.go tables

# Describir una tabla
go run claude-code/db-connector.go describe users

# Ejecutar una query
go run claude-code/db-connector.go query "SELECT @@VERSION"
```

## Tests manuales

### Verificar modo solo lectura

Con `MSSQL_READ_ONLY=true`, confirma que las queries de escritura se bloquean:

```bash
go run claude-code/db-connector.go query "INSERT INTO some_table VALUES (1)"
# Debe devolver: Query blocked: read-only mode
```

### Verificar whitelist

Con `MSSQL_WHITELIST_TABLES=temp_ai`, confirma que solo esa tabla acepta escritura:

```bash
go run claude-code/db-connector.go query "INSERT INTO temp_ai (data) VALUES ('test')"
# Debe ejecutar correctamente
```

## Entorno de test

Usa siempre una base de datos separada para testing. Nunca ejecutes tests contra producción.
