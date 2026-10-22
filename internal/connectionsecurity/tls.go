// Package connectionsecurity shares DSN security policy across MCP and CLI.
package connectionsecurity

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/microsoft/go-mssqldb/msdsn"
)

// ValidateDSNField rejects separators in interpolated ADO connection fields.
func ValidateDSNField(name, value string) error {
	if strings.ContainsAny(value, ";\r\n\x00") {
		return fmt.Errorf("invalid character in %s: ';', newline and null bytes are not allowed (use MSSQL_CONNECTION_STRING with a URL-form DSN for exotic values)", name)
	}
	return nil
}

// ProductionTLSViolation interprets a DSN using the driver's own parser.
// Parser errors are deliberately not exposed because they may include secrets.
func ProductionTLSViolation(connStr string) string {
	cfg, err := msdsn.Parse(connStr)
	if err != nil {
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

// EnforceProductionTLS denies insecure settings unless explicitly opted in.
func EnforceProductionTLS(label, connStr string) error {
	violation := ProductionTLSViolation(connStr)
	if violation == "" {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MSSQL_ALLOW_INSECURE_TLS")), "true") {
		slog.Warn(fmt.Sprintf("%s: %s in production — permitted because MSSQL_ALLOW_INSECURE_TLS=true", label, violation))
		return nil
	}
	return fmt.Errorf("%s: %s in production. Refusing to start. Correct the TLS settings, or set MSSQL_ALLOW_INSECURE_TLS=true to intentionally allow it (needed only for legacy SQL Server 2000/2008/2012 that negotiate TLS 1.0)", label, violation)
}
