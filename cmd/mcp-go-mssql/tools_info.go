package main

import (
	"fmt"
	"os"
	osuser "os/user"
	"strings"
)

// get_database_info.

// toolGetDatabaseInfo handles the get_database_info tool.
func (s *MCPMSSQLServer) toolGetDatabaseInfo(id interface{}, params CallToolParams) *MCPResponse {
	var info strings.Builder
	if s.isDynamic {
		fmt.Fprintf(&info, "MCP Server Process ID: %d\n", os.Getpid())
		info.WriteString("Dynamic connection lifetime: this server process. The selected alias persists across tool calls and turns, but not across process restarts.\n")
	}

	if s.getDB() == nil && s.isDynamic {
		info.WriteString("Database Status: DISCONNECTED\n\n")
		info.WriteString("No active dynamic database connection in this process. Use dynamic_available to list configured aliases, then dynamic_connect with the desired alias.\n")
		info.WriteString("If an alias was previously connected, check whether the MCP host restarted/replaced the server process or dynamic_disconnect was called. Pool idle/lifetime limits do not clear the selected alias.\n")
	} else if s.getDB() == nil {
		info.WriteString("Database Status: DISCONNECTED\n\n")

		// Show current configuration so Claude can diagnose
		info.WriteString("=== Current Configuration ===\n")
		if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
			info.WriteString("Connection: Custom connection string (MSSQL_CONNECTION_STRING)\n")
		} else {
			server := os.Getenv("MSSQL_SERVER")
			if server == "" {
				info.WriteString("MSSQL_SERVER: NOT SET (required)\n")
			} else {
				info.WriteString("MSSQL_SERVER: " + server + "\n")
			}
			database := os.Getenv("MSSQL_DATABASE")
			if database != "" {
				info.WriteString("MSSQL_DATABASE: " + database + "\n")
			} else {
				info.WriteString("MSSQL_DATABASE: not set\n")
			}
			auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
			if auth == "" {
				auth = "sql"
			}
			info.WriteString("MSSQL_AUTH: " + auth + "\n")
			if auth == "sql" { //nolint:staticcheck // QF1003: switch would be slightly cleaner but if-else is fine here
				if os.Getenv("MSSQL_USER") == "" {
					info.WriteString("MSSQL_USER: NOT SET (required for SQL auth)\n")
				} else {
					info.WriteString("MSSQL_USER: " + os.Getenv("MSSQL_USER") + "\n")
				}
				if os.Getenv("MSSQL_PASSWORD") == "" {
					info.WriteString("MSSQL_PASSWORD: NOT SET (required for SQL auth)\n")
				} else {
					info.WriteString("MSSQL_PASSWORD: ***\n")
				}
			} else if auth == "integrated" || auth == "windows" {
				if u, err := osuser.Current(); err == nil {
					info.WriteString("Windows User: " + u.Username + "\n")
				}
			}
			port := os.Getenv("MSSQL_PORT")
			if port == "" {
				port = "1433"
			}
			info.WriteString("MSSQL_PORT: " + port + "\n")
			info.WriteString("DEVELOPER_MODE: " + os.Getenv("DEVELOPER_MODE") + "\n")
			encryptVal := os.Getenv("MSSQL_ENCRYPT")
			if encryptVal != "" {
				info.WriteString("MSSQL_ENCRYPT: " + encryptVal + "\n")
			}
			if isDynamicMode() {
				info.WriteString("DYNAMIC_MODE: true (dynamic_* tools available)\n")
				info.WriteString("NOTE: Per-alias security contexts are in development. Current ops use the (guarded) global posture above.\n")
			}
		}

		// Diagnostic hints for Claude to suggest fixes
		info.WriteString("\n=== Possible Causes ===\n")
		if os.Getenv("MSSQL_SERVER") == "" && os.Getenv("MSSQL_CONNECTION_STRING") == "" {
			info.WriteString("- MSSQL_SERVER environment variable is not set\n")
		} else {
			auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
			devMode := strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true"
			encrypt := "true"
			if devMode {
				if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
					encrypt = strings.ToLower(envEncrypt)
				} else {
					encrypt = "false"
				}
			}

			if auth == "sql" || auth == "" {
				if os.Getenv("MSSQL_USER") == "" || os.Getenv("MSSQL_PASSWORD") == "" {
					info.WriteString("- Missing MSSQL_USER or MSSQL_PASSWORD for SQL authentication\n")
				}
			}
			if encrypt == "true" {
				info.WriteString("- TLS encryption is ENABLED. If the server is SQL Server 2008/2012 or doesn't have TLS certificates, set MSSQL_ENCRYPT=false with DEVELOPER_MODE=true\n")
			}
			if !devMode {
				info.WriteString("- Production mode requires valid TLS certificates. For internal/dev servers, set DEVELOPER_MODE=true\n")
			}
			if auth == "integrated" || auth == "windows" {
				info.WriteString("- Windows Integrated Auth: verify the Windows user has SQL Server login permissions\n")
				info.WriteString("- Check that SQL Server allows Windows Authentication mode\n")
				info.WriteString("- For remote servers, verify Active Directory connectivity\n")
			}
			info.WriteString("- Verify the server is reachable and SQL Server service is running\n")
			info.WriteString("- Check firewall rules allow connections on the configured port\n")
		}
	} else {
		info.WriteString("Database Status: Connected\n")

		s.dynamicMu.RLock()
		activeAlias := s.activeAlias
		s.dynamicMu.RUnlock()

		if activeAlias != "" {
			// Dynamic mode - show the active alias and its specific posture
			if alias, ok := s.dynamicAliases[activeAlias]; ok {
				effective := s.getEffectiveConfig()
				fmt.Fprintf(&info, "Active Dynamic Alias: %s\n", activeAlias)
				fmt.Fprintf(&info, "  Server/Database: %s / %s\n", alias.Server, alias.Database)
				ro := "READ-ONLY"
				if !effective.readOnly {
					if len(effective.whitelistTables) > 0 {
						ro = "WRITABLE (whitelist-restricted)"
					} else {
						ro = "WRITABLE (FULL ACCESS - no whitelist)"
					}
				}
				fmt.Fprintf(&info, "  Security Posture: %s\n", ro)
				if !effective.readOnly && len(effective.whitelistTables) > 0 {
					fmt.Fprintf(&info, "  Allowed modification tables: %s\n", strings.Join(effective.whitelistTables, ", "))
				}
			}
		} else if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
			info.WriteString("Connection: Custom connection string\n")
			info.WriteString("Mode: " + func() string {
				if os.Getenv("DEVELOPER_MODE") == "true" {
					return "Development"
				} else {
					return "Production"
				}
			}() + "\n")
		} else {
			info.WriteString("Server: " + os.Getenv("MSSQL_SERVER") + "\n")
			info.WriteString("Database: " + os.Getenv("MSSQL_DATABASE") + "\n")
			encrypt := "Enabled (TLS)"
			if os.Getenv("DEVELOPER_MODE") == "true" && os.Getenv("MSSQL_ENCRYPT") != "true" {
				encrypt = "Disabled (Development)"
			}
			info.WriteString("Encryption: " + encrypt + "\n")
		}

		// Show read-only status and whitelist (cached config)
		whitelist := s.getWhitelistedTables()
		if s.config.readOnly {
			info.WriteString("Access Mode: READ-ONLY (all modifications and stored procedures blocked)\n")
			if len(whitelist) > 0 {
				info.WriteString("Whitelisted Tables: configured but inactive while READ_ONLY=true\n")
			} else {
				info.WriteString("Whitelisted Tables: NONE\n")
			}
		} else {
			if len(whitelist) > 0 {
				info.WriteString("Access Mode: Whitelist-protected (modifications restricted)\n")
				info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
				info.WriteString("Note: Only whitelisted tables can be modified. All other tables are read-only.\n")
			} else {
				info.WriteString("Access Mode: Full access\n")
			}
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: info.String(),
				},
			},
		},
	}
}
