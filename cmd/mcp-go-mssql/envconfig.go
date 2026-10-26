package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configuredReadOnly defaults to read-only; writes require an explicit false.
func configuredReadOnly(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return true, fmt.Errorf("MSSQL_READ_ONLY must be true or false")
	}
}

// Environment loading and dynamic-alias configuration.

// parseWhitelistTables parses a comma-separated whitelist into normalized lowercase slice.
func parseWhitelistTables(env string) []string {
	if env == "" {
		return nil
	}
	tables := strings.Split(env, ",")
	var normalized []string
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table != "" {
			normalized = append(normalized, strings.ToLower(table))
		}
	}
	return normalized
}

// loadDotEnvIfPresent loads a .env file from the same directory as the executable
// if it exists. It only sets variables that are not already present in the environment
// (MCP host-passed env takes precedence). This enables the documented dynamic
// connection workflow where credentials live next to the exe.
//
// Users running multiple isolated MCP server instances (classic + dynamic, or multiple
// classic for different DBs) can set MSSQL_IGNORE_LOCAL_ENV=true in their .mcp.json
// "env" block to completely disable .env loading for that instance. This provides
// strong isolation even if a .env file was accidentally left next to the executable.
func loadDotEnvIfPresent(secLogger *SecurityLogger) {
	// Nuclear option for isolation: completely ignore any .env next to the exe.
	// Intended for classic servers configured purely via Claude Desktop .mcp.json env.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("MSSQL_IGNORE_LOCAL_ENV"))); v == "true" || v == "1" || v == "yes" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return // cannot determine location, skip .env (rely on passed env)
	}
	exeDir := filepath.Dir(exePath)
	envPath := filepath.Join(exeDir, ".env")

	// #nosec G304 -- envPath is derived from os.Executable(), not user input
	data, err := os.ReadFile(envPath)
	if err != nil {
		return // no .env next to exe is normal/OK
	}

	loaded := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			// Remove surrounding quotes if present
			if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
				(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
				val = val[1 : len(val)-1]
			}
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
				loaded++
			}
		}
	}
	if loaded > 0 {
		secLogger.Printf("Loaded %d variables from .env next to executable (keys only; values redacted)", loaded)
	}
}

// hasDynamicAliases returns true if any MSSQL_DYNAMIC_* variables are present in env.
// Note: This is only used for auto-detection when MSSQL_DYNAMIC_MODE is not explicitly set.
func hasDynamicAliases() bool {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "MSSQL_DYNAMIC_") {
			return true
		}
	}
	return false
}

// isDynamicMode reports whether the server should operate in dynamic multi-connection mode
// (exposing dynamic_available / dynamic_connect / etc. and loading per-alias configs).
//
// Precedence (highest to lowest):
//  1. Explicit MSSQL_DYNAMIC_MODE=false  → always classic (even if DYNAMIC_* vars exist anywhere)
//  2. Explicit MSSQL_DYNAMIC_MODE=true   → always dynamic
//  3. No explicit setting:
//     - If any classic connection config is present (MSSQL_SERVER, MSSQL_CONNECTION_STRING,
//     or MSSQL_DATABASE) → classic mode. This protects users who configure classic servers
//     via .mcp.json "env" blocks or who have polluted parent environments.
//     - Otherwise, if any MSSQL_DYNAMIC_* vars exist → dynamic mode (auto-detect).
//     - Otherwise → classic (no DB configured).
//
// This design allows safe co-existence of multiple mcp-go-mssql instances (some classic,
// some dynamic) under Claude Desktop, even when the host process environment contains
// stray dynamic variables from other .env files or previous sessions.
func isDynamicMode() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MSSQL_DYNAMIC_MODE")))
	if mode == "false" || mode == "0" || mode == "no" || mode == "off" {
		return false // explicit opt-out always wins, multiple spellings accepted
	}
	if mode == "true" || mode == "1" || mode == "yes" || mode == "on" {
		return true
	}

	// No explicit MSSQL_DYNAMIC_MODE → decide based on what the user actually configured.
	hasClassicConfig := os.Getenv("MSSQL_SERVER") != "" ||
		os.Getenv("MSSQL_CONNECTION_STRING") != "" ||
		os.Getenv("MSSQL_CONNECTION_STRING_VAULT_SECRET") != "" ||
		os.Getenv("MSSQL_DATABASE") != ""

	if hasClassicConfig {
		// User (or .mcp.json) provided a default connection → treat as classic.
		// Ignore any stray MSSQL_DYNAMIC_* that may have leaked from parent env or other .env files.
		return false
	}

	// No classic default configured → fall back to presence of dynamic aliases.
	return hasDynamicAliases()
}

