# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **secure** Go-based MCP (Model Context Protocol) server implementation that provides MSSQL database connectivity for critical data environments. The application serves as a hardened bridge between MCP clients and Microsoft SQL Server databases, with comprehensive security features including TLS encryption, rate limiting, input validation, and IP whitelisting.

## Architecture

The codebase implements a security-first architecture with these key components:

- **Config/Configs structs**: Define configuration with security parameters (TLS, rate limiting, IP restrictions)
- **SecurityLogger**: Dedicated security event logging with sanitization
- **RateLimiter**: IP-based rate limiting to prevent abuse
- **MCPServer struct**: Handles secure server instances with input validation and SQL injection protection
- **Connection security**: TLS support, connection timeouts, and IP whitelisting
- **Database security**: Encrypted connections, connection pooling, and prepared statement support

## Security Features

### Database Security
- **Mandatory encryption**: Database connections FORCE TLS (encrypt=true, trustservercertificate=false)
- **Connection pooling**: Limited connection counts to prevent resource exhaustion
- **SQL Injection Protection**: Uses prepared statements exclusively - NO dynamic SQL
- **Secure error handling**: Generic error messages to clients, detailed logs internally

### Network Security
- **TLS encryption**: Optional TLS for client connections
- **IP whitelisting**: Restrict access to specific client IPs
- **Rate limiting**: Configurable requests per second per IP
- **Connection timeouts**: Prevent hanging connections

### Logging Security
- **Security event logging**: Dedicated security logger for all security events
- **Data sanitization**: Automatic removal of sensitive data from logs
- **Connection tracking**: Log all connection attempts with IP addresses

## Development Commands

### Initial Setup
```bash
go mod init mcp-go-mssql
go mod tidy
```

### Build
```bash
go build
```

### Development Mode
```bash
# Development with detailed errors
# Set "developer_mode": true in config.json
go run main.go

# Production mode (default)
# Set "developer_mode": false in config.json  
go run main.go
```

### Production Deployment
```bash
# Build for production
go build -ldflags "-w -s" -o mcp-go-mssql-secure

# Run with production config
./mcp-go-mssql-secure
```

## Secure Configuration

### MCP Protocol Implementation
This server now implements the proper MCP (Model Context Protocol) using stdin/stdout JSON-RPC communication, compatible with Claude Desktop.

### For Claude Desktop Integration
1. Copy `config.example.json` to `config.json` and customize for your environment
2. **NEVER commit config.json to version control**
3. Place the compiled `mcp-go-mssql.exe` in the same directory as your config.json
4. Configuration uses environment variables passed through Claude Desktop

### Environment Variables
The server reads database connection from these environment variables:
- `MSSQL_SERVER`: SQL Server hostname/IP
- `MSSQL_DATABASE`: Database name
- `MSSQL_USER`: Username for connection
- `MSSQL_PASSWORD`: Password for connection
- `MSSQL_PORT`: Port (defaults to 1433)
- `DEVELOPER_MODE`: "true" for detailed errors and relaxed TLS certificate validation, "false" for production

### TLS Certificate Handling
- **Production Mode** (`DEVELOPER_MODE=false`): Requires valid, trusted TLS certificates
- **Development Mode** (`DEVELOPER_MODE=true`): Allows self-signed or untrusted certificates
- **Always Encrypted**: All database connections use TLS encryption regardless of mode

### Claude Desktop Configuration Example:
```json
{
  "mcpServers": {
    "production-db": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "your_user",
        "MSSQL_PASSWORD": "your_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "false"
      }
    },
    "dev-db": {
      "command": "mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "dev-server.local",
        "MSSQL_DATABASE": "DevDatabase",
        "MSSQL_USER": "dev_user",
        "MSSQL_PASSWORD": "dev_password",
        "MSSQL_PORT": "1433",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

### Critical Security Parameters:
- `DEVELOPER_MODE`: 
  - `"false"` for production: Strict TLS certificate validation, generic error messages
  - `"true"` for development: Allows untrusted certificates, detailed error messages
- **Database Encryption**: Always uses TLS encryption (`encrypt=true`)
- **Certificate Validation**: 
  - Production: `trustservercertificate=false` (requires valid certificates)
  - Development: `trustservercertificate=true` (allows self-signed certificates)

## Security Best Practices

### Configuration Security
1. **Environment Variables**: Use environment variables for sensitive data
2. **File Permissions**: 
   - **Windows**: Use `icacls config.json /inheritance:r /grant:r "%USERNAME%:R"` 
   - **Linux/Unix**: Use `chmod 600 config.json`
3. **Credential Rotation**: Regularly rotate database passwords
4. **Network Isolation**: Deploy in secure network segments

### Database Security
1. **Least Privilege**: Use database users with minimal required permissions
2. **Connection Limits**: Set appropriate connection pool limits
3. **Audit Logging**: Enable database audit logs
4. **SSL/TLS**: Always use encrypted database connections

### Deployment Security
1. **Binary Security**: Use stripped binaries (`-ldflags "-w -s"`)
2. **Container Security**: Run in non-root containers when containerized
3. **Network Security**: Use firewalls and network segmentation
4. **Monitoring**: Implement security monitoring and alerting

## Dependencies

- `github.com/denisenkom/go-mssqldb`: Microsoft SQL Server driver with TLS support
- Go standard library: crypto/tls, context, regexp for security features

## IMPORTANT SECURITY NOTES

⚠️  **This application handles critical database data. Always:**
1. Use TLS for all connections (both client and database)
2. Implement proper firewall rules
3. Monitor security logs regularly
4. Keep dependencies updated
5. Test security configurations before production deployment
6. Use strong authentication credentials
7. Implement network segmentation

## Troubleshooting

### TLS Certificate Issues
If you see errors like "certificate signed by unknown authority":

1. **For Development**: Set `DEVELOPER_MODE=true` to allow self-signed certificates
2. **For Production**: 
   - Install proper SSL certificates on your SQL Server
   - Or configure your certificate authority in the client system
   - Never use `DEVELOPER_MODE=true` in production

### Connection Testing
Use the included test utility:
```bash
cd test
go run test-connection.go
```

### Common Issues
- **"Database not connected"**: Check if environment variables are set correctly
- **TLS handshake failed**: Use `DEVELOPER_MODE=true` for self-signed certificates
- **Login failed**: Verify username/password and SQL Server authentication mode
- **Network error**: Check firewall rules and SQL Server port (default 1433)