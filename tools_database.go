package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	osuser "os/user"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mcp-go-mssql/internal/sqlguard"
)

// handleGetDatabaseInfo returns database connection status and configuration info
func (s *MCPMSSQLServer) handleGetDatabaseInfo(id interface{}, params CallToolParams) *MCPResponse {
	var info strings.Builder

	defaultDB := s.getDB()
	dynamicConns := s.listDynamicConnections()
	hasDefaultDB := defaultDB != nil
	hasDynamicConns := len(dynamicConns) > 0

	if !hasDefaultDB && !hasDynamicConns {
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
			if auth == "sql" {
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
		info.WriteString("Database Status: Connected\n\n")
		if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
			info.WriteString("Connection: Custom connection string\n")
			info.WriteString("Mode: " + func() string {
				if os.Getenv("DEVELOPER_MODE") == "true" {
					return "Development"
				} else {
					return "Production"
				}
			}() + "\n")
		} else {
			// Check if this is an auto-connected dynamic alias
			autoConnectedAlias := os.Getenv("MSSQL_DYNAMIC_ACTIVE_ALIAS")
			if autoConnectedAlias != "" {
				info.WriteString("Dynamic Connection: " + autoConnectedAlias + " (auto-connected)\n")
			} else if os.Getenv("MSSQL_DYNAMIC_MODE") == "true" {
				info.WriteString("Dynamic Mode: enabled\n")
			}
			info.WriteString("Server: " + os.Getenv("MSSQL_SERVER") + "\n")
			info.WriteString("Database: " + os.Getenv("MSSQL_DATABASE") + "\n")
			encrypt := "Enabled (TLS)"
			if os.Getenv("DEVELOPER_MODE") == "true" && os.Getenv("MSSQL_ENCRYPT") != "true" {
				encrypt = "Disabled (Development)"
			}
			info.WriteString("Encryption: " + encrypt + "\n")
		}
		if hasDynamicConns {
			info.WriteString("\nDynamic Connections: " + strconv.Itoa(len(dynamicConns)) + " active\n")
			info.WriteString("Aliases: ")
			aliases := make([]string, 0, len(dynamicConns))
			for _, c := range dynamicConns {
				aliases = append(aliases, c.Alias)
			}
			info.WriteString(strings.Join(aliases, ", ") + "\n")
			info.WriteString("\nUse connection=<alias> in query_database to query a specific database.\n")
		}

		// Show read-only status and whitelist (cached config)
		whitelist := s.getWhitelistedTables()
		isWildcard := len(whitelist) == 1 && whitelist[0] == "*"
		if s.config.readOnly {
			if isWildcard {
				info.WriteString("Access Mode: READ-ONLY with wildcard whitelist (*)\n")
				info.WriteString("Whitelisted Tables: * (all tables allowed for modification)\n")
				info.WriteString("Note: SELECT allowed on all tables. Modifications allowed on ALL tables (wildcard).\n")
			} else if len(whitelist) > 0 {
				info.WriteString("Access Mode: READ-ONLY with whitelist exceptions\n")
				info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
				info.WriteString("Note: SELECT allowed on all tables. Modifications (INSERT/UPDATE/DELETE/CREATE/DROP) only allowed on whitelisted tables.\n")
			} else {
				info.WriteString("Access Mode: READ-ONLY (SELECT queries only)\n")
				info.WriteString("Whitelisted Tables: NONE (all modifications blocked)\n")
			}
		} else {
			if isWildcard {
				info.WriteString("Access Mode: Full access (wildcard whitelist — same as no read-only)\n")
			} else if len(whitelist) > 0 {
				info.WriteString("Access Mode: Whitelist-protected (modifications restricted)\n")
				info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
				info.WriteString("Note: Only whitelisted tables can be modified. All other tables are read-only.\n")
			} else {
				info.WriteString("Access Mode: Full access\n")
			}
		}

		// Show allowed cross-databases
		if len(s.config.allowedDatabases) > 0 {
			info.WriteString("Cross-Database Access: " + strings.Join(s.config.allowedDatabases, ", ") + "\n")
			info.WriteString("Note: You can query these databases using 3-part names (e.g., DatabaseName.dbo.TableName). Use explore with database parameter to list their tables.\n")
		}
	}

	// Annotation: diagnostics for the LLM; high priority when disconnected
	ann := annAssistantLow
	if !hasDefaultDB && !hasDynamicConns {
		ann = annAssistantHigh
	}
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        info.String(),
					Annotations: ann,
				},
			},
		},
	}
}

