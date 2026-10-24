package main

import (
	"strings"
	"testing"
)

func TestReadOnlyFalsePositivesAndRealBatches(t *testing.T) {
	s := newTestMCPServer()
	s.config.readOnly = true

	allowed := []string{
		"SELECT 1",
		"SELECT 1;",
		";WITH c AS (SELECT 1 AS n) SELECT n FROM c",
		"/* lead */\r\n;\r\n-- tail\nWITH c AS (SELECT 1 AS n) SELECT n FROM c",
		`SELECT 1 AS "a;b"`,
		"SELECT 'primera\nGO\núltima' AS value",
		"SELECT 1 /* exterior /* interior */ ; sigue siendo comentario */",
		"SELECT @PRINT",
		"SELECT 'DELETE' AS operation",
		"SELECT 1 /* DELETE explicado en un comentario */",
		"SELECT N'DELETE' AS operation",
		"SELECT 1 UNION ALL SELECT 2",
		"SELECT 1 UNION SELECT 2 EXCEPT SELECT 3 INTERSECT SELECT 4",
		"SELECT * FROM (SELECT 1 AS n) AS t",
		"SELECT 'it''s fine' AS v",
		"SELECT [a]];b] FROM t",
		`SELECT "say ""hi""" AS v`,
		"SELECT 1 /* comment */ INTO_not_really FROM t",
		"SET STATISTICS TIME ON; SELECT 1 AS \"a;b\"; SET STATISTICS TIME OFF",
	}
	for _, q := range allowed {
		t.Run("allow/"+q, func(t *testing.T) {
			if err := s.validateReadOnlyQuery(q); err != nil {
				t.Fatalf("single read rejected: %v", err)
			}
		})
	}

	blocked := []struct {
		query string
		part  string
	}{
		{"SELECT 1; DELETE FROM prod", "multiple SQL statements are not allowed"},
		{"SELECT 1 DELETE FROM prod", "multiple SQL statements are not allowed"},
		{"SELECT 1; INSERT INTO prod VALUES (1)", "multiple SQL statements are not allowed"},
		{"SELECT 1; UPDATE prod SET a = 1", "multiple SQL statements are not allowed"},
		{"SELECT 1; DROP TABLE prod", "multiple SQL statements are not allowed"},
		{"SELECT 1; ALTER TABLE prod ADD a int", "multiple SQL statements are not allowed"},
		{"SELECT 1; TRUNCATE TABLE prod", "multiple SQL statements are not allowed"},
		{"SELECT 1; MERGE prod AS t USING src AS s ON 1 = 0 WHEN NOT MATCHED THEN INSERT (a) VALUES (1)", "multiple SQL statements are not allowed"},
		{"SELECT 1; EXEC xp_cmdshell 'dir'", "multiple SQL statements are not allowed"},
		{"SELECT 1 /* c */ INTO prod", "SELECT INTO"},
		{"SELECT 1 /* outer /* nested */ */ INTO prod", "SELECT INTO"},
		{"WITH c AS (DELETE FROM prod) SELECT 1", "forbidden operation"},
		{";DELETE FROM prod", "only SELECT"},
		{"SELECT 'it''s'; DELETE FROM prod", "multiple SQL statements are not allowed"},
		{"/* c */; DELETE FROM prod", "only SELECT"},
		{"USE master", "only SELECT"},
		{"GRANT SELECT ON prod TO public", "multiple SQL statements are not allowed"},
		{"REVOKE SELECT ON prod FROM public", "multiple SQL statements are not allowed"},
		{"BEGIN TRAN", "only SELECT"},
		{"WAITFOR DELAY '00:00:01'", "only SELECT"},
		{"IF 1 = 1 SELECT 1", "multiple SQL statements are not allowed"},
		{"EXEC(N'DEL' + N'ETE FROM prod')", "only SELECT"},
		{"EXEC dbo.unlisted_procedure", "only SELECT"},
		{"SELECT 1\nGO\nDELETE FROM prod", "GO batch separators are not supported"},
		{"SELECT 1\nGO 3\nSELECT 2", "GO batch separators are not supported"},
		{"SELECT 'secret-value", "unterminated SQL literal, identifier, or comment"},
		{"SELECT [open", "unterminated SQL literal, identifier, or comment"},
		{"SELECT 1 /* secret-value", "unterminated SQL literal, identifier, or comment"},
	}
	for _, tc := range blocked {
		t.Run("block/"+tc.query, func(t *testing.T) {
			err := s.validateReadOnlyQuery(tc.query)
			if err == nil {
				t.Fatal("unsafe query accepted")
			}
			if !strings.Contains(err.Error(), tc.part) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.part)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("error leaked input: %q", err.Error())
			}
		})
	}
}

func TestExplainQueryClassification(t *testing.T) {
	s := newTestMCPServer()
	if err := explainQueryPolicy(`SELECT 1 AS "a;b"`); err != nil {
		t.Fatalf("false positive rejected: %v", err)
	}
	if op := s.extractOperation("SELECT 'DELETE' AS operation"); op != "SELECT" {
		t.Fatalf("literal classified as %s", op)
	}
	if op := s.extractOperation("WITH c AS (SELECT 'DELETE' AS operation) SELECT * FROM c"); op != "SELECT" {
		t.Fatalf("CTE literal classified as %s", op)
	}
	err := explainQueryPolicy("SELECT 1; DELETE FROM prod")
	if err == nil || !strings.Contains(err.Error(), "multiple SQL statements") {
		t.Fatalf("real batch error = %v", err)
	}
	err = explainQueryPolicy("SELECT 1\nGO\nSELECT 2")
	if err == nil || !strings.Contains(err.Error(), "GO batch separators") || strings.Contains(err.Error(), "SELECT 2") {
		t.Fatalf("GO error = %v", err)
	}
	if op := s.extractOperation("WITH c AS (DELETE FROM prod) SELECT 1"); op != "DELETE" {
		t.Fatalf("modifying CTE classified as %s", op)
	}
}
