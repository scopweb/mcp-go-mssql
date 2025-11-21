# 🗄️ MCP Go MSSQL - Microsoft SQL Server MCP Server

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24.6-blue.svg)](https://golang.org/)
[![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-green.svg)](https://modelcontextprotocol.io/)
[![SQL Server](https://img.shields.io/badge/SQL%20Server-2008%2B-red.svg)](https://www.microsoft.com/sql-server)

**A secure, production-ready MCP server for Microsoft SQL Server integration with Claude Desktop and other MCP clients**

[🚀 Quick Start](#-quick-start) • [📖 Documentation](#-features) • [🔒 Security](#-security-features) • [💬 Community](#-community)

</div>

---

## 📋 Overview

MCP Go MSSQL is a **secure** Go-based Model Context Protocol (MCP) server that provides seamless Microsoft SQL Server connectivity for AI assistants like Claude Desktop. Built with security-first principles, it enables safe database operations with comprehensive protection against SQL injection and data leaks.

### Why Choose MCP Go MSSQL?

- ✅ **Production-Ready**: Battle-tested security with TLS encryption, connection pooling, and prepared statements
- ✅ **Universal Compatibility**: Supports SQL Server 2008 through 2022 and Azure SQL Database
- ✅ **Zero Code**: Works out-of-the-box with Claude Desktop via MCP protocol
- ✅ **Microsoft Official Driver**: Uses the latest `microsoft/go-mssqldb` driver (v1.9.3)
- ✅ **Enterprise Security**: Sanitized logging, input validation, and configurable encryption
- ✅ **Dual Mode**: MCP server for Claude Desktop + CLI tool for direct use

---

## 🎯 Key Features

### 🔧 Core Capabilities

| Feature | Description |
|---------|-------------|
| **Query Execution** | Execute SELECT queries with automatic result formatting |
| **Schema Discovery** | List all tables, views, and stored procedures |
| **Table Inspection** | Describe table structure with columns, types, and constraints |
| **Safe Queries** | All queries use prepared statements (SQL injection protection) |
| **Connection Pooling** | Optimized resource management with configurable limits |
| **TLS Encryption** | Mandatory encrypted connections to database |

### 🔒 Security Features

- **SQL Injection Protection**: Uses prepared statements exclusively
- **Data Sanitization**: Automatic removal of sensitive data from logs
- **TLS/SSL Enforcement**: Encrypted database connections
- **Input Validation**: Comprehensive validation of all inputs
- **Error Handling**: Generic error messages to clients, detailed logs internally
- **Connection Timeouts**: Prevent hanging connections and DoS
- **Developer/Production Modes**: Configurable security levels

### 🌐 Compatibility

| SQL Server Version | Status | Notes |
|-------------------|--------|-------|
| SQL Server 2022 | ✅ Full Support | Latest features |
| SQL Server 2019 | ✅ Full Support | Tested |
| SQL Server 2017 | ✅ Full Support | All features available |
| SQL Server 2016 | ✅ Full Support | |
| SQL Server 2012-2014 | ✅ Full Support | |
| SQL Server 2008 R2 | ✅ Supported | Requires SP2 |
| SQL Server 2008 | ✅ Supported | Requires SP3 + CU3 |
| Azure SQL Database | ✅ Full Support | Recommended |

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.20+** (for building from source)
- **SQL Server** (any version 2008+) or Azure SQL Database
- **Claude Desktop** (for MCP integration)

### Installation

#### Option 1: Download Pre-built Binary (Recommended)

```bash
# Download latest release
# Windows
curl -L https://github.com/scopweb/mcp-go-mssql/releases/latest/download/mcp-go-mssql.exe -o mcp-go-mssql.exe

# Linux
curl -L https://github.com/scopweb/mcp-go-mssql/releases/latest/download/mcp-go-mssql-linux -o mcp-go-mssql
chmod +x mcp-go-mssql

# macOS
curl -L https://github.com/scopweb/mcp-go-mssql/releases/latest/download/mcp-go-mssql-macos -o mcp-go-mssql
chmod +x mcp-go-mssql
```

#### Option 2: Build from Source

```bash
git clone https://github.com/scopweb/mcp-go-mssql.git
cd mcp-go-mssql
go build -o mcp-go-mssql.exe
```

### Configuration

#### For Claude Desktop

Edit your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mssql-production": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "your_user",
        "MSSQL_PASSWORD": "your_secure_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

#### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MSSQL_SERVER` | ✅ Yes | - | SQL Server hostname/IP |
| `MSSQL_DATABASE` | ✅ Yes | - | Database name |
| `MSSQL_USER` | ✅ Yes | - | Username |
| `MSSQL_PASSWORD` | ✅ Yes | - | Password |
| `MSSQL_PORT` | No | `1433` | SQL Server port |
| `DEVELOPER_MODE` | No | `false` | `true` = dev mode with detailed errors<br>`false` = production mode |

### Test Connection

```bash
# Set environment variables
export MSSQL_SERVER=your-server.database.windows.net
export MSSQL_DATABASE=YourDatabase
export MSSQL_USER=your_user
export MSSQL_PASSWORD=your_password
export DEVELOPER_MODE=true

# Test connection
./test/test-connection
```

---

## 📚 Usage Examples

### In Claude Desktop

Once configured, you can ask Claude:

```
"List all tables in the database"
"Describe the structure of the Users table"
"Execute: SELECT TOP 10 * FROM Products WHERE Price > 100"
```

### CLI Tool Usage

```bash
# List all tables
go run claude-code/db-connector.go tables

# Describe table structure
go run claude-code/db-connector.go describe Users

# Execute query
go run claude-code/db-connector.go query "SELECT * FROM Products WHERE Price > 100"

# Test connection
go run claude-code/db-connector.go test

# Show database info
go run claude-code/db-connector.go info
```

---

## 🏗️ Architecture

```
┌─────────────────┐
│  Claude Desktop │
│   (MCP Client)  │
└────────┬────────┘
         │ MCP Protocol (stdio)
         │ JSON-RPC over stdin/stdout
         ▼
┌─────────────────────────┐
│   MCP Go MSSQL Server   │
│  ┌──────────────────┐   │
│  │ Security Layer   │   │  - Input validation
│  │                  │   │  - SQL injection protection
│  │                  │   │  - Data sanitization
│  └────────┬─────────┘   │
│           │             │
│  ┌────────▼─────────┐   │
│  │ Connection Pool  │   │  - TLS encryption
│  │                  │   │  - Timeout management
│  │                  │   │  - Resource limits
│  └────────┬─────────┘   │
└───────────┼─────────────┘
            │ TLS/SSL
            ▼
   ┌────────────────┐
   │  SQL Server    │
   │  Database      │
   └────────────────┘
```

### Components

1. **MCP Server** (`main.go`): Implements Model Context Protocol for Claude Desktop
2. **CLI Tool** (`claude-code/db-connector.go`): Direct database access tool
3. **Security Layer**: Input validation, SQL injection protection, data sanitization
4. **Connection Management**: TLS encryption, pooling, timeout handling

---

## 🔐 Security Best Practices

### Configuration Security

```bash
# ✅ DO: Use environment variables
export MSSQL_PASSWORD="secure_password"

# ❌ DON'T: Hardcode credentials
MSSQL_PASSWORD="password123"  # Never do this!

# ✅ DO: Set restrictive file permissions
chmod 600 .env

# ✅ DO: Use production mode
DEVELOPER_MODE=false
```

### Network Security

- ✅ Always use TLS encryption (`encrypt=true`)
- ✅ Use firewall rules to restrict access
- ✅ Implement network segmentation
- ✅ Use VPN for remote connections

### Database Security

- ✅ Use least-privilege database accounts
- ✅ Enable database audit logging
- ✅ Regularly rotate credentials
- ✅ Monitor connection attempts

### Production Checklist

- [ ] `DEVELOPER_MODE=false` in production
- [ ] Strong passwords (16+ characters)
- [ ] TLS encryption enabled
- [ ] Firewall rules configured
- [ ] Database user has minimal permissions
- [ ] Credentials stored securely (not in git)
- [ ] Regular security audits
- [ ] Connection logs monitored

---

## 🐛 Troubleshooting

### Common Issues

#### "mssql: Incorrect syntax near '?'"
**Cause**: Using old version with SQL parameter bug
**Solution**: Update to latest version (v1.0.0+)

#### "certificate signed by unknown authority"
**Cause**: Self-signed or untrusted TLS certificate
**Solution**: Set `DEVELOPER_MODE=true` (development only) or install proper certificates

#### "Login failed for user"
**Cause**: Incorrect credentials or authentication mode
**Solution**:
- Verify credentials
- Ensure SQL Server authentication is enabled
- Check user permissions

#### "Network error"
**Cause**: Firewall blocking port 1433
**Solution**:
- Open port 1433 in firewall
- Verify SQL Server is listening on correct port
- Check network connectivity

### Debug Mode

For detailed error messages during development:

```bash
export DEVELOPER_MODE=true
```

⚠️ **Warning**: Never use `DEVELOPER_MODE=true` in production!

---

## 📦 Available MCP Tools

When connected to Claude Desktop, the following tools are available:

### `execute_query`
Execute SELECT queries safely with automatic parameter binding.

**Example:**
```
Claude: "Show me the top 5 products by price"
```

### `list_tables`
List all tables and views in the database with schema information.

**Example:**
```
Claude: "What tables are available?"
```

### `describe_table`
Get detailed table structure including columns, data types, and constraints.

**Example:**
```
Claude: "Describe the structure of the Orders table"
```

---

## 🔄 Recent Updates

### v1.1.0 (Latest)
- ✅ Migrated to official Microsoft SQL Server driver (v1.9.3)
- ✅ Updated security libraries (`golang.org/x/crypto v0.42.0`)
- ✅ Fixed SQL parameter syntax bug in `describe_table`
- ✅ Improved error handling and logging
- ✅ Enhanced documentation

### v1.0.0
- 🎉 Initial release
- ✅ MCP protocol implementation
- ✅ SQL Server 2008-2022 support
- ✅ TLS encryption support
- ✅ CLI tool included

---

## 🤝 Community

### Get Help

- 📖 [Documentation](https://github.com/scopweb/mcp-go-mssql)
- 💬 [GitHub Issues](https://github.com/scopweb/mcp-go-mssql/issues)
- 🐛 [Report a Bug](https://github.com/scopweb/mcp-go-mssql/issues/new)
- 💡 [Feature Request](https://github.com/scopweb/mcp-go-mssql/issues/new)

### Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) first.

### MCP Server Directories

This server is listed on:
- [PulseMCP](https://www.pulsemcp.com/)
- [Smithery](https://smithery.ai/)
- [mcpservers.org](https://mcpservers.org/)

---

## 📜 License

MIT License - see [LICENSE](LICENSE) file for details

---

## 🙏 Acknowledgments

- [Anthropic](https://www.anthropic.com/) for Claude and the Model Context Protocol
- [Microsoft](https://github.com/microsoft/go-mssqldb) for the official Go SQL Server driver
- The Go and SQL Server communities

---

<div align="center">

**Made with ❤️ for the MCP and SQL Server communities**

⭐ Star this repo if you find it useful!

</div>