// handleQueryDatabase executes SQL queries against the database
func (s *MCPMSSQLServer) handleQueryDatabase(id interface{}, params CallToolParams) *MCPResponse {
	// Resolve the target connection. Three policy sources can apply:
	//   1) Default DB: server-wide guard (s.guard) and server-wide flags.
	//   2) Dynamic alias: the alias' own guard + per-connection flags.
	// connGuard = nil means "fall back to s.guard" downstream.
	var db *sql.DB
	var connGuard *sqlguard.Guard
	// schemaAutopilot/schemaSkip are the autopilot/skip_schema_validation
	// flags effective for THIS request. They feed the schema-existence
	// check below, which is the only place these two flags matter.
	schemaAutopilot := s.config.autopilot
	schemaSkip := s.config.skipSchemaValidation

	connectionName, hasConnection := params.Arguments["connection"].(string)

	if hasConnection && connectionName != "" {
		// Use dynamic connection (with its own policy).
		connInfo, ok := s.getDynamicConnectionInfo(connectionName)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error: Unknown dynamic connection '%s'. Use dynamic_list to see active connections.", connectionName),
							Annotations: annBothHigh,
						},
					},
					IsError: true,
				},
			}
		}
		db = connInfo.DB
		connGuard = connInfo.guard
		schemaAutopilot = connInfo.autopilot
		schemaSkip = connInfo.skipSchemaValidation
	} else {
		// Use default connection (server-wide policy).
		db = s.getDB()
		if db == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
						},
					},
					IsError: true,
				},
			}
		}
	}

	query, ok := params.Arguments["query"].(string)
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Missing or invalid 'query' parameter",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Best-effort schema validation: check that referenced tables exist.
	// If INFORMATION_SCHEMA is not accessible (permissions), validation is silently skipped.
	// Skipped when AUTOPILOT or SKIP_SCHEMA_VALIDATION is enabled — the flags come from the
	// per-connection policy when a dynamic alias is in use, otherwise from the server-wide config.
	s.secLogger.Printf("SCHEMA_VALIDATION: autopilot=%v skip_schema_validation=%v", schemaAutopilot, schemaSkip)
	if !schemaAutopilot && !schemaSkip {
		if validationErr := s.validateTablesExist(ctx, query); validationErr != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        *validationErr,
							Annotations: annBothHigh,
						},
					},
					IsError: true,
				},
			}
		}
	}

	results, err := s.executeSecureQueryWithDB(ctx, db, connGuard, query)
	if err != nil {
		// Check if this is a confirmation-required error
		if confirmErr, ok := err.(*ConfirmationRequiredError); ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Error: &MCPError{
					Code:    DestructiveConfirmationCode,
					Message: confirmErr.Error(),
					Data: map[string]interface{}{
						"operation":   confirmErr.Operation,
						"target":      confirmErr.Target,
						"token":       confirmErr.Token,
						"expires_in":  confirmErr.ExpiresIn,
						"confirm_url": "use confirm_operation tool with this token to execute",
					},
				},
			}
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Query Error: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// DDL/DML with no result set (ALTER, CREATE, DROP, INSERT, UPDATE, DELETE, etc.)
	if results == nil || len(results) == 0 {
		op := strings.ToUpper(strings.Fields(strings.TrimSpace(query))[0])
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("%s executed successfully. No rows returned.", op),
						Annotations: annBothQuery,
					},
				},
			},
		}
	}

	// Format results as JSON
	resultBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error formatting results: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("Query executed successfully. %d rows returned:\n%s", len(results), string(resultBytes)),
					Annotations: annBothQuery,
				},
			},
		},
	}
}

