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
		{"stacked without semicolon", "SELECT 1\nDELETE FROM prod", true},
		{"admin then delete without semicolon", "EXEC sys.sp_help N'x'\nDELETE FROM prod", true},
		{"union select is one statement", "SELECT a FROM t UNION SELECT b FROM u", false},
		{"select into with comment separator", "SELECT 1 AS id INTO/**/prod", false},
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

// TestWhitelistEnforcedWhenReadOnlyFalse covers the fix for the writable-alias
// whitelist bypass: a configured table whitelist must be enforced even when
// read-only mode is disabled (MSSQL_DYNAMIC_<ALIAS>_READ_ONLY=false with
// _WHITELIST_TABLES, or the classic equivalent). Previously the whitelist was
// silently ignored in that combination, granting full write access.
func TestWhitelistEnforcedWhenReadOnlyFalse(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = false
	server.config.whitelistTables = parseWhitelistTables("temp_ai")

	// Modification on a non-whitelisted table must be denied.
	if err := server.validateTablePermissions("DELETE FROM users WHERE id = 1"); err == nil {
		t.Error("expected DELETE on non-whitelisted table to be denied when a whitelist is configured (readOnly=false)")
	}

	// Modification on a whitelisted table must pass the permission check
	// (classic context: no active alias, so no confirmation is required).
	if err := server.validateTablePermissions("DELETE FROM temp_ai WHERE id = 1"); err != nil {
		t.Errorf("expected DELETE on whitelisted table to be allowed, got: %v", err)
	}

	// validateReadOnlyQuery must defer modification checks to the whitelist
	// branch instead of returning early.
	if err := server.validateReadOnlyQuery("DELETE FROM temp_ai WHERE id = 1"); err != nil {
		t.Errorf("validateReadOnlyQuery should defer to the whitelist check, got: %v", err)
	}

	// Dangerous system procedures must still be blocked in whitelist mode.
	if err := server.validateReadOnlyQuery("EXEC xp_cmdshell 'dir'"); err == nil {
		t.Error("expected xp_cmdshell to be blocked in whitelist mode even with readOnly=false")
	}

	// Stacked statements must still be rejected in a whitelist-restricted context.
	if err := server.validateReadOnlyQuery("SELECT 1; DELETE FROM temp_ai"); err == nil {
		t.Error("expected stacked query to be blocked in whitelist mode even with readOnly=false")
	}
}

// TestConfirmationBoundToAlias covers the fix for cross-alias confirmation
// portability: a confirmation issued for writable alias A must not be
// consumable after switching to writable alias B.
func TestConfirmationBoundToAlias(t *testing.T) {
	newServer := func() *MCPMSSQLServer {
		s := newTestMCPServer()
		s.isDynamic = true
		s.pendingConfirmation = &PendingConfirmation{
			Operation:   "DELETE",
			Tables:      []string{"temp_ai"},
			Description: "DELETE on tables: temp_ai (alias APP_A)",
			Alias:       "APP_A",
			QueryHash:   queryHash("DELETE FROM temp_ai"),
			Approved:    true,
			ExpiresAt:   time.Now().Add(time.Minute),
		}
		return s
	}

	// A different alias must not consume the confirmation (and it stays pending).
	s := newServer()
	if s.isOperationConfirmed("DELETE", []string{"temp_ai"}, "APP_B", "DELETE FROM temp_ai") {
		t.Error("confirmation issued for alias APP_A must not be consumable on alias APP_B")
	}

	// The originating alias consumes it (one-time use).
	if !s.isOperationConfirmed("DELETE", []string{"temp_ai"}, "APP_A", "DELETE FROM temp_ai") {
		t.Error("confirmation should be consumable on the alias it was issued for")
	}

	// Alias comparison is case-insensitive (aliases are normalized to upper).
	s = newServer()
	if !s.isOperationConfirmed("DELETE", []string{"temp_ai"}, "app_a", "DELETE FROM temp_ai") {
		t.Error("alias binding should be case-insensitive")
	}
}

func TestDynamicWritableAliasRequiresWhitelist(t *testing.T) {
	server := newTestMCPServer()
	server.isDynamic = true
	server.activeAlias = "WORK"
	server.dynamicAliases = map[string]DynamicAlias{
		"WORK": {Alias: "WORK", ReadOnly: false},
	}

	if err := server.validateTablePermissions("DELETE FROM temp_ai WHERE id = 1"); err == nil {
		t.Fatal("writable dynamic alias without a whitelist must deny modifications")
	}
}

