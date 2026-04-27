# Configuration Matrix

Documento canónico de qué controla cada flag de configuración en `mcp-go-mssql`, qué NO controla, y cómo interactúan entre sí. Cada afirmación está respaldada por el código en `main.go`. Si encuentras divergencia entre este documento y el código, **el código es la verdad** y este documento debe corregirse.

---

## Tabla rápida

| Flag | Default | Capa que controla | Una línea |
|---|---|---|---|
| `DEVELOPER_MODE` | `false` | TLS + verbosidad de errores | Modo dev: TLS laxo, errores detallados. Modo prod: TLS estricto, errores genéricos. |
| `MSSQL_ENCRYPT` | (vacío) | Cifrado de conexión | Solo efectivo en `DEVELOPER_MODE=true`. Permite SQL Server 2008/2012 sin TLS 1.2. |
| `MSSQL_READ_ONLY` | `false` | Operaciones SQL permitidas | Solo SELECT/WITH/SHOW/DESCRIBE/EXPLAIN; modificaciones bloqueadas salvo whitelist. |
| `MSSQL_WHITELIST_TABLES` | (vacío) | Excepción a `READ_ONLY` | Tablas listadas son modificables. **Solo aplica si `READ_ONLY=true`**. `*` = todas. |
| `MSSQL_CONFIRM_DESTRUCTIVE` | `true` | DDL destructivo sobre objetos existentes | Requiere `confirm_operation` con token (TTL 5 min). |
| `MSSQL_AUTOPILOT` | `false` | Validación de existencia de tablas | Salta `validateTablesExist`. **No** salta confirmación destructiva ni read-only. |
| `MSSQL_SKIP_SCHEMA_VALIDATION` | `false` | Validación de existencia de tablas | Idéntico efecto a AUTOPILOT, flag independiente. Skip efectivo = `AUTOPILOT OR SKIP_SCHEMA_VALIDATION`. |
| `MSSQL_ALLOWED_DATABASES` | (vacío) | Acceso a otras DBs del mismo servidor | Permite SELECT sobre `[OtherDB].schema.table`. **Modificaciones cross-DB siempre bloqueadas.** |
| `MSSQL_CONFIRM_DESTRUCTIVE` (override en CI) | `"false"` | Idem | Quita la barrera de confirmación. Pensado para automatización. |

---

## Detalle por flag

### `DEVELOPER_MODE`

| | |
|---|---|
| **Activa** | TLS laxo (`trustservercertificate=true`), error verbose en respuestas, permite override de `MSSQL_ENCRYPT`. |
| **NO afecta** | READ_ONLY, WHITELIST, CONFIRM_DESTRUCTIVE, schema validation, logs de seguridad, timeouts. |
| **Interacciones** | Si `DEVELOPER_MODE=true` y `MSSQL_ENCRYPT` no está seteado → fuerza `encrypt=false`. En producción, `MSSQL_ENCRYPT=false` se ignora silenciosamente y siempre se usa `encrypt=true`. |
| **Anti-patrón** | Activarlo en producción para esquivar errores de TLS. La protección TLS es la única que `DEVELOPER_MODE` desactiva — no hay nada más que ganar y se pierde la única capa de transporte. |

### `MSSQL_READ_ONLY`

| | |
|---|---|
| **Activa** | Solo se permiten queries cuyo prefix sea `SELECT`, `WITH`, `SHOW`, `DESCRIBE` o `EXPLAIN`. Resto rechazado salvo si la tabla está en `WHITELIST_TABLES`. |
| **NO afecta** | Procedimientos peligrosos del sistema (`xp_cmdshell`, `sp_executesql`, `sp_OAcreate`, etc.) — esos están **siempre** bloqueados, independientes de READ_ONLY. Igual con `OPENROWSET`, `OPENDATASOURCE`, `SELECT INTO`. |
| **Interacciones** | Sin `WHITELIST_TABLES` → ninguna escritura. Con `WHITELIST_TABLES` → escritura solo en las tablas listadas, validada por análisis multi-tabla (los JOINs hacia tablas no whitelisted bloquean la query entera). |
| **Anti-patrón** | Confiar en READ_ONLY como única protección sin definir whitelist explícita: cualquier escritura quedará bloqueada con error 500-style en vez de mensaje claro. Define `WHITELIST_TABLES` aunque sea para tablas inexistentes, así el error es informativo. |

### `MSSQL_WHITELIST_TABLES`

