package sqlguard

import (
	"strings"
	"testing"
)

// These tests cover the headline contracts of the package: the pure
// validators and the Guard methods. The intent is to give CI a fast,
// hermetic regression net for the SQL-validation logic that used to live
// inside main.go's monolith. They are deliberately scenario-focused
// (table-driven where it pays off) rather than exhaustive line coverage —
// extending them is welcome.

func TestParseWhitelistTables(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single", "users", []string{"users"}},
		{"multi normalised", "Users, ORDERS , products", []string{"users", "orders", "products"}},
		{"wildcard literal", "*", []string{"*"}},
		{"wildcard with spaces", "  *  ", []string{"*"}},
		{"trailing empties dropped", "a,,b,  ,", []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseWhitelistTables(tc.in)
			if !equalStrSlice(got, tc.want) {
				t.Fatalf("ParseWhitelistTables(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseAllowedDatabases(t *testing.T) {
	got := ParseAllowedDatabases("DB_One, db_two , ,DB_THREE")
	want := []string{"db_one", "db_two", "db_three"}
	if !equalStrSlice(got, want) {
		t.Fatalf("ParseAllowedDatabases = %v, want %v", got, want)
	}

	if ParseAllowedDatabases("") != nil {
		t.Fatalf("empty input should return nil")
	}
}

func TestStripStringLiteralsRemovesContent(t *testing.T) {
	in := `SELECT 'DROP TABLE users' FROM t WHERE name = 'O''Brien'`
	out := StripStringLiterals(in)
	// The string contents must be gone — but the surrounding query keeps shape.
	if strings.Contains(out, "DROP TABLE") {
		t.Fatalf("string literal content leaked: %q", out)
	}
	if !strings.Contains(out, "FROM t") {
		t.Fatalf("non-literal SQL was lost: %q", out)
	}
	if strings.Count(out, "'") != 4 {
		// Two literals, two pairs of quotes preserved.
		t.Fatalf("expected 4 quote characters preserved, got %q", out)
	}
}

func TestStripLeadingComments(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                 "SELECT 1",
		"  SELECT 1":               "SELECT 1",
		"-- a\nSELECT 1":           "SELECT 1",
		"/* a */ SELECT 1":         "SELECT 1",
		"/* a */ -- b\n  SELECT 1": "SELECT 1",
	}
	for in, want := range cases {
		if got := StripLeadingComments(in); got != want {
			t.Errorf("StripLeadingComments(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractOperation(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM t":                                "SELECT",
		"  insert INTO t VALUES (1)":                     "INSERT",
		"DELETE FROM t":                                  "DELETE",
		"WITH x AS (SELECT 1) DELETE FROM t":             "DELETE",
		"WITH x AS (SELECT 1) SELECT * FROM t":           "SELECT",
		"-- comment\nUPDATE t SET a = 1":                 "UPDATE",
		"   ":                                            "SELECT",
	}
	for in, want := range cases {
		if got := ExtractOperation(in); got != want {
			t.Errorf("ExtractOperation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractDestructiveOpType(t *testing.T) {
	cases := map[string]string{
		"DROP TABLE users":     "DROP TABLE",
		"drop view v":          "DROP VIEW",
		"ALTER TABLE t ADD c":  "ALTER TABLE",
		"TRUNCATE TABLE t":     "TRUNCATE TABLE",
		"DROP procedure p":     "DROP PROC",
		"DROP function f":      "DROP FUNCTION",
		"INSERT INTO t (a)":    "INSERT (detected)",
		"":                     "UNKNOWN",
	}
	for in, want := range cases {
		if got := ExtractDestructiveOpType(in); got != want {
			t.Errorf("ExtractDestructiveOpType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractDDLTargetObjects(t *testing.T) {
	got := ExtractDDLTargetObjects("DROP TABLE [dbo].[users]")
	if len(got) != 1 || got[0].Schema != "dbo" || got[0].Name != "users" || got[0].ObjType != ObjectTypeTable {
		t.Fatalf("DROP TABLE parse: %+v", got)
	}

	got = ExtractDDLTargetObjects("ALTER VIEW reports.v_orders AS SELECT 1")
	if len(got) != 1 || got[0].Schema != "reports" || got[0].Name != "v_orders" || got[0].ObjType != ObjectTypeView {
		t.Fatalf("ALTER VIEW parse: %+v", got)
	}

	if got := ExtractDDLTargetObjects("SELECT 1"); len(got) != 0 {
		t.Fatalf("non-DDL should return empty, got %+v", got)
	}
}

func TestExtractTableRefs(t *testing.T) {
	got := ExtractTableRefs("SELECT * FROM [Sales].[Orders] o JOIN dbo.customers c ON c.id = o.customer_id")
	if len(got) < 2 {
		t.Fatalf("expected ≥2 refs, got %+v", got)
	}

	// System schema preserved as schema.table in the Table field.
	got = ExtractTableRefs("SELECT name FROM sys.objects")
	found := false
	for _, r := range got {
		if r.Table == "sys.objects" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected sys.objects to be preserved as schema.table, got %+v", got)
	}

	// Cross-database qualifier surfaces in Database.
	got = ExtractTableRefs("SELECT * FROM OtherDB.dbo.foo")
	if len(got) == 0 || got[0].Database != "otherdb" {
		t.Fatalf("expected cross-db reference to OtherDB, got %+v", got)
	}

	// Subquery sweep finds nested table refs.
	got = ExtractTableRefs("SELECT * FROM (SELECT name FROM sys.objects) AS x")
	found = false
	for _, r := range got {
		if r.Table == "sys.objects" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("subquery sweep missed sys.objects: %+v", got)
	}
}

func TestValidateStructuralSafety_BlocksCommentedKeyword(t *testing.T) {
	bad := []string{
		"SELECT 1 /* DROP */ FROM t",
		"/* DROP */ SELECT 1",
		"SELECT 1 -- DROP\nFROM t",
		// CHAR concatenation
		"SELECT CHAR(83)+CHAR(69)+CHAR(76)",
		// dangerous hint
		"SELECT * FROM t WITH (NOLOCK)",
		// timing
		"WAITFOR DELAY '0:0:1'",
		// exfiltration
		"SELECT * FROM OPENROWSET('a','b','c')",
		"SELECT * INTO #tmp FROM t",
		// SELECT INTO with comment between tokens — stripping comments
		// must NOT create a smuggling path; the regex still has to match
		// after StripAllComments collapses the comment to whitespace.
		"SELECT * /*x*/ INTO #tmp FROM t",
	}
	for _, q := range bad {
		if err := ValidateStructuralSafety(q); err == nil {
			t.Errorf("expected rejection for %q, got nil", q)
		}
	}

	good := []string{
		"SELECT 1 FROM t",
		"SELECT name FROM users WHERE id = @p1",
		// Quoted data containing the word DROP must NOT trigger.
		"SELECT 'this drops a table' FROM t",
		// "SELECT INTO" mentioned inside a line comment is benign and
		// must not be flagged as exfiltration. Regression test for the
		// false positive seen in the wild.
		"-- nota: este paso simula SELECT INTO sin crear tabla\nSELECT col FROM t",
		// Same idea but with a block comment.
		"/* SELECT INTO en docs */ SELECT col FROM t",
	}
	for _, q := range good {
		if err := ValidateStructuralSafety(q); err != nil {
			t.Errorf("unexpected rejection for %q: %v", q, err)
		}
	}
}

func TestValidateUnicodeSafety(t *testing.T) {
	// Latin-only is fine.
	if err := ValidateUnicodeSafety("SELECT * FROM users"); err != nil {
		t.Errorf("clean ASCII rejected: %v", err)
	}

	// Cyrillic homoglyph spelling out SELECT must be rejected.
	if err := ValidateUnicodeSafety("ѕELECT * FROM t"); err == nil {
		// note: 'ѕ' is U+0455 — close enough to bait the homoglyph detector
		t.Errorf("expected rejection for Cyrillic-prefixed SELECT")
	}
}

func TestGuard_ValidateReadOnly_StrictNoWhitelist(t *testing.T) {
	g := New(Config{ReadOnly: true})

	// SELECT ok
	if err := g.ValidateReadOnly("SELECT * FROM users"); err != nil {
		t.Errorf("SELECT rejected: %v", err)
	}
	// INSERT blocked
	if err := g.ValidateReadOnly("INSERT INTO t VALUES (1)"); err == nil {
		t.Error("INSERT not rejected under strict read-only")
	}
	// xp_cmdshell always blocked
	if err := g.ValidateReadOnly("EXEC xp_cmdshell 'whoami'"); err == nil {
		t.Error("xp_cmdshell not rejected")
	}
}

func TestGuard_ValidateReadOnly_WithWhitelist(t *testing.T) {
	g := New(Config{ReadOnly: true, Whitelist: []string{"temp_ai"}})

	// Modification flows through to ValidateTablePermissions — read-only itself only
	// blocks the dangerous SP set when whitelist is present.
	if err := g.ValidateReadOnly("INSERT INTO temp_ai VALUES (1)"); err != nil {
		t.Errorf("read-only-with-whitelist should let INSERT through: %v", err)
	}
	if err := g.ValidateReadOnly("EXEC sp_executesql N'SELECT 1'"); err == nil {
		t.Error("sp_executesql not rejected under read-only-with-whitelist")
	}
}

func TestGuard_ValidateReadOnly_Disabled(t *testing.T) {
	g := New(Config{ReadOnly: false})
	if err := g.ValidateReadOnly("DROP TABLE users"); err != nil {
		t.Errorf("read-only disabled must allow anything: %v", err)
	}
}

func TestGuard_ValidateTablePermissions(t *testing.T) {
	g := New(Config{ReadOnly: true, Whitelist: []string{"temp_ai"}})

	// Modify on whitelisted ok
	if err := g.ValidateTablePermissions("INSERT INTO temp_ai (a) VALUES (1)"); err != nil {
		t.Errorf("whitelisted insert rejected: %v", err)
	}
	// Modify on non-whitelisted rejected
	if err := g.ValidateTablePermissions("INSERT INTO users (a) VALUES (1)"); err == nil {
		t.Error("non-whitelisted insert not rejected")
	}
	// JOIN to non-whitelisted table is rejected even when modify target is whitelisted
	if err := g.ValidateTablePermissions("DELETE temp_ai FROM temp_ai t JOIN users u ON u.id = t.user_id"); err == nil {
		t.Error("multi-table modify with non-whitelisted JOIN target not rejected")
	}
	// Cross-database modify always blocked
	if err := g.ValidateTablePermissions("INSERT INTO OtherDB.dbo.temp_ai VALUES (1)"); err == nil {
		t.Error("cross-db modify not rejected")
	}
	// SELECT bypasses the per-table check
	if err := g.ValidateTablePermissions("SELECT * FROM users"); err != nil {
		t.Errorf("SELECT should bypass permissions, got %v", err)
	}
}

func TestGuard_ValidateTablePermissions_WildcardAllowsCurrentDB(t *testing.T) {
	g := New(Config{ReadOnly: true, Whitelist: []string{"*"}})
	if err := g.ValidateTablePermissions("INSERT INTO any_table VALUES (1)"); err != nil {
		t.Errorf("wildcard whitelist rejected current-db modify: %v", err)
	}
	// But cross-database is still blocked.
	if err := g.ValidateTablePermissions("INSERT INTO OtherDB.dbo.t VALUES (1)"); err == nil {
		t.Error("wildcard whitelist must NOT allow cross-db")
	}
}

func TestGuard_ValidateSubqueriesForRestrictedTables(t *testing.T) {
	g := New(Config{ReadOnly: true, Whitelist: []string{"temp_ai"}})

	// Subquery touches non-whitelisted users → rejected.
	if err := g.ValidateSubqueriesForRestrictedTables("SELECT * FROM (SELECT name FROM users) x"); err == nil {
		t.Error("subquery referencing non-whitelisted table not rejected")
	}
	// Subquery only touches whitelisted → allowed.
	if err := g.ValidateSubqueriesForRestrictedTables("SELECT * FROM (SELECT a FROM temp_ai) x"); err != nil {
		t.Errorf("whitelisted-only subquery rejected: %v", err)
	}
	// Disabled when read-only is off.
	g2 := New(Config{ReadOnly: false, Whitelist: []string{"temp_ai"}})
	if err := g2.ValidateSubqueriesForRestrictedTables("SELECT * FROM (SELECT name FROM users) x"); err != nil {
		t.Errorf("non-read-only must skip subquery check: %v", err)
	}
}

func TestGuard_IsAllowedDatabase(t *testing.T) {
	g := New(Config{AllowedDatabases: []string{"db_one", "db_two"}})
	if !g.IsAllowedDatabase("db_one") {
		t.Error("db_one should be allowed")
	}
	if g.IsAllowedDatabase("db_three") {
		t.Error("db_three should not be allowed")
	}
}

// captureLogger collects formatted lines so tests can assert that
// permission decisions are logged. Implements the Logger interface.
type captureLogger struct{ lines []string }

func (c *captureLogger) Printf(format string, args ...interface{}) {
	c.lines = append(c.lines, format)
}

func TestGuard_LoggerInjection(t *testing.T) {
	cap := &captureLogger{}
	g := New(Config{ReadOnly: true, Whitelist: []string{"temp_ai"}, Logger: cap})
	_ = g.ValidateTablePermissions("INSERT INTO temp_ai VALUES (1)")
	if len(cap.lines) == 0 {
		t.Error("expected at least one Printf call from the guard")
	}
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
