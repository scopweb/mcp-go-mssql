package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mcp-go-mssql/internal/sqlguard"
)

// handleDynamicConnect connects to a pre-configured dynamic database connection
func (s *MCPMSSQLServer) handleDynamicConnect(id interface{}, params CallToolParams) *MCPResponse {
	// Connect to a pre-configured dynamic connection from .env
	// No credentials needed - they are loaded from MSSQL_DYNAMIC_<ALIAS>_XXX env vars
	alias, ok := params.Arguments["alias"].(string)
	if !ok || alias == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Missing or invalid 'alias' parameter",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Build connection string from MSSQL_DYNAMIC_<ALIAS>_XXX env vars
	prefix := "MSSQL_DYNAMIC_" + strings.ToUpper(alias) + "_"
	server := os.Getenv(prefix + "SERVER")
	database := os.Getenv(prefix + "DATABASE")
	user := os.Getenv(prefix + "USER")
	password := os.Getenv(prefix + "PASSWORD")
	portStr := os.Getenv(prefix + "PORT")
	auth := os.Getenv(prefix + "AUTH")

	isIntegratedAuth := strings.ToLower(auth) == "integrated" || strings.ToLower(auth) == "windows"

	// Validate: for integrated auth, server and database are enough; otherwise need all 4
	if server == "" || database == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Dynamic connection '%s' not found or incomplete in configuration. Use dynamic_list to see available connections.", alias),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}
	if !isIntegratedAuth && (user == "" || password == "") {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Dynamic connection '%s' requires MSSQL_DYNAMIC_%s_USER and MSSQL_DYNAMIC_%s_PASSWORD (or use MSSQL_DYNAMIC_%s_AUTH=integrated for Windows authentication).", alias, strings.ToUpper(alias), strings.ToUpper(alias), strings.ToUpper(alias)),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	port := 1433
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	encrypt := "true"
	if s.devMode {
		encrypt = "false"
	}
	trustCert := "false"
	if s.devMode {
		trustCert = "true"
	}

	var connStr string
	if isIntegratedAuth {
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;encrypt=%s;trustservercertificate=%s;integrated security=SSPI;connection timeout=30;command timeout=30",
			server, port, database, encrypt, trustCert,
		)
	} else {
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
			server, port, database, user, password, encrypt, trustCert,
		)
	}

	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Failed to open connection: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Failed to connect: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	// Load per-connection security config. Cross-database is intentionally
	// NOT supported here — a dynamic connection points to a single DB and
	// queries crossing into other DBs from inside it would defeat the
	// per-connection policy model.
	connInfo := &ConnectionInfo{
		Alias:                alias,
		DB:                   db,
		Server:               server,
		Database:             database,
		User:                 user,
		CreatedAt:            time.Now(),
		readOnly:             strings.ToLower(os.Getenv(prefix+"READ_ONLY")) == "true",
		autopilot:            strings.ToLower(os.Getenv(prefix+"AUTOPILOT")) == "true",
		skipSchemaValidation: strings.ToLower(os.Getenv(prefix+"SKIP_SCHEMA_VALIDATION")) == "true",
	}

	if wl := os.Getenv(prefix + "WHITELIST_TABLES"); wl != "" {
		connInfo.whitelistTables = sqlguard.ParseWhitelistTables(wl)
	}

	// Build the per-connection guard so subsequent query_database calls
	// against this alias enforce its own policy instead of falling back
	// to the server-wide one.
	connInfo.guard = sqlguard.New(sqlguard.Config{
		ReadOnly:  connInfo.readOnly,
		Whitelist: connInfo.whitelistTables,
		Logger:    s.secLogger,
	})

	s.addDynamicConnectionInfo(alias, connInfo)
	s.secLogger.Printf("Dynamic connection '%s' activated: %s/%s (read_only=%v autopilot=%v skip_schema=%v whitelist=%v)",
		alias, server, database, connInfo.readOnly, connInfo.autopilot, connInfo.skipSchemaValidation, connInfo.whitelistTables)

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("Connected to '%s' (%s/%s)", alias, server, database),
					Annotations: annBothQuery,
				},
			},
		},
	}
}

