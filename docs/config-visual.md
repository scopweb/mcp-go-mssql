# MCP MSSQL Server — Configuración Visual

## Modos de Operación

```
╔══════════════════════════════════════════════════════════════════════════╗
║                          PRODUCTION MODE                                  ║
║                      (DEVELOPER_MODE=false)                               ║
╠══════════════════════════════════════════════════════════════════════════╣
║  MSSQL_READ_ONLY=true                     │  MSSQL_READ_ONLY=false        ║
║  ─────────────────────────────────────    │  ──────────────────────────   ║
║  SELECT ✅ | INSERT/UPDATE/DELETE ❌       │  SELECT ✅ | Modificar ✅      ║
║  (excepto en WHITELIST tables)            │  (excepto en WHITELIST tables)║
╠══════════════════════════════════════════════════════════════════════════╣
║  CONFIRM_DESTRUCTIVE=true (default)       │  CONFIRM_DESTRUCTIVE=false    ║
║  ─────────────────────────────────────     │  ──────────────────────────  ║
║  DROP/ALTER/CREATE necesita confirmación  │  DROP/ALTER/CREATE libre      ║
║  (token expires 5 min)                    │  (para CI/CD)                 ║
╠══════════════════════════════════════════════════════════════════════════╣
║  SCHEMA_VALIDATION=true (default)          │  SKIP_SCHEMA_VALIDATION=true ║
║  ─────────────────────────────────────     │  ──────────────────────────  ║
║  Valida que tablas existan antes de       │  Puede consultar tablas      ║
║  ejecutar query                            │  que no existen               ║
╚══════════════════════════════════════════════════════════════════════════╝

╔══════════════════════════════════════════════════════════════════════════╗
║                        AUTOPILOT MODE                                     ║
║                  (MSSQL_AUTOPILOT=true)                                   ║
╠══════════════════════════════════════════════════════════════════════════╣
║                                                                          ║
║    ✅ Skipa SCHEMA_VALIDATION → Consulta tablas inexistentes             ║
║       (equivalente a SKIP_SCHEMA_VALIDATION=true)                        ║
║                                                                          ║
║    ❌ NO skipea CONFIRMAR_DESTRUCTIVA → DROP/ALTER/CREATE sobre objetos  ║
║       existentes siguen requiriendo confirm_operation                    ║
║                                                                          ║
║    ❌ MANTIENE READ_ONLY protection → Si MSSQL_READ_ONLY=true,           ║
║       bloquea INSERT/UPDATE/DELETE aunque AUTOPILOT=true                 ║
║                                                                          ║
║    ❌ MANTIENE WHITELIST protection → Solo tablas en whitelist            ║
║       pueden ser modificadas                                             ║
║                                                                          ║
║    ⚠️  Effective skip de schema = AUTOPILOT OR SKIP_SCHEMA_VALIDATION   ║
║                                                                          ║
╚══════════════════════════════════════════════════════════════════════════╝
```

---

## Esquema de Seguridad (lo que SE BLOQUEA)

```
┌─────────────────────────────────────────────────────────────────┐
│                    READ ONLY MODE                               │
│              (MSSQL_READ_ONLY=true)                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   SELECT      ✅ Permitido                                       │
│   INSERT      ❌ Bloqueado (excepto en WHITELIST)               │
│   UPDATE      ❌ Bloqueado (excepto en WHITELIST)               │
│   DELETE      ❌ Bloqueado (excepto en WHITELIST)               │
│   CREATE      ❌ Bloqueado                                       │
│   DROP        ❌ Bloqueado                                       │
│   ALTER       ❌ Bloqueado                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Esquema de Protección WHITELIST

```
┌─────────────────────────────────────────────────────────────────┐
│              WHITELIST TABLES                                   │
│        (MSSQL_WHITELIST_TABLES=tabla1,tabla2)                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Tablas en whitelist:                                           │
│   → INSERT ✅                                                    │
│   → UPDATE ✅                                                    │
│   → DELETE ✅                                                    │
│   → CREATE/DROP/ALTER ✅                                        │
│                                                                 │
│   Tablas NO en whitelist:                                       │
│   → SELECT ✅ (siempre permitido)                              │
│   → INSERT ❌                                                   │
│   → UPDATE ❌                                                   │
│   → DELETE ❌                                                   │
│   → CREATE/DROP/ALTER ❌                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Configuración Mínima para Acceso TOTAL

