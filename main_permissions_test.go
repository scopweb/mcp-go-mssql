package main

import (
	"os"
	"testing"
)

// TestExtractAllTablesFromQuery tests table extraction from various SQL queries
func TestExtractAllTablesFromQuery(t *testing.T) {
	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "Simple SELECT",
			query:    "SELECT * FROM users",
			expected: []string{"users"},
		},
		{
			name:     "SELECT with JOIN",
			query:    "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			expected: []string{"users", "orders"},
		},
		{
			name:     "INSERT SELECT",
			query:    "INSERT INTO temp_ai SELECT * FROM products",
			expected: []string{"temp_ai", "products"},
		},
		{
			name:     "UPDATE with subquery",
			query:    "UPDATE temp_ai SET col = (SELECT value FROM secrets WHERE id = 1)",
			expected: []string{"temp_ai", "secrets"},
		},
		{
			name:     "DELETE with JOIN",
			query:    "DELETE temp_ai FROM temp_ai t1 INNER JOIN users t2 ON t1.id = t2.id",
			expected: []string{"temp_ai", "users"},
		},
		{
			name:     "Multiple JOINs",
			query:    "SELECT * FROM temp_ai t1 JOIN products p ON t1.id = p.id JOIN orders o ON p.id = o.product_id",
			expected: []string{"temp_ai", "products", "orders"},
		},
		{
			name:     "CREATE VIEW",
			query:    "CREATE VIEW v_temp_ia AS SELECT * FROM temp_ai",
			expected: []string{"v_temp_ia", "temp_ai"},
		},
		{
			name:     "DROP TABLE",
			query:    "DROP TABLE temp_ai",
			expected: []string{"temp_ai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tables := server.extractAllTablesFromQuery(tt.query)

			// Check if all expected tables are found
			for _, expectedTable := range tt.expected {
				found := false
				for _, table := range tables {
					if table == expectedTable {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected table '%s' not found in query: %s\nFound tables: %v",
						expectedTable, tt.query, tables)
				}
			}

			// Check for unexpected tables (optional warning)
			if len(tables) != len(tt.expected) {
				t.Logf("Warning: Expected %d tables, found %d in query: %s",
					len(tt.expected), len(tables), tt.query)
			}
		})
	}
}

// TestValidateTablePermissions tests the whitelist validation logic
func TestValidateTablePermissions(t *testing.T) {
	// Save and restore original env vars
	originalReadOnly := os.Getenv("MSSQL_READ_ONLY")
	originalWhitelist := os.Getenv("MSSQL_WHITELIST_TABLES")
	defer func() {
		os.Setenv("MSSQL_READ_ONLY", originalReadOnly)
		os.Setenv("MSSQL_WHITELIST_TABLES", originalWhitelist)
	}()

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	tests := []struct {
		name          string
		readOnly      string
		whitelist     string
		query         string
		shouldSucceed bool
		description   string
	}{
		{
			name:          "SELECT allowed without whitelist",
			readOnly:      "true",
			whitelist:     "",
			query:         "SELECT * FROM users",
			shouldSucceed: true,
			description:   "SELECT queries should always be allowed in read-only mode",
		},
		{
			name:          "UPDATE blocked without whitelist",
			readOnly:      "true",
			whitelist:     "",
			query:         "UPDATE users SET name = 'test'",
			shouldSucceed: false,
			description:   "UPDATE should be blocked when whitelist is empty",
		},
		{
			name:          "UPDATE allowed on whitelisted table",
			readOnly:      "true",
			whitelist:     "temp_ai,v_temp_ia",
			query:         "UPDATE temp_ai SET col = 'value'",
			shouldSucceed: true,
			description:   "UPDATE should be allowed on whitelisted table",
		},
		{
			name:          "UPDATE blocked on non-whitelisted table",
			readOnly:      "true",
			whitelist:     "temp_ai,v_temp_ia",
			query:         "UPDATE users SET name = 'test'",
			shouldSucceed: false,
			description:   "UPDATE should be blocked on non-whitelisted table",
		},
		{
			name:          "DELETE with JOIN to non-whitelisted table blocked",
			readOnly:      "true",
			whitelist:     "temp_ai",
			query:         "DELETE temp_ai FROM temp_ai t1 INNER JOIN users t2 ON t1.id = t2.id",
			shouldSucceed: false,
			description:   "DELETE with JOIN to non-whitelisted table should be blocked",
		},
		{
			name:          "INSERT SELECT from non-whitelisted table blocked",
			readOnly:      "true",
			whitelist:     "temp_ai",
			query:         "INSERT INTO temp_ai SELECT * FROM products",
			shouldSucceed: false,
			description:   "INSERT...SELECT from non-whitelisted table should be blocked",
		},
		{
			name:          "UPDATE with subquery from non-whitelisted table blocked",
			readOnly:      "true",
			whitelist:     "temp_ai",
			query:         "UPDATE temp_ai SET col = (SELECT password FROM users)",
			shouldSucceed: false,
			description:   "UPDATE with subquery accessing non-whitelisted table should be blocked",
		},
		{
			name:          "CREATE VIEW allowed on whitelisted name",
			readOnly:      "true",
			whitelist:     "v_temp_ia,temp_ai",
			query:         "CREATE VIEW v_temp_ia AS SELECT * FROM temp_ai",
			shouldSucceed: true,
			description:   "CREATE VIEW should be allowed when all tables are whitelisted",
		},
		{
			name:          "All operations allowed in non-read-only mode",
			readOnly:      "false",
			whitelist:     "",
			query:         "DELETE FROM users WHERE id = 1",
			shouldSucceed: true,
			description:   "All operations should be allowed when read-only mode is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			os.Setenv("MSSQL_READ_ONLY", tt.readOnly)
			os.Setenv("MSSQL_WHITELIST_TABLES", tt.whitelist)

			err := server.validateTablePermissions(tt.query)

			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected query to succeed but got error: %v\nQuery: %s\nDescription: %s",
					err, tt.query, tt.description)
			}

			if !tt.shouldSucceed && err == nil {
				t.Errorf("Expected query to fail but it succeeded\nQuery: %s\nDescription: %s",
					tt.query, tt.description)
			}
		})
	}
}

