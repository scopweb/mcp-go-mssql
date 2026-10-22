package sqlguard

import (
	"strings"
	"testing"
)

func TestHasSelectInto(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{`SELECT 1 INTO "dest"`, true},
		{`SELECT 1 INTO/**/[dest]`, true},
		{`SELECT 1 /* outer /* nested */ comment */ INTO dest`, true},
		{`SELECT 'INTO dest' AS note`, false},
		{`SELECT [INTO], "INTO" FROM src`, false},
		{`SELECT 1 -- INTO dest`, false},
		{`SELECT 'unterminated`, true},
	} {
		if got := HasSelectInto(tc.query); got != tc.want {
			t.Errorf("%q: %v", tc.query, got)
		}
	}
}

func TestWriteTarget(t *testing.T) {
	for _, q := range []string{
		`INSERT dbo.target SELECT * FROM source`,
		`INSERT TOP (1) INTO [dbo].[target] SELECT * FROM source`,
		`INSERT /* a /* b */ c */ "dbo" . "target" VALUES (1)`,
	} {
		got, err := WriteTarget(q, "INSERT")
		if err != nil || got != "dbo.target" {
			t.Errorf("%q: %q, %v", q, got, err)
		}
	}
	for _, q := range []string{`INSERT @target VALUES (1)`, `INSERT server.db.dbo.target VALUES (1)`, `INSERT [dbo.target] VALUES (1)`, `INSERT TOP (1`, `INSERT dbo..target VALUES (1)`} {
		if _, err := WriteTarget(q, "INSERT"); err == nil {
			t.Errorf("ambiguous target accepted: %q", q)
		}
	}
}

func FuzzQuotedSelectInto(f *testing.F) {
	for _, name := range []string{"dest", "dbo.target", `a"b`, "a]b", "INTO", "/*x*/"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		if !HasSelectInto("SELECT 1 AS n INTO " + quoted) {
			t.Fatal("quoted write target escaped detection")
		}
		if HasSelectInto("SELECT " + quoted + " FROM source") {
			t.Fatal("quoted identifier became a keyword")
		}
	})
}
