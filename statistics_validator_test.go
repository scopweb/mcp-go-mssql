package main

import (
	"strings"
	"testing"
)

// TestValidateReadOnlyQuery_StatisticsWrapper covers the wrapper branch
// added to validateReadOnlyQuery. Each case uses newTestMCPServer() and
// toggles readOnly / whitelistTables explicitly, then calls
// validateReadOnlyQuery directly.
func TestValidateReadOnlyQuery_StatisticsWrapper(t *testing.T) {
	cases := []struct {
		name           string
		query          string
		readOnly       bool
		whitelist      string
		wantErr        bool
		wantErrSubstr  string
	}{
		{
			name:     "TIME ON wrapping a SELECT passes in strict read-only mode",
			query:    "SET STATISTICS TIME ON; SELECT 1",
			readOnly: true,
			wantErr:  false,
		},
		{
			name:     "IO ON wrapping a SELECT passes in strict read-only mode",
			query:    "SET STATISTICS IO ON; SELECT * FROM x",
			readOnly: true,
			wantErr:  false,
		},
		{
			name:          "wrapper around a modification is rejected in strict mode",
			query:         "SET STATISTICS TIME ON; DELETE FROM x",
			readOnly:      true,
			wantErr:       true,
			wantErrSubstr: "only SELECT",
		},
		{
			name:          "wrapper around a multi-statement base is rejected",
			query:         "SET STATISTICS TIME ON; SELECT 1; SELECT 2",
			readOnly:      true,
			wantErr:       true,
			wantErrSubstr: "multiple SQL statements",
		},
		{
			name:          "wrapper around a non-wrapper trailing statement is rejected",
			query:         "SET STATISTICS TIME ON; SELECT 1; PRINT 'hi'",
			readOnly:      true,
			wantErr:       true,
			wantErrSubstr: "multiple SQL statements",
		},
		{
			name:     "asymmetric wrapper (TIME ON + IO OFF) passes",
			query:    "SET STATISTICS TIME ON; SELECT 1; SET STATISTICS IO OFF",
			readOnly: true,
			wantErr:  false,
		},
		{
			name:     "leading + trailing TIME wrappers around SELECT pass",
			query:    "SET STATISTICS TIME ON; SELECT 1; SET STATISTICS TIME OFF",
			readOnly: true,
			wantErr:  false,
		},
		{
			name:     "wrapper around a whitelisted modification passes in whitelist mode",
			query:    "SET STATISTICS TIME ON; DELETE FROM prod_users",
			readOnly: true,
			whitelist: "prod_users",
			wantErr:  false,
		},
		{
			name:     "wrapper around full-access DELETE passes when read-only is off",
			query:    "SET STATISTICS TIME ON; DELETE FROM x",
			readOnly: false,
			wantErr:  false,
		},
		{
			name:          "wrapper around a banned SP is rejected",
			query:         "SET STATISTICS TIME ON; EXEC sp_password 'a','b','c'",
			readOnly:      true,
			wantErr:       true,
			wantErrSubstr: "read-only mode",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestMCPServer()
			s.config.readOnly = tc.readOnly
			if tc.whitelist != "" {
				s.config.whitelistTables = parseWhitelistTables(tc.whitelist)
			}
			err := s.validateReadOnlyQuery(tc.query)
			if tc.wantErr && err == nil {
				t.Fatalf("validateReadOnlyQuery(%q) = nil; want error containing %q", tc.query, tc.wantErrSubstr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateReadOnlyQuery(%q) = %v; want nil", tc.query, err)
			}
			if tc.wantErr && tc.wantErrSubstr != "" && !strings.Contains(strings.ToUpper(err.Error()), strings.ToUpper(tc.wantErrSubstr)) {
				t.Errorf("validateReadOnlyQuery(%q) error = %q; want substring %q", tc.query, err.Error(), tc.wantErrSubstr)
			}
		})
	}
}

// TestValidateReadOnlyQuery_StatisticsWrapper_NonRegression confirms that
// real multi-statement batches the security layer used to flag continue
// to be rejected by validateReadOnlyQuery after the wrapper branch was
// added. None of these should be classified as a wrapper.
func TestValidateReadOnlyQuery_StatisticsWrapper_NonRegression(t *testing.T) {
	cases := []string{
		"SELECT 1; DELETE FROM prod",
		"SELECT 'x;y'; DELETE FROM prod",
		"INSERT INTO x VALUES (1); SELECT * FROM x",
		"UPDATE x SET y=1; SELECT * FROM x",
		"SET NOCOUNT ON; SELECT 1",
		"SET STATISTICS TIME ON; DROP TABLE x; SELECT 1",
		"SET STATISTICS TIME 1; SELECT 1",
		"SET STATISTICS TIME ON; SELECT 1; SET STATISTICS TIME ON",
	}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			s := newTestMCPServer()
			s.config.readOnly = true
			err := s.validateReadOnlyQuery(q)
			if err == nil {
				t.Errorf("validateReadOnlyQuery(%q) = nil; want error", q)
			}
		})
	}
}