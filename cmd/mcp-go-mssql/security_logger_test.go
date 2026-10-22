package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAuditCannotBeSuppressedByClient(t *testing.T) {
	var output bytes.Buffer
	s := &MCPMSSQLServer{secLogger: newSecurityLogger(&output)}
	s.handleRequest(MCPRequest{ID: 1, Method: "logging/setLevel", Params: map[string]interface{}{"level": "error"}})
	s.secLogger.Printf("Permission violation blocked")
	s.secLogger.Debugf("diagnostic marker")
	if !strings.Contains(output.String(), "Permission violation blocked") {
		t.Fatal("audit event suppressed")
	}
	if strings.Contains(output.String(), "diagnostic marker") {
		t.Fatal("diagnostic level not respected")
	}
}

func TestCredentialRedactionWithSpacesAndDelimiters(t *testing.T) {
	for _, input := range []string{
		`server=localhost;password=first second;encrypt=true`,
		`password="first;second";encrypt=true`,
		`sqlserver://user:first%20second@localhost?encrypt=true`,
		`pwd={first;second}`,
	} {
		var output bytes.Buffer
		logger := newSecurityLogger(&output)
		logger.Printf("connection error: %s", input)
		if strings.Contains(output.String(), "first") || strings.Contains(output.String(), "second") {
			t.Fatal("credential leaked")
		}
	}
}
