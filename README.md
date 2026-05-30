# MCP Go MSSQL

**The most secure way to give Claude, Grok, and other AI agents access to Microsoft SQL Server.**

Production-grade MCP server + CLI with mandatory TLS, SQL injection protection, granular read-only mode, and explicit support for legacy SQL Server 2008/2012.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Security](https://img.shields.io/badge/Security-Hardened-green)](SECURITY.md)
[![MCP Compatible](https://img.shields.io/badge/MCP-Compatible-purple)](https://modelcontextprotocol.io)

**🌐 Full documentation & guides:** [mcp-go-mssql.scopweb.com](https://mcp-go-mssql.scopweb.com)

---

## Why This Project Exists

Most MCP SQL connectors are either:
- Too permissive for production use, or
- Incompatible with older corporate SQL Server instances.

**MCP-Go-MSSQL** was built specifically for real enterprise environments where:
- You need **AI access** to data without risking destructive operations
- You still run **SQL Server 2008/2012** (or any version without modern TLS)
- You want **fine-grained control** (per-table whitelist, read-only mode, dynamic connections)

It is used in production with both Claude Desktop and Grok Build.

---

## Key Security Features

- **Mandatory TLS** by default (configurable for legacy servers)
- **Prepared statements only** — zero dynamic SQL
- **Read-only mode + whitelist** — block all modifications except on explicitly allowed tables (even through JOINs/subqueries)
- **Per-connection security contexts** (dynamic mode)
- **Automatic sensitive data sanitization** in logs
- **Connection pooling + timeouts** to prevent resource exhaustion

---

## Quick Start (Claude Desktop)

### 1. Download the latest release

Get the prebuilt binary for your platform from the [Releases](https://github.com/scopweb/mcp-go-mssql/releases) page.

### 2. Add to your Claude Desktop config

```json
{
  "mcpServers": {
    "mssql": {
      "command": "C:\\MCPs\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "your_password",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true"
      }
    }
  }
}
```

### 3. (Recommended) Use AI-Safe mode

For maximum safety in production, combine:

```env
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
```

This allows the AI to read everything but only modify the tables you explicitly whitelist.

**Full guides (including Windows Auth, dynamic connections, legacy SQL Server, and Grok Build setup):** [Documentation](https://mcp-go-mssql.scopweb.com)

---

## Quick Start (Development / From Source)

```bash
git clone https://github.com/scopweb/mcp-go-mssql.git
cd mcp-go-mssql

# Build
go build -ldflags "-w -s" -o mcp-go-mssql.exe

# Run with your .env (see .env.example)
go run main.go
```

See the full development guide in the [documentation](https://mcp-go-mssql.scopweb.com/despliegue/desarrollo).

## Configuration

### Claude Desktop Configuration Examples

**AI-Safe Production Configuration (RECOMMENDED for AI Assistants):**
```json
{
  "mcpServers": {
    "production-db-ai-safe": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "secure_password",
        "MSSQL_PORT": "1433",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

**Standard Production Configuration:**
```json
{
  "mcpServers": {
    "production-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
          "MSSQL_AUTH": "sql",
          "MSSQL_USER": "user",
          "MSSQL_PASSWORD": "password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

**Windows Integrated Authentication (SSPI - Named Pipes):**

*Option 1: Access a specific database:*
```json
{
  "mcpServers": {
    "production-db-windows-auth": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

*Option 2: Access all databases (no database specified):*
```json
{
  "mcpServers": {
    "production-db-windows-auth-all": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": ".",
        "MSSQL_AUTH": "integrated",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

> ℹ️ **Windows Auth Note:** Uses current Windows user credentials automatically (no passwords needed). `MSSQL_SERVER="."` for local server, `"localhost"`, or server hostname for remote servers. `MSSQL_DATABASE` is optional - if omitted, connects to user's default database. Works with Active Directory and local Windows accounts. See the project website for [Windows Authentication Guide](https://mcp-go-mssql.scopweb.com/configuracion/autenticacion-windows/).

**Legacy SQL Server (Custom Connection String):**
```json
{
  "mcpServers": {
    "legacy-db": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_CONNECTION_STRING": "sqlserver://sa:YourPassword@legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

### Environment Variables

All database connections use environment variables for security. See `.env.example` for complete configuration examples.

**Required Variables (when not using custom connection string):**
- `MSSQL_SERVER`: SQL Server hostname or IP address
- `MSSQL_AUTH`: Authentication mode (`sql`, `integrated`/`windows`, or `azure`)

**For SQL Server Authentication (`MSSQL_AUTH=sql` or not set):**
- `MSSQL_DATABASE`: Database name to connect to (required)
- `MSSQL_USER`: Username for SQL Server authentication (required)
- `MSSQL_PASSWORD`: Password for SQL Server authentication (required)

**For Windows Integrated Authentication (`MSSQL_AUTH=integrated` or `windows`):**
- `MSSQL_DATABASE`: Database name (optional - if omitted, connects to default database for the Windows user)
- No `MSSQL_USER` or `MSSQL_PASSWORD` needed - uses Windows credentials automatically

**Optional Variables:**
- `MSSQL_PORT`: SQL Server port (default: 1433)
- `MSSQL_ENCRYPT`: Override encryption setting (`"true"` or `"false"`)
- `MSSQL_CONNECTION_STRING`: **Complete custom connection string** (overrides all other MSSQL_* settings)
- `MSSQL_AUTH`: Authentication mode for connecting to SQL Server. Supported values:
  - `sql` (default) - SQL Server authentication using `MSSQL_USER` and `MSSQL_PASSWORD`. Requires `MSSQL_DATABASE`.
  - `integrated` or `windows` - Windows Integrated Authentication (SSPI). Only supported on Windows; the process runs under the current Windows user's credentials and must have proper DB permissions. `MSSQL_DATABASE` is optional - if omitted, connects to the user's default database. **Key benefits:** No passwords in config files, uses Active Directory/Windows security, seamless single sign-on.
  - `azure` - Azure Active Directory authentication (advanced; may require additional config and is not fully implemented by default).
- `MSSQL_READ_ONLY`: **Security restriction** (`"true"` allows only SELECT queries, `"false"` allows all operations)
- `MSSQL_WHITELIST_TABLES`: **Granular permissions** (comma-separated list of tables/views allowed for modification when `MSSQL_READ_ONLY=true`)
  - Example: `"temp_ai,v_temp_ia"`
  - Enables AI to modify specific tables while protecting production data
  - Validates ALL tables in queries (including JOINs, subqueries, CTEs)
  - See the project website for [Whitelist Security](https://mcp-go-mssql.scopweb.com/seguridad/whitelist-tablas/) for details
- `DEVELOPER_MODE`:
  - `"true"`: Development mode (detailed errors, allows self-signed certificates, disables encryption by default)
  - `"false"`: Production mode (generic errors, strict certificate validation, forces encryption)

**🔧 Custom Connection String Priority:**
When `MSSQL_CONNECTION_STRING` is set, all other `MSSQL_*` variables are ignored except `DEVELOPER_MODE`.

**Environment Setup Examples:**
```bash
# Azure SQL Database (Production)
MSSQL_SERVER=your-server.database.windows.net
MSSQL_DATABASE=YourAzureDB
MSSQL_USER=your_user@your-server
MSSQL_PASSWORD=your_secure_password
DEVELOPER_MODE=false

# Local Development (No Encryption)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
DEVELOPER_MODE=true

# Local Development (Force Encryption)
MSSQL_SERVER=localhost
MSSQL_DATABASE=DevDB
MSSQL_USER=dev_user
MSSQL_PASSWORD=dev_password
MSSQL_ENCRYPT=true
DEVELOPER_MODE=true

# Legacy SQL Server (e.g., SQL Server 2008) - Custom Connection String
MSSQL_CONNECTION_STRING=sqlserver://sa:YourPassword@legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
DEVELOPER_MODE=true

# Read-Only Mode (Security Restricted)
MSSQL_SERVER=server.example.com
MSSQL_DATABASE=MyDatabase
MSSQL_USER=readonly_user
MSSQL_PASSWORD=readonly_password
MSSQL_READ_ONLY=true
MSSQL_MAX_QUERY_SIZE=2097152
DEVELOPER_MODE=false

# Windows Integrated Authentication (SSPI) - runs under the current Windows user
# Example 1: Connect to specific database
MSSQL_AUTH=integrated
MSSQL_SERVER=localhost
MSSQL_DATABASE=YourDatabase
DEVELOPER_MODE=false

# Example 2: Connect to default database (database name optional)
MSSQL_AUTH=integrated
MSSQL_SERVER=.
DEVELOPER_MODE=true

# Example 3: Remote server with domain authentication
MSSQL_AUTH=integrated
MSSQL_SERVER=SQL-SERVER.company.local
MSSQL_DATABASE=ProductionDB
DEVELOPER_MODE=false

# AI-Safe Mode with Whitelist (RECOMMENDED for AI Assistants)
MSSQL_SERVER=prod-server.database.windows.net
MSSQL_DATABASE=ProductionDB
MSSQL_USER=ai_user
MSSQL_PASSWORD=secure_password
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,v_temp_ia
DEVELOPER_MODE=false
```

## Security Features

### Database Access Control
- **Granular table permissions** with whitelist system:
  - Validate ALL tables in queries (FROM, JOIN, subqueries, CTEs)
  - Block unauthorized access even through complex SQL patterns
  - Perfect for AI assistants accessing production databases
  - Example: `DELETE temp_ai FROM temp_ai JOIN users` → BLOCKED if `users` not whitelisted
  - See [WHITELIST_SECURITY.md](WHITELIST_SECURITY.md) for complete guide
- **Read-only mode** for query-only access
- **Prepared statements** to prevent SQL injection
- **Input validation** and sanitization

### Connection Security
- **Configurable TLS encryption** for database connections:
  - Production: Forces encryption (`encrypt=true`)
  - Development: Allows disabling encryption for local SQL Server instances
- **Flexible certificate validation**:
  - Production: Strict certificate validation (`trustservercertificate=false`)
  - Development: Allows self-signed certificates (`trustservercertificate=true`)
- **Connection pooling** with resource limits
- **Secure error handling** with production/development modes

## Requirements

- Go 1.26+
- Microsoft SQL Server with TLS support
- Network access to SQL Server (port 1433)

## Troubleshooting

### Connection Issues

**TLS Certificate Issues:**
```
Error: "certificate signed by unknown authority"
Solution: Set DEVELOPER_MODE=true for self-signed certificates
```

**Encryption Issues:**
```
Error: "SSL Provider: No credentials are available in the security package"
Solution: Set DEVELOPER_MODE=true to disable encryption for local SQL Server
```

**Force No Encryption (Development):**
```bash
# For local SQL Server without TLS
DEVELOPER_MODE=true
# This automatically sets encrypt=false for development
```

**TLS Handshake Issues (Legacy SQL Server):**
```
Error: "TLS Handshake failed: tls: server selected unsupported protocol version"
Solution: Use custom connection string with URL format for SQL Server 2008/2012
```

**Connection String Formats:**

**Standard Format (Modern SQL Server 2014+):**
```bash
# Automatically used when individual variables are set
MSSQL_SERVER=server.example.com
MSSQL_DATABASE=MyDatabase
MSSQL_USER=username
MSSQL_PASSWORD=password
DEVELOPER_MODE=true
```

**URL Format (Legacy SQL Server 2008-2012):**
```bash
# Use this for older SQL Server versions
MSSQL_CONNECTION_STRING=sqlserver://username:password@server:1433?database=dbname&encrypt=disable&trustservercertificate=true
DEVELOPER_MODE=true
```

**No Encryption (Development):**
```bash
# For local SQL Server without TLS
DEVELOPER_MODE=true
# This automatically sets encrypt=false for development
```

### Connection Test
```bash
# Make sure environment variables are set first
cd test
go run test-connection.go

# For debugging connection issues
cd debug
go run debug-connection.go
```

### Security Notes
- ⚠️ **Never commit `.env` or `config.json` files** with real credentials
- ✅ **Always use environment variables** for sensitive data
- 🔒 **Use strong passwords** and enable TLS encryption
- 🏢 **For production**: Set `DEVELOPER_MODE=false` and use valid certificates

### Usage Options

### 🖥️ Claude Desktop (MCP Server)
Use `main.go` as an MCP server with Claude Desktop:

**Available Tools:**
| Tool | Description |
|------|-------------|
| `get_database_info` | Check connection status, encryption, and access mode |
| `query_database` | Execute SQL queries securely (prepared statements) |
| `list_tables` | List all tables and views in the database |
| `describe_table` | Get column structure (supports `schema.table` format) |
| `list_databases` | List all user databases on the server |
| `get_indexes` | Get indexes for a specific table |
| `get_foreign_keys` | Get FK relationships (incoming and outgoing) |
| `list_stored_procedures` | List all stored procedures |
| `execute_procedure` | Execute whitelisted stored procedures |

**Environment Variables for New Features:**
```bash
# Whitelist stored procedures for execute_procedure tool
MSSQL_WHITELIST_PROCEDURES="sp_GetCustomerOrders,sp_GenerateReport"
```

### 💻 Claude Code (CLI Tool)  
Use `claude-code/db-connector.go` directly with Claude Code:

```bash
cd claude-code
go run db-connector.go test                    # Test connection
go run db-connector.go tables                  # List tables
go run db-connector.go query "SELECT ..."      # Execute queries
```

See [claude-code/README.md](claude-code/README.md) for detailed Claude Code integration.

## 📚 Documentation

All documentation is on the [project website](https://mcp-go-mssql.scopweb.com/).

### For Users
- **[AI Usage Guide](https://mcp-go-mssql.scopweb.com/guias/uso-con-ia/)** - How Claude/AI works with security restrictions
- **[Windows Authentication Guide](https://mcp-go-mssql.scopweb.com/configuracion/autenticacion-windows/)** - Setup and troubleshooting for Windows Integrated Auth (SSPI)
- **[Whitelist Security](https://mcp-go-mssql.scopweb.com/seguridad/whitelist-tablas/)** - Configure granular table permissions

### For Developers
- **[CLAUDE.md](CLAUDE.md)** - Project documentation for Claude Code

### For Security
- **[Security Audit](https://mcp-go-mssql.scopweb.com/seguridad/auditoria/)** - Security assessment and findings
- **[SECURITY.md](SECURITY.md)** - Security policy and vulnerability reporting

## Project Structure

```
mcp-go-mssql/
├── main.go                          # MCP server for Claude Desktop
├── build.bat                        # Windows build script
├── cli/                             # CLI database tool
│   ├── db-connector.go             # CLI database tool
│   └── README.md                   # CLI documentation
├── internal/                        # Internal packages
├── test/                            # Tests
│   ├── security/                   # Security test suite
│   └── test-connection.go          # Connection testing
├── website/                         # Starlight documentation site
├── scripts/                         # Utility scripts
├── .env.example                    # Environment variables template
├── config.example.json             # Claude Desktop config template
├── CLAUDE.md                       # Claude Code project documentation
└── README.md                       # This file
```

## License

This project is designed for secure database connectivity in critical environments.