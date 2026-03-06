---
title: Windows Authentication (SSPI)
description: Configuring Windows integrated authentication for MCP-Go-MSSQL
---

Windows integrated authentication (SSPI) allows connecting without specifying a username and password, using the current Windows user's credentials.

## Requirements

- SQL Server configured to accept Windows Authentication
- The Windows user must have database access
- Only works in Windows environments

## Configuration

```bash
MSSQL_AUTH=integrated
MSSQL_SERVER=your-server
MSSQL_DATABASE=YourDatabase
```

There is no need to set `MSSQL_USER` or `MSSQL_PASSWORD`.

## Generated connection string

```
server=your-server;database=YourDatabase;integrated security=sspi;encrypt=true
```

## Claude Desktop

```json
{
  "mssql-windows": {
    "command": "mcp-go-mssql.exe",
    "env": {
      "MSSQL_SERVER": "your-server",
      "MSSQL_DATABASE": "YourDatabase",
      "MSSQL_AUTH": "integrated",
      "DEVELOPER_MODE": "false",
      "MSSQL_READ_ONLY": "true"
    }
  }
}
```

## Troubleshooting

- **Login failed**: Verify that the Windows user has a login in SQL Server
- **SSPI handshake failed**: Verify that SQL Server accepts Windows Authentication in its configuration
- **Cannot generate SSPI context**: May indicate DNS or Kerberos issues in the domain
