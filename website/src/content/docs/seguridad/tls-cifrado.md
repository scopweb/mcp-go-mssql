---
title: TLS y cifrado
description: Configuración del cifrado TLS para conexiones de base de datos
---

El cifrado TLS es **obligatorio en producción**. En desarrollo el cifrado está desactivado por defecto (`encrypt=false`) para facilitar instancias locales, y los servidores legacy (SQL Server 2000/2008/2012) pueden funcionar mediante una opción explícita (`MSSQL_ALLOW_INSECURE_TLS`).

## Comportamiento por modo

### Modo producción (`DEVELOPER_MODE=false`)

- `encrypt=true` — Cifrado obligatorio
- `trustservercertificate=false` — Requiere certificados válidos y de confianza
- **Fail-closed**: si una cadena de conexión (global `MSSQL_CONNECTION_STRING`, por alias `_CONNECTION_STRING` o por alias `_ENCRYPT`) desactiva el cifrado (`encrypt=false`/`disable`) o la validación de certificados (`trustservercertificate=true`), el servidor **se niega a arrancar** en lugar de degradar TLS silenciosamente
- Errores genéricos sin información técnica

### Modo desarrollo (`DEVELOPER_MODE=true`)

- `encrypt=false` — Cifrado desactivado para SQL Server local
- `trustservercertificate=true` — Permite certificados autofirmados
- Errores detallados para depuración

## Forzar cifrado en desarrollo

Si necesitas cifrado en desarrollo:

```bash
MSSQL_ENCRYPT=true
DEVELOPER_MODE=true
```

Esto activa el cifrado pero permite certificados autofirmados.

## Cadenas de conexión TLS

### Producción (Azure SQL)
```
server=prod.database.windows.net;database=ProdDB;encrypt=true;trustservercertificate=false
```

### Desarrollo local
```
server=localhost;database=DevDB;encrypt=false;trustservercertificate=true
```

### Desarrollo con cifrado
```
server=localhost;database=DevDB;encrypt=true;trustservercertificate=true
```

## Permiso explícito para TLS inseguro (`MSSQL_ALLOW_INSECURE_TLS`)

En producción (`DEVELOPER_MODE=false`), cualquier configuración que desactive el cifrado o la validación de certificados **aborta el arranque del servidor** (fail-closed). Para servidores legacy que no pueden actualizarse (SQL Server 2000/2008/2012, que solo negocian TLS 1.0), existe una vía de escape explícita:

```bash
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING=sqlserver://USER:PASS@legacy-sql2000.local:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

- Con `MSSQL_ALLOW_INSECURE_TLS=true` la conexión continúa y se registra un **WARN auditable en cada conexión afectada**.
- Si no se establece, TLS inseguro en producción **aborta el arranque**.
- Es la forma sancionada de mantener servidores legacy funcionando: la excepción se vuelve consciente y buscable en lugar de accidental.

## Resolución de problemas TLS

### "certificate signed by unknown authority"
- **Causa:** Certificado autofirmado o CA no reconocida
- **Desarrollo:** Establecer `DEVELOPER_MODE=true`
- **Producción:** Instalar certificados SSL válidos en SQL Server

### "SSL Provider: No credentials are available"
- **Causa:** SQL Server local sin configuración TLS
- **Solución:** Establecer `DEVELOPER_MODE=true` para desactivar cifrado local

### "TLS Handshake failed"
- **Causa:** SQL Server legacy (2008/2012) no soporta TLS 1.2, que es el mínimo requerido por el driver Go
- **Solución (desarrollo):** Configurar `MSSQL_ENCRYPT=false` junto con `DEVELOPER_MODE=true`
- **Solución (producción):** Usar `MSSQL_CONNECTION_STRING` en forma URL con `encrypt=disable` más el opt-in explícito `MSSQL_ALLOW_INSECURE_TLS=true`

**Desarrollo:**
```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

**Producción:**
```bash
DEVELOPER_MODE=false
MSSQL_ALLOW_INSECURE_TLS=true
MSSQL_CONNECTION_STRING=sqlserver://USER:PASS@legacy-sql2000.local:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

> Esto desactiva TLS en la conexión. Solo usar para servidores legacy que no pueden actualizarse. Para SQL Server 2016+ y Azure SQL, mantener cifrado activado.
