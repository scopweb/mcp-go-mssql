---
title: Solución de problemas
description: Problemas comunes y soluciones para MCP-Go-MSSQL
---

## Errores de conexión

### "Database not connected"

Las variables de entorno no están configuradas o son incorrectas.

```bash
echo "Server: $MSSQL_SERVER"
echo "Database: $MSSQL_DATABASE"
echo "User: $MSSQL_USER"
```

### "TLS handshake failed" / "Certificate signed by unknown authority"

**Desarrollo:** Configura `DEVELOPER_MODE=true` para aceptar certificados autofirmados.

**SQL Server 2008/2012:** Estos servidores no soportan TLS 1.2. Añade `MSSQL_ENCRYPT=false` junto con `DEVELOPER_MODE=true`.

```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

**Producción:** Instala certificados SSL válidos en SQL Server o configura la CA en el sistema.

### "Login failed for user"

- Verifica usuario y contraseña
- Confirma que SQL Server está en modo de autenticación mixto (SQL + Windows)
- Verifica que el usuario tiene acceso a la base de datos especificada

### "Network error" / "Connection refused"

- Confirma que SQL Server está escuchando en el puerto correcto (default 1433)
- Revisa las reglas de firewall
- Verifica que el servicio SQL Server está ejecutándose

## Errores de ejecución

### "Query blocked: read-only mode"

La consulta intenta modificar datos y `MSSQL_READ_ONLY=true` está activo. Si necesitas modificar tablas específicas, añádelas a `MSSQL_WHITELIST_TABLES`.

### "Table not in whitelist"

La tabla referenciada en una operación de escritura no está en `MSSQL_WHITELIST_TABLES`. Verifica el nombre exacto de la tabla.

### "SSPI handshake failed"

Verifica que SQL Server acepta autenticación Windows y que el usuario de Windows tiene un login configurado.

## Diagnóstico mejorado con Claude

Cuando Claude recibe un error "Database not connected", ahora automáticamente puede llamar a `get_database_info` para obtener:

- **Configuración actual**: servidor, base de datos, modo de autenticación, cifrado, puerto
- **Causas posibles**: variables faltantes, incompatibilidad TLS, problemas de permisos
- **Soluciones específicas**: según el escenario detectado (SQL 2008, auth integrada, etc.)

Esto permite a Claude diagnosticar y resolver problemas de conexión sin intervención manual.

## Verificar la conexión

```bash
cd test
go run test-connection.go
```