// loadDynamicAliases scans the environment for MSSQL_DYNAMIC_<ALIAS>_* variables
// and builds a map of DynamicAlias with their individual security posture.
// This enables the secure "one application - multiple related databases" pattern.
func loadDynamicAliases(secLogger *SecurityLogger) map[string]DynamicAlias {
	return loadDynamicAliasesFromEnvironment(secLogger, os.Environ())
}

func loadDynamicAliasesFromEnvironment(secLogger *SecurityLogger, environment []string) map[string]DynamicAlias {
	aliases := make(map[string]DynamicAlias)
	prefix := "MSSQL_DYNAMIC_"

	// Collect all relevant env vars
	envVars := make(map[string]string)
	for _, env := range environment {
		if idx := strings.Index(env, "="); idx > 0 {
			key := env[:idx]
			val := env[idx+1:]
			if strings.HasPrefix(key, prefix) {
				envVars[key] = val
			}
		}
	}

	// Find all unique aliases. Aliases may contain underscores, so split on a
	// known configuration suffix rather than the first underscore.
	fieldSuffixes := []string{
		"_CONNECTION_STRING", "_WHITELIST_TABLES", "_READ_ONLY", "_DATABASE",
		"_PASSWORD", "_SERVER", "_ENCRYPT", "_PORT", "_USER",
	}
	aliasSet := make(map[string]bool)
	for key := range envVars {
		rest := strings.TrimPrefix(key, prefix)
		for _, suffix := range fieldSuffixes {
			if alias, found := strings.CutSuffix(rest, suffix); found && alias != "" {
				aliasSet[strings.ToUpper(alias)] = true
				break
			}
		}
	}

	for alias := range aliasSet {
		// === SECURITY BY DEFAULT ===
		// Every dynamic alias defaults to the safest possible configuration.
		// If the user does not explicitly set MSSQL_DYNAMIC_<ALIAS>_READ_ONLY=false,
		// the alias will be read-only. This is intentional and by design.
		a := DynamicAlias{
			Alias:           alias,
			ReadOnly:        true, // SAFE DEFAULT: read-only unless explicitly set to false
			WhitelistTables: nil,  // No writes allowed by default
		}

		// Load connection fields
		if v := envVars[prefix+alias+"_SERVER"]; v != "" {
			a.Server = v
		}
		if v := envVars[prefix+alias+"_DATABASE"]; v != "" {
			a.Database = v
		}
		if v := envVars[prefix+alias+"_USER"]; v != "" {
			a.User = v
		}
		if v := envVars[prefix+alias+"_PASSWORD"]; v != "" {
			a.Password = v
		}

		// Per-alias connection tuning. ConnectionString wins over Encrypt/Port
		// (the connector in connectToDynamicAlias honors that priority). These
		// fields are needed for legacy SQL Server 2000/2008/2012 instances whose
		// TLS 1.0 handshake is rejected by modern Go runtimes (see
		// .github/ISSUES/01-sql-server-2008-support.md for the classic-mode
		// equivalent of MSSQL_CONNECTION_STRING).
		if v := envVars[prefix+alias+"_ENCRYPT"]; v != "" {
			a.Encrypt = strings.ToLower(strings.TrimSpace(v))
		}
		if v := envVars[prefix+alias+"_PORT"]; v != "" {
			a.Port = strings.TrimSpace(v)
		}
		if v := envVars[prefix+alias+"_CONNECTION_STRING"]; v != "" {
			a.ConnectionString = v
		}

		// Per-alias security posture - ONLY override if explicitly provided
		// This enforces "secure by default"
		if ro := envVars[prefix+alias+"_READ_ONLY"]; ro != "" {
			switch strings.ToLower(strings.TrimSpace(ro)) {
			case "true":
				a.ReadOnly = true
			case "false":
				a.ReadOnly = false
			default:
				secLogger.Printf("Ignoring dynamic alias '%s': invalid READ_ONLY value", alias)
				continue
			}
		}
		// If not set → remains true (safe default)

		if wl := envVars[prefix+alias+"_WHITELIST_TABLES"]; wl != "" {
			a.WhitelistTables = parseWhitelistTables(wl)
		}
		// If not set and ReadOnly=false → whitelist remains empty = no modifications allowed (very safe)

		aliases[alias] = a

		// Security logging (never log credentials)
		secLogger.Printf("Loaded dynamic alias '%s' (readOnly=%v, whitelistTables=%d) [secure defaults applied]",
			alias, a.ReadOnly, len(a.WhitelistTables))
	}

	if len(aliases) > 0 {
		secLogger.Printf("Dynamic mode: %d aliases loaded with individual security contexts", len(aliases))
	}

	return aliases
}
