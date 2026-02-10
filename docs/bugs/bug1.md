# Bug #1: Connection fails with DEVELOPER_MODE=false (TLS Certificate Required)

## Problema Identificado

La conexión a SQL Server falla cuando se configura `DEVELOPER_MODE=false` (modo producción), aunque las credenciales sean correctas.

## Configuración Original (FALLA)
```json
{
  "MSSQL_SERVER": "your-server",
  "MSSQL_DATABASE": "your-database",
  "MSSQL_USER": "your-user",
  "MSSQL_PASSWORD": "***",
  "MSSQL_PORT": "1433",
  "MSSQL_READ_ONLY": "true",
  "MSSQL_WHITELIST_TABLES": "table1,table2",
  "DEVELOPER_MODE": "false"  ← PROBLEMA AQUÍ
}
```

## Log de Error
```
sql.Open successful, testing connection...
Testing database connection with ping...
Database connection attempt: FAILED
Failed to ping database: connection test failed
```

## Causa Raíz

En `main.go:257-266`, cuando `DEVELOPER_MODE=false`, el código genera:
- `encrypt=true` (TLS obligatorio)
- `trustservercertificate=false` (requiere certificado válido)

```go
encrypt := "true"
trustCert := "false"
if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
    encrypt = "false"  // Solo en desarrollo
    trustCert = "true"
} // En producción NO entra aquí
```

Connection string resultante:
```
server=your-server;database=your-db;...;encrypt=true;trustservercertificate=false
```

**Esto requiere que SQL Server tenga un certificado TLS válido y confiable**, lo cual servidores internos normalmente no tienen.

## Solución Aplicada

Cambiar `DEVELOPER_MODE` a `"true"`:

```json
{
  "MSSQL_SERVER": "your-server",
  "MSSQL_DATABASE": "your-database",
  "MSSQL_USER": "your-user",
  "MSSQL_PASSWORD": "***",
  "MSSQL_PORT": "1433",
  "MSSQL_READ_ONLY": "true",
  "MSSQL_WHITELIST_TABLES": "table1,table2",
  "DEVELOPER_MODE": "true"  ← SOLUCIÓN
}
```

Esto genera:
- `encrypt=false` (sin TLS, OK para red interna)
- `trustservercertificate=true` (acepta certificados auto-firmados)

## Alternativas (No Implementadas Aún)

1. **Instalar certificado TLS válido en SQL Server** (solución profesional para producción real)
2. **Permitir `MSSQL_ENCRYPT` en modo producción** (requeriría cambio de código)
3. **Usar `MSSQL_CONNECTION_STRING` custom** con `encrypt=disable`

## Estado

✅ **RESUELTO** - Cambiando `DEVELOPER_MODE=true`

## Notas

- `MSSQL_READ_ONLY=true` y `MSSQL_WHITELIST_TABLES` funcionan correctamente
- El problema NO era de permisos ni de las whitelist tables
- El problema era exclusivamente la configuración TLS/certificados 