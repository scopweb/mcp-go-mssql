package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/microsoft/go-mssqldb/msdsn"
)

// Connection-string construction and the production TLS fail-closed check.

// validateDSNField rejects values that could break out of their field in an
// ADO-style ("key=value;...") connection string. A password or server name
// containing ';' (or control characters) would otherwise let an operator
// accidentally — or an attacker who can influence these env vars — inject
// extra connection parameters such as "encrypt=false". The auto-built DSN uses
// ';' as the field separator, so these characters must not appear in values.
func validateDSNField(name, value string) error {
	if strings.ContainsAny(value, ";\r\n\x00") {
		return fmt.Errorf("invalid character in %s: ';', newline and null bytes are not allowed (use MSSQL_CONNECTION_STRING with a URL-form DSN for exotic values)", name)
	}
	return nil
}

// insecureTLSExplicitlyAllowed reports whether the operator has explicitly
// opted in to insecure database TLS via MSSQL_ALLOW_INSECURE_TLS=true. This is
// the intentional, auditable escape hatch for the legacy SQL Server
// 2000/2008/2012 case (TLS 1.0 only). Without it, insecure TLS in production
// fails closed instead of degrading silently (M2).
func insecureTLSExplicitlyAllowed() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("MSSQL_ALLOW_INSECURE_TLS")), "true")
}

// productionTLSViolation returns a non-empty, human-readable reason when a
// connection string would degrade transport security in a way that must not be
// permitted silently in production: encryption disabled, certificate
// validation disabled, or an implicit encryption setting. Driver defaults vary
// by DSN form and version, so production must never rely on an omitted value.
func productionTLSViolation(connStr string) string {
	// Use the driver's parser so URL escaping, quoted/braced values and
	// keyword synonyms have exactly the same meaning at validation and dial.
	// Preserve case: passwords and certificate paths may be case-sensitive.
	cfg, err := msdsn.Parse(connStr)
	if err != nil {
		// Parser errors may contain DSN values; do not expose them.
		return "connection string could not be validated"
	}
	if _, ok := cfg.Parameters[msdsn.Encrypt]; !ok {
		return "encryption is not explicitly enabled (missing encrypt=true or encrypt=strict)"
	}

	if cfg.Encryption != msdsn.EncryptionRequired && cfg.Encryption != msdsn.EncryptionStrict {
		return "encryption is disabled (encrypt=false)"
	}
	if cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify {
		return "server certificate validation is disabled (trustservercertificate=true)"
	}
	return ""
}

// enforceProductionTLS fails closed when a connection would use insecure TLS in
// production, unless the operator explicitly opts in via
// MSSQL_ALLOW_INSECURE_TLS=true (M2). When opted in, it downgrades to an
// auditable warning so the intentional legacy exception stays visible in the
// logs. In development mode this must not be called (callers gate on
// isProduction).
func enforceProductionTLS(label, connStr string) error {
	violation := productionTLSViolation(connStr)
	if violation == "" {
		return nil
	}
	if insecureTLSExplicitlyAllowed() {
		slog.Warn(fmt.Sprintf("%s: %s in production — permitted because MSSQL_ALLOW_INSECURE_TLS=true", label, violation))
		return nil
	}
	return fmt.Errorf("%s: %s in production. Refusing to start. Correct the TLS settings, or set MSSQL_ALLOW_INSECURE_TLS=true to intentionally allow it (needed only for legacy SQL Server 2000/2008/2012 that negotiate TLS 1.0)", label, violation)
}

func buildSecureConnectionString() (string, error) {
	// Check for custom connection string first
	if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
		isProduction := strings.ToLower(os.Getenv("DEVELOPER_MODE")) != "true"

		if isProduction {
			// Fail closed on insecure TLS in production (M2).
			if err := enforceProductionTLS("MSSQL_CONNECTION_STRING", customConnStr); err != nil {
				return "", err
			}
		}

		return customConnStr, nil
	}

	server := os.Getenv("MSSQL_SERVER")
	database := os.Getenv("MSSQL_DATABASE")
	user := os.Getenv("MSSQL_USER")
	password := os.Getenv("MSSQL_PASSWORD")
	port := os.Getenv("MSSQL_PORT")
	auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))

	if auth == "" {
		auth = "sql"
	}

	if server == "" {
		return "", fmt.Errorf("missing required environment variable: MSSQL_SERVER")
	}

	// Reject field-breaking characters in any value that will be interpolated
	// into the ADO-style connection string below.
	for name, val := range map[string]string{
		"MSSQL_SERVER": server, "MSSQL_DATABASE": database,
		"MSSQL_USER": user, "MSSQL_PASSWORD": password, "MSSQL_PORT": port,
	} {
		if err := validateDSNField(name, val); err != nil {
			return "", err
		}
	}

	// For Windows Auth, database is optional (allows exploring all databases)
	// For SQL Auth, database is required
	if auth == "sql" {
		if database == "" {
			return "", fmt.Errorf("missing required environment variable for SQL auth: MSSQL_DATABASE")
		}
		if user == "" || password == "" {
			return "", fmt.Errorf("missing required environment variables for SQL auth: MSSQL_USER, MSSQL_PASSWORD")
		}
	}

	if port == "" {
		port = "1433"
	}

	// For development mode, allow disabling encryption and untrusted certificates
	encrypt := "true"
	trustCert := "false"
	if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
		// In development mode, allow disabling encryption for local SQL Server instances
		if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
			encrypt = strings.ToLower(envEncrypt)
		} else {
			// Default to false for development mode to match local SQL Server setups
			encrypt = "false"
		}
		trustCert = "true"
	}

	// Build connection string depending on requested authentication mode
	switch auth {
	case "integrated", "windows":
		// Windows Integrated Authentication (SSPI)
		// The process will use the credentials of the Windows user running it
		// Database is optional - if not specified, connects to default database
		var connStr string
		if database != "" {
			connStr = fmt.Sprintf("server=%s;port=%s;database=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, port, database, encrypt, trustCert,
			)
		} else {
			// No database specified - connect to master or default database
			connStr = fmt.Sprintf("server=%s;port=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, port, encrypt, trustCert,
			)
		}
		return connStr, nil
	case "azure":
		// Azure AD auth needs an additional implementation to obtain tokens
		return "", fmt.Errorf("Azure AD authentication not implemented in buildSecureConnectionString; use MSSQL_CONNECTION_STRING or set MSSQL_AUTH=sql") //nolint:staticcheck // intentional capitalization for user-facing error
	default:
		// Default to SQL Server authentication
		return fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
			server, port, database, user, password, encrypt, trustCert,
		), nil
	}
}

