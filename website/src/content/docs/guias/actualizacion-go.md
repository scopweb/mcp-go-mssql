---
title: Actualización de Go
description: Guía para actualizar la versión de Go en MCP-Go-MSSQL
---

## Versión requerida

MCP-Go-MSSQL requiere Go 1.25.0 o superior.

## Verificar versión actual

```bash
go version
```

## Actualizar Go

### Windows

Descarga el instalador desde [go.dev/dl](https://go.dev/dl/) y ejecuta el `.msi`.

### Linux

```bash
# Descargar (ajustar versión)
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz

# Instalar
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
```

### macOS

```bash
brew upgrade go
```

## Después de actualizar

```bash
# Verificar
go version

# Actualizar dependencias
go mod tidy

# Compilar
go build

# Ejecutar tests
go test ./...
```

## Compatibilidad

- El archivo `go.mod` especifica la versión mínima de Go
- Las dependencias se gestionan automáticamente con Go modules
- El driver `go-mssqldb` es compatible con Go 1.21+
