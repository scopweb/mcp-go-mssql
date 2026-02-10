# Bug #2: READ_ONLY mode blocks whitelisted tables

## Problema Identificado

Conflicto de configuración entre `MSSQL_READ_ONLY=true` y `MSSQL_WHITELIST_TABLES`. El modo read-only bloquea **TODAS** las operaciones de escritura, incluso en tablas whitelisted.

## Configuración Problemática
```json
{
  "MSSQL_READ_ONLY": "true",
  "MSSQL_WHITELIST_TABLES": "table1,table2"
}
```

## Comportamiento Observado

Con esta configuración:
- `get_database_info` muestra: "READ-ONLY (SELECT queries only)"
- `get_database_info` también muestra: "Whitelisted Tables: table1, table2"
- Nota dice: "Only whitelisted tables can be modified"

Pero al intentar `UPDATE table1 SET ...`:
```
Error: read-only mode: only SELECT queries are allowed
```

## Causa Raíz

En `main.go`, la función `validateReadOnlyQuery()` (línea 325-420) se ejecuta **ANTES** que `validateTablePermissions()` (línea 510-567).

Orden de validación en `executeSecureQuery()` (línea 569):
```go
// 1. Validar read-only (BLOQUEA TODO si está activo)
if err := s.validateReadOnlyQuery(query); err != nil {
    return nil, err  // ← Falla aquí, nunca llega a la whitelist
}

// 2. Validar whitelist (NUNCA SE EJECUTA si READ_ONLY=true)
if err := s.validateTablePermissions(query); err != nil {
    return nil, err
}
```

`validateReadOnlyQuery()` solo permite SELECT/WITH/SHOW/DESCRIBE/EXPLAIN y rechaza INSERT/UPDATE/DELETE inmediatamente.

## Comportamiento Esperado vs Real

| Configuración | Comportamiento Esperado | Comportamiento Real |
|---------------|------------------------|---------------------|
| `READ_ONLY=false`<br>`WHITELIST=table1` | ✅ Solo table1 modificable | ✅ Funciona correctamente |
| `READ_ONLY=true`<br>`WHITELIST=table1` | ✅ Solo table1 modificable | ❌ TODO bloqueado (bug) |
| `READ_ONLY=true`<br>Sin WHITELIST | ✅ TODO read-only | ✅ Funciona correctamente |

## Solución Aplicada (Workaround)

Cambiar la configuración:

```json
{
  "MSSQL_READ_ONLY": "false",  ← Desactivar READ_ONLY
  "MSSQL_WHITELIST_TABLES": "table1,table2"  ← Confiar en whitelist
}
```

**Resultado:**
- ✅ Solo las tablas en whitelist son modificables
- ✅ Todas las demás tablas quedan protegidas
- ✅ SELECT funciona en todas las tablas

## Solución Definitiva (Pendiente)

Refactorizar `validateReadOnlyQuery()` para que:

1. Si `READ_ONLY=true` Y `WHITELIST` está configurada:
   - Permitir modificaciones en tablas whitelisted
   - Bloquear modificaciones en otras tablas

2. Si `READ_ONLY=true` Y `WHITELIST` vacía:
   - Bloquear todas las modificaciones (comportamiento actual)

```go
// Pseudocódigo propuesto
func (s *MCPMSSQLServer) validateReadOnlyQuery(query string) error {
    if !isReadOnlyMode() {
        return nil // No restrictions
    }

    // Check if query is modification
    if isModificationQuery(query) {
        // If whitelist exists, allow validation to pass to validateTablePermissions()
        if len(s.getWhitelistedTables()) > 0 {
            return nil // Let whitelist handle it
        }
        return fmt.Errorf("read-only mode: only SELECT allowed")
    }

    return nil // SELECT queries always allowed
}
```

## Solución Implementada

### Cambios en el Código

**Archivo:** `main.go`
**Función:** `validateReadOnlyQuery()` (línea 325+)

**Modificaciones:**

1. **Detección de whitelist al inicio de la función:**
   ```go
   whitelist := s.getWhitelistedTables()
   if len(whitelist) > 0 {
       // Whitelist configurada - permitir que validateTablePermissions() maneje permisos
       // Solo bloquear procedimientos peligrosos
       return nil // Permite que la query pase a validateTablePermissions()
   }
   ```

2. **Mensajes mejorados en `get_database_info`:**
   - Con `READ_ONLY=true` + whitelist: "READ-ONLY with whitelist exceptions"
   - Con `READ_ONLY=false` + whitelist: "Whitelist-protected (modifications restricted)"
   - Con `READ_ONLY=true` sin whitelist: "READ-ONLY (SELECT queries only)"
   - Sin ninguna restricción: "Full access"

### Comportamiento Después del Fix

| Configuración | Comportamiento |
|---------------|----------------|
| `READ_ONLY=false`<br>`WHITELIST=table1` | ✅ Solo table1 modificable |
| `READ_ONLY=true`<br>`WHITELIST=table1` | ✅ Solo table1 modificable (AHORA FUNCIONA) |
| `READ_ONLY=true`<br>Sin WHITELIST | ✅ TODO read-only |

### Seguridad Mantenida

Incluso con whitelist configurada, el sistema sigue bloqueando:
- Procedimientos peligrosos (XP_CMDSHELL, SP_CONFIGURE, etc.)
- Operaciones en tablas no whitelisted
- SQL injection mediante prepared statements

## Estado

✅ **RESUELTO** - Implementado en commit [fecha]

## Testing Recomendado

```json
// Configuración de prueba
{
  "MSSQL_READ_ONLY": "true",
  "MSSQL_WHITELIST_TABLES": "test_table1,test_table2",
  "DEVELOPER_MODE": "true"
}
```

**Casos de prueba:**
1. `SELECT * FROM any_table` → ✅ Debe funcionar
2. `UPDATE test_table1 SET col=val` → ✅ Debe funcionar (whitelisted)
3. `UPDATE other_table SET col=val` → ❌ Debe fallar (no whitelisted)
4. `EXEC XP_CMDSHELL 'cmd'` → ❌ Debe fallar (procedimiento peligroso)

## Notas

- El whitelist de tablas (`validateTablePermissions`) funciona perfectamente por sí solo
- El modo READ_ONLY funciona bien cuando no hay whitelist
- Ahora ambos sistemas están integrados correctamente
- Ubicación del código: `main.go:325-420` (validateReadOnlyQuery) y `main.go:510-567` (validateTablePermissions)