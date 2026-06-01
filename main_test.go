package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// loadEnvFile loads environment variables from a file if it exists
func loadEnvFile(filePath string) error {
	file, err := os.Open(filePath) // #nosec G304 - paths are hardcoded test fixtures
	if err != nil {
		// File doesn't exist, which is ok - environment vars may be set elsewhere
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE format
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Only set if not already set
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	return scanner.Err()
}

// Test configuration
// SECURITY: Do NOT hardcode credentials here. Tests must load from .env.test
func setupTestEnv() {
	// Try to load .env.test if it exists (for local testing)
	loadEnvFile(".env.test")
	loadEnvFile("../.env.test")

	// Load defaults only for non-sensitive testing values
	if os.Getenv("MSSQL_PORT") == "" {
		os.Setenv("MSSQL_PORT", "1433")
	}
	if os.Getenv("DEVELOPER_MODE") == "" {
		os.Setenv("DEVELOPER_MODE", "true")
	}
	if os.Getenv("MSSQL_AUTH") == "" {
		os.Setenv("MSSQL_AUTH", "sql")
	}

	// Database tests will skip themselves if the required environment
	// variables (MSSQL_SERVER, MSSQL_DATABASE, etc.) are not set.
}

// newTestMCPServer creates an MCPMSSQLServer with rate limiter initialized for testing.
func newTestMCPServer() *MCPMSSQLServer {
	s := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}
	s.rateLimiter.maxTokens = 1000
	s.rateLimiter.tokens = 1000
	s.rateLimiter.lastReset = time.Now()
	s.rateLimiter.interval = time.Minute
	return s
}

func TestSecurityLoggerSanitization(t *testing.T) {
	logger := NewSecurityLogger()

	testCases := []struct {
		name     string
		input    string
		expected bool // true if should be sanitized
	}{
		{
			name:     "Password in connection string",
			input:    "server=test;password=secret123;user=admin",
			expected: true,
		},
		{
			name:     "Multiple sensitive fields",
			input:    "password=secret;token=abc123;key=xyz789",
			expected: true,
		},
		{
			name:     "No sensitive data",
			input:    "server=test;user=admin;database=mydb",
			expected: false,
		},
		{
			name:     "Case insensitive password",
			input:    "PASSWORD=Secret123;PWD=test123",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := logger.sanitizeForLogging(tc.input)

			if tc.expected {
				// Should contain *** for sanitized fields
				if !strings.Contains(result, "***") {
					t.Errorf("Expected sanitized output to contain ***, got: %s", result)
				}
				// Original input should be different from result (sanitized)
				if result == tc.input {
					t.Errorf("Expected sanitized output to be different from input: %s", result)
				}
			} else {
				// Should remain unchanged
				if result != tc.input {
					t.Errorf("Expected unchanged output, got: %s", result)
				}
			}
		})
	}
}

