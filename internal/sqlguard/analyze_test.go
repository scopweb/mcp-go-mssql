package sqlguard

import (
	"strings"
	"testing"
)

func TestAnalyzeFalsePositivesAndRejections(t *testing.T) {
	const secret = "super-secret-literal"
	cases := []struct {
		name       string
		query      string
		statements int
		goSep      bool
		wantErr    bool
		words      []string
		absent     []string
	}{
		{name: "plain select", query: "SELECT 1", statements: 1, words: []string{"SELECT"}},
		{name: "trailing semicolon", query: "SELECT 1;", statements: 1},
		{name: "leading cte semicolon", query: ";WITH c AS (SELECT 1 AS n) SELECT n FROM c", statements: 1, words: []string{"WITH", "SELECT"}, absent: []string{"DELETE"}},
		{name: "spaces and comments around leading semicolon", query: "  /* antes */ \r\n ; \r\n -- despues\n WITH c AS (SELECT 1 AS n) SELECT n FROM c", statements: 1},
		{name: "double quoted semicolon", query: `SELECT 1 AS "a;b"`, statements: 1, absent: []string{"A;B"}},
		{name: "go inside multiline literal", query: "SELECT 'primera\nGO\núltima' AS value", statements: 1, absent: []string{"GO"}},
		{name: "nested block comment hides semicolon", query: "SELECT 1 /* exterior /* interior */ ; sigue siendo comentario */", statements: 1},
		{name: "variable is not a keyword", query: "SELECT @PRINT", statements: 1, words: []string{"SELECT"}, absent: []string{"PRINT"}},
		{name: "delete in literal", query: "SELECT 'DELETE' AS operation", statements: 1, absent: []string{"DELETE"}},
		{name: "delete in comment", query: "SELECT 1 /* DELETE explicado en un comentario */", statements: 1, absent: []string{"DELETE"}},
		{name: "unicode literal", query: "SELECT N'DELETE; GO' AS operation", statements: 1, absent: []string{"DELETE", "GO"}},
		{name: "escaped string semicolon", query: "SELECT 'a''; DROP TABLE x' AS v", statements: 1, absent: []string{"DROP"}},
		{name: "bracket escape", query: "SELECT [a]];b] FROM t", statements: 1},
		{name: "double quote escape", query: `SELECT "a"";b" AS v`, statements: 1},
		{name: "union all", query: "SELECT 1 UNION ALL SELECT 2", statements: 1},
		{name: "except intersect", query: "SELECT 1 EXCEPT SELECT 2 INTERSECT SELECT 3", statements: 1},
		{name: "subquery", query: "SELECT * FROM (SELECT 1 AS n) AS t", statements: 1},
		{name: "comment does not join tokens", query: "SELECT/**/1", statements: 1, words: []string{"SELECT"}},
		{name: "stacked select delete", query: "SELECT 1; DELETE FROM prod", statements: 2, words: []string{"SELECT", "DELETE"}},
		{name: "delete after leading empty separator", query: ";DELETE FROM prod", statements: 1, words: []string{"DELETE"}},
		{name: "delete after escaped literal", query: "SELECT 'it''s'; DELETE FROM prod", statements: 2},
		{name: "real go", query: "SELECT 1\nGO\nDELETE FROM prod", statements: 1, goSep: true},
		{name: "go with count", query: "SELECT 1\nGO 10\nSELECT 2", goSep: true},
		{name: "go with comment", query: "SELECT 1\nGO -- batch\nSELECT 2", goSep: true},
		{name: "unterminated string", query: "SELECT '" + secret, wantErr: true},
		{name: "unterminated identifier", query: "SELECT [open", wantErr: true},
		{name: "unterminated quote", query: `SELECT "open`, wantErr: true},
		{name: "unterminated comment", query: "SELECT 1 /* " + secret, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Analyze(tc.query)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected lexical error")
				}
				if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), tc.query) {
					t.Fatalf("lexical error leaked input: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if tc.statements != 0 && got.Statements != tc.statements {
				t.Fatalf("statements = %d, want %d", got.Statements, tc.statements)
			}
			if got.GO != tc.goSep {
				t.Fatalf("GO = %v, want %v", got.GO, tc.goSep)
			}
			for _, w := range tc.words {
				if !ExecutableHas(got.Words, w) {
					t.Fatalf("missing word %s in %v", w, got.Words)
				}
			}
			for _, w := range tc.absent {
				if ExecutableHas(got.Words, w) || ExecutableHas(got.TopLevel, w) {
					t.Fatalf("word %s was treated as executable: %v", w, got.Words)
				}
			}
		})
	}
}

func TestMaskCommentsDoesNotJoinTokens(t *testing.T) {
	got, err := MaskComments("SELECT/**/1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "SELECT1") {
		t.Fatalf("comments were deleted instead of spaced: %q", got)
	}
}

func TestAnalyzeDepthIsBounded(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxCommentDepth+2; i++ {
		b.WriteString("/*")
	}
	b.WriteString("SELECT 1")
	if _, err := Analyze(b.String()); err == nil {
		t.Fatal("comment depth above the limit was authorized")
	}
}

func TestAnalyzeLongLiteralStaysLinear(t *testing.T) {
	query := "SELECT '" + strings.Repeat("x", 200000) + "; DELETE' AS v"
	got, err := Analyze(query)
	if err != nil {
		t.Fatal(err)
	}
	if got.Statements != 1 || ExecutableHas(got.Words, "DELETE") {
		t.Fatalf("long literal was split or executed: %+v", got)
	}
}
