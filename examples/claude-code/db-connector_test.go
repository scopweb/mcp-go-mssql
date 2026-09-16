package main

import "testing"

func TestIsReadOnlyQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"SELECT * FROM users", true},
		{"WITH cte AS (SELECT 1 AS id) SELECT * FROM cte", true},
		{"SELECT 1 INTO temp_ai", false},
		{"SELECT 1 AS id INTO/**/temp_ai", false},
		{"SELECT 1; DELETE FROM users", false},
		{"SELECT 1\nDELETE FROM users", false},
		{"DELETE FROM users", false},
		{"EXEC dbo.proc", false},
	}
	for _, test := range tests {
		if got := isReadOnlyQuery(test.query); got != test.want {
			t.Errorf("isReadOnlyQuery(%q) = %v, want %v", test.query, got, test.want)
		}
	}
}
