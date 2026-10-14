package main

import "testing"

func TestReadOnlyUnionAll(t *testing.T) {
	s := newTestMCPServer()
	s.config.readOnly = true
	for _, query := range []string{
		"SELECT 1 UNION ALL SELECT 2",
		"select 1 union /* comment */ all\nselect 2; -- done",
		"SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3",
		"SELECT 1 UNION SELECT 2 EXCEPT SELECT 3 INTERSECT SELECT 4",
		"WITH c AS (SELECT 1 AS n UNION ALL SELECT 2) SELECT n FROM c UNION ALL SELECT 3",
		"SELECT * FROM (SELECT 1 AS n UNION ALL SELECT 2) AS t",
		"SET STATISTICS TIME ON; SELECT 1 UNION ALL SELECT 2; SET STATISTICS TIME OFF;",
	} {
		t.Run(query, func(t *testing.T) {
			if err := s.validateReadOnlyQuery(query); err != nil {
				t.Fatalf("single read-only query rejected: %v", err)
			}
		})
	}
	for _, query := range []string{
		"SELECT 1 UNION ALL SELECT 2; SELECT 3",
		"SELECT 1 UNION ALL SELECT 2 SELECT 3",
		"SELECT 1 ALL SELECT 2",
		"SELECT 1 UNION ALL SELECT 2 DELETE FROM prod",
		"SELECT 1 UNION ALL SELECT 2; DROP TABLE prod",
		"SELECT 1 UNION ALL SELECT 2 INTO prod",
		"SELECT 1 UNION ALL SELECT 2\nGO\nSELECT 3",
	} {
		t.Run(query, func(t *testing.T) {
			if err := s.validateReadOnlyQuery(query); err == nil {
				t.Fatal("unsafe query accepted")
			}
		})
	}
}