func TestBuildSecureConnectionString(t *testing.T) {
	t.Run("Valid configuration", func(t *testing.T) {
		t.Setenv("MSSQL_CONNECTION_STRING", "")
		t.Setenv("MSSQL_SERVER", "testserver")
		t.Setenv("MSSQL_DATABASE", "testdb")
		t.Setenv("MSSQL_USER", "testuser")
		t.Setenv("MSSQL_PASSWORD", "testpass")
		t.Setenv("MSSQL_PORT", "1433")
		t.Setenv("MSSQL_AUTH", "sql")
		t.Setenv("DEVELOPER_MODE", "true")

		connStr, err := buildSecureConnectionString()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(connStr, "server=") {
			t.Errorf("Connection string should contain server")
		}
		if !strings.Contains(connStr, "database=") {
			t.Errorf("Connection string should contain database")
		}
	})

	t.Run("Missing required variables", func(t *testing.T) {
		t.Setenv("MSSQL_CONNECTION_STRING", "")
		t.Setenv("MSSQL_SERVER", "")
		t.Setenv("MSSQL_DATABASE", "test")
		t.Setenv("MSSQL_USER", "user")
		t.Setenv("MSSQL_PASSWORD", "pass")
		t.Setenv("MSSQL_AUTH", "sql")

		_, err := buildSecureConnectionString()
		if err == nil {
			t.Errorf("Expected error for missing MSSQL_SERVER, got none")
		}
	})

	t.Run("Production mode settings", func(t *testing.T) {
		t.Setenv("MSSQL_CONNECTION_STRING", "")
		t.Setenv("MSSQL_SERVER", "testserver")
		t.Setenv("MSSQL_DATABASE", "testdb")
		t.Setenv("MSSQL_USER", "testuser")
		t.Setenv("MSSQL_PASSWORD", "testpass")
		t.Setenv("MSSQL_AUTH", "sql")
		t.Setenv("DEVELOPER_MODE", "false")

		connStr, err := buildSecureConnectionString()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(connStr, "encrypt=true") {
			t.Errorf("In production mode, should have encrypt=true")
		}
		if !strings.Contains(connStr, "trustservercertificate=false") {
			t.Errorf("In production mode, should not trust server certificate")
		}
	})

	t.Run("Integrated authentication (Windows)", func(t *testing.T) {
		t.Setenv("MSSQL_CONNECTION_STRING", "")
		t.Setenv("MSSQL_SERVER", "testserver")
		t.Setenv("MSSQL_AUTH", "integrated")
		t.Setenv("MSSQL_USER", "")
		t.Setenv("MSSQL_PASSWORD", "")
		t.Setenv("DEVELOPER_MODE", "true")

		connStr, err := buildSecureConnectionString()
		if err != nil {
			t.Fatalf("Expected no error for integrated auth, got: %v", err)
		}

		if !strings.Contains(strings.ToLower(connStr), "integrated security=sspi") {
			t.Errorf("Expected integrated security in connection string for integrated auth, got: %s", connStr)
		}
		if strings.Contains(strings.ToLower(connStr), "user id=") || strings.Contains(strings.ToLower(connStr), "password=") {
			t.Errorf("Connection string for integrated auth should not include user or password: %s", connStr)
		}
	})
}

func TestMCPServerInitialization(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	// Test initialize request
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-1",
		Method:  "initialize",
		Params:  InitializeParams{ProtocolVersion: "2025-06-18"},
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatalf("Expected response, got nil")
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got: %v", response.Error)
	}

	// Check response structure
	if response.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got: %s", response.JSONRPC)
	}
	if response.ID != "test-1" {
		t.Errorf("Expected ID test-1, got: %v", response.ID)
	}

	// Verify protocol version negotiation echoes client's version
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("Failed to unmarshal init result: %v", err)
	}
	if initResult.ProtocolVersion != "2025-06-18" {
		t.Errorf("Expected server to echo client protocolVersion '2025-06-18', got: %s", initResult.ProtocolVersion)
	}
}

func TestMCPVersionNegotiation(t *testing.T) {
	server := newTestMCPServer()

	versions := []string{"2025-06-18", "2025-11-25", "2024-11-05"}
	for _, ver := range versions {
		t.Run("version="+ver, func(t *testing.T) {
			req := MCPRequest{
				JSONRPC: "2.0",
				ID:      "test-ver",
				Method:  "initialize",
				Params:  InitializeParams{ProtocolVersion: ver},
			}
			response := server.handleRequest(req)
			resultBytes, _ := json.Marshal(response.Result)
			var initResult InitializeResult
			json.Unmarshal(resultBytes, &initResult)
			if initResult.ProtocolVersion != ver {
				t.Errorf("Expected echoed version %s, got %s", ver, initResult.ProtocolVersion)
			}
		})
	}
}

func TestMCPToolsList(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-tools",
		Method:  "tools/list",
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatalf("Expected response, got nil")
	}

	if response.Error != nil {
		t.Errorf("Expected no error, got: %v", response.Error)
	}

	// Parse tools list
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var toolsResult ToolsListResult
	err = json.Unmarshal(resultBytes, &toolsResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal tools result: %v", err)
	}

	// In the default test server (no MSSQL_DYNAMIC_* vars, no explicit DYNAMIC_MODE=true,
	// and the test helper does not set classic MSSQL_* either), we are in classic mode.
	// Therefore only the 6 core tools are exposed. This is the desired behavior:
	// classic servers (the majority of real .mcp.json usage) must not advertise
	// dynamic_* tools so the AI does not get confused.
	expectedTools := []string{
		"query_database", "get_database_info", "explore", "inspect", "execute_procedure", "explain_query",
	}
	if len(toolsResult.Tools) != len(expectedTools) {
		t.Errorf("Expected %d tools (classic mode), got %d. Tools: %+v",
			len(expectedTools), len(toolsResult.Tools), toolsResult.Tools)
	}

	for _, expectedTool := range expectedTools {
		found := false
		for _, tool := range toolsResult.Tools {
			if tool.Name == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found", expectedTool)
		}
	}

	// Verify that no dynamic tools leaked into a classic server
	for _, tool := range toolsResult.Tools {
		if strings.HasPrefix(tool.Name, "dynamic_") || tool.Name == "confirm_operation" {
			t.Errorf("Dynamic tool %s should not be present on a classic test server", tool.Name)
		}
	}
}

func TestInputValidation(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid query",
			input:   "SELECT * FROM users WHERE id = 1",
			wantErr: false,
		},
		{
			name:    "Empty query",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Very large query",
			input:   strings.Repeat("A", 2000000), // 2MB
			wantErr: true,
		},
		{
			name:    "Normal size query",
			input:   strings.Repeat("A", 1000), // 1KB
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := server.validateBasicInput(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for input: %s", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error for input: %s, got: %v", tc.name, err)
			}
		})
	}
}

func TestInspectDependencies(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	// All inspect detail types should return IsError=true when DB is disconnected
	detailTypes := []string{"columns", "indexes", "foreign_keys", "dependencies", "all"}
	for _, dt := range detailTypes {
		t.Run("detail="+dt+"_no_db", func(t *testing.T) {
			params := CallToolParams{
				Name:      "inspect",
				Arguments: map[string]interface{}{"table_name": "users", "detail": dt},
			}
			resp := server.handleToolCall("test-id", params)
			if resp == nil {
				t.Fatal("Expected response, got nil")
			}
			resultBytes, _ := json.Marshal(resp.Result)
			var result CallToolResult
			json.Unmarshal(resultBytes, &result)
			if !result.IsError {
				t.Errorf("detail=%s: expected IsError=true when DB disconnected", dt)
			}
		})
	}

	// Missing table_name should return error
	t.Run("missing_table_name", func(t *testing.T) {
		params := CallToolParams{
			Name:      "inspect",
			Arguments: map[string]interface{}{"detail": "dependencies"},
		}
		resp := server.handleToolCall("test-id", params)
		resultBytes, _ := json.Marshal(resp.Result)
		var result CallToolResult
		json.Unmarshal(resultBytes, &result)
		if !result.IsError {
			t.Error("Expected IsError=true when table_name is missing")
		}
	})
}

func TestExploreViewsType(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	// All explore types should return IsError=true when DB is disconnected
	exploreTypes := []string{"tables", "views", "databases", "procedures"}
	for _, exploreType := range exploreTypes {
		t.Run("type="+exploreType+"_no_db", func(t *testing.T) {
			params := CallToolParams{
				Name:      "explore",
				Arguments: map[string]interface{}{"type": exploreType},
			}
			resp := server.handleToolCall("test-id", params)
			if resp == nil {
				t.Fatal("Expected response, got nil")
			}
			resultBytes, _ := json.Marshal(resp.Result)
			var result CallToolResult
			json.Unmarshal(resultBytes, &result)
			if !result.IsError {
				t.Errorf("type=%s: expected IsError=true when DB disconnected", exploreType)
			}
		})
	}

	// type=search without pattern should return error even without DB
	t.Run("type=search_missing_pattern", func(t *testing.T) {
		params := CallToolParams{
			Name:      "explore",
			Arguments: map[string]interface{}{"type": "search"},
		}
		resp := server.handleToolCall("test-id", params)
		resultBytes, _ := json.Marshal(resp.Result)
		var result CallToolResult
		json.Unmarshal(resultBytes, &result)
		if !result.IsError {
			t.Error("Expected IsError=true when pattern is missing for type=search")
		}
	})
}

