---
title: Claude Code CLI - Overview
description: Command-line tool for connecting Claude Code with MSSQL databases
---

## Overview

The Claude Code CLI (`db-connector.go`) is a command-line tool that allows Claude Code to interact directly with Microsoft SQL Server databases without needing to configure Claude Desktop.

### Key Features

- **Direct access**: Connects Claude Code with MSSQL without intermediaries
- **Security**: Same security features as the MCP server
- **Simplicity**: Simple commands for common operations
- **Environment variables**: Uses the same environment variables as the MCP server

### Use Cases

The CLI is ideal for:

- Quick development and testing
- Automated scripts
- Database exploration
- Administrative operations

### Requirements

- Go 1.26 or higher
- Configured environment variables (see [Environment Variables](/en/configuracion/variables-entorno))
- Network access to SQL Server

### Location

The CLI source code is located at `claude-code/db-connector.go` in the project repository.

### Next Steps

- [Available commands](/en/cli/comandos)
- [Environment variables](/en/configuracion/variables-entorno)
- [Basic configuration](/en/inicio/configuracion)
