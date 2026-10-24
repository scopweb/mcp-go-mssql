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
		{`SELECT 1 INTO "temp_ai"`, false},
		{`SELECT 1 INTO "dbo" . "temp_ai"`, false},
		{"SELECT 1 AS id INTO/**/temp_ai", false},
		{"SELECT 1; DELETE FROM users", false},
		{"SELECT 1\nDELETE FROM users", false},
		{"DELETE FROM users", false},
		{"EXEC dbo.proc", false},
		{";WITH c AS (SELECT 1 AS n) SELECT n FROM c", true},
		{"SELECT 'DELETE' AS operation", true},
		{"SELECT 'primera\nGO\núltima' AS value", true},
		{"SELECT 1\nGO\nSELECT 2", false},
		{"SELECT 'unterminated", false},
	}
	for _, test := range tests {
		if got := isReadOnlyQuery(test.query); got != test.want {
			t.Errorf("isReadOnlyQuery(%q) = %v, want %v", test.query, got, test.want)
		}
	}
}

func TestCLIProductionTLS(t *testing.T) {
	t.Setenv("MSSQL_ALLOW_INSECURE_TLS", "")
	for _, dsn := range []string{
		"server=localhost;encrypt=false",
		"server=localhost;encrypt=true;trustservercertificate=true",
		"server=localhost",
		"sqlserver://localhost?encrypt=disable",
	} {
		t.Setenv("MSSQL_CONNECTION_STRING", dsn)
		if _, err := buildCLIConnectionString(&DatabaseConfig{}); err == nil {
			t.Errorf("insecure DSN accepted: %s", dsn)
		}
	}
	t.Setenv("MSSQL_CONNECTION_STRING", "sqlserver://localhost?encrypt=true&trustservercertificate=false")
	got, err := buildCLIConnectionString(&DatabaseConfig{})
	if err != nil || got != "sqlserver://localhost?encrypt=true&trustservercertificate=false" {
		t.Fatalf("secure URL changed/rejected: %s, %v", got, err)
	}
	t.Setenv("MSSQL_CONNECTION_STRING", "server=localhost;encrypt=false")
	t.Setenv("MSSQL_ALLOW_INSECURE_TLS", "true")
	if _, err := buildCLIConnectionString(&DatabaseConfig{}); err != nil {
		t.Fatal(err)
	}
}

func TestCLIRejectsDSNFieldInjection(t *testing.T) {
	t.Setenv("MSSQL_CONNECTION_STRING", "")
	if _, err := buildCLIConnectionString(&DatabaseConfig{Server: "localhost;encrypt=false"}); err == nil {
		t.Fatal("field injection accepted")
	}
}
