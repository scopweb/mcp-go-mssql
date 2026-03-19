---
title: TLS y cifrado
description: Configuración del cifrado TLS para conexiones de base de datos
---

Todas las conexiones a la base de datos están protegidas por cifrado TLS.

## Comportamiento por modo

### Modo producción (`DEVELOPER_MODE=false`)

- `encrypt=true` — Cifrado obligatorio
- `trustservercertificate=false` — Requiere certificados válidos y de confianza
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
- **Solución:** Configurar `MSSQL_ENCRYPT=false` junto con `DEVELOPER_MODE=true`

```bash
DEVELOPER_MODE=true
MSSQL_ENCRYPT=false
```

> Esto desactiva TLS en la conexión. Solo usar para servidores legacy que no pueden actualizarse. Para SQL Server 2016+ y Azure SQL, mantener cifrado activado.
