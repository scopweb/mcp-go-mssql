---
title: Resumen de seguridad
description: Visión general de las características de seguridad de MCP-Go-MSSQL
---

MCP-Go-MSSQL implementa múltiples capas de seguridad para proteger bases de datos en entornos de producción.

## Características de seguridad

### Seguridad de base de datos
- **Cifrado TLS obligatorio** para todas las conexiones en producción
- **Protección contra SQL Injection** con prepared statements exclusivamente
- **Connection pooling** con límites para prevenir agotamiento de recursos
- **Timeouts de conexión** configurables

### Seguridad de aplicación
- **Logging seguro** con sanitización automática de datos sensibles
- **Manejo de errores seguro** — mensajes genéricos al cliente, detalles en logs internos
- **Validación de entrada** con límites de tamaño de consulta
- **Validación multi-tabla** — detecta acceso no autorizado via JOINs/subqueries

### Control de acceso
- **Modo solo lectura** — bloquea INSERT/UPDATE/DELETE
- **Whitelist de tablas** — control granular sobre tablas modificables
- **Configuración por roles** — diferentes configs para diferentes entornos

### Autenticación
- **Múltiples métodos** — SQL Server, Windows Integrated (SSPI), connection strings personalizados
- **Modos dev/prod** — diferente nivel de strictness TLS
- **Variables de entorno** — credenciales nunca en código fuente
- **Plantillas de configuración** — `.env.example` con valores seguros por defecto

## Cumplimiento

- OWASP Top 10 (2021)
- CWE Top 25 (2024)
- NIST Cybersecurity Framework
- Best practices de Go para bases de datos

## Mejores prácticas

### Hacer
- Usar variables de entorno para todas las credenciales
- Crear base de datos separada para tests
- Establecer permisos restrictivos (600) en archivos `.env`
- Habilitar modo solo lectura para acceso de IA
- Monitorear logs de seguridad regularmente
- Mantener dependencias actualizadas

### No hacer
- Hardcodear credenciales en código fuente
- Usar base de datos de producción para testing
- Hacer commit de `.env` o `config.json` a Git
- Desplegar con `DEVELOPER_MODE=true` en producción
- Deshabilitar TLS/cifrado
- Registrar datos sensibles en logs
