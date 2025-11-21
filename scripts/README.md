# Build and Test Scripts

This directory contains all build and testing scripts for the mcp-go-mssql project.

## Build Scripts

### Windows
```bash
.\scripts\build.bat
```
- Checks Go installation
- Creates `build/` directory
- Downloads dependencies (`go mod tidy`)
- Compiles optimized production binary
- Output: `build/mcp-go-mssql.exe`

### Linux / macOS
```bash
bash scripts/build.sh
```
- Checks Go installation
- Creates `build/` directory
- Downloads dependencies (`go mod tidy`)
- Compiles optimized production binary
- Output: `build/mcp-go-mssql`

## Test Scripts

### Connection Testing (Windows PowerShell)
```powershell
.\scripts\test-mcp-server.ps1
```
Tests the MCP server connection and basic functionality.

### Connection Timing Test (Windows PowerShell)
```powershell
.\scripts\test-connection-timing.ps1
```
Measures connection performance and timing characteristics.

### Persistent Connection Test (Windows PowerShell)
```powershell
.\scripts\test-persistent-mcp.ps1
```
Tests persistent connections and long-running scenarios.

### Connection Testing (Linux / macOS)
```bash
bash scripts/test-mcp-server.sh
```
Tests the MCP server connection on Unix-like systems.

## Dependency Management

### Update Dependencies (Windows)
```bash
.\scripts\update-deps.bat
```
Updates all Go dependencies to their latest compatible versions.

## Environment Setup

Before running any scripts, ensure:

1. **Go is installed** (version 1.24.9 or later)
   ```bash
   go version
   ```

2. **Environment variables are set** (for tests)
   ```bash
   # Windows PowerShell
   $env:MSSQL_SERVER = "your-server"
   $env:MSSQL_DATABASE = "your-database"
   $env:MSSQL_USER = "your-user"
   $env:MSSQL_PASSWORD = "your-password"

   # Linux / macOS
   export MSSQL_SERVER="your-server"
   export MSSQL_DATABASE="your-database"
   export MSSQL_USER="your-user"
   export MSSQL_PASSWORD="your-password"
   ```

3. **Database credentials are correct**
   - Verify SQL Server is accessible
   - Confirm authentication mode (SQL Server authentication)
   - Test connection manually if needed

## Build Output

After building successfully, the executable will be located at:
- **Windows:** `build/mcp-go-mssql.exe`
- **Linux/macOS:** `build/mcp-go-mssql`

### Using with Claude Desktop

1. Copy the executable to a stable location:
   ```bash
   cp build/mcp-go-mssql.exe "C:\path\to\claude\servers\"
   ```

2. Update Claude Desktop configuration (`claude_desktop_config.json`):
   ```json
   {
     "mcpServers": {
       "mcp-go-mssql": {
         "command": "C:\\path\\to\\claude\\servers\\mcp-go-mssql.exe",
         "env": {
           "MSSQL_SERVER": "your-server.database.windows.net",
           "MSSQL_DATABASE": "YourDatabase",
           "MSSQL_USER": "your_user",
           "MSSQL_PASSWORD": "your_password",
           "DEVELOPER_MODE": "false"
         }
       }
     }
   }
   ```

3. Restart Claude Desktop

## Troubleshooting

### Build Fails: "go: command not found"
- Ensure Go is installed and in your PATH
- Verify installation: `go version`
- Restart your terminal after installing Go

### Build Fails: Permission Denied (Linux/macOS)
```bash
chmod +x scripts/build.sh
```

### Test Scripts Won't Run (Windows)
PowerShell execution policy may be blocking scripts:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Connection Test Fails
- Verify environment variables are set correctly
- Check SQL Server is running and accessible
- Confirm firewall allows connections on port 1433
- Test manually: `telnet your-server 1433`

## CI/CD Integration

These scripts can be integrated into GitHub Actions or other CI/CD systems:

```yaml
# Example: GitHub Actions
- name: Build
  run: bash scripts/build.sh

- name: Run Tests
  run: go test ./...
  env:
    MSSQL_SERVER: ${{ secrets.MSSQL_SERVER }}
    MSSQL_DATABASE: ${{ secrets.MSSQL_DATABASE }}
    MSSQL_USER: ${{ secrets.MSSQL_USER }}
    MSSQL_PASSWORD: ${{ secrets.MSSQL_PASSWORD }}
```

## See Also

- [CLAUDE.md](../CLAUDE.md) - Development instructions
- [README.md](../README.md) - Project documentation
- [go.mod](../go.mod) - Go module dependencies
