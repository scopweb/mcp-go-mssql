package main

import (
	"strings"
	"testing"

	"mcp-go-mssql/internal/sqlguard"
)

// TestExtractAllTablesFromQuery tests table extraction from various SQL queries
func TestExtractAllTablesFromQuery(t *testing.T) {
	server := newTestMCPServer()

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
	server := newTestMCPServer()

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
			// Update cached config for this test case
			server.config.readOnly = tt.readOnly == "true"
			server.config.whitelistTables = sqlguard.ParseWhitelistTables(tt.whitelist)
			newTestGuard(server)

			err := server.guard.ValidateTablePermissions(tt.query)

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
	server := newTestMCPServer()

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
			// Update cached config for this test case
			server.config.whitelistTables = sqlguard.ParseWhitelistTables(tt.whitelist)

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
			operation := sqlguard.ExtractOperation(tt.query)

			if operation != tt.expected {
				t.Errorf("Expected operation '%s', got '%s' for query: %s",
					tt.expected, operation, tt.query)
			}
		})
	}
}

// TestBUG001_StripStringLiterals verifies that stripStringLiterals removes content
// inside string literals so that security patterns inside strings don't trigger
// false positives (e.g. 'WITH (NOLOCK) is dangerous' should not be flagged).
func TestBUG001_StripStringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // should NOT be present in output
	}{
		{
			name:     "NOLOCK inside single-quoted string is removed",
			input:    "SELECT 'WITH (NOLOCK) es peligrosa' AS info",
			contains: "NOLOCK",
		},
		{
			name:     "SQL keyword inside string is removed",
			input:    "SELECT 'DROP TABLE users' AS warning_text",
			contains: "DROP",
		},
		{
			name:     "WAITFOR inside string is removed",
			input:    "SELECT 'WAITFOR DELAY is bad' AS info",
			contains: "WAITFOR",
		},
		{
			name:     "Escaped quotes preserved correctly",
			input:    "SELECT 'it''s a test' AS info",
			contains: "test", // content between quotes should be stripped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqlguard.StripStringLiterals(tt.input)
			if strings.Contains(result, tt.contains) {
				t.Errorf("stripStringLiterals should remove '%s' from string literals.\nInput:  %s\nOutput: %s",
					tt.contains, tt.input, result)
			}
		})
	}

	// Verify that content OUTSIDE strings is preserved
	t.Run("content outside strings preserved", func(t *testing.T) {
		input := "SELECT col FROM users WHERE name = 'test value'"
		result := sqlguard.StripStringLiterals(input)
		if !strings.Contains(result, "SELECT") || !strings.Contains(result, "FROM") || !strings.Contains(result, "users") {
			t.Errorf("stripStringLiterals should preserve SQL outside strings.\nInput:  %s\nOutput: %s", input, result)
		}
	})

	// Verify quotes themselves are preserved (structure intact)
	t.Run("quote markers preserved", func(t *testing.T) {
		input := "SELECT 'hello' AS col"
		result := sqlguard.StripStringLiterals(input)
		if !strings.Contains(result, "''") {
			t.Errorf("stripStringLiterals should preserve empty quotes ''.\nInput:  %s\nOutput: %s", input, result)
		}
	})
}

// TestBUG001_NoFalsePositiveOnHintsInStrings verifies the full chain:
// containsDangerousHints should NOT flag NOLOCK inside a string literal.
func TestBUG001_NoFalsePositiveOnHintsInStrings(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantHit bool
	}{
		{
			name:    "Real NOLOCK hint — should be detected",
			query:   "SELECT * FROM users WITH (NOLOCK)",
			wantHit: true,
		},
		{
			name:    "NOLOCK inside string literal — should NOT be detected",
			query:   "SELECT 'WITH (NOLOCK) es peligrosa' AS info",
			wantHit: false,
		},
		{
			name:    "READUNCOMMITTED inside string — should NOT be detected",
			query:   "SELECT 'WITH (READUNCOMMITTED)' AS info",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqlguard.ContainsDangerousHints(tt.query)
			if got != tt.wantHit {
				t.Errorf("sqlguard.ContainsDangerousHints(%q) = %v, want %v", tt.query, got, tt.wantHit)
			}
		})
	}
}