// handleDynamicList lists all active dynamic database connections
func (s *MCPMSSQLServer) handleDynamicList(id interface{}, params CallToolParams) *MCPResponse {
	connections := s.listDynamicConnections()
	if len(connections) == 0 {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "No active dynamic connections. Use dynamic_connect to establish one.",
						Annotations: annBothQuery,
					},
				},
			},
		}
	}

	var info strings.Builder
	info.WriteString("Active dynamic connections:\n")
	for _, conn := range connections {
		age := time.Since(conn.CreatedAt).Round(time.Second)
		info.WriteString(fmt.Sprintf("- %s (%s/%s) - connected %s ago\n", conn.Alias, conn.Server, conn.Database, age))
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        info.String(),
					Annotations: annBothQuery,
				},
			},
		},
	}
}

// handleDynamicAvailable lists all pre-configured dynamic connections from .env
func (s *MCPMSSQLServer) handleDynamicAvailable(id interface{}, params CallToolParams) *MCPResponse {
	// List all pre-configured connections from .env file directly (without connecting)
	// This helps the AI discover what aliases are available without exposing credentials
	var info strings.Builder
	found := false
	info.WriteString("Available dynamic connections (use dynamic_connect to activate):\n")

	// Find .env - check next to executable first, then current directory
	envPath := ".env"
	exeDir := getExecutableDir()
	if exeDir != "" {
		if execEnvPath := filepath.Join(exeDir, ".env"); fileExists(execEnvPath) {
			envPath = execEnvPath
		}
	}

	// Read .env file directly to discover configured aliases
	envFile, err := os.Open(envPath)
	if err == nil {
		defer envFile.Close()
		scanner := bufio.NewScanner(envFile)
		knownAliases := make(map[string]bool)
		for scanner.Scan() {
			line := scanner.Text()
			// Parse KEY=VALUE lines
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Track which aliases have SERVER defined
				if strings.HasPrefix(key, "MSSQL_DYNAMIC_") && strings.HasSuffix(key, "_SERVER") && value != "" {
					alias := strings.TrimPrefix(strings.TrimSuffix(key, "_SERVER"), "MSSQL_DYNAMIC_")
					knownAliases[alias] = true
				}
			}
		}
		// Now output aliases that have SERVER configured
		for alias := range knownAliases {
			dbName := ""
			serverVal := ""
			// Re-scan to get DATABASE for this alias (we need to store it from scan above)
			envFile.Seek(0, 0)
			scanner := bufio.NewScanner(envFile)
			for scanner.Scan() {
				line := scanner.Text()
				if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key == "MSSQL_DYNAMIC_"+alias+"_DATABASE" {
						dbName = value
					}
					if key == "MSSQL_DYNAMIC_"+alias+"_SERVER" {
						serverVal = value
					}
				}
			}
			if serverVal != "" {
				info.WriteString(fmt.Sprintf("- %s (%s/%s)\n", alias, serverVal, dbName))
				found = true
			}
		}
	}

	if !found {
		info.WriteString("No dynamic connections configured. Add MSSQL_DYNAMIC_<ALIAS>_SERVER to .env")
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        info.String(),
					Annotations: annBothQuery,
				},
			},
		},
	}
}

// handleDynamicDisconnect closes a dynamic database connection
func (s *MCPMSSQLServer) handleDynamicDisconnect(id interface{}, params CallToolParams) *MCPResponse {
	alias, ok := params.Arguments["alias"].(string)
	if !ok || alias == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Missing or invalid 'alias' parameter",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	if err := s.removeDynamicConnection(alias); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        err.Error(),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	s.secLogger.Printf("Dynamic connection '%s' closed", alias)

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("Disconnected '%s'", alias),
					Annotations: annBothQuery,
				},
			},
		},
	}
}