func TestConfirmationRequiresExactApprovedQuery(t *testing.T) {
	server := newTestMCPServer()
	server.isDynamic = true
	server.activeAlias = "WORK"
	server.dynamicAliases = map[string]DynamicAlias{
		"WORK": {Alias: "WORK", ReadOnly: false, WhitelistTables: []string{"temp_ai"}},
	}
	query := "DELETE FROM temp_ai WHERE id = 1"

	if err := server.validateTablePermissions(query); err == nil {
		t.Fatal("first modification must require confirmation")
	}
	if err := server.validateTablePermissions(query); err == nil {
		t.Fatal("replaying a pending modification must not bypass confirmation")
	}

	response := server.handleToolCall("confirm", CallToolParams{
		Name:      "confirm_operation",
		Arguments: map[string]interface{}{"description": server.pendingConfirmation.Description},
	})
	if response.Result.(CallToolResult).IsError {
		t.Fatalf("exact confirmation should succeed: %+v", response.Result)
	}
	if err := server.validateTablePermissions(query); err != nil {
		t.Fatalf("approved exact query should be allowed: %v", err)
	}
	if err := server.validateTablePermissions("DELETE FROM temp_ai WHERE id = 2"); err == nil {
		t.Fatal("approval must not cover a different statement")
	}
}

func TestReadOnlyClassifiesSelectIntoAsWrite(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	query := "SELECT 1 AS id INTO temp_ai"
	if err := server.validateReadOnlyQuery(query); err == nil {
		t.Fatal("SELECT INTO must be denied without a whitelist")
	}

	server.config.whitelistTables = []string{"temp_ai"}
	if err := server.validateReadOnlyQuery(query); err == nil {
		t.Fatal("SELECT INTO must remain blocked when READ_ONLY is true, even with a whitelist")
	}
}

func TestReadOnlyIsAbsoluteWithWhitelist(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = []string{"temp_ai"}

	for _, query := range []string{
		"INSERT INTO temp_ai VALUES (1)",
		"UPDATE temp_ai SET value = 1",
		"DELETE FROM temp_ai",
		"CREATE TABLE temp_ai_copy (id int)",
		"DROP TABLE temp_ai",
		"MERGE temp_ai AS t USING temp_ai AS s ON 1 = 0 WHEN NOT MATCHED THEN INSERT (id) VALUES (1)",
	} {
		if err := server.validateReadOnlyQuery(query); err == nil {
			t.Errorf("READ_ONLY=true allowed write query %q", query)
		}
		if err := server.validateTablePermissions(query); err == nil {
			t.Errorf("READ_ONLY=true permission gate allowed write query %q", query)
		}
	}
}

func TestStackedStatementsWithoutSemicolonAreBlocked(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = []string{"Ferratge", "_Consultas"}

	blocked := []string{
		"EXEC sys.sp_help N'dbo.Ferratge'\nDELETE FROM Ferratge",
		"SELECT 1\nDELETE FROM Ferratge",
		"SELECT 1 AS id INTO/**/dbo.audit_probe",
		"SELECT 1 SET TRANSACTION ISOLATION LEVEL SERIALIZABLE",
		"EXEC sys.sp_spaceused @updateusage = N'TRUE'",
		"EXEC app.sp_help N'dbo.Ferratge'",
		"EXEC(N'DEL' + N'ETE FROM Ferratge')",
		"SELECT 1; DELETE FROM Ferratge",
		"SELECT 1\nGO\nDELETE FROM Ferratge",
	}
	for _, q := range blocked {
		if err := server.validateReadOnlyQuery(q); err == nil {
			t.Errorf("read-only query was accepted: %q", q)
		}
		if err := server.validateTablePermissions(q); err == nil && !strings.HasPrefix(strings.ToUpper(stripLeadingComments(q)), "SELECT") && !strings.HasPrefix(strings.ToUpper(stripLeadingComments(q)), "EXEC") {
			t.Errorf("table permission gate accepted write query: %q", q)
		}
	}

	allowed := []string{
		"SELECT TOP 10 * FROM sys.tables",
		"EXEC sys.sp_help N'dbo.Ferratge'",
		"EXEC dbo.sp_columns @table_name = N'Ferratge'",
		"SELECT a FROM t UNION SELECT b FROM t",
		"WITH cte AS (SELECT 1 AS id) SELECT * FROM cte",
	}
	for _, q := range allowed {
		if err := server.validateReadOnlyQuery(q); err != nil {
			t.Errorf("legitimate read-only query was blocked: %q (%v)", q, err)
		}
	}
}

