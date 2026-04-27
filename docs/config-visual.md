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
║    ✅ Skipa CONFIRMAR_DESTRUCTIVA → No necesita confirm_operation        ║
║    ✅ Skipa SCHEMA_VALIDATION → Consulta tablas inexistentes             ║
║    ✅ Skipa CONFIRMAR_DESTRUCTIVA → DROP/ALTER/CREATE libre               ║
║                                                                          ║
║    ❌ MANTIENE READ_ONLY protection → Si MSSQL_READ_ONLY=true,           ║
║       bloquea INSERT/UPDATE/DELETE aunque AUTOPILOT=true                 ║
║                                                                          ║
║    ❌ MANTIENE WHITELIST protection → Solo tablas en whitelist            ║
║       pueden ser modificadas                                             ║
║                                                                          ║
║    ⚠️  AUTOPILOT NO skipea READ_ONLY ni WHITELIST                       ║
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
║                    MODO DESARROLLADOR SEGURO                      ║
║              (la configuración más abierta posible)               ║
╠═══════════════════════════════════════════════════════════════════╣
║                                                                   ║
║   MSSQL_CONNECTION_STRING  →  Solo conexión                      ║
║   DEVELOPER_MODE            →  "true" (errores detallados)         ║
║   MSSQL_AUTOPILOT          →  "true" (skip todo)                  ║
║                                                                   ║
║   NO poner:                                                        ║
║   ❌ MSSQL_READ_ONLY=true                                          ║
║   ❌ MSSQL_WHITELIST_TABLES (vacío = sin límites)                  ║
║   ❌ MSSQL_CONFIRM_DESTRUCTIVE=false                               ║
║                                                                   ║
║   Resultado:                                                      ║
║   → SELECT/INSERT/UPDATE/DELETE ✅                                ║
║   → CREATE/DROP/ALTER ✅                                          ║
║   → Sin confirmación destructiva ✅                                ║
║   → Sin validación de schema ✅                                    ║
║   → Tablas no existentes pueden consultarse ✅                    ║
║   → Todas las tablas modificables ✅                               ║
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
| `MSSQL_AUTOPILOT` | `false` | Skip confirmación + schema | Todo normal |
| `DEVELOPER_MODE` | `false` | Errores detallados + TLS laxo | Errores genéricos + TLS strict |

---

## Ejemplo: Tu Config Actual (GDP Server)

```
MSSQL_CONNECTION_STRING: ...encrypt=disable...
DEVELOPER_MODE: "true"
MSSQL_READ_ONLY: "true"       ← ❌ BLOQUEA modificaciones
MSSQL_WHITELIST_TABLES: "*"  ← ✅ Todas las tablas
MSSQL_AUTOPILOT: "true"       ← ✅ Skip confirmación y schema
```

**Resultado:** AUTOPILOT no puede abrir el candado de READ_ONLY. La protección de solo lectura sigue activa aunque AUTOPILOT=true.

**Para habilitar todo:** quitar `MSSQL_READ_ONLY` o poner `"false"`.