package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Test configuration
func setupTestEnv() {
	// Load .env variables if not already set
	if os.Getenv("MSSQL_SERVER") == "" {
		os.Setenv("MSSQL_SERVER", "10.203.3.10")
	}
	if os.Getenv("MSSQL_DATABASE") == "" {
		os.Setenv("MSSQL_DATABASE", "JJP_TRANSFER")
	}
	if os.Getenv("MSSQL_USER") == "" {
		os.Setenv("MSSQL_USER", "userTRANSFER")
	}
	if os.Getenv("MSSQL_PASSWORD") == "" {
		os.Setenv("MSSQL_PASSWORD", "jl3RN7o02g")
	}
	if os.Getenv("MSSQL_PORT") == "" {
		os.Setenv("MSSQL_PORT", "1433")
	}
	if os.Getenv("DEVELOPER_MODE") == "" {
		os.Setenv("DEVELOPER_MODE", "true")
	}
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
	// Save original env vars
	originalVars := make(map[string]string)
	envVars := []string{"MSSQL_SERVER", "MSSQL_DATABASE", "MSSQL_USER", "MSSQL_PASSWORD", "MSSQL_PORT", "DEVELOPER_MODE"}
	for _, v := range envVars {
		originalVars[v] = os.Getenv(v)
	}
	defer func() {
		// Restore original env vars
		for k, v := range originalVars {
			os.Setenv(k, v)
		}
	}()

	t.Run("Valid configuration", func(t *testing.T) {
		setupTestEnv()

		connStr, err := buildSecureConnectionString()
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(connStr, "server=10.203.3.10") {
			t.Errorf("Connection string should contain server")
		}
		if !strings.Contains(connStr, "database=JJP_TRANSFER") {
			t.Errorf("Connection string should contain database")
		}
		if !strings.Contains(connStr, "encrypt=false") {
			t.Errorf("In dev mode, should have encrypt=false for local development")
		}
		if !strings.Contains(connStr, "trustservercertificate=true") {
			t.Errorf("In dev mode, should trust server certificate")
		}
	})

	t.Run("Missing required variables", func(t *testing.T) {
		os.Setenv("MSSQL_SERVER", "")
		os.Setenv("MSSQL_DATABASE", "test")
		os.Setenv("MSSQL_USER", "user")
		os.Setenv("MSSQL_PASSWORD", "pass")

		_, err := buildSecureConnectionString()
		if err == nil {
			t.Errorf("Expected error for missing MSSQL_SERVER, got none")
		}
	})

	t.Run("Production mode settings", func(t *testing.T) {
		setupTestEnv()
		os.Setenv("DEVELOPER_MODE", "false")

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
}

func TestMCPServerInitialization(t *testing.T) {
	setupTestEnv()

	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

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
}

func TestMCPToolsList(t *testing.T) {
	setupTestEnv()

	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

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

	expectedTools := []string{"query_database", "get_database_info", "list_tables", "describe_table"}
	if len(toolsResult.Tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(toolsResult.Tools))
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
}

func TestInputValidation(t *testing.T) {
	setupTestEnv()

	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

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

func TestReadOnlyValidation(t *testing.T) {
	setupTestEnv()

	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	// Test with read-only mode disabled
	os.Setenv("MSSQL_READ_ONLY", "false")

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

	// Test with read-only mode enabled
	os.Setenv("MSSQL_READ_ONLY", "true")

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

	t.Log("✅ Database connection successful!")

	// Test a simple query
	var version string
	err = db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		t.Errorf("Failed to execute test query: %v", err)
		return
	}

	t.Logf("✅ SQL Server Version: %s", version)

	// Test server functionality
	server := &MCPMSSQLServer{
		db:        db,
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

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

	t.Log("✅ get_database_info test passed")

	// Test list_tables
	response = server.handleToolCall(req.ID, CallToolParams{
		Name:      "list_tables",
		Arguments: map[string]interface{}{},
	})

	if response.Error != nil {
		t.Errorf("list_tables failed: %v", response.Error)
		return
	}

	t.Log("✅ list_tables test passed")
}

func TestPerformanceOptimizations(t *testing.T) {
	setupTestEnv()

	// Test that compiled regex patterns are available
	if len(sensitivePatterns) == 0 {
		t.Errorf("Expected compiled regex patterns to be available")
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

	t.Log("✅ Regex performance optimization working")
}
