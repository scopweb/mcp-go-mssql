package main

import (
	"os"
	"testing"
	"unicode/utf8"
)

func FuzzValidateBasicInput(f *testing.F) {
	f.Add("")
	f.Add("SELECT * FROM users")
	f.Add("'; DROP TABLE users; --")
	f.Add("<script>alert(1)</script>")
	f.Add("\x00\x01\x02")

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	f.Fuzz(func(t *testing.T, input string) {
		err := server.validateBasicInput(input)
		if err == nil {
			// If accepted, must be valid UTF-8 and within size limits
			if !utf8.ValidString(input) {
				t.Errorf("accepted invalid UTF-8: %q", input)
			}
			if len(input) == 0 {
				t.Error("accepted empty input")
			}
		}
	})
}

func FuzzValidateReadOnlyQuery(f *testing.F) {
	f.Add("SELECT * FROM users")
	f.Add("INSERT INTO users VALUES (1)")
	f.Add("-- comment\nSELECT 1")
	f.Add("/* block */SELECT 1")
	f.Add("'; DROP TABLE users; --")
	f.Add("UPDATE users SET x=1")
	f.Add("DELETE FROM users")

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
		config: serverConfig{
			readOnly: true,
		},
	}

	f.Fuzz(func(t *testing.T, query string) {
		// Should not panic
		_ = server.validateReadOnlyQuery(query)
	})
}

func FuzzSanitizeForLogging(f *testing.F) {
	f.Add("server=test;password=secret123;user=admin")
	f.Add("password=abc;token=xyz;key=123")
	f.Add("normal string without secrets")
	f.Add("")

	logger := NewSecurityLogger()

	f.Fuzz(func(t *testing.T, input string) {
		result := logger.sanitizeForLogging(input)
		// Sanitized output must be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("sanitized output is not valid UTF-8 for input: %q", input)
		}
	})
}

func FuzzStripLeadingComments(f *testing.F) {
	f.Add("SELECT 1")
	f.Add("-- comment\nSELECT 1")
	f.Add("/* block */ SELECT 1")
	f.Add("-- unclosed")
	f.Add("/* unclosed")
	f.Add("  \t\n  SELECT 1")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := stripLeadingComments(input)
		if !utf8.ValidString(result) {
			t.Errorf("result is not valid UTF-8 for input: %q", input)
		}
	})
}

func FuzzExtractOperation(f *testing.F) {
	f.Add("SELECT * FROM users")
	f.Add("INSERT INTO t VALUES (1)")
	f.Add("UPDATE t SET x=1")
	f.Add("DELETE FROM t")
	f.Add("-- comment\nDROP TABLE t")
	f.Add("WITH cte AS (SELECT 1) UPDATE t SET x=1")

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	f.Fuzz(func(t *testing.T, query string) {
		op := server.extractOperation(query)
		validOps := map[string]bool{
			"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true,
			"DROP": true, "CREATE": true, "ALTER": true, "TRUNCATE": true, "MERGE": true,
		}
		if !validOps[op] {
			t.Errorf("unexpected operation %q for query: %q", op, query)
		}
	})
}

func FuzzExtractAllTablesFromQuery(f *testing.F) {
	f.Add("SELECT * FROM users")
	f.Add("SELECT * FROM [dbo].[users] JOIN orders ON 1=1")
	f.Add("INSERT INTO temp SELECT * FROM src")
	f.Add("")

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}

	f.Fuzz(func(t *testing.T, query string) {
		tables := server.extractAllTablesFromQuery(query)
		for _, tbl := range tables {
			if tbl == "" {
				t.Error("empty table name extracted")
			}
		}
	})
}

func FuzzValidateTablePermissions(f *testing.F) {
	f.Add("SELECT * FROM users")
	f.Add("UPDATE temp_ai SET x=1")
	f.Add("DELETE temp_ai FROM temp_ai JOIN users ON 1=1")
	f.Add("INSERT INTO temp_ai SELECT * FROM secrets")

	server := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
		config: serverConfig{
			readOnly:        true,
			whitelistTables: []string{"temp_ai"},
		},
	}

	// Set env for any remaining os.Getenv calls
	os.Setenv("MSSQL_READ_ONLY", "true")
	os.Setenv("MSSQL_WHITELIST_TABLES", "temp_ai")

	f.Fuzz(func(t *testing.T, query string) {
		// Should not panic
		_ = server.validateTablePermissions(query)
	})
}
