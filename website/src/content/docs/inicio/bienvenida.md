---
title: Bienvenida
description: Qué es MCP-Go-MSSQL y por qué lo necesitas
---

**MCP-Go-MSSQL** es un puente seguro entre Claude y tu base de datos Microsoft SQL Server. Escrito en Go, te permite consultar, analizar y operar datos directamente desde la conversación, sin salir de Claude Desktop ni de Claude Code.

## Por qué existe este proyecto

Los asistentes de IA son increíblemente útiles para trabajar con datos, pero conectarlos a una base de datos de producción da miedo. Un error y puedes perder datos críticos. MCP-Go-MSSQL resuelve ese problema: le da a Claude acceso completo de lectura y un espacio controlado de escritura, para que puedas aprovechar la IA sin arriesgar nada.

## Dos formas de usarlo

1. **Servidor MCP** (`main.go`) — Se integra con Claude Desktop a través del protocolo MCP. Configúralo una vez y Claude tendrá las herramientas de base de datos disponibles en cada conversación.

2. **CLI para Claude Code** (`claude-code/db-connector.go`) — Acceso directo por línea de comandos. Ideal para desarrollo, scripts y automatización.

Ambos comparten las mismas capas de seguridad: TLS, prepared statements, modo solo lectura y whitelist.

## Qué puedes hacer

- **Explorar** la estructura de tu base de datos: tablas, columnas, índices, foreign keys
- **Consultar** datos con queries SQL completas: JOINs, CTEs, funciones de ventana
- **Analizar** información de producción sin riesgo de modificación accidental
- **Operar** tablas temporales autorizadas para que la IA procese y transforme datos
- **Ejecutar** procedimientos almacenados de forma controlada

## Seguridad de serie

No es un módulo adicional que activas por separado. La seguridad está integrada en cada capa:

- **Cifrado TLS** obligatorio en producción
- **Prepared statements** exclusivos — imposible inyectar SQL
- **Modo solo lectura** que bloquea cualquier escritura no autorizada
- **Whitelist de tablas** para permitir escritura granular donde tú decidas
- **Validación multi-tabla** que detecta acceso no autorizado via JOINs y subqueries
- **Logging seguro** que nunca registra credenciales ni datos sensibles

## Requisitos

- **Go 1.26+** ([descargar](https://go.dev/dl/))
- **Microsoft SQL Server** con soporte TLS (2012 o superior recomendado)
- Acceso de red al puerto de SQL Server (1433 por defecto)

## Estructura del proyecto

```
mcp-go-mssql/
├── main.go                    # Servidor MCP para Claude Desktop
├── claude-code/
│   └── db-connector.go        # CLI para Claude Code
├── test/
│   └── security/              # Suite de tests de seguridad
├── scripts/                   # Scripts de build y utilidad
└── website/                   # Esta documentación
```

## Siguiente paso

Sigue con la [Instalación](/inicio/instalacion/) para tener el servidor funcionando en minutos.