// handleExplore lists database objects (tables, views, procedures, databases)
func (s *MCPMSSQLServer) handleExplore(id interface{}, params CallToolParams) *MCPResponse {
	// Resolve target DB and guard: use dynamic connection if specified, otherwise default DB
	var db *sql.DB
	var guard *sqlguard.Guard

	if connName, ok := params.Arguments["connection"].(string); ok && connName != "" {
		connInfo, ok := s.getDynamicConnectionInfo(connName)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error: Unknown dynamic connection '%s'. Use dynamic_list to see active connections.", connName),
							Annotations: annBothHigh,
						},
					},
					IsError: true,
				},
			}
		}
		db = connInfo.DB
		guard = connInfo.guard
	} else {
		db = s.getDB()
		guard = nil // nil means executeSecureQueryWithDB falls back to s.guard
		if db == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
						},
					},
					IsError: true,
				},
			}
		}
	}

	exploreType := "tables"
	if t, ok := params.Arguments["type"].(string); ok && t != "" {
		exploreType = t
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var results []map[string]interface{}
	var err error
	var label string

	switch exploreType {
	case "databases":
		label = "Databases found"
		query := `
			SELECT
				name as database_name,
				database_id,
				create_date,
				state_desc as state
			FROM sys.databases
			WHERE database_id > 4
			ORDER BY name
		`
		results, err = s.executeSecureQueryWithDB(ctx, db, guard, query)

	case "procedures":
		label = "Stored procedures found"
		schemaFilter, _ := params.Arguments["schema"].(string)
		filterVal, _ := params.Arguments["filter"].(string)
		if schemaFilter != "" && filterVal != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE SCHEMA_NAME(p.schema_id) = @p1 AND p.name LIKE @p2
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, schemaFilter, "%"+filterVal+"%")
		} else if schemaFilter != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE SCHEMA_NAME(p.schema_id) = @p1
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, schemaFilter)
		} else if filterVal != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE p.name LIKE @p1
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, "%"+filterVal+"%")
		} else {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query)
		}

	case "search":
		pattern, ok := params.Arguments["pattern"].(string)
		if !ok || pattern == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: "Error: 'pattern' is required when type=search", Annotations: annBothHigh},
					},
					IsError: true,
				},
			}
		}
		searchIn, _ := params.Arguments["search_in"].(string)
		likePattern := "%" + pattern + "%"
		if searchIn == "definition" {
			label = fmt.Sprintf("Objects matching '%s' in definition", pattern)
			query := `
				SELECT
					o.type_desc      AS object_type,
					SCHEMA_NAME(o.schema_id) AS schema_name,
					o.name           AS object_name,
					m.definition     AS definition_snippet
				FROM sys.sql_modules m
				JOIN sys.objects     o ON o.object_id = m.object_id
				WHERE m.definition LIKE @p1
				ORDER BY o.type_desc, o.name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, likePattern)
		} else {
			label = fmt.Sprintf("Objects matching '%s' in name", pattern)
			query := `
				SELECT
					o.type_desc      AS object_type,
					SCHEMA_NAME(o.schema_id) AS schema_name,
					o.name           AS object_name,
					o.create_date    AS created,
					o.modify_date    AS modified
				FROM sys.objects o
				WHERE o.name LIKE @p1
				  AND o.type IN ('U','V','P','FN','IF','TF')
				ORDER BY o.type_desc, o.name
			`
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, likePattern)
		}

	case "views":
		label = "Views found"
		viewFilter, _ := params.Arguments["filter"].(string)
		if viewFilter != "" {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v WHERE v.TABLE_NAME LIKE @p1 ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, "%"+viewFilter+"%")
		} else {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			results, err = s.executeSecureQueryWithDB(ctx, db, guard, query)
		}

	default: // "tables"
		// Check if user wants to explore a specific allowed database
		dbFilter, _ := params.Arguments["database"].(string)

		if dbFilter != "" {
			// Explore a specific cross-database
			dbFilterLower := strings.ToLower(strings.Trim(dbFilter, "[] "))
			if !s.isAllowedDatabase(dbFilterLower) {
				allowedList := strings.Join(s.config.allowedDatabases, ", ")
				if allowedList == "" {
					allowedList = "(none configured)"
				}
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      id,
					Result: CallToolResult{
						Content: []ContentItem{
							{
								Type:        "text",
								Text:        fmt.Sprintf("Error: database '%s' is not in MSSQL_ALLOWED_DATABASES. Allowed: %s", dbFilter, allowedList),
								Annotations: annBothHigh,
							},
						},
						IsError: true,
					},
				}
			}
			// Validate database name for safe interpolation
			if !regexp.MustCompile(`^[\w]+$`).MatchString(dbFilterLower) {
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      id,
					Result: CallToolResult{
						Content: []ContentItem{
							{Type: "text", Text: "Error: invalid database name", Annotations: annBothHigh},
						},
						IsError: true,
					},
				}
			}

			label = fmt.Sprintf("Tables and views in [%s]", dbFilter)
			if filterVal, ok := params.Arguments["filter"].(string); ok && filterVal != "" {
				query := fmt.Sprintf(`
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM [%s].INFORMATION_SCHEMA.Tables
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					  AND TABLE_NAME LIKE @p1
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`, dbFilterLower)
				results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, "%"+filterVal+"%")
			} else {
				query := fmt.Sprintf(`
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM [%s].INFORMATION_SCHEMA.Tables
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`, dbFilterLower)
				results, err = s.executeSecureQueryWithDB(ctx, db, guard, query)
			}
		} else {
			// Default: current database + summary of allowed databases
			label = "Tables and views found"
			if filterVal, ok := params.Arguments["filter"].(string); ok && filterVal != "" {
				filterPattern := "%" + filterVal + "%"
				query := `
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM INFORMATION_SCHEMA.Tables
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					  AND TABLE_NAME LIKE @p1
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`
				results, err = s.executeSecureQueryWithDB(ctx, db, guard, query, filterPattern)
			} else {
				query := `
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM INFORMATION_SCHEMA.Tables
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`
				results, err = s.executeSecureQueryWithDB(ctx, db, guard, query)
			}

			// Append cross-database info if allowed databases are configured
			if err == nil && len(s.config.allowedDatabases) > 0 {
				label = fmt.Sprintf("Tables and views found (current database + %d allowed cross-databases: %s — use explore with database parameter to list their tables)",
					len(s.config.allowedDatabases), strings.Join(s.config.allowedDatabases, ", "))
			}
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error in explore: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	resultBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error formatting results: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
					Annotations: annBothExplore,
				},
			},
		},
	}
}

// handleInspect returns table structure (columns, indexes, foreign keys, dependencies)
func (s *MCPMSSQLServer) handleInspect(id interface{}, params CallToolParams) *MCPResponse {
	// Resolve target DB and guard: use dynamic connection if specified, otherwise default DB
	var db *sql.DB
	var guard *sqlguard.Guard

	if connName, ok := params.Arguments["connection"].(string); ok && connName != "" {
		connInfo, ok := s.getDynamicConnectionInfo(connName)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error: Unknown dynamic connection '%s'. Use dynamic_list to see active connections.", connName),
							Annotations: annBothHigh,
						},
					},
					IsError: true,
				},
			}
		}
		db = connInfo.DB
		guard = connInfo.guard
	} else {
		db = s.getDB()
		guard = nil
		if db == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
						},
					},
					IsError: true,
				},
			}
		}
	}

	tableName, ok := params.Arguments["table_name"].(string)
	if !ok || tableName == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{Type: "text", Text: "Error: Missing or invalid 'table_name' parameter", Annotations: annBothHigh},
				},
				IsError: true,
			},
		}
	}

	schemaName := "dbo"
	if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
		schemaName = schema
	}
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		if len(parts) == 2 {
			schemaName = strings.Trim(parts[0], "[]")
			tableName = strings.Trim(parts[1], "[]")
		}
	}
	tableName = strings.Trim(tableName, "[]")

	detail := "columns"
	if d, ok := params.Arguments["detail"].(string); ok && d != "" {
		detail = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	columnsQuery := `
		SELECT
			COLUMN_NAME as column_name,
			DATA_TYPE as data_type,
			IS_NULLABLE as is_nullable,
			COLUMN_DEFAULT as default_value,
			CHARACTER_MAXIMUM_LENGTH as max_length,
			ORDINAL_POSITION as position
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = @p1 AND TABLE_NAME = @p2
		ORDER BY ORDINAL_POSITION
	`
	indexesQuery := `
		SELECT
			i.name as index_name,
			i.type_desc as index_type,
			i.is_unique,
			i.is_primary_key,
			STRING_AGG(c.name, ', ') WITHIN GROUP (ORDER BY ic.key_ordinal) as columns
		FROM sys.indexes i
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		INNER JOIN sys.tables t ON i.object_id = t.object_id
		INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE t.name = @p1 AND s.name = @p2 AND i.name IS NOT NULL
		GROUP BY i.name, i.type_desc, i.is_unique, i.is_primary_key
		ORDER BY i.is_primary_key DESC, i.name
	`
	fkQuery := `
		SELECT
			fk.name as constraint_name,
			OBJECT_SCHEMA_NAME(fk.parent_object_id) as from_schema,
			OBJECT_NAME(fk.parent_object_id) as from_table,
			COL_NAME(fkc.parent_object_id, fkc.parent_column_id) as from_column,
			OBJECT_SCHEMA_NAME(fk.referenced_object_id) as to_schema,
			OBJECT_NAME(fk.referenced_object_id) as to_table,
			COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) as to_column,
			fk.delete_referential_action_desc as on_delete,
			fk.update_referential_action_desc as on_update
		FROM sys.foreign_keys fk
		INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		INNER JOIN sys.tables t ON fk.parent_object_id = t.object_id
		INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE (t.name = @p1 AND s.name = @p2)
		   OR (OBJECT_NAME(fk.referenced_object_id) = @p1 AND OBJECT_SCHEMA_NAME(fk.referenced_object_id) = @p2)
		ORDER BY fk.name
	`

	if detail == "all" {
		colResults, err := s.executeSecureQueryWithDB(ctx, db, guard, columnsQuery, schemaName, tableName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting columns: %v", err), Annotations: annBothHigh}}, IsError: true,
			}}
		}
		idxResults, err := s.executeSecureQueryWithDB(ctx, db, guard, indexesQuery, tableName, schemaName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting indexes: %v", err), Annotations: annBothHigh}}, IsError: true,
			}}
		}
		fkResults, err := s.executeSecureQueryWithDB(ctx, db, guard, fkQuery, tableName, schemaName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting foreign keys: %v", err), Annotations: annBothHigh}}, IsError: true,
			}}
		}
		depsAllQuery := `
			SELECT
				SCHEMA_NAME(o.schema_id)  AS referencing_schema,
				o.name                    AS referencing_object,
				o.type_desc               AS referencing_type,
				sed.is_caller_dependent,
				sed.is_ambiguous
			FROM sys.sql_expression_dependencies sed
			JOIN sys.objects o ON o.object_id = sed.referencing_id
			WHERE sed.referenced_entity_name = @p1
			  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
			ORDER BY o.type_desc, referencing_schema, referencing_object
		`
		depsResults, _ := s.executeSecureQueryWithDB(ctx, db, guard, depsAllQuery, tableName, schemaName) // #nosec G104 - dependencies query is optional, errors handled gracefully
		combined := map[string]interface{}{
			"columns":      colResults,
			"indexes":      idxResults,
			"foreign_keys": fkResults,
			"dependencies": depsResults,
		}
		resultBytes, err := json.MarshalIndent(combined, "", "  ")
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error formatting results: %v", err), Annotations: annBothHigh}}, IsError: true,
			}}
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Full inspection of '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
						Annotations: annBothInspect,
					},
				},
			},
		}
	}

	var results []map[string]interface{}
	var err error
	var label string

	switch detail {
	case "indexes":
		label = fmt.Sprintf("Indexes for '%s.%s'", schemaName, tableName)
		results, err = s.executeSecureQueryWithDB(ctx, db, guard, indexesQuery, tableName, schemaName)
	case "foreign_keys":
		label = fmt.Sprintf("Foreign keys for '%s.%s'", schemaName, tableName)
		results, err = s.executeSecureQueryWithDB(ctx, db, guard, fkQuery, tableName, schemaName)
	case "dependencies":
		label = fmt.Sprintf("Objects that depend on '%s.%s'", schemaName, tableName)
		depsQuery := `
			SELECT
				SCHEMA_NAME(o.schema_id)  AS referencing_schema,
				o.name                    AS referencing_object,
				o.type_desc               AS referencing_type,
				sed.is_caller_dependent,
				sed.is_ambiguous
			FROM sys.sql_expression_dependencies sed
			JOIN sys.objects o ON o.object_id = sed.referencing_id
			WHERE sed.referenced_entity_name = @p1
			  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
			ORDER BY o.type_desc, referencing_schema, referencing_object
		`
		results, err = s.executeSecureQueryWithDB(ctx, db, guard, depsQuery, tableName, schemaName)
	default: // "columns"
		label = fmt.Sprintf("Table structure for '%s'", tableName)
		results, err = s.executeSecureQueryWithDB(ctx, db, guard, columnsQuery, schemaName, tableName)
		if err == nil && len(results) == 0 {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: fmt.Sprintf("Table '%s' not found", tableName), Annotations: annBothHigh},
					},
					IsError: true,
				},
			}
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error in inspect: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	resultBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error formatting results: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
					Annotations: annBothInspect,
				},
			},
		},
	}
}

// handleExplainQuery returns the query execution plan without executing the query
func (s *MCPMSSQLServer) handleExplainQuery(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.", Annotations: annAssistantHigh}},
				IsError: true,
			},
		}
	}

	query, ok := params.Arguments["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: Missing or invalid 'query' parameter", Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}

	// Only allow SELECT queries for safety (always enforced, regardless of MSSQL_READ_ONLY)
	if op := sqlguard.ExtractOperation(query); op != "SELECT" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: explain_query only accepts SELECT queries, got: " + op, Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a dedicated connection so SET SHOWPLAN_TEXT applies only to this query
	conn, err := s.getDB().Conn(ctx)
	if err != nil {
		connErrMsg := "Error acquiring connection"
		if s.devMode {
			connErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: connErrMsg, Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}
	defer conn.Close()

	// Enable showplan (does not execute the query, only returns the plan)
	if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_TEXT ON"); err != nil {
		showplanErrMsg := "Error enabling SHOWPLAN"
		if s.devMode {
			showplanErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: showplanErrMsg, Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup
		planErrMsg := "Error getting execution plan"
		if s.devMode {
			planErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: planErrMsg, Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			planLines = append(planLines, line)
		}
	}
	_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup

	if len(planLines) == 0 {
		planLines = []string{"(no plan returned — query may be too simple or unsupported)"}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        "Execution plan:\n\n" + strings.Join(planLines, "\n"),
					Annotations: annBothExplain,
				},
			},
		},
	}
}