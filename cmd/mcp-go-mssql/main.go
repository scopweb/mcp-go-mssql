// Command mcp-go-mssql is a stdio MCP server for Microsoft SQL Server.
// Production defaults are fail-closed: TLS is required, statements are
// prepared, and read-only mode denies writes even if a table whitelist is set.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	osuser "os/user"
	"strings"
	"sync"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

func main() {
	// Initialize security logger
	secLogger := NewSecurityLogger()
	secLogger.Printf("Starting secure MCP-MSSQL server")

	// Load .env from executable directory (supports documented dynamic mode workflow)
	// Host-passed environment variables always take precedence.
	loadDotEnvIfPresent(secLogger)

	// Resolve only explicit credential references; never export vault values to the process environment.
	environment, err := resolveVaultEnvironment(context.Background(), os.Environ())
	if err != nil {
		secLogger.Printf("Vault configuration failed: %v", err)
		os.Exit(1)
	}
	getenv := environmentLookup(environment)

	// Determine mode once. We use this both to decide whether to load dynamic aliases
	// and to expose the correct tool surface to the AI.
	dynamicMode := isDynamicMode()

	// Only load dynamic aliases when operating in dynamic mode.
	// This prevents stray MSSQL_DYNAMIC_* variables (inherited from Claude Desktop's
	// parent environment, previous shell sessions, or leftover .env files in other folders)
	// from polluting classic single-connection server instances.
	var dynamicAliases map[string]DynamicAlias
	if dynamicMode {
		dynamicAliases = loadDynamicAliasesFromEnvironment(secLogger, environment)
	} else {
		dynamicAliases = make(map[string]DynamicAlias)
	}

	// Check for developer mode
	devMode := strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true"
	if devMode {
		secLogger.Printf("DEVELOPER MODE ENABLED - Detailed errors will be shown")
	}

	// Cache configuration once at startup (avoid os.Getenv on every request)
	readOnly, err := configuredReadOnly(os.Getenv("MSSQL_READ_ONLY"))
	if err != nil {
		secLogger.Printf("Invalid security configuration: %v", err)
		os.Exit(1)
	}
	cfg := serverConfig{
		readOnly:        readOnly,
		whitelistTables: parseWhitelistTables(os.Getenv("MSSQL_WHITELIST_TABLES")),
		whitelistProcs:  os.Getenv("MSSQL_WHITELIST_PROCEDURES"),
	}

	// === SECURITY GUARD: Dynamic mode + global READ_ONLY=false is fatal ===
	// This combination (as reported in production incidents) allows an AI (or
	// prompt-injected tool call) to dynamic_connect to any preconfigured internal
	// corporate database and execute arbitrary DML/DDL because enforcement is global.
	if isDynamicMode() && !cfg.readOnly {
		secLogger.Printf("*** FATAL SECURITY MISCONFIGURATION DETECTED ***")
		secLogger.Printf("DYNAMIC multi-connection mode is active (MSSQL_DYNAMIC_MODE or MSSQL_DYNAMIC_* vars present)")
		secLogger.Printf("BUT top-level MSSQL_READ_ONLY is explicitly false (full access mode).")
		secLogger.Printf("This would expose ALL dynamic databases (including production ones) to unrestricted write operations via query_database after dynamic_connect.")
		secLogger.Printf("SAFETY ACTION: Forcing READ-ONLY mode globally for this session. Dynamic aliases will inherit safe defaults.")
		secLogger.Printf("Recommendation: Remove global MSSQL_READ_ONLY=false and use explicit per-alias READ_ONLY settings with table whitelists.")
		cfg.readOnly = true
		cfg.whitelistTables = nil // start strict; per-alias can relax later
	}

	// Create MCP server without database initially
	server := &MCPMSSQLServer{
		db:             nil,
		secLogger:      secLogger,
		devMode:        devMode,
		config:         cfg,
		isDynamic:      dynamicMode,
		dynamicAliases: dynamicAliases,
	}
	// Initialize rate limiter: 60 tool calls per minute
	server.rateLimiter.maxTokens = 60
	server.rateLimiter.tokens = 60
	server.rateLimiter.lastReset = time.Now()
	server.rateLimiter.interval = time.Minute

	// Usage accounting: one counted record per tool call, written to stderr
	// as JSON (component="usage"). It counts facts about requests that really
	// happened and never carries SQL, results or credentials.
	usageLogger := NewUsageLogger(usageLogEnabled())
	if usageLogger.enabled {
		secLogger.Printf("Usage accounting enabled (stderr, component=usage)")
	}

	// Try to establish database connection (non-fatal)
	// Use context for cancellation and WaitGroup for clean shutdown
	connCtx, connCancel := context.WithCancel(context.Background())
	var connWg sync.WaitGroup
	connWg.Add(1)
	go func() {
		defer connWg.Done()
		defer func() {
			if r := recover(); r != nil {
				secLogger.Printf("Recovered panic in background connection goroutine: %v (this should not happen - please report)", r)
			}
		}()
		// Give MCP protocol time to initialize
		select {
		case <-time.After(2 * time.Second):
		case <-connCtx.Done():
			return
		}

		// Check if we have required environment variables
		serverHost := os.Getenv("MSSQL_SERVER")
		database := os.Getenv("MSSQL_DATABASE")
		user := os.Getenv("MSSQL_USER")
		password := getenv("MSSQL_PASSWORD")
		authMode := os.Getenv("MSSQL_AUTH")

		customConnStr := getenv("MSSQL_CONNECTION_STRING")
		if customConnStr != "" {
			secLogger.Printf("Using custom connection string (value omitted)")
		} else {
			auth := strings.ToLower(authMode)
			if auth == "" {
				auth = "sql"
			}

			if auth == "integrated" || auth == "windows" {
				secLogger.Printf("Environment variables - Server: %s, Database: %s, Auth: INTEGRATED (Windows), DevMode: %s",
					serverHost,
					func() string {
						if database != "" {
							return database
						}
						return "(not specified - will use default)"
					}(),
					os.Getenv("DEVELOPER_MODE"))

				// Show current Windows user
				if u, err := osuser.Current(); err == nil {
					secLogger.Printf("Running as Windows user: %s", u.Username)
				}
			} else {
				secLogger.Printf("Environment variables - Server: %s, Database: %s, Auth: SQL, User: %s, Password: %s, DevMode: %s",
					serverHost, database, user,
					func() string {
						if password != "" {
							return "***"
						} else {
							return "MISSING"
						}
					}(),
					os.Getenv("DEVELOPER_MODE"))
			}
		}

		// Log only non-sensitive configuration settings
		safeEnvVars := []string{"MSSQL_SERVER", "MSSQL_DATABASE", "MSSQL_PORT", "MSSQL_AUTH", "MSSQL_READ_ONLY", "MSSQL_WHITELIST_TABLES", "DEVELOPER_MODE", "MSSQL_DYNAMIC_MODE"}
		secLogger.Printf("Configuration settings:")
		for _, key := range safeEnvVars {
			if val := os.Getenv(key); val != "" {
				secLogger.Printf("  %s=%s", key, val)
			}
		}
		if isDynamicMode() {
			secLogger.Printf("  DYNAMIC_ALIASES_DETECTED=true (see dynamic_available tool at runtime)")
		} else {
			secLogger.Printf("  DYNAMIC_MODE=false (classic single-connection mode)")
		}

		// Log security settings (using cached config)
		if server.config.readOnly {
			secLogger.Printf("READ-ONLY MODE ENABLED - Only SELECT queries allowed")
		} else {
			secLogger.Printf("Full access mode enabled")
		}

		if customConnStr == "" && serverHost == "" {
			if isDynamicMode() {
				secLogger.Printf("Dynamic multi-connection mode enabled - no default connection configured (use dynamic_connect after startup)")
			} else {
				secLogger.Printf("No MSSQL_SERVER or MSSQL_CONNECTION_STRING environment variable - database features disabled")
			}
			return
		}

		// Build secure connection string
		connStr, err := buildSecureConnectionStringFromEnvironment(getenv)
		if err != nil {
			if devMode {
				secLogger.Printf("Failed to build connection string: %v", err)
			} else {
				secLogger.Printf("Failed to build connection string: configuration error")
			}
			return
		}

		// Connect to MSSQL. Use mssql.NewConnector + sql.OpenDB so the driver
		// delivers informational messages via the sqlexp.ReturnMessage queue.
		secLogger.Printf("Attempting to connect to MSSQL server...")
		connector, err := mssql.NewConnector(connStr)
		if err != nil {
			if devMode {
				secLogger.Printf("mssql.NewConnector failed: %v", err)
			} else {
				secLogger.Printf("Failed to connect: connector build error")
			}
			return
		}
		db := sql.OpenDB(connector)
		secLogger.Printf("Connector built, testing connection...")

		// Configure optimized connection pool
		db.SetMaxOpenConns(10)                  // More concurrent connections
		db.SetMaxIdleConns(5)                   // More idle connections for reuse
		db.SetConnMaxLifetime(30 * time.Minute) // Shorter lifetime for fresher connections
		db.SetConnMaxIdleTime(5 * time.Minute)  // Quick cleanup of unused connections

		// Test connection with longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		secLogger.Printf("Testing database connection with ping...")
		if err := db.PingContext(ctx); err != nil {
			secLogger.LogConnectionAttempt(false)
			if devMode {
				secLogger.Printf("Database ping failed: %v", err)
				trustCert := "false"
				if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
					trustCert = "true"
				}

				auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
				if auth == "" {
					auth = "sql"
				}

				if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
					secLogger.Printf("Using custom connection string format")
				} else if auth == "integrated" || auth == "windows" {
					secLogger.Printf("Using Windows Integrated Authentication (SSPI)")
					secLogger.Printf("Troubleshooting tips for integrated auth:")
					secLogger.Printf("  1. Ensure your Windows user has permission in SQL Server")
					secLogger.Printf("  2. Check if SQL Server is configured to allow Windows Authentication")
					secLogger.Printf("  3. Try using server='.' or server='localhost' or server='(local)'")
					secLogger.Printf("  4. Verify TCP/IP or Named Pipes are enabled in SQL Server Configuration Manager")

					// Get current Windows user
					if u, err := osuser.Current(); err == nil {
						secLogger.Printf("  Running as Windows user: %s\\%s", u.Username, u.Name)
					}
				} else {
					encrypt := "true"
					if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
						if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
							encrypt = strings.ToLower(envEncrypt)
						} else {
							encrypt = "false"
						}
					}
					secLogger.Printf("Connection string format: server=SERVER;port=PORT;database=DB;user id=USER;password=***;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30", encrypt, trustCert)
				}
			} else {
				secLogger.Printf("Failed to ping database: connection test failed")
			}
			if cerr := db.Close(); cerr != nil {
				secLogger.Printf("Error closing DB after failed ping: %v", cerr)
			}
			return
		}

		secLogger.LogConnectionAttempt(true)
		secLogger.Printf("Database connection established successfully")

		// Update server with working database connection
		server.setDB(db)
	}()

	// Start MCP protocol handler
	scanner := bufio.NewScanner(os.Stdin)
	// Set explicit buffer limit (4MB) to prevent silent truncation and limit memory usage
	const maxScanBuf = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanBuf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			secLogger.Printf("Invalid JSON received: %v", err)
			// MCP spec MUST: respond with -32700 Parse error for invalid JSON
			parseErrResp := &MCPResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &MCPError{
					Code:    -32700,
					Message: "Parse error",
				},
			}
			if respBytes, err := json.Marshal(parseErrResp); err == nil {
				fmt.Println(string(respBytes))
			}
			continue
		}

		// MCP spec: all messages MUST be JSON-RPC 2.0
		if req.JSONRPC != "2.0" {
			invalidResp := &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32600,
					Message: "Invalid Request: missing or incorrect jsonrpc version, must be \"2.0\"",
				},
			}
			if respBytes, err := json.Marshal(invalidResp); err == nil {
				fmt.Println(string(respBytes))
			}
			continue
		}

		// Never log request payloads: SQL literals may contain credentials or PII.
		secLogger.Printf("Processing request: method=%s", req.Method)

		// Bounded: a handler that never returns must not stall the loop (see handler_deadline.go)
		requestStart := time.Now()
		outcome := server.handleWithDeadlineOutcome(req)
		response := outcome.response

		// Only send response if one is needed (not for notifications)
		if response != nil {
			responseBytes, err := json.Marshal(response)
			if err != nil {
				secLogger.Printf("Failed to marshal response: %v", err)
				continue
			}

			fmt.Println(string(responseBytes))

			// Count the call once, at the transport boundary, where the
			// bytes on the wire are already known.
			if tool := toolNameFromParams(req.Method, req.Params); tool != "" {
				rows, truncated, cellsTruncated, responseTruncated, textBytes, structuredBytes, isToolError := usageFromResponse(response)
				alias, readOnly := server.usageContext()
				usageLogger.LogCall(UsageRecord{
					CallID:            usageLogger.NextCallID(),
					Tool:              tool,
					Method:            req.Method,
					Status:            usageStatus(response, outcome.timedOut, isToolError),
					ElapsedMS:         time.Since(requestStart).Milliseconds(),
					ResponseBytes:     len(responseBytes),
					TextBytes:         textBytes,
					StructuredBytes:   structuredBytes,
					Rows:              rows,
					RowTruncated:      truncated,
					CellsTruncated:    cellsTruncated,
					ResponseTruncated: responseTruncated,
					Alias:             alias,
					ReadOnly:          readOnly,
				})
			}
		}
	}
	// Note: we intentionally do not recover here; a panic in the main loop is fatal
	// and will cause the host to restart us (logged). The recover is inside handleRequest for per-request safety.

	if err := scanner.Err(); err != nil && err != io.EOF {
		secLogger.Printf("Scanner error: %v", err)
	}

	// Release all database connections (classic + dynamic aliases) before exiting.
	server.closeAllConnections()

	// Clean shutdown: cancel connection goroutine and wait for it
	connCancel()
	connWg.Wait()
}
