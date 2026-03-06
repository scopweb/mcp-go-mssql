---
title: Rendimiento
description: Optimización de rendimiento de MCP-Go-MSSQL
---

## Connection pooling

MCP-Go-MSSQL usa el connection pool integrado del driver `go-mssqldb`. Las conexiones se reutilizan automáticamente.

### Configuración del pool

El pool se configura con límites para prevenir el agotamiento de recursos:

- **Máximo de conexiones abiertas**: Limitado para evitar saturar el servidor SQL
- **Conexiones idle**: Se mantienen abiertas para reutilización rápida
- **Timeouts**: Las conexiones que exceden el timeout se cierran automáticamente

## Prepared statements

Todas las queries usan `PrepareContext()`, lo que permite a SQL Server cachear los planes de ejecución y mejorar el rendimiento en queries repetidas.

## Recomendaciones

### Queries eficientes

- Usa `SELECT` con columnas específicas en lugar de `SELECT *`
- Limita los resultados con `TOP` o `OFFSET/FETCH`
- Aprovecha índices existentes en las cláusulas `WHERE`

### Monitoreo

- Observa los tiempos de respuesta de las queries en los logs
- Usa `get_indexes` para verificar que las tablas tienen índices adecuados
- Consulta `get_database_info` para ver estadísticas generales

### Timeouts

Los timeouts de conexión previenen queries que se ejecutan indefinidamente. Si una query legítima excede el timeout, considera optimizarla o aumentar el límite.
