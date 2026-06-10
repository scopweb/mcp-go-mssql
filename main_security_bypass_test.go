package main

import (
	"strings"
	"testing"
	"time"
)

// TestHasMultipleStatements covers the statement-separator detector that
// closes the stacked-query bypass. It must ignore semicolons that live inside
// string literals, bracketed identifiers and comments, and must treat a lone
// trailing semicolon as a single statement.
func TestHasMultipleStatements(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  bool
	}{
		{"plain select", "SELECT * FROM users", false},
		{"trailing semicolon", "SELECT * FROM users;", false},
		{"trailing semicolon + whitespace", "SELECT 1;   \n\t", false},
		{"trailing semicolon + comment", "SELECT 1; -- done", false},
		{"trailing semicolon + block comment", "SELECT 1; /* done */", false},
		{"stacked select+delete", "SELECT 1; DELETE FROM prod", true},
		{"stacked with comment between", "SELECT 1; -- x\nDROP TABLE prod", true},
		{"semicolon inside string literal", "SELECT ';' AS sep FROM users", false},
		{"semicolon inside escaped string", "SELECT 'a''; DROP TABLE x' AS v", false},
		{"semicolon inside bracket identifier", "SELECT [col;name] FROM users", false},
		{"semicolon inside line comment", "SELECT 1 -- a;b\nFROM users", false},
		{"semicolon inside block comment", "SELECT 1 /* a;b */ FROM users", false},
		{"real second statement after literal", "SELECT 'x;y'; DELETE FROM prod", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasMultipleStatements(tc.query); got != tc.want {
				t.Errorf("hasMultipleStatements(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

// TestStackedQueryBypassBlocked verifies the headline vulnerability is fixed:
// in whitelist mode, a query that looks like a SELECT but smuggles a
// modification via a second statement must be rejected.
func TestStackedQueryBypassBlocked(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = parseWhitelistTables("temp_ai")

	bypassAttempts := []string{
		"SELECT 1; DELETE FROM prod_users",
		"SELECT * FROM temp_ai; DROP TABLE prod_users",
		"SELECT 1; UPDATE prod_users SET admin = 1",
		"SELECT 1 /* sneaky */; TRUNCATE TABLE prod_users",
	}
	for _, q := range bypassAttempts {
		t.Run(q, func(t *testing.T) {
			if err := server.validateReadOnlyQuery(q); err == nil {
				t.Errorf("expected stacked query to be blocked, but it passed: %q", q)
			}
		})
	}

	// A legitimate single SELECT with a trailing semicolon must still pass.
	if err := server.validateReadOnlyQuery("SELECT * FROM temp_ai;"); err != nil {
		t.Errorf("legitimate single SELECT was wrongly blocked: %v", err)
	}
}

// TestModifyWithNoExtractableTableFailsClosed verifies the fail-closed change:
// a modification operation whose target table the regex extractor cannot
// identify (e.g. MERGE, which has no extraction pattern) must be denied rather
// than silently allowed.
func TestModifyWithNoExtractableTableFailsClosed(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = parseWhitelistTables("temp_ai")

	// MERGE is in the modify-operation set but has no table-extraction pattern.
	// Crafted to avoid the INTO/UPDATE/DELETE/FROM keywords the extractor keys on.
	q := "MERGE prod_users AS t USING src AS s ON t.id = s.id WHEN NOT MATCHED THEN INSERT (x) VALUES (1)"
	tables := server.extractAllTablesFromQuery(q)
	if len(tables) != 0 {
		t.Skipf("extractor now finds tables %v for MERGE; test premise no longer holds", tables)
	}
	if err := server.validateTablePermissions(q); err == nil {
		t.Errorf("expected modify op with no extractable table to be denied (fail-closed), but it passed: %q", q)
	}
}

// TestProcedureParamNameInjectionPattern guards the parameter-name validation
// used by execute_procedure. Parameter names are interpolated into the EXEC
// text, so anything but a plain (optionally @-prefixed) identifier must be
// rejected.
func TestProcedureParamNameInjectionPattern(t *testing.T) {
	valid := []string{"id", "@id", "user_id", "@UserID", "p1", "_internal"}
	for _, name := range valid {
		if !validParamNamePattern.MatchString(name) {
			t.Errorf("expected %q to be a valid parameter name", name)
		}
	}

	malicious := []string{
		"x = 1; DROP TABLE Users --",
		"id=1 OR 1=1",
		"id; SHUTDOWN",
		"id WITH (NOLOCK)",
		"@id'",
		"",
		"1id",
		"id name",
	}
	for _, name := range malicious {
		if validParamNamePattern.MatchString(name) {
			t.Errorf("expected %q to be rejected as a parameter name", name)
		}
	}
}

// TestValidateDSNField ensures field-breaking characters are rejected before
// they reach an ADO-style connection string.
func TestValidateDSNField(t *testing.T) {
	bad := []string{
		"pass;encrypt=false",
		"server\nname",
		"value\rwith-cr",
		"null\x00byte",
	}
	for _, v := range bad {
		if err := validateDSNField("FIELD", v); err == nil {
			t.Errorf("expected validateDSNField to reject %q", v)
		}
	}
	good := []string{"normalPassword123!", "user@server", "Str0ng#Pass", "1433"}
	for _, v := range good {
		if err := validateDSNField("FIELD", v); err != nil {
			t.Errorf("expected validateDSNField to accept %q, got %v", v, err)
		}
	}
}

// TestBuildSecureConnectionStringRejectsInjection verifies that a password
// carrying a field separator cannot inject extra connection parameters.
func TestBuildSecureConnectionStringRejectsInjection(t *testing.T) {
	t.Setenv("MSSQL_CONNECTION_STRING", "")
	t.Setenv("MSSQL_SERVER", "srv")
	t.Setenv("MSSQL_DATABASE", "db")
	t.Setenv("MSSQL_USER", "user")
	t.Setenv("MSSQL_PASSWORD", "p;encrypt=false;trustservercertificate=true")
	t.Setenv("MSSQL_AUTH", "sql")
	t.Setenv("DEVELOPER_MODE", "false")

	if _, err := buildSecureConnectionString(); err == nil {
		t.Error("expected connection-string build to fail on injected password, got nil error")
	}
}

// TestConfirmationRequiresIntent verifies the hardened confirm_operation no
// longer accepts an arbitrary description. It must reference the operation and
// a target table (or echo the pending description).
func TestConfirmationRequiresIntent(t *testing.T) {
	newServer := func() *MCPMSSQLServer {
		s := newTestMCPServer()
		s.isDynamic = true
		s.pendingConfirmation = &PendingConfirmation{
			Operation:   "DELETE",
			Tables:      []string{"temp_ai"},
			Description: "DELETE on tables: temp_ai",
			ExpiresAt:   time.Now().Add(time.Minute),
		}
		return s
	}

	isAccepted := func(resp *MCPResponse) bool {
		r, ok := resp.Result.(CallToolResult)
		if !ok {
			return false
		}
		return !r.IsError
	}

	// Vague but long description (would have passed the old len>10 rule).
	s := newServer()
	resp := s.handleToolCall(1, CallToolParams{
		Name:      "confirm_operation",
		Arguments: map[string]interface{}{"description": "yes go ahead please do it now"},
	})
	if isAccepted(resp) {
		t.Error("vague description should NOT be accepted by hardened confirm_operation")
	}

	// Description that names the operation and the table is accepted.
	s = newServer()
	resp = s.handleToolCall(1, CallToolParams{
		Name:      "confirm_operation",
		Arguments: map[string]interface{}{"description": "delete the old rows from temp_ai"},
	})
	if !isAccepted(resp) {
		t.Errorf("intent-bearing description should be accepted, got: %+v", resp.Result)
	}

	// Echoing the pending description verbatim is accepted.
	s = newServer()
	resp = s.handleToolCall(1, CallToolParams{
		Name:      "confirm_operation",
		Arguments: map[string]interface{}{"description": "DELETE on tables: temp_ai"},
	})
	if !isAccepted(resp) {
		t.Errorf("verbatim echo should be accepted, got: %+v", resp.Result)
	}
}

// TestStackedQueryAllowedInFullAccess documents that the multi-statement guard
// only applies in restricted contexts; full-access mode is unaffected because
// there is no read-only/whitelist policy to bypass.
func TestStackedQueryAllowedInFullAccess(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = false
	if err := server.validateReadOnlyQuery("SELECT 1; DELETE FROM whatever"); err != nil {
		t.Errorf("full-access mode should not reject multi-statement queries, got: %v", err)
	}
}

// Ensure the test file's intent strings stay readable in failures.
var _ = strings.TrimSpace