| | |
|---|---|
| **Activa** | Excepción a READ_ONLY. Acepta lista CSV (`temp_ai,v_temp_ia`) o wildcard (`*` = todas las tablas de la DB actual). |
| **NO afecta** | SELECTs (READ_ONLY siempre permite SELECT sobre cualquier tabla de la DB actual). Cross-database modifications (siempre bloqueadas, sin excepción). Procedures (la whitelist es solo para tablas; el campo `MSSQL_WHITELIST_PROCEDURES` se lee pero hoy **no tiene enforcement** — ver "Hallazgos" al final). |
| **Interacciones** | Sin `READ_ONLY=true` la whitelist se ignora por completo. Para que tenga efecto, **siempre** combinar con `READ_ONLY=true`. |
| **Anti-patrón** | Listar tablas críticas pensando que las "expone solo a AI": al revés, listarlas las hace **modificables**. Lo seguro es listar solo tablas-staging para la AI. |

### `MSSQL_CONFIRM_DESTRUCTIVE`

| | |
|---|---|
| **Activa** | DDL destructivo sobre objetos **existentes** (`DROP TABLE`, `DROP VIEW`, `DROP PROCEDURE`, `DROP FUNCTION`, `ALTER VIEW`, `ALTER TABLE`, `TRUNCATE TABLE`) requiere un token de confirmación generado por la propia herramienta y entregado al usuario; el usuario debe llamar `confirm_operation` con ese token en menos de 5 minutos. |
| **NO afecta** | INSERT/UPDATE/DELETE (no son DDL). CREATE sobre objetos que **no** existen aún (no hay nada que destruir). DDL sobre objetos inexistentes (no hay nada que destruir). |
| **Interacciones** | **AUTOPILOT NO la desactiva** (esto contradice afirmaciones antiguas en docs ya corregidas). Para CI/CD donde la confirmación interactiva es imposible, `MSSQL_CONFIRM_DESTRUCTIVE=false` es el flag correcto. |
| **Anti-patrón** | Combinar `MSSQL_CONFIRM_DESTRUCTIVE=false` con `READ_ONLY=false` y sin whitelist en producción. Pierdes las dos capas que filtran DDL accidental. |

### `MSSQL_AUTOPILOT`

| | |
|---|---|
| **Activa** | Salta `validateTablesExist`. La AI puede ejecutar queries contra tablas que aún no existen sin que la herramienta intercepte con un error de "tabla no encontrada". |
| **NO afecta** | Confirmación destructiva, READ_ONLY, WHITELIST, validación SQL estructural (comentarios anidados, hints, unicode). Estas siguen activas. |
| **Interacciones** | Con `SKIP_SCHEMA_VALIDATION` el efecto es OR: cualquiera de los dos basta para saltar la validación. |
| **Cuándo usarlo** | Desarrollo donde la AI iterará sobre tablas en construcción y los errores de "tabla no existe" son ruido. Combinar con `WHITELIST_TABLES` para acotar el scope. |

### `MSSQL_SKIP_SCHEMA_VALIDATION`

| | |
|---|---|
| **Activa** | Mismo efecto exacto que `AUTOPILOT` sobre la validación de existencia de tablas. |
| **NO afecta** | Lo mismo que AUTOPILOT (no toca confirmación, READ_ONLY, ni WHITELIST). |
| **Por qué existe como flag separado** | Para poder saltar validación de schema **sin** activar otras semánticas que `AUTOPILOT` adquiera en el futuro. Hoy ambos hacen lo mismo, pero el desacople permite que `AUTOPILOT` evolucione (p. ej. añadiendo skip de logs verbosos, prompts simplificados, etc.) sin forzarte a aceptar todo el paquete cuando solo querías saltar la comprobación de schema. |

### `MSSQL_ALLOWED_DATABASES`

| | |
|---|---|
| **Activa** | Permite SELECTs con nombre de 3 partes (`[OtherDB].schema.table`) sobre las DBs listadas. La herramienta `explore` con parámetro `database` también las soporta. |
| **NO afecta** | Modificaciones cross-database — **siempre bloqueadas**, independientemente de `READ_ONLY` o `WHITELIST_TABLES`. La regla es: cualquier `tableRef` con `database != ""` se rechaza al validar permisos. |
| **Interacciones** | El usuario SQL configurado debe tener permisos reales sobre esas DBs en el servidor; el flag solo abre la puerta a nivel de la herramienta, no concede permisos reales. |
| **Anti-patrón** | Listar todas las DBs del servidor "por si acaso". Cada DB añadida es superficie de lectura adicional para la AI. Lista solo las que efectivamente vas a consultar. |

---

## Presets típicos

### Producción AI-safe (recomendado para asistentes con AI)

```env
MSSQL_SERVER=prod.example.com
MSSQL_DATABASE=ProductionDB
MSSQL_USER=ai_user
MSSQL_PASSWORD=...
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
MSSQL_CONFIRM_DESTRUCTIVE=true
```

**Resultado**: AI puede leer toda la DB, pero solo modificar `temp_ai` y `v_temp_ia`. DDL destructivo requiere confirmación. TLS estricto.

