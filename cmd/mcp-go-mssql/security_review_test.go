package main

import "testing"

func TestReadOnlyQuotedSelectInto(t *testing.T) {
	s := newTestMCPServer()
	s.config.readOnly = true
	for _, q := range []string{
		`SELECT 1 AS n INTO "audit_probe"`,
		`SELECT 1 AS n INTO /* target */ "dbo" . "audit_probe"`,
		`SELECT 1 AS n INTO [audit_probe]`,
		`WITH c AS (SELECT 1 n) SELECT n INTO "audit_probe" FROM c`,
	} {
		if err := s.validateReadOnlyQuery(q); err == nil {
			t.Errorf("read-only accepted %q", q)
		}
		if err := s.validateTablePermissions(q); err == nil {
			t.Errorf("permissions accepted %q", q)
		}
	}
}

func TestWhitelistValidatesInsertDestination(t *testing.T) {
	s := newTestMCPServer()
	s.config.readOnly = false
	s.config.whitelistTables = []string{"dbo.audit_allowed"}
	for _, q := range []string{
		`INSERT dbo.audit_denied SELECT * FROM audit_allowed`,
		`INSERT TOP (1) dbo.audit_denied SELECT * FROM audit_allowed`,
		`INSERT INTO "dbo"."audit_denied" SELECT * FROM audit_allowed`,
		`INSERT /* target */ dbo . audit_denied SELECT * FROM audit_allowed`,
		`SELECT * INTO "audit_denied" FROM audit_allowed`,
	} {
		if err := s.validateTablePermissions(q); err == nil {
			t.Errorf("permissions accepted %q", q)
		}
	}
	for _, q := range []string{
		`INSERT audit_allowed SELECT * FROM audit_allowed`,
		`INSERT INTO audit_allowed VALUES (1)`,
		`INSERT dbo.audit_allowed VALUES (1)`,
		`INSERT TOP (1) "dbo"."audit_allowed" VALUES (1)`,
	} {
		if err := s.validateTablePermissions(q); err != nil {
			t.Errorf("valid query rejected: %v", err)
		}
	}
}

func TestConfiguredReadOnly(t *testing.T) {
	for _, tc := range []struct {
		value             string
		readOnly, invalid bool
	}{
		{"", true, false}, {"true", true, false}, {" TRUE ", true, false},
		{"false", false, false}, {" false ", false, false},
		{"tru", true, true}, {"0", true, true},
	} {
		got, err := configuredReadOnly(tc.value)
		if got != tc.readOnly || (err != nil) != tc.invalid {
			t.Errorf("%q: got %v, %v", tc.value, got, err)
		}
	}
}