// buildAliasConnectionString constructs the SQL Server connection string for a
// dynamic alias. The function mirrors the override + per-alias tuning + devMode
// fallback strategy used by buildSecureConnectionString (classic mode), so the
// per-alias path and the global path behave identically.
//
// Priority:
//  1. alias.ConnectionString (full override, URL or ADO DSN) — wins outright.
//     This is the recommended workaround for legacy SQL Server 2000/2008/2012
//     instances that only negotiate TLS 1.0 (see .github/ISSUES/01-sql-server-2008-support.md).
//  2. alias.Encrypt + alias.Port — per-alias tuning on top of an auto-built DSN.
//  3. devMode — last-resort defaults (encrypt=false/trustCert=true in dev,
//     encrypt=true/trustCert=false in production).
//
// Default timeouts (connection timeout=30; command timeout=30) are appended
// when the user-supplied override does not include them.
//
// In production mode, the function emits slog.Warn for insecure settings
// (encrypt=false, missing encrypt, trustservercertificate=true), exactly like
// buildSecureConnectionString does for the global MSSQL_CONNECTION_STRING.
func buildAliasConnectionString(alias *DynamicAlias, devMode bool, aliasName string) (string, error) {
	if alias == nil {
		return "", fmt.Errorf("nil alias")
	}

	// Priority 1: full override per alias (URL or ADO DSN).
	if alias.ConnectionString != "" {
		cs := alias.ConnectionString
		isProduction := !devMode

		if isProduction {
			// Fail closed on insecure TLS in production (M2).
			if err := enforceProductionTLS(fmt.Sprintf("dynamic alias '%s' connection string", aliasName), cs); err != nil {
				return "", err
			}
		}
		return cs, nil
	}

	// Priority 2 + 3: per-alias Encrypt/Port, then devMode defaults.
	encrypt := "true"
	trustCert := "false"
	if devMode {
		encrypt = "false"
		trustCert = "true"
	}
	if alias.Encrypt != "" {
		encrypt = alias.Encrypt
	}

	// Fail closed if a per-alias _ENCRYPT override would disable encryption in
	// production (M2). devMode already relaxes TLS intentionally, so only the
	// production path is guarded.
	if !devMode {
		if err := enforceProductionTLS(fmt.Sprintf("dynamic alias '%s'", aliasName), "encrypt="+strings.ToLower(encrypt)); err != nil {
			return "", err
		}
	}

	port := alias.Port
	if port == "" {
		port = "1433"
	}

	// Reject field-breaking characters in interpolated values (see
	// validateDSNField). Operators with exotic credentials should use the
	// per-alias _CONNECTION_STRING override (URL form) instead.
	for name, val := range map[string]string{
		aliasName + "_SERVER": alias.Server, aliasName + "_DATABASE": alias.Database,
		aliasName + "_USER": alias.User, aliasName + "_PASSWORD": alias.Password,
		aliasName + "_PORT": port, aliasName + "_ENCRYPT": encrypt,
	} {
		if err := validateDSNField(name, val); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
		alias.Server, port, alias.Database, alias.User, alias.Password, encrypt, trustCert,
	), nil
}

// isLegacyTLSPivotError reports whether err looks like the Go runtime
// rejecting a server that only negotiates TLS 1.0 (protocol version 301).
// This is the fingerprint error you get when connecting to a SQL Server
// 2000/2008/2012 instance that has no TLS 1.2 support, with a modern Go
// runtime that refuses to negotiate below TLS 1.2. The wording covers both
// Go's exact phrasing ("protocol version 301") and a broader
// "unsupported protocol" fallback in case the Go error format changes.
func isLegacyTLSPivotError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "protocol version 301") ||
		strings.Contains(msg, "unsupported protocol")
}