### Producción solo-lectura (sin escritura ninguna)

```env
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
# sin MSSQL_WHITELIST_TABLES → ninguna tabla modificable
```

**Resultado**: SELECT en todo, INSERT/UPDATE/DELETE/DDL en nada.

### Desarrollo local con autopilot

```env
DEVELOPER_MODE=true
MSSQL_AUTOPILOT=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia,mi_vista
MSSQL_READ_ONLY=true
```

**Resultado**: AI ignora errores de "tabla no existe" (puede iterar libremente sobre tablas en construcción), pero solo puede modificar las whitelisted. DDL destructivo sobre objetos existentes sigue requiriendo confirmación. TLS laxo (autoindicado certs).

### CI/CD (automation)

```env
DEVELOPER_MODE=false
MSSQL_READ_ONLY=false
MSSQL_CONFIRM_DESTRUCTIVE=false
```

**Resultado**: Sin barrera interactiva, scripts pueden ejecutar DDL sin tokens. **Riesgo asumido**: el script tiene autoridad total sobre la DB. Asegura permisos SQL granulares en el usuario.

### SQL Server 2008/2012 (legacy sin TLS 1.2)

```env
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
MSSQL_AUTH=integrated
```

**Resultado**: Conexión sin cifrado obligado (las versiones antiguas no soportan TLS 1.2). Solo viable en redes confiables.

---

## Protecciones siempre-activas (no las desactiva ningún flag)

Independientemente de la combinación de flags:

- **Procedimientos del sistema peligrosos**: `xp_cmdshell`, `sp_executesql`, `sp_OAcreate`, `sp_OAdestroy`, `sp_addextendedproc`, etc. están siempre bloqueados.
- **Lectura cross-server**: `OPENROWSET`, `OPENDATASOURCE`, `OPENQUERY` siempre bloqueadas.
- **Exfiltración por SELECT INTO**: `SELECT * INTO ...` siempre bloqueada.
- **Modificaciones cross-database**: cualquier `INSERT/UPDATE/DELETE/DDL` con nombre 3-partes (`[OtherDB].schema.table`) siempre bloqueada, aunque la DB destino esté en `MSSQL_ALLOWED_DATABASES` y la tabla en `MSSQL_WHITELIST_TABLES`.
- **Validación estructural SQL**: comentarios anidados, hints maliciosos, unicode RTL/homoglyphs siempre bloqueados.
- **Whitelist multi-tabla**: si la query tiene `JOIN` o subqueries, **todas** las tablas referenciadas deben estar permitidas; basta una no autorizada para bloquear la query entera.
- **Prepared statements**: las queries internas de la herramienta (validación, exploración) siempre usan parámetros tipados (`@p1`, `@p2`); no hay concatenación dinámica.
- **TLS en producción**: con `DEVELOPER_MODE=false`, el cifrado y la validación de certificados son obligatorios — no hay flag que los desactive.
- **Logging de eventos de seguridad**: `secLogger.Printf` se ejecuta independientemente del modo, con sanitización de credenciales.

---

## Hallazgos pendientes

Documentados aquí para no perderlos, **no implican riesgo activo** pero conviene resolverlos:

- **`MSSQL_WHITELIST_PROCEDURES`**: el campo `whitelistProcs` se lee de `os.Getenv("MSSQL_WHITELIST_PROCEDURES")` en `serverConfig`, pero **no aparece referenciado en ninguna lógica de validación**. Es un flag fantasma similar al que era `MSSQL_SKIP_SCHEMA_VALIDATION` antes de su implementación reciente. Decisión pendiente: implementar enforcement, o eliminar el campo de la config.
- **`ConnectionInfo.autopilot` y `ConnectionInfo.skipSchemaValidation`**: las conexiones dinámicas (`dynamic_connect`) cargan estos campos por conexión, pero `validateTablesExist` solo consulta `s.config.*` (la config global), nunca el `connInfo.*` del alias específico. En la práctica significa que `MSSQL_<alias>_AUTOPILOT=true` en una conexión dinámica se ignora silenciosamente.
- **Matriz cartesiana no probada explícitamente**: muchos de los comportamientos descritos arriba están cubiertos por tests individuales, pero las **combinaciones** (por ejemplo "AUTOPILOT=true + READ_ONLY=true + WHITELIST con CTE recursivo") no tienen tests dedicados. Sería útil un test table-driven que enumere las combinaciones críticas.

---

## Referencias

- `main.go` — implementación canónica.
- [`SECURITY.md`](../SECURITY.md) — política de reporte de vulnerabilidades.
- [`config-visual.md`](config-visual.md) — vista visual complementaria (estilo ASCII art) de los presets más comunes.
- [`CLAUDE.md`](../CLAUDE.md) — guía operativa con ejemplos de variables.
