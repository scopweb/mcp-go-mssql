---
title: Análisis de seguridad
description: Análisis detallado del modelo de seguridad de MCP-Go-MSSQL
---

MCP-Go-MSSQL ha sido diseñado siguiendo estándares de seguridad reconocidos y aplica defensa en profundidad en todas las capas.

## Modelo de amenazas

### Vectores de ataque cubiertos

| Vector | Mitigación |
|--------|------------|
| SQL Injection | Prepared statements exclusivos, sin concatenación dinámica |
| SQL Injection (IA) | Bloqueo de CHAR concat, comentarios inline, homoglyphs Unicode |
| Acceso no autorizado | Modo solo lectura + whitelist de tablas |
| Dirty reads (IA) | Bloqueo de table hints (NOLOCK, READUNCOMMITTED, TABLOCK) |
| Timing attacks (IA) | Bloqueo de WAITFOR DELAY |
| Exfiltración de datos (IA) | Bloqueo de OPENROWSET/OPENDATASOURCE |
| Intercepción de datos | TLS obligatorio en todas las conexiones |
| Agotamiento de recursos | Connection pooling con límites configurables |
| Fuga de información | Errores genéricos al cliente, detalles solo en logs internos |
| Escalada de privilegios | Validación multi-tabla en JOINs y subqueries |
| Homoglyph obfuscation (IA) | Detección de caracteres Cyrillic/Greek que imitan ASCII |
| Unicode control chars (IA) | Bloqueo de RTL override, zero-width spaces |

### Cumplimiento de estándares

- **OWASP Top 10 (2021)**: A01-Broken Access Control, A03-Injection, A02-Cryptographic Failures
- **CWE Top 25 (2024)**: CWE-89 (SQL Injection), CWE-306 (Missing Auth), CWE-798 (Hardcoded Credentials)
- **NIST Cybersecurity Framework**: Identify, Protect, Detect, Respond

## Análisis por capa

### Capa de transporte

- Cifrado TLS obligatorio (`encrypt=true`)
- Validación de certificados en producción (`trustservercertificate=false`)
- Certificados autofirmados solo permitidos en modo desarrollo

### Capa de aplicación

- Sanitización automática de datos sensibles en logs
- Límite de tamaño de consulta (1 MB)
- Rechazo de entrada vacía
- Bloqueo de comandos del sistema (`xp_cmdshell`, `OPENROWSET`, etc.)

### Capa de datos

- Prepared statements para todas las queries sin excepción
- Validación de todas las tablas referenciadas en modificaciones
- Connection pooling con límites de conexiones activas
- Timeouts configurables para prevenir conexiones colgadas

## Recomendaciones

1. Ejecutar con `MSSQL_READ_ONLY=true` en producción
2. Definir `MSSQL_WHITELIST_TABLES` solo para tablas temporales de IA
3. Usar un usuario de base de datos con permisos mínimos
4. Monitorear los logs de seguridad periódicamente
5. Rotar credenciales de forma regular
