---
title: Producción
description: Guía de despliegue en producción de MCP-Go-MSSQL
---

## Compilar el binario

```bash
go build -ldflags "-w -s" -o mcp-go-mssql
```

Los flags `-w -s` eliminan información de debug y reducen el tamaño del binario.

## Variables de entorno

```bash
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=ProductionDB
MSSQL_USER=prod_user
MSSQL_PASSWORD=strong_password
DEVELOPER_MODE=false
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

## Checklist de producción

- [ ] `DEVELOPER_MODE=false`
- [ ] `MSSQL_READ_ONLY=true` (recomendado para IA)
- [ ] Certificados TLS válidos en SQL Server
- [ ] Usuario de base de datos con permisos mínimos
- [ ] Permisos restrictivos en archivos `.env` (600)
- [ ] `.env` excluido de control de versiones
- [ ] Binario compilado con flags de stripping
- [ ] Monitoreo de logs de seguridad configurado
- [ ] Firewall configurado para restringir acceso al puerto SQL

## Seguridad en producción

- El cifrado TLS es obligatorio y no se puede desactivar
- Los certificados autofirmados son rechazados (`trustservercertificate=false`)
- Los errores muestran mensajes genéricos al cliente
- Los detalles técnicos solo aparecen en logs internos
