---
title: CLI de Claude Code - Resumen
description: Herramienta de línea de comandos para conectar Claude Code con bases de datos MSSQL
---

## Resumen

El CLI de Claude Code (`db-connector.go`) es una herramienta de línea de comandos que permite a Claude Code interactuar directamente con bases de datos Microsoft SQL Server sin necesidad de configurar Claude Desktop.

### Características principales

- **Acceso directo**: Conecta Claude Code con MSSQL sin intermediarios
- **Seguridad**: Mismas características de seguridad que el servidor MCP
- **Simplicidad**: Comandos simples para operaciones comunes
- **Variables de entorno**: Usa las mismas variables de entorno que el servidor MCP

### Casos de uso

El CLI es ideal para:

- Desarrollo y pruebas rápidas
- Scripts automatizados
- Exploración de bases de datos
- Operaciones administrativas

### Requisitos

- Go 1.26 o superior
- Variables de entorno configuradas (ver [Variables de entorno](/configuracion/variables-entorno))
- Acceso de red al servidor SQL Server

### Ubicación

El código fuente del CLI se encuentra en `claude-code/db-connector.go` en el repositorio del proyecto.

### Próximos pasos

- [Comandos disponibles](/cli/comandos)
- [Variables de entorno](/configuracion/variables-entorno)
- [Configuración básica](/inicio/configuracion)
