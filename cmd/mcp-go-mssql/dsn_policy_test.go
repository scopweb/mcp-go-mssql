package main

import "testing"

func TestProductionTLSDriverSemantics(t *testing.T) {
	for _, dsn := range []string{
		"server=x;encrypt=optional",
		"server=x;encrypt=f",
		`server=x;encrypt="false"`,
		"odbc:server=x;encrypt={false}",
		"sqlserver://u:p@x?encrypt=%66alse",
		"server=x;encrypt=true;trust server certificate=t",
		`server=x;encrypt=true;trustservercertificate="yes"`,
		"sqlserver://u:p@x?encrypt=true&trustservercertificate=%74rue",
		"server=x;password=encrypt=true",
		"server=x;encrypt=invalid",
	} {
		t.Run(dsn, func(t *testing.T) {
			if productionTLSViolation(dsn) == "" {
				t.Fatal("insecure or invalid DSN accepted")
			}
		})
	}
	for _, dsn := range []string{
		`server=x;encrypt="true";trust server certificate=no`,
		"server=x;encrypt=mandatory",
		"odbc:server=x;encrypt={true};trustservercertificate={false}",
		"sqlserver://u:p@x?encrypt=%74rue",
		`server=x;password="encrypt=false";encrypt=true`,
	} {
		t.Run(dsn, func(t *testing.T) {
			if reason := productionTLSViolation(dsn); reason != "" {
				t.Fatalf("secure DSN rejected: %s", reason)
			}
		})
	}
}
