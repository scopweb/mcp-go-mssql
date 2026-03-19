---
title: Go Upgrade
description: Guide for upgrading the Go version in MCP-Go-MSSQL
---

## Required version

MCP-Go-MSSQL requires Go 1.26.0 or later.

## Check current version

```bash
go version
```

## Upgrade Go

### Windows

Download the installer from [go.dev/dl](https://go.dev/dl/) and run the `.msi`.

### Linux

```bash
# Download (adjust version)
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz

# Install
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
```

### macOS

```bash
brew upgrade go
```

## After upgrading

```bash
# Verify
go version

# Update dependencies
go mod tidy

# Build
go build

# Run tests
go test ./...
```

## Compatibility

- The `go.mod` file specifies the minimum Go version
- Dependencies are managed automatically with Go modules
- The `go-mssqldb` driver is compatible with Go 1.26+