func TestReadOnlyValidation(t *testing.T) {
	setupTestEnv()

	server := newTestMCPServer()

	// Test with read-only mode disabled (cached config)
	server.config.readOnly = false

	testCases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "SELECT query",
			query:   "SELECT * FROM users",
			wantErr: false,
		},
		{
			name:    "INSERT query - should be allowed when read-only is false",
			query:   "INSERT INTO users (name) VALUES ('test')",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := server.validateReadOnlyQuery(tc.query)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for query: %s", tc.query)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error for query: %s, got: %v", tc.query, err)
			}
		})
	}

	// Test with read-only mode enabled (cached config)
	server.config.readOnly = true

	readOnlyTestCases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "SELECT query",
			query:   "SELECT * FROM users",
			wantErr: false,
		},
		{
			name:    "INSERT query - should be blocked",
			query:   "INSERT INTO users (name) VALUES ('test')",
			wantErr: true,
		},
		{
			name:    "UPDATE query - should be blocked",
			query:   "UPDATE users SET name = 'test' WHERE id = 1",
			wantErr: true,
		},
		{
			name:    "DELETE query - should be blocked",
			query:   "DELETE FROM users WHERE id = 1",
			wantErr: true,
		},
		{
			name:    "WITH CTE query",
			query:   "WITH cte AS (SELECT * FROM users) SELECT * FROM cte",
			wantErr: false,
		},
		{
			name:    "SELECT with created_at column - should NOT be blocked",
			query:   "SELECT created_at FROM users",
			wantErr: false,
		},
		{
			name:    "SELECT with update_count column - should NOT be blocked",
			query:   "SELECT update_count FROM users",
			wantErr: false,
		},
		{
			name:    "SELECT with deleted flag - should NOT be blocked",
			query:   "SELECT deleted FROM users WHERE deleted = 0",
			wantErr: false,
		},
		// === New admin/schema introspection cases (safe EXEC allowed in read-only) ===
		{
			name:    "EXEC sp_help - allowed for schema discovery",
			query:   "EXEC sp_help 'dbo.Users'",
			wantErr: false,
		},
		{
			name:    "EXEC sp_helptext - allowed for procedure source",
			query:   "EXEC sp_helptext 'dbo.MyProc'",
			wantErr: false,
		},
		{
			name:    "EXEC sp_spaceused - allowed for table size info",
			query:   "EXEC sp_spaceused 'dbo.Orders'",
			wantErr: false,
		},
		{
			name:    "EXECUTE sp_columns - allowed (alternative syntax)",
			query:   "EXECUTE sp_columns @table_name = 'Products'",
			wantErr: false,
		},
		{
			name:    "EXEC sp_help with schema prefix - allowed",
			query:   "EXEC dbo.sp_help 'Sales.Customers'",
			wantErr: false,
		},
		// These must still be blocked
		{
			name:    "EXEC sp_executesql - must remain blocked",
			query:   "EXEC sp_executesql N'SELECT 1'",
			wantErr: true,
		},
		{
			name:    "EXEC xp_cmdshell - always blocked",
			query:   "EXEC xp_cmdshell 'dir'",
			wantErr: true,
		},
		{
			name:    "EXEC unknown procedure - blocked",
			query:   "EXEC sp_made_up_procedure 'foo'",
			wantErr: true,
		},
	}

	for _, tc := range readOnlyTestCases {
		t.Run(tc.name+"_readonly", func(t *testing.T) {
			err := server.validateReadOnlyQuery(tc.query)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for query in read-only mode: %s", tc.query)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error for query in read-only mode: %s, got: %v", tc.query, err)
			}
		})
	}
}