func TestWritableWhitelistStillRejectsUnseparatedStack(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = false
	server.config.whitelistTables = []string{"Ferratge"}
	if err := server.validateReadOnlyQuery("SELECT 1\nDELETE FROM Ferratge"); err == nil {
		t.Fatal("writable whitelist must still reject concatenated statements")
	}
}

func TestReadOnlyAllowsOnlySafeAdministrativeProcedures(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = []string{"temp_ai"}
	if err := server.validateReadOnlyQuery("EXEC sp_help 'dbo.temp_ai'"); err != nil {
		t.Fatalf("safe administrative procedure should remain available: %v", err)
	}
	if err := server.validateReadOnlyQuery("EXEC dbo.unlisted_procedure"); err == nil {
		t.Fatal("unlisted procedure must be blocked in read-only mode")
	}
}

func TestTableWhitelistCannotAuthorizeProcedureViaQuery(t *testing.T) {
	server := newTestMCPServer()
	server.config.readOnly = true
	server.config.whitelistTables = []string{"temp_ai"}
	if err := server.validateReadOnlyQuery("EXEC dbo.unlisted_procedure"); err == nil {
		t.Fatal("procedure execution must use its dedicated procedure whitelist")
	}
}

func TestSanitizeForLoggingRedactsURLPassword(t *testing.T) {
	secret := "must-not-appear"
	got := NewSecurityLogger().sanitizeForLogging("sqlserver://user:" + secret + "@db.example:1433?database=app")
	if strings.Contains(got, secret) {
		t.Fatalf("URL password leaked in sanitized log: %q", got)
	}
}

func TestProductionRejectsCustomDSNWithoutEncryption(t *testing.T) {
	t.Setenv("DEVELOPER_MODE", "false")
	t.Setenv("MSSQL_ALLOW_INSECURE_TLS", "false")
	t.Setenv("MSSQL_CONNECTION_STRING", "server=db.example;database=app;user id=user;password=pass")
	if _, err := buildSecureConnectionString(); err == nil {
		t.Fatal("production must reject a custom DSN without explicit encryption")
	}
}

func TestDynamicAliasWithUnderscoreAndInvalidReadOnly(t *testing.T) {
	t.Setenv("MSSQL_DYNAMIC_APP_MAIN_SERVER", "db.example")
	t.Setenv("MSSQL_DYNAMIC_APP_MAIN_DATABASE", "app")
	t.Setenv("MSSQL_DYNAMIC_APP_MAIN_READ_ONLY", "true")
	aliases := loadDynamicAliases(NewSecurityLogger())
	if _, ok := aliases["APP_MAIN"]; !ok {
		t.Fatalf("alias with underscore was not loaded: %#v", aliases)
	}

	t.Setenv("MSSQL_DYNAMIC_APP_MAIN_READ_ONLY", "tru")
	aliases = loadDynamicAliases(NewSecurityLogger())
	if _, ok := aliases["APP_MAIN"]; ok {
		t.Fatal("alias with an invalid READ_ONLY value must be rejected")
	}
}

// TestProductionTLSViolationWhitespaceVariants ensures the production TLS
// fail-closed check cannot be evaded by whitespace around '=' or by boolean
// synonyms in the connection string.
func TestProductionTLSViolationWhitespaceVariants(t *testing.T) {
	violating := []string{
		"server=x;encrypt=false",
		"server=x;encrypt = false",
		"server=x; encrypt=disable ",
		"server=x;encrypt\t=\tdisable",
		"server=x;encrypt=no",
		"server=x;encrypt=0",
		"sqlserver://u:p@h:1433?database=d&encrypt=disable&trustservercertificate=true",
		"server=x;trustservercertificate=true",
		"server=x;trustservercertificate = true",
		"server=x;trustservercertificate=yes",
		"server=x;trustservercertificate=1",
	}
	for _, cs := range violating {
		if productionTLSViolation(strings.ToLower(cs)) == "" {
			t.Errorf("expected TLS violation to be detected for %q", cs)
		}
	}

	safe := []string{
		"server=x;encrypt=true;trustservercertificate=false",
		"server=x;encrypt=strict;trustservercertificate=false",
		"server=x;database=my reporting db;encrypt=true",
		"sqlserver://u:p@h:1433?database=d&encrypt=true",
	}
	for _, cs := range safe {
		if v := productionTLSViolation(strings.ToLower(cs)); v != "" {
			t.Errorf("unexpected TLS violation for %q: %s", cs, v)
		}
	}
}

// Ensure the test file's intent strings stay readable in failures.
var _ = strings.TrimSpace