// TestBUG001_StructuralSafetyNoFalsePositive verifies the full validation chain
// does not produce false positives for dangerous patterns inside string literals.
func TestBUG001_StructuralSafetyNoFalsePositive(t *testing.T) {
	safeLiteralQueries := []struct {
		name  string
		query string
	}{
		{"NOLOCK in string", "SELECT 'WITH (NOLOCK) es peligrosa' AS info"},
		{"WAITFOR in string", "SELECT 'WAITFOR DELAY is dangerous' AS info"},
		{"OPENROWSET in string", "SELECT 'OPENROWSET is blocked' AS info"},
	}

	for _, tt := range safeLiteralQueries {
		t.Run(tt.name, func(t *testing.T) {
			err := sqlguard.ValidateStructuralSafety(tt.query)
			if err != nil {
				t.Errorf("False positive: sqlguard.ValidateStructuralSafety(%q) returned error: %v", tt.query, err)
			}
		})
	}
}

// TestBUG002_SubquerySystemSchemaBlocked verifies that subqueries referencing
// system schema tables (sys.*, INFORMATION_SCHEMA.*) are blocked by whitelist validation.
func TestBUG002_SubquerySystemSchemaBlocked(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = []string{"users", "orders"}
	newTestGuard(server)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "sys.objects in subquery — must be blocked",
			query:   "SELECT * FROM (SELECT name FROM sys.objects) AS x",
			wantErr: true,
		},
		{
			name:    "sys.columns in subquery — must be blocked",
			query:   "SELECT * FROM (SELECT name FROM sys.columns) AS x",
			wantErr: true,
		},
		{
			name:    "INFORMATION_SCHEMA.TABLES in subquery — must be blocked",
			query:   "SELECT * FROM (SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES) AS x",
			wantErr: true,
		},
		{
			name:    "Whitelisted table in subquery — should pass",
			query:   "SELECT * FROM (SELECT name FROM users) AS x",
			wantErr: false,
		},
		{
			name:    "Non-whitelisted user table in subquery — must be blocked",
			query:   "SELECT * FROM (SELECT secret FROM passwords) AS x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.guard.ValidateSubqueriesForRestrictedTables(tt.query)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for query: %s", tt.query)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for query: %s, got: %v", tt.query, err)
			}
		})
	}
}

// TestBUG002_ExtractTableRefsIncludesSystemSchemas verifies that extractTableRefs
// includes system schema tables with qualified names (e.g. "sys.objects").
func TestBUG002_ExtractTableRefsIncludesSystemSchemas(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectTable   string
		shouldContain bool
	}{
		{
			name:          "sys.objects extracted with qualified name",
			query:         "SELECT name FROM sys.objects",
			expectTable:   "sys.objects",
			shouldContain: true,
		},
		{
			name:          "sys.columns extracted with qualified name",
			query:         "SELECT * FROM sys.columns WHERE object_id = 1",
			expectTable:   "sys.columns",
			shouldContain: true,
		},
		{
			name:          "INFORMATION_SCHEMA.TABLES extracted",
			query:         "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES",
			expectTable:   "information_schema.tables",
			shouldContain: true,
		},
		{
			name:          "Regular table still works",
			query:         "SELECT * FROM users",
			expectTable:   "users",
			shouldContain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := sqlguard.ExtractTableRefs(tt.query)
			found := false
			var tableNames []string
			for _, ref := range refs {
				tableNames = append(tableNames, ref.Table)
				if ref.Table == tt.expectTable {
					found = true
				}
			}
			if tt.shouldContain && !found {
				t.Errorf("Expected table '%s' in refs, got: %v", tt.expectTable, tableNames)
			}
		})
	}
}