// Integration test - only runs if database is available
func TestDatabaseConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setupTestEnv()

	// Clear custom connection string
	t.Setenv("MSSQL_CONNECTION_STRING", "")

	if os.Getenv("MSSQL_SERVER") == "" {
		t.Skip("MSSQL_SERVER not set, skipping integration test")
	}

	// Try to build connection string
	connStr, err := buildSecureConnectionString()
	if err != nil {
		t.Fatalf("Failed to build connection string: %v", err)
	}

	// Try to connect
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		t.Fatalf("Failed to open connection: %v", err)
	}
	defer db.Close()

	// Configure connection pool as in production
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Test ping
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		t.Logf("Database connection failed (this is expected if DB is not available): %v", err)
		t.Skip("Database not available, skipping connection test")
		return
	}

	t.Log("Database connection successful")

	// Test a simple query
	var version string
	err = db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		t.Errorf("Failed to execute test query: %v", err)
		return
	}

	t.Logf("SQL Server Version: %s", version)

	// Test server functionality
	server := newTestMCPServer()
	server.setDB(db)

	// Test get_database_info
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-dbinfo",
		Method:  "tools/call",
		Params: CallToolParams{
			Name:      "get_database_info",
			Arguments: map[string]interface{}{},
		},
	}

	response := server.handleToolCall(req.ID, CallToolParams{
		Name:      "get_database_info",
		Arguments: map[string]interface{}{},
	})

	if response.Error != nil {
		t.Errorf("get_database_info failed: %v", response.Error)
		return
	}

	t.Log("get_database_info test passed")

	// Test explore (replaces list_tables)
	response = server.handleToolCall(req.ID, CallToolParams{
		Name:      "explore",
		Arguments: map[string]interface{}{},
	})

	if response.Error != nil {
		t.Errorf("explore failed: %v", response.Error)
		return
	}

	t.Log("explore test passed")
}

func TestPerformanceOptimizations(t *testing.T) {
	setupTestEnv()

	// Test that compiled regex patterns are available
	if len(sensitivePatterns) == 0 {
		t.Errorf("Expected compiled regex patterns to be available")
	}

	// Test that table extraction regex patterns are pre-compiled
	if len(tableExtractionPatterns) == 0 {
		t.Errorf("Expected table extraction regex patterns to be pre-compiled")
	}

	// Test performance of sanitization
	logger := NewSecurityLogger()
	input := "server=test;password=secret123;user=admin;token=abc123"

	// Run multiple times to ensure compiled patterns are reused
	for i := 0; i < 100; i++ {
		result := logger.sanitizeForLogging(input)
		if !strings.Contains(result, "***") {
			t.Errorf("Sanitization failed on iteration %d", i)
			break
		}
	}
}

func TestExplainQueryValidation(t *testing.T) {
	server := newTestMCPServer()

	testCases := []struct {
		name     string
		query    string
		wantSELECT bool
	}{
		{"Valid SELECT", "SELECT * FROM users", true},
		{"Valid SELECT with JOIN", "SELECT u.name FROM users u JOIN orders o ON u.id = o.user_id", true},
		{"INSERT blocked", "INSERT INTO users (name) VALUES ('x')", false},
		{"UPDATE blocked", "UPDATE users SET name='x'", false},
		{"DELETE blocked", "DELETE FROM users WHERE id=1", false},
		{"DROP blocked", "DROP TABLE users", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// explain_query uses extractOperation to enforce SELECT-only
			op := server.extractOperation(tc.query)
			isSelect := op == "SELECT"
			if isSelect != tc.wantSELECT {
				t.Errorf("query %q: got op=%s (isSelect=%v), want isSelect=%v", tc.query, op, isSelect, tc.wantSELECT)
			}
		})
	}

	// Test that explain_query returns error when DB is not connected
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-explain",
		Method:  "tools/call",
		Params:  map[string]interface{}{},
	}
	params := CallToolParams{
		Name:      "explain_query",
		Arguments: map[string]interface{}{"query": "SELECT 1"},
	}
	response := server.handleToolCall(req.ID, params)
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	resultBytes, _ := json.Marshal(response.Result)
	var result CallToolResult
	json.Unmarshal(resultBytes, &result)
	if !result.IsError {
		t.Error("Expected IsError=true when DB is disconnected")
	}
}

func TestProcedureNameValidation(t *testing.T) {
	server := newTestMCPServer()

	testCases := []struct {
		name    string
		proc    string
		wantErr bool
	}{
		{"Simple name", "my_proc", false},
		{"Schema qualified", "dbo.my_proc", false},
		{"Bracketed", "[dbo].[my_proc]", false},
		{"With semicolon", "my_proc; DROP TABLE users", true},
		{"With spaces", "my proc", true},
		{"With parentheses", "my_proc()", true},
		{"With single quote", "my_proc'", true},
		{"Empty name", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := server.validateProcedureName(tc.proc)
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for procedure name: %s", tc.proc)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error for procedure name: %s, got: %v", tc.proc, err)
			}
		})
	}
}

