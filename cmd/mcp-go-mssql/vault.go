package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const vaultReferenceSuffix = "_VAULT_SECRET"

func environmentLookup(environment []string) func(string) string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		if key, value, ok := strings.Cut(entry, "="); ok {
			values[key] = value
		}
	}
	return func(key string) string { return values[key] }
}

// resolveVaultEnvironment overlays explicit credential references on a private
// snapshot. Secret values never become process environment variables or MCP results.
func resolveVaultEnvironment(ctx context.Context, environment []string) ([]string, error) {
	getenv := environmentLookup(environment)
	references := make(map[string]string)
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		target, ok := strings.CutSuffix(key, vaultReferenceSuffix)
		if !ok || !strings.HasPrefix(key, "MSSQL_") {
			continue
		}
		valid := target == "MSSQL_PASSWORD" || target == "MSSQL_CONNECTION_STRING"
		if alias, found := strings.CutPrefix(target, "MSSQL_DYNAMIC_"); found {
			for _, suffix := range []string{"_PASSWORD", "_CONNECTION_STRING"} {
				if name, matched := strings.CutSuffix(alias, suffix); matched && name != "" {
					valid = true
				}
			}
		}
		if !valid || strings.TrimSpace(value) == "" {
			return nil, errors.New("invalid vault credential reference")
		}
		if getenv(target) != "" {
			return nil, errors.New("vault reference conflicts with a locally configured credential")
		}
		references[target] = value
	}
	if len(references) == 0 {
		return environment, nil
	}
	for target := range references {
		if prefix, ok := strings.CutSuffix(target, "PASSWORD"); ok {
			if getenv(prefix+"CONNECTION_STRING") != "" || references[prefix+"CONNECTION_STRING"] != "" {
				return nil, errors.New("vault password reference conflicts with a connection string")
			}
		}
	}
	secrets, err := fetchVaultSecrets(ctx, getenv("MSSQL_VAULT_URL"), getenv("MSSQL_VAULT_API_KEY"), getenv("MSSQL_VAULT_ENVIRONMENT"))
	if err != nil {
		return nil, err
	}
	result := append([]string(nil), environment...)
	for target, reference := range references {
		value, ok := secrets[reference]
		if !ok || value == "" {
			return nil, errors.New("a referenced vault secret is missing or empty")
		}
		result = append(result, target+"="+value)
	}
	return result, nil
}

// fetchVaultSecrets implements the corporate vault's application API (not its admin login).
// Errors intentionally omit URLs, response bodies and transport details that could contain secrets.
func fetchVaultSecrets(ctx context.Context, baseURL, apiKey, environment string) (map[string]string, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("MSSQL_VAULT_URL must be an HTTPS URL without credentials, query or fragment")
	}
	if apiKey == "" || strings.ContainsAny(apiKey, "\r\n") {
		return nil, errors.New("MSSQL_VAULT_API_KEY is missing or invalid")
	}
	if environment != "dev" && environment != "staging" && environment != "prod" {
		return nil, errors.New("MSSQL_VAULT_ENVIRONMENT must be dev, staging or prod")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/secrets/" + environment
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("cannot create vault request")
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Do not send application secrets through ambient HTTP proxies.
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("vault request failed (check connectivity, certificate and timeout)")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned HTTP %d", response.StatusCode)
	}
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return nil, errors.New("vault response unreadable or exceeds 1 MiB")
	}
	var secrets map[string]string
	if err := json.Unmarshal(body, &secrets); err != nil || secrets == nil {
		return nil, errors.New("invalid vault secrets response")
	}
	return secrets, nil
}
