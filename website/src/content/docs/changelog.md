---
title: Changelog
description: Historial de cambios de MCP-Go-MSSQL
---

Todos los cambios relevantes de este proyecto se documentan aquí.

## Último cambio

### Nueva herramienta: `explain_query`

- Muestra el **plan de ejecución estimado** de un query SELECT sin ejecutarlo
- Usa `SET SHOWPLAN_TEXT ON` en una conexión dedicada (aislada del pool)
- Solo acepta SELECT — validación siempre activa, independientemente de `MSSQL_READ_ONLY`
- Útil para análisis de rendimiento de queries con Claude

---

### Actualización de dependencias (2026-03-06)

**Dependencias actualizadas:**
- `github.com/microsoft/go-mssqldb` v1.9.4 → **v1.9.8** (correcciones del driver)
- `golang.org/x/crypto` v0.45.0 → **v0.48.0** (parches de seguridad)
- `golang.org/x/text` v0.31.0 → **v0.34.0**
- `github.com/golang-jwt/jwt/v5` v5.3.0 → **v5.3.1**
- Nueva dep transitiva: `github.com/shopspring/decimal v1.4.0` (precisión decimal en go-mssqldb v1.9.8)

**Auditoría:** `govulncheck ./...` → Sin vulnerabilidades detectadas

---

### Documentación y estabilidad

**Nuevas funcionalidades:**
- Sitio de documentación completo con Starlight (ES + EN)
- Guía de actualización de Go
- Roadmap de integración MCP
- Tema visual scopweb con modo oscuro/claro

**Correcciones:**
- Resolución de race condition en el pool de conexiones
- Eliminación de falsos positivos en la validación de modo solo lectura
- Corrección de errores de compilación en la suite de tests

**Seguridad:**
- Cifrado TLS obligatorio en todas las conexiones de producción
- Protección SQL injection con prepared statements exclusivos
- Modo solo lectura con whitelist granular de tablas
- Validación multi-tabla que cubre JOINs, subqueries y CTEs
- Logging de seguridad con sanitización automática de credenciales

**Infraestructura:**
- Licencia MIT añadida
- Scripts de build con salida consistente en directorio `build/`
- Referencias internas sanitizadas para publicación

## Versiones anteriores

### Primer release

- 9 herramientas MCP: query_database, list_tables, describe_table, get_database_info, list_databases, get_indexes, get_foreign_keys, list_stored_procedures, execute_procedure
- Servidor MCP compatible con Claude Desktop via JSON-RPC 2.0
- CLI para Claude Code con comandos test, info, tables, describe, query
- Soporte para autenticación SQL Server, Windows Integrated (SSPI) y Azure AD
- Connection strings personalizados para configuraciones especiales
- Modo desarrollo con certificados autofirmados y errores detallados