// TestGetWhitelistedTables tests whitelist parsing
func TestGetWhitelistedTables(t *testing.T) {
	// Save and restore original env var
	originalWhitelist := os.Getenv("MSSQL_WHITELIST_TABLES")
	defer os.Setenv("MSSQL_WHITELIST_TABLES", originalWhitelist)

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	tests := []struct {
		name       string
		whitelist  string
		expected   []string
		shouldHave []string
	}{
		{
			name:       "Empty whitelist",
			whitelist:  "",
			expected:   []string{},
			shouldHave: []string{},
		},
		{
			name:       "Single table",
			whitelist:  "temp_ai",
			expected:   []string{"temp_ai"},
			shouldHave: []string{"temp_ai"},
		},
		{
			name:       "Multiple tables",
			whitelist:  "temp_ai,v_temp_ia",
			expected:   []string{"temp_ai", "v_temp_ia"},
			shouldHave: []string{"temp_ai", "v_temp_ia"},
		},
		{
			name:       "Tables with spaces",
			whitelist:  " temp_ai , v_temp_ia , another_table ",
			expected:   []string{"temp_ai", "v_temp_ia", "another_table"},
			shouldHave: []string{"temp_ai", "v_temp_ia", "another_table"},
		},
		{
			name:       "Case normalization",
			whitelist:  "TEMP_AI,V_Temp_Ia",
			expected:   []string{"temp_ai", "v_temp_ia"},
			shouldHave: []string{"temp_ai", "v_temp_ia"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MSSQL_WHITELIST_TABLES", tt.whitelist)

			tables := server.getWhitelistedTables()

			// Check if all expected tables are present
			for _, expectedTable := range tt.shouldHave {
				found := false
				for _, table := range tables {
					if table == expectedTable {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected table '%s' not found in whitelist\nFound: %v",
						expectedTable, tables)
				}
			}

			// Check count
			if len(tables) != len(tt.expected) {
				t.Errorf("Expected %d tables, found %d\nExpected: %v\nFound: %v",
					len(tt.expected), len(tables), tt.expected, tables)
			}
		})
	}
}

// TestExtractOperation tests SQL operation extraction
func TestExtractOperation(t *testing.T) {
	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "SELECT",
			query:    "SELECT * FROM users",
			expected: "SELECT",
		},
		{
			name:     "INSERT",
			query:    "INSERT INTO temp_ai VALUES (1, 'test')",
			expected: "INSERT",
		},
		{
			name:     "UPDATE",
			query:    "UPDATE temp_ai SET col = 'value'",
			expected: "UPDATE",
		},
		{
			name:     "DELETE",
			query:    "DELETE FROM temp_ai WHERE id = 1",
			expected: "DELETE",
		},
		{
			name:     "CREATE TABLE",
			query:    "CREATE TABLE temp_ai (id INT, name VARCHAR(50))",
			expected: "CREATE",
		},
		{
			name:     "DROP TABLE",
			query:    "DROP TABLE temp_ai",
			expected: "DROP",
		},
		{
			name:     "WITH CTE followed by UPDATE",
			query:    "WITH cte AS (SELECT id FROM users) UPDATE temp_ai SET col = (SELECT id FROM cte)",
			expected: "UPDATE",
		},
		{
			name:     "Query with comments",
			query:    "-- This is a comment\nUPDATE temp_ai SET col = 'value'",
			expected: "UPDATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation := server.extractOperation(tt.query)

			if operation != tt.expected {
				t.Errorf("Expected operation '%s', got '%s' for query: %s",
					tt.expected, operation, tt.query)
			}
		})
	}
}
