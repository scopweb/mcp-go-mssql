---
title: Desarrollo
description: Configuración del entorno de desarrollo para MCP-Go-MSSQL
---

## Requisitos

- Go 1.25.0 o superior
- Microsoft SQL Server (local o remoto)
- Git

## Configuración inicial

```bash
git clone https://github.com/DavidSerrano-Rodriguez/mcp-go-mssql.git
cd mcp-go-mssql
go mod tidy
```

## Variables de entorno

```bash
cp .env.example .env
# Editar .env con credenciales de desarrollo
source .env
```

Ejemplo para desarrollo local:

```bash
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=sa
MSSQL_PASSWORD=DevPassword123
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false
```

## Ejecutar en modo desarrollo

```bash
go run main.go
```

En modo desarrollo (`DEVELOPER_MODE=true`):
- Se permiten certificados TLS autofirmados
- Los errores muestran detalles técnicos completos
- El cifrado sigue siendo obligatorio

## Probar la conexión

```bash
cd test
go run test-connection.go
```

## CLI de Claude Code

```bash
go run claude-code/db-connector.go test
go run claude-code/db-connector.go tables
go run claude-code/db-connector.go query "SELECT @@VERSION"
```

## Compilar

```bash
go build
```