```
╔═══════════════════════════════════════════════════════════════════╗
║                    MODO DESARROLLADOR ABIERTO                     ║
║              (la configuración más permisiva)                     ║
╠═══════════════════════════════════════════════════════════════════╣
║                                                                   ║
║   MSSQL_CONNECTION_STRING       →  Solo conexión                  ║
║   DEVELOPER_MODE                →  "true" (errores detallados)    ║
║   MSSQL_AUTOPILOT               →  "true" (skip schema)           ║
║   MSSQL_CONFIRM_DESTRUCTIVE     →  "false" (sin confirmación)    ║
║                                                                   ║
║   NO poner:                                                        ║
║   ❌ MSSQL_READ_ONLY=true                                          ║
║   ❌ MSSQL_WHITELIST_TABLES (vacío = sin límites de escritura)    ║
║                                                                   ║
║   Resultado:                                                      ║
║   → SELECT/INSERT/UPDATE/DELETE ✅                                ║
║   → CREATE/DROP/ALTER ✅                                          ║
║   → Sin confirmación destructiva ✅ (por CONFIRM_DESTRUCTIVE=false)║
║   → Sin validación de schema ✅ (por AUTOPILOT)                    ║
║   → Tablas no existentes pueden consultarse ✅                    ║
║   → Todas las tablas modificables ✅                               ║
║                                                                   ║
║   ⚠️  Nota: AUTOPILOT por sí solo NO desactiva la confirmación    ║
║       destructiva. Para esquivarla hace falta también             ║
║       MSSQL_CONFIRM_DESTRUCTIVE=false.                             ║
║                                                                   ║
╚═══════════════════════════════════════════════════════════════════╝
```

---

## Resumen de Variables

| Variable | Default | True = | False = |
|----------|---------|--------|---------|
| `MSSQL_READ_ONLY` | `false` | Solo SELECT permitido | Todo permitido (excepto whitelist) |
| `MSSQL_WHITELIST_TABLES` | _(ninguna)_ | Solo esas tablas = modificables | Todas las tablas = modificables |
| `MSSQL_CONFIRM_DESTRUCTIVE` | `true` | DROP/ALTER necesitan confirmar | Sin confirmación |
| `MSSQL_SKIP_SCHEMA_VALIDATION` | `false` | Puede consultar tablas inexistentes | Valida tablas existen |
| `MSSQL_AUTOPILOT` | `false` | Skip schema validation (NO skipea confirmación) | Schema validado |
| `DEVELOPER_MODE` | `false` | Errores detallados + TLS laxo | Errores genéricos + TLS strict |

---

## Ejemplo: Config con READ_ONLY + WHITELIST=* + AUTOPILOT

```
MSSQL_CONNECTION_STRING: ...encrypt=disable...
DEVELOPER_MODE: "true"
MSSQL_READ_ONLY: "true"        ← Activa modo read-only
MSSQL_WHITELIST_TABLES: "*"    ← Wildcard: todas las tablas modificables
MSSQL_AUTOPILOT: "true"        ← Skip schema validation (NO skipea confirmación)
```

**Resultado real**:
- SELECT en cualquier tabla ✅
- INSERT/UPDATE/DELETE en cualquier tabla ✅ (gracias al wildcard `*`)
- DROP/ALTER/TRUNCATE en objetos existentes → **siguen requiriendo `confirm_operation`**
- Validación de existencia de tablas saltada (la AI puede consultar tablas inexistentes sin error)

**Si quieres saltar también la confirmación destructiva**, añade `MSSQL_CONFIRM_DESTRUCTIVE=false`.

**Si quieres bloquear escrituras de nuevo**, quita `MSSQL_WHITELIST_TABLES` o cámbialo a una lista concreta.