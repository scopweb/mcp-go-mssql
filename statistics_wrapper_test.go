package main

import (
	"strings"
	"testing"
)

// TestParseStatisticsWrapper_Positive covers the patterns that MUST be
// recognized as a SET STATISTICS wrapper. Each case asserts the base
// statement and the net effect.
func TestParseStatisticsWrapper_Positive(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		base    string
		timeOn  bool
		timeOff bool
		ioOn    bool
		ioOff   bool
	}{
		{
			name:   "leading TIME ON",
			query:  "SET STATISTICS TIME ON; SELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:    "leading TIME OFF",
			query:   "SET STATISTICS TIME OFF; SELECT 1",
			base:    "SELECT 1",
			timeOff: true,
		},
		{
			name:  "leading IO ON",
			query: "SET STATISTICS IO ON; SELECT 1",
			base:  "SELECT 1",
			ioOn:  true,
		},
		{
			name:  "leading IO OFF",
			query: "SET STATISTICS IO OFF; SELECT 1",
			base:  "SELECT 1",
			ioOff: true,
		},
		{
			name:   "leading TIME ON + trailing TIME OFF",
			query:  "SET STATISTICS TIME ON; SELECT 1; SET STATISTICS TIME OFF",
			base:   "SELECT 1",
			timeOn: true, timeOff: true,
		},
		{
			name:   "lowercase",
			query:  "set statistics time on; select 1",
			base:   "select 1",
			timeOn: true,
		},
		{
			name:   "extra whitespace",
			query:  "SET   STATISTICS   TIME   ON   ;   SELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:   "line comment before wrapper",
			query:  "-- turn on timing\nSET STATISTICS TIME ON; SELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:   "block comment before wrapper",
			query:  "/* diag */ SET STATISTICS TIME ON; SELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:    "line comment between wrapper statements",
			query:   "SET STATISTICS TIME ON; -- off\nSET STATISTICS TIME OFF; SELECT 1",
			base:    "SELECT 1",
			timeOn:  true,
			timeOff: true,
		},
		{
			name:   "asymmetric TIME ON + IO OFF",
			query:  "SET STATISTICS TIME ON; SELECT 1; SET STATISTICS IO OFF",
			base:   "SELECT 1",
			timeOn: true,
			ioOff:  true,
		},
		{
			name:   "base first then trailing wrapper",
			query:  "SELECT 1; SET STATISTICS TIME ON",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			// Regression for bug #2 in
			// revision-bugs-estadisticas-mcp-go-mssql.md: the fast-path
			// used to short-circuit on a non-'S' first rune, which
			// wrongly rejected this base-first wrapper whose base
			// starts with WITH (and therefore 'W'). The wrapper must
			// still be detected and unwrapped correctly.
			name:   "base-first wrapper with CTE (base starts with W)",
			query:  "WITH x AS (SELECT 1 AS a) SELECT * FROM x; SET STATISTICS TIME ON",
			base:   "WITH x AS (SELECT 1 AS a) SELECT * FROM x",
			timeOn: true,
		},
		{
			name:    "base-first wrapper with DELETE (base starts with D)",
			query:   "DELETE FROM temp_ai WHERE id = 1; SET STATISTICS TIME OFF",
			base:    "DELETE FROM temp_ai WHERE id = 1",
			timeOff: true,
		},
		{
			name:  "base-first wrapper with INSERT (base starts with I)",
			query: "INSERT INTO temp_ai VALUES (1); SET STATISTICS IO OFF",
			base:  "INSERT INTO temp_ai VALUES (1)",
			ioOff: true,
		},
		{
			name:   "semicolon inside string literal in base",
			query:  "SET STATISTICS TIME ON; SELECT ';' AS sep",
			base:   "SELECT ';' AS sep",
			timeOn: true,
		},
		{
			name:   "trailing semicolon on wrapper before base",
			query:  "SET STATISTICS TIME ON;\nSELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:   "tab between tokens",
			query:  "SET\tSTATISTICS\tTIME\tON;\tSELECT 1",
			base:   "SELECT 1",
			timeOn: true,
		},
		{
			name:   "trailing semicolon and trailing whitespace",
			query:  "SET STATISTICS TIME ON; SELECT 1; ",
			base:   "SELECT 1",
			timeOn: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := parseStatisticsWrapper(tc.query)
			if err != nil {
				t.Fatalf("parseStatisticsWrapper(%q) returned error: %v", tc.query, err)
			}
			if w == nil {
				t.Fatalf("parseStatisticsWrapper(%q) returned nil wrapper", tc.query)
			}
			if w.BaseStatement != tc.base {
				t.Errorf("BaseStatement = %q, want %q", w.BaseStatement, tc.base)
			}
			if w.TimeOn != tc.timeOn {
				t.Errorf("TimeOn = %v, want %v", w.TimeOn, tc.timeOn)
			}
			if w.TimeOff != tc.timeOff {
				t.Errorf("TimeOff = %v, want %v", w.TimeOff, tc.timeOff)
			}
			if w.IOOn != tc.ioOn {
				t.Errorf("IOOn = %v, want %v", w.IOOn, tc.ioOn)
			}
			if w.IOOff != tc.ioOff {
				t.Errorf("IOOff = %v, want %v", w.IOOff, tc.ioOff)
			}
		})
	}
}

