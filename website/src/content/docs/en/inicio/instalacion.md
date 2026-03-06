---
title: Installation
description: How to install MCP-Go-MSSQL
---

## Prerequisites

- **Go 1.24+** installed ([download](https://go.dev/dl/))
- **Microsoft SQL Server** accessible on the network
- **Git** to clone the repository

## Clone the repository

```bash
git clone https://github.com/DavidSerrano-Rodriguez/mcp-go-mssql.git
cd mcp-go-mssql
```

## Install dependencies

```bash
go mod tidy
```

## Build

### Quick build (Windows)

```bash
build.bat
```

### Manual build

```bash
# Windows
go build -o mcp-go-mssql.exe

# Linux/macOS
go build -o mcp-go-mssql
```

### Production build (optimized binary)

```bash
go build -ldflags "-w -s" -o mcp-go-mssql-secure
```

The `-w -s` flags strip debug information from the binary, reducing its size and making reverse engineering harder.

## Verify the installation

```bash
# Check that the binary was created correctly
./mcp-go-mssql --help
```

## Dependencies

| Package | Description |
|---------|-------------|
| `github.com/microsoft/go-mssqldb` | Official Microsoft driver for SQL Server |
| `golang.org/x/crypto` | Extended cryptographic support |
| `golang.org/x/text` | Text processing |
| `github.com/stretchr/testify` | Testing framework |