// TestDynamicModeDetection covers the critical precedence rules added to prevent
// classic .mcp.json configurations from being hijacked by stray MSSQL_DYNAMIC_*
// variables (from parent process, previous shells, or .env files next to other exes).
func TestDynamicModeDetection(t *testing.T) {
	tests := []struct {
		name        string
		setEnv      map[string]string
		wantDynamic bool
		description string
	}{
		{
			name: "explicit false always wins (even with dynamic vars present)",
			setEnv: map[string]string{
				"MSSQL_DYNAMIC_MODE":       "false",
				"MSSQL_DYNAMIC_APP_SERVER": "sql01.local",
				"MSSQL_DYNAMIC_APP_DATABASE": "TestDB",
				"MSSQL_SERVER":             "",
			},
			wantDynamic: false,
			description: "User explicitly wants classic mode for isolation",
		},
		{
			name: "explicit true forces dynamic",
			setEnv: map[string]string{
				"MSSQL_DYNAMIC_MODE": "true",
			},
			wantDynamic: true,
		},
		{
			name: "classic config present (MSSQL_SERVER) forces classic even with dynamic vars",
			setEnv: map[string]string{
				"MSSQL_DYNAMIC_MODE":       "", // not set
				"MSSQL_SERVER":             "10.0.0.5",
				"MSSQL_DATABASE":           "JJP_TRANSFER",
				"MSSQL_DYNAMIC_FOO_SERVER": "other",
			},
			wantDynamic: false,
			description: "Core fix: .mcp.json classic configs must win over environment pollution",
		},
		{
			name: "no classic config + dynamic vars present → dynamic (auto-detect)",
			setEnv: map[string]string{
				"MSSQL_DYNAMIC_MODE":        "",
				"MSSQL_SERVER":              "",
				"MSSQL_DYNAMIC_PROD_SERVER": "prod.local",
				"MSSQL_DYNAMIC_PROD_DATABASE": "ProdDB",
			},
			wantDynamic: true,
		},
		{
			name: "explicit false with various spellings",
			setEnv: map[string]string{
				"MSSQL_DYNAMIC_MODE":     "NO",
				"MSSQL_DYNAMIC_X_SERVER": "x",
			},
			wantDynamic: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use t.Setenv so state is automatically restored after the subtest
			for k, v := range tc.setEnv {
				t.Setenv(k, v)
			}

			got := isDynamicMode()
			if got != tc.wantDynamic {
				t.Errorf("isDynamicMode() = %v, want %v\nCase: %s\nEnv snapshot: %+v",
					got, tc.wantDynamic, tc.description, tc.setEnv)
			}
		})
	}
}

// TestClassicServerDoesNotExposeDynamicTools verifies that when running in classic
// mode (the common .mcp.json direct config case), the tools/list response does not
// include any dynamic_* or confirm_operation tools.
func TestClassicServerDoesNotExposeDynamicTools(t *testing.T) {
	setupTestEnv()

	// Force classic mode via the new precedence rule
	t.Setenv("MSSQL_DYNAMIC_MODE", "")
	t.Setenv("MSSQL_SERVER", "localhost")
	t.Setenv("MSSQL_DATABASE", "TestDB")
	t.Setenv("MSSQL_DYNAMIC_ANYTHING_SERVER", "should-be-ignored")

	server := newTestMCPServer()
	// Simulate what main() does
	server.isDynamic = isDynamicMode()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-tools-classic",
		Method:  "tools/list",
	}

	resp := server.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var toolsResult ToolsListResult
	_ = json.Unmarshal(resultBytes, &toolsResult)

	dynamicToolCount := 0
	for _, tool := range toolsResult.Tools {
		if strings.HasPrefix(tool.Name, "dynamic_") || tool.Name == "confirm_operation" {
			dynamicToolCount++
			t.Logf("Unexpected dynamic tool exposed: %s", tool.Name)
		}
	}

	if dynamicToolCount > 0 {
		t.Errorf("Classic server exposed %d dynamic tools (expected 0)", dynamicToolCount)
	}

	if len(toolsResult.Tools) != 6 {
		t.Errorf("Expected exactly 6 core tools in classic mode, got %d", len(toolsResult.Tools))
	}
}
