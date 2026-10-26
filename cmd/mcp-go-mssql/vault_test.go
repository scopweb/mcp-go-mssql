package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// These tests replace the default transport only to trust the test CA. They must not run in parallel.
func trustedVaultServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	previous := http.DefaultTransport
	transport := previous.(*http.Transport).Clone()
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
		transport.CloseIdleConnections()
		server.Close()
	})
	return server
}

func TestVaultCredentialResolution(t *testing.T) {
	const secret = "vault-password=with-equals"
	requests := 0
	server := trustedVaultServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/api/secrets/prod" || r.Header.Get("X-Api-Key") != "test-key" {
			t.Error("unexpected vault request")
		}
		fmt.Fprintf(w, `{"db-password":%q,"legacy-dsn":"server=legacy;password=test;encrypt=true;trustservercertificate=false","MSSQL_READ_ONLY":"false"}`, secret)
	})
	env := []string{
		"MSSQL_VAULT_URL=" + server.URL,
		"MSSQL_VAULT_API_KEY=test-key", "MSSQL_VAULT_ENVIRONMENT=prod",
		"MSSQL_SERVER=sql", "MSSQL_DATABASE=db", "MSSQL_USER=reader", "MSSQL_READ_ONLY=true",
		"MSSQL_PASSWORD_VAULT_SECRET=db-password",
		"MSSQL_DYNAMIC_APP_MAIN_SERVER=sql", "MSSQL_DYNAMIC_APP_MAIN_USER=reader",
		"MSSQL_DYNAMIC_APP_MAIN_PASSWORD_VAULT_SECRET=db-password",
		"MSSQL_DYNAMIC_LEGACY_CONNECTION_STRING_VAULT_SECRET=legacy-dsn",
	}
	before := os.Getenv("MSSQL_PASSWORD")
	resolved, err := resolveVaultEnvironment(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	getenv := environmentLookup(resolved)
	if requests != 1 || getenv("MSSQL_PASSWORD") != secret || getenv("MSSQL_READ_ONLY") != "true" {
		t.Fatal("incorrect credential overlay or policy changed")
	}
	if os.Getenv("MSSQL_PASSWORD") != before || environmentLookup(env)("MSSQL_PASSWORD") != "" {
		t.Fatal("resolution modified the input or process environment")
	}
	dsn, err := buildSecureConnectionStringFromEnvironment(getenv)
	if err != nil || !strings.Contains(dsn, "password="+secret+";") {
		t.Fatal("classic connection did not consume vault credentials")
	}
	aliases := loadDynamicAliasesFromEnvironment(NewSecurityLogger(), resolved)
	if aliases["APP_MAIN"].Password != secret || !aliases["APP_MAIN"].ReadOnly || aliases["LEGACY"].ConnectionString == "" {
		t.Fatal("dynamic alias credentials or read-only defaults incorrect")
	}
}

func TestVaultFailuresAreClosedAndRedacted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, "sensitive-response-value"},
		{"server-error", 500, "sensitive-response-value"},
		{"malformed", 200, "sensitive-response-value"},
		{"missing", 200, `{}`},
		{"empty", 200, `{"password":""}`},
		{"null", 200, `null`},
		{"wrong-type", 200, `{"password":12}`},
		{"oversized", 200, strings.Repeat("x", (1<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := trustedVaultServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			result, err := resolveVaultEnvironment(context.Background(), []string{
				"MSSQL_VAULT_URL=" + server.URL, "MSSQL_VAULT_API_KEY=sensitive-api-key",
				"MSSQL_VAULT_ENVIRONMENT=prod", "MSSQL_PASSWORD_VAULT_SECRET=password",
			})
			if err == nil || result != nil {
				t.Fatal("expected failure without partial credentials")
			}
			if strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), server.URL) {
				t.Fatal("error leaked sensitive information")
			}
		})
	}
}

func TestVaultRejectsRedirectsAndUntrustedTLS(t *testing.T) {
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
	}))
	defer target.Close()
	server := trustedVaultServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	})
	if _, err := fetchVaultSecrets(context.Background(), server.URL, "key", "prod"); err == nil || redirected {
		t.Fatal("redirect was followed or accepted")
	}
	// Remove the test trust root to verify normal certificate validation.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = nil
	if _, err := fetchVaultSecrets(context.Background(), server.URL, "key", "prod"); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
}

func TestVaultConfigurationValidation(t *testing.T) {
	for _, entry := range []string{
		"MSSQL_READ_ONLY_VAULT_SECRET=policy",
		"MSSQL_PASSWORD_VAULT_SECRET=",
		"MSSQL_DYNAMIC__PASSWORD_VAULT_SECRET=password",
	} {
		if _, err := resolveVaultEnvironment(context.Background(), []string{entry}); err == nil {
			t.Fatal("invalid reference accepted")
		}
	}
	for _, conflict := range []string{"MSSQL_PASSWORD=local", "MSSQL_CONNECTION_STRING=local", "MSSQL_CONNECTION_STRING_VAULT_SECRET=dsn"} {
		if _, err := resolveVaultEnvironment(context.Background(), []string{"MSSQL_PASSWORD_VAULT_SECRET=password", conflict}); err == nil {
			t.Fatal("ambiguous credential source accepted")
		}
	}
	for _, address := range []string{"http://localhost:8443", "https://user:pass@localhost", "https://localhost?token=x", "https://localhost/#fragment", "://invalid"} {
		if _, err := fetchVaultSecrets(context.Background(), address, "key", "prod"); err == nil {
			t.Fatal("unsafe vault URL accepted")
		}
	}
	for _, environment := range []string{"", "production", "../prod"} {
		if _, err := fetchVaultSecrets(context.Background(), "https://localhost", "key", environment); err == nil {
			t.Fatal("invalid vault environment accepted")
		}
	}
	if _, err := fetchVaultSecrets(context.Background(), "https://localhost", "", "prod"); err == nil {
		t.Fatal("missing API key accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchVaultSecrets(ctx, "https://localhost", "key", "prod"); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := resolveVaultEnvironment(context.Background(), []string{"MSSQL_PASSWORD=existing"}); err != nil {
		t.Fatal("vault became mandatory without references")
	}
}

func TestVaultClassicModeDetection(t *testing.T) {
	t.Setenv("MSSQL_DYNAMIC_MODE", "")
	t.Setenv("MSSQL_CONNECTION_STRING_VAULT_SECRET", "connection")
	t.Setenv("MSSQL_DYNAMIC_OTHER_SERVER", "sql")
	if isDynamicMode() {
		t.Fatal("classic vault DSN was mistaken for dynamic mode")
	}
}
