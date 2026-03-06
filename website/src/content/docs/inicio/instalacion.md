---
title: Instalación
description: Cómo instalar MCP-Go-MSSQL
---

## Requisitos previos

- **Go 1.24+** instalado ([descargar](https://go.dev/dl/))
- **Microsoft SQL Server** accesible en red
- **Git** para clonar el repositorio

## Clonar el repositorio

```bash
git clone https://github.com/DavidSerrano-Rodriguez/mcp-go-mssql.git
cd mcp-go-mssql
```

## Instalar dependencias

```bash
go mod tidy
```

## Compilar

### Compilación rápida (Windows)

```bash
build.bat
```

### Compilación manual

```bash
# Windows
go build -o mcp-go-mssql.exe

# Linux/macOS
go build -o mcp-go-mssql
```

### Compilación de producción (binario optimizado)

```bash
go build -ldflags "-w -s" -o mcp-go-mssql-secure
```

Las flags `-w -s` eliminan información de depuración del binario, reduciendo su tamaño y dificultando la ingeniería inversa.

## Verificar la instalación

```bash
# Comprobar que el binario se creó correctamente
./mcp-go-mssql --help
```

## Dependencias

| Paquete | Descripción |
|---------|-------------|
| `github.com/microsoft/go-mssqldb` | Driver oficial de Microsoft para SQL Server |
| `golang.org/x/crypto` | Soporte criptográfico extendido |
| `golang.org/x/text` | Procesamiento de texto |
| `github.com/stretchr/testify` | Framework de testing |