// TestParseStatisticsWrapper_Negative covers inputs that MUST NOT be
// recognized as a SET STATISTICS wrapper. The validator must reject them
// and fall through to normal read-only checks (which then reject them
// anyway for the dangerous ones).
func TestParseStatisticsWrapper_Negative(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"numeric arg", "SET STATISTICS TIME 1; SELECT 1"},
		{"modification smuggled in", "SET STATISTICS TIME ON; DROP TABLE x; SELECT 1"},
		{"multi-statement base, two SELECTs", "SET STATISTICS TIME ON; SELECT 1; SELECT 2"},
		{"missing ON/OFF value", "SET STATISTICS TIME; SELECT 1"},
		{"empty base", "SET STATISTICS TIME ON; ; SELECT 1"},
		{"missing semicolon between wrapper and base", "SET STATISTICS TIME ON SELECT 1"},
		{"non-wrapper trailing", "SET STATISTICS TIME ON; SELECT 1; PRINT 'hi'"},
		{"no whitespace between tokens", "SETSTATISTICSTIME ON; SELECT 1"},
		{"duplicate leading TIME ON", "SET STATISTICS TIME ON; SET STATISTICS TIME ON; SELECT 1"},
		{"duplicate leading TIME OFF", "SET STATISTICS TIME OFF; SET STATISTICS TIME OFF; SELECT 1"},
		{"both leading TIME ON and OFF", "SET STATISTICS TIME ON; SET STATISTICS TIME OFF; SELECT 1"},
		{"both leading IO ON and OFF", "SET STATISTICS IO ON; SET STATISTICS IO OFF; SELECT 1"},
		{"empty input", ""},
		{"whitespace only", "   \n\t  "},
		{"plain SELECT", "SELECT 1"},
		{"plain SELECT with trailing semicolon", "SELECT 1;"},
		{"plain WITH cte", "WITH x AS (SELECT 1 AS a) SELECT * FROM x"},
		{"sp_help (admin proc, not a wrapper)", "EXEC sp_help 'x'"},
		{"SET without STATISTICS", "SET NOCOUNT ON; SELECT 1"},
		{"lower-case but with embedded SET keyword", "select set statistics time on from dual"},
		{"wrapper missing kind", "SET STATISTICS; SELECT 1"},
		{"wrapper with bogus kind", "SET STATISTICS FOO ON; SELECT 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := parseStatisticsWrapper(tc.query)
			if err == nil {
				t.Fatalf("parseStatisticsWrapper(%q) accepted the input (wrapper=%+v); expected ErrNotAWrapper", tc.query, w)
			}
			if w != nil {
				t.Errorf("parseStatisticsWrapper(%q) returned non-nil wrapper on error path: %+v", tc.query, w)
			}
			if !strings.Contains(err.Error(), "not a SET STATISTICS wrapper") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

// TestParseStatisticsWrapper_NonRegression confirms that real multi-statement
// batches the security layer used to flag continue to be rejected.
func TestParseStatisticsWrapper_NonRegression(t *testing.T) {
	realMultiStatement := []string{
		"SELECT 1; DELETE FROM prod",
		"SELECT 'x;y'; DELETE FROM prod",
		"SELECT 1; SELECT 2; SELECT 3",
		"INSERT INTO x VALUES (1); SELECT * FROM x",
	}
	for _, q := range realMultiStatement {
		t.Run(q, func(t *testing.T) {
			if w, err := parseStatisticsWrapper(q); err == nil {
				t.Fatalf("parseStatisticsWrapper(%q) accepted the input (wrapper=%+v); expected ErrNotAWrapper", q, w)
			}
		})
	}
}
