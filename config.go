package main

import (
	"bufio"
	"os"
	"strings"

	"mcp-go-mssql/internal/sqlguard"
)

// loadEnvFile reads a .env file and sets environment variables from KEY=VALUE lines.
// Empty lines and lines starting with # are skipped.
// Only sets variables that are not already set (existing env vars take precedence).
func loadEnvFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		// File doesn't exist - that's ok, env vars may be set elsewhere
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE format
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Only set if not already set
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	return scanner.Err()
}

// discoverFirstDynamicConnection reads .env and returns the first alias with MSSQL_DYNAMIC_<ALIAS>_SERVER configured.
// Returns empty string if no dynamic connections are found.
func discoverFirstDynamicConnection() string {
	envFile, err := os.Open(".env")
	if err != nil {
		return ""
	}
	defer envFile.Close()

	knownAliases := make(map[string]bool)
	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		line := scanner.Text()
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.HasPrefix(key, "MSSQL_DYNAMIC_") && strings.HasSuffix(key, "_SERVER") && value != "" {
				alias := strings.TrimPrefix(strings.TrimSuffix(key, "_SERVER"), "MSSQL_DYNAMIC_")
				knownAliases[alias] = true
			}
		}
	}
	// Return first alias found
	for alias := range knownAliases {
		return alias
	}
	return ""
}

func getenvBool(name string) bool {
	return strings.ToLower(os.Getenv(name)) == "true"
}

func getenvBoolDefault(name string, def bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return def
	}
	return strings.ToLower(val) == "true"
}

// loadConfig builds a serverConfig from the current environment. Returns the
// populated struct and any non-fatal warnings to log. Errors are reserved for
// outright contradictory configurations — none today, but the signature
// accommodates future hard validation without an API change.
func loadConfig() (serverConfig, []string, error) {
	cfg := serverConfig{
		readOnly:             getenvBool("MSSQL_READ_ONLY"),
		whitelistTables:      sqlguard.ParseWhitelistTables(os.Getenv("MSSQL_WHITELIST_TABLES")),
		whitelistProcs:       os.Getenv("MSSQL_WHITELIST_PROCEDURES"),
		allowedDatabases:     sqlguard.ParseAllowedDatabases(os.Getenv("MSSQL_ALLOWED_DATABASES")),
		confirmDestructive:   getenvBoolDefault("MSSQL_CONFIRM_DESTRUCTIVE", true),
		autopilot:            getenvBool("MSSQL_AUTOPILOT"),
		skipSchemaValidation: getenvBool("MSSQL_SKIP_SCHEMA_VALIDATION"),
	}
	devMode := getenvBool("DEVELOPER_MODE")
	return cfg, cfg.Warnings(devMode), nil
}

// Warnings returns non-fatal configuration warnings. These are advisory
// and do not prevent the server from starting — the intent is to surface
// contradictions before they cause surprises at runtime.
//
// Examples:
//   - READ_ONLY=false with WHITELIST_TABLES set (whitelist ignored)
//   - AUTOPILOT=true in production (schema validation is your safety net)
//
// A warning is NOT emitted for combinations we support today but that might
// look contradictory at first glance (e.g. READ_ONLY=true with WHITELIST=*).
// Those work as documented and shouldn't noise up logs for users who
// existing today shouldn't break on upgrade.
func (c *serverConfig) Warnings(devMode bool) []string {
	var w []string

	// Whitelist set but read-only off → whitelist is silently ignored.
	if len(c.whitelistTables) > 0 && !c.readOnly {
		w = append(w, "MSSQL_WHITELIST_TABLES is set but MSSQL_READ_ONLY is not 'true' — the whitelist has no effect; set MSSQL_READ_ONLY=true to enable it")
	}

	// AUTOPILOT in production is suspicious — schema validation is the only
	// thing it disables, but doing so against a prod database means the AI
	// can issue queries against tables that don't exist without being
	// caught by the wrapper.
	if c.autopilot && !devMode {
		w = append(w, "MSSQL_AUTOPILOT=true in production is risky — schema validation is your safety net; only enable if you need the AI to work without schema checks against a non-critical dev database")
	}

	// CROSS_DATABASE without explicit read-only is a common oversight —
	// users expect cross-database queries to be read-only by default.
	if len(c.allowedDatabases) > 0 && !c.readOnly {
		w = append(w, "MSSQL_ALLOWED_DATABASES is set but MSSQL_READ_ONLY is not 'true' — cross-database queries can modify data; set MSSQL_READ_ONLY=true if you only want SELECT")
	}

	return w
}
