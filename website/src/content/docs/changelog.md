---
title: Changelog
description: Historial de cambios de MCP-Go-MSSQL
---

Todos los cambios relevantes de este proyecto se documentan aquí.

## Últimos cambios

### Modo Dynamic Multi-Connection

Cuando `MSSQL_DYNAMIC_MODE=true` está habilitado, el servidor puede conectar a múltiples bases de datos desde una única instancia MCP. Las conexiones se pre-configuran en `.env` y la IA solo ve alias seguros — **sin datos sensibles expuestos**.

**Nuevas variables:**
- `MSSQL_DYNAMIC_MODE` (default: `false`) — Habilita conexiones dinámicas
- `MSSQL_DYNAMIC_MAX_CONNECTIONS` (default: `10`) — Máximo de conexiones activas

**Nuevas herramientas:** `dynamic_connect`, `dynamic_list`, `dynamic_disconnect`

**Configuración de conexiones (`.env`):**
```bash
MSSQL_DYNAMIC_IDENTITY_SERVER=10.203.3.11
MSSQL_DYNAMIC_IDENTITY_DATABASE=JJP_CRM_IDENTITY
MSSQL_DYNAMIC_IDENTITY_USER=ppp
MSSQL_DYNAMIC_IDENTITY_PASSWORD=ppppp
```

**Seguridad por conexión:**
- `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY`
- `MSSQL_DYNAMIC_<ALIAS>_WHITELIST_TABLES`
- `MSSQL_DYNAMIC_<ALIAS>_AUTOPILOT`

**En Claude Desktop** solo necesitas:
```json
{"MSSQL_DYNAMIC_MODE": "true"}
```
(sin credenciales en el JSON)

---

### Confirmación de operaciones destructivas

**Nueva característica:** Sistema de confirmación para operaciones DDL que modifican o destruyen objetos existentes.

**Nuevas variables:**
- `MSSQL_CONFIRM_DESTRUCTIVE` (default: `true`) — Requiere confirmación para `ALTER VIEW`, `DROP TABLE`, etc. en objetos existentes
- `MSSQL_AUTOPILOT` (default: `false`) — Modo autónomo: skip confirmación + skip validación schema. Whitelist sigue activo

**Nueva herramienta:** `confirm_operation` — Confirmar operaciones destructivas pendientes con token.

**Operaciones que requieren confirmación:**

| Operación | Objetivo |
|-----------|----------|
| `ALTER VIEW` | Vista existente |
| `DROP TABLE` | Tabla existente |
| `DROP VIEW` | Vista existente |
| `DROP PROCEDURE` | Procedimiento existente |
| `DROP FUNCTION` | Función existente |
| `ALTER TABLE` | Tabla existente |
| `TRUNCATE TABLE` | Tabla existente |

**Tokens:**
- Generados con `crypto/rand` (32-char hex)
- Válidos 5 minutos
- Un solo uso (se eliminan tras ejecución o expiración)
- Solo para objetos que **ya existen** — `CREATE TABLE new_table` no requiere confirmación

---

### Consultas cross-database (`MSSQL_ALLOWED_DATABASES`)

**Nueva variable:** `MSSQL_ALLOWED_DATABASES`
- Permite consultar múltiples bases de datos desde un solo conector MCP
- Formato: lista separada por comas, ej: `"JJP_Carregues,JJP_Ferratge_PROD"`
- Habilita queries con nombres de 3 partes: `SELECT * FROM OtherDB.dbo.TableName`
- La validación de schema verifica que las tablas existan en la BD destino
- Las modificaciones cross-database están **siempre bloqueadas** (seguridad)

**Mejoras en herramientas:**
- `explore` acepta nuevo parámetro `database` para listar tablas de BDs permitidas
- `get_database_info` muestra las bases de datos cruzadas configuradas
- Mensajes de error claros cuando se referencia una BD no permitida

**Corrección de regex:**
- El parser de nombres de tabla ahora soporta 3 partes (`database.schema.table`)
- Corrige falsos errores "table not found" para referencias cualificadas como `dbo.TableName`

---

### Conformidad con la spec MCP (2025-11-25)

- **Content annotations**: todas las respuestas incluyen campos `audience` y `priority`
- **Tool annotations**: `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` en cada herramienta
- **Rate limiter**: 60 llamadas por minuto (token bucket)
- **JSON-RPC 2.0**: validación estricta, códigos de error apropiados (-32600, -32601, -32602, -32700)
- **`logging/setLevel`** para control dinámico del nivel de log
- **`ping`** para health checks
- **Shutdown limpio** con cierre ordenado de conexiones

---

### Validación de schema para `query_database`

- Antes de ejecutar, valida que todas las tablas/vistas referenciadas existan en la BD
- Parsea referencias de tablas en JOINs, subqueries, CTEs y nombres de 3 partes
- Sugerencias "Did you mean?" con distancia de Levenshtein cuando una tabla no se encuentra
- Omite la validación silenciosamente si `INFORMATION_SCHEMA` no es accesible
- Excluye automáticamente objetos de esquemas del sistema (`INFORMATION_SCHEMA`, `sys`)

---

### Soporte SQL Server 2008/2012 y diagnóstico mejorado

**Nueva variable:** `MSSQL_ENCRYPT`
- Controla el cifrado TLS de forma independiente en modo desarrollo
- `MSSQL_ENCRYPT=false` es **necesario para SQL Server 2008/2012** que no soportan TLS 1.2
- Solo efectivo con `DEVELOPER_MODE=true`. En producción el cifrado es siempre obligatorio

**Correcciones de conexión:**
- Añadido `port` a la connection string de autenticación integrada (antes se omitía)
- Corregido `encrypt=true` hardcodeado en los conectores CLI y pkg
- `MSSQL_DATABASE` ahora es opcional para autenticación integrada en todos los conectores

**Diagnóstico mejorado para Claude:**
- `get_database_info` cuando no hay conexión muestra: configuración completa + causas posibles + soluciones específicas
- Todos los errores "Database not connected" guían a Claude a usar `get_database_info` para diagnóstico
- Errores de query en producción incluyen sugerencias de acción (verificar sintaxis, permisos, usar `explore`)

---

### `inspect` — nuevo `detail=dependencies`

- Muestra qué objetos SQL (vistas, procedimientos, funciones) **dependen de una tabla** dada
- Usa `sys.sql_expression_dependencies` para análisis de impacto
- Devuelve: `referencing_schema`, `referencing_object`, `referencing_type`
- También incluido en `detail=all`
- Útil para evaluar el impacto antes de cambiar el esquema de una tabla

---

### `explore` — nuevo `type=views`

- Lista solo las **vistas** de la base de datos con metadatos enriquecidos: `schema_name`, `view_name`, `check_option`, `is_updatable`, `definition_preview` (300 chars)
- Soporta parámetro `filter` para filtrar por nombre (LIKE)
- Complementa `type=tables` que sigue devolviendo tablas y vistas mezcladas

---

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
