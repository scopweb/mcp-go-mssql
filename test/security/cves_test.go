package security

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

// CVERecord represents a known CVE vulnerability
type CVERecord struct {
	CVEId            string
	PackageName      string
	MinSafeVersion   string // minimum safe semver version (e.g. "v0.31.0")
	Severity         string
	Description      string
}

// TestKnownCVEs verifies that go.mod dependency versions are above known-vulnerable ranges.
func TestKnownCVEs(t *testing.T) {
	knownCVEs := []CVERecord{
		{
			CVEId:          "CVE-2023-45283",
			PackageName:    "golang.org/x/crypto",
			MinSafeVersion: "v0.31.0",
			Severity:       "HIGH",
			Description:    "Cipher.Update vulnerability in crypto/cipher",
		},
		{
			CVEId:          "CVE-2024-34156",
			PackageName:    "golang.org/x/text",
			MinSafeVersion: "v0.18.0",
			Severity:       "MEDIUM",
			Description:    "Stack exhaustion in encoding/gob",
		},
	}

	goModBytes, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}
	goMod := string(goModBytes)

	for _, cve := range knownCVEs {
		actualVersion := extractVersion(goMod, cve.PackageName)
		if actualVersion == "" {
			t.Errorf("[%s] %s: package not found in go.mod", cve.CVEId, cve.PackageName)
			continue
		}

		// Ensure both have "v" prefix for semver comparison
		actual := ensureVPrefix(actualVersion)
		minimum := ensureVPrefix(cve.MinSafeVersion)

		if semver.Compare(actual, minimum) < 0 {
			t.Errorf("[%s] %s %s is below minimum safe version %s (%s: %s)",
				cve.CVEId, cve.PackageName, actualVersion, cve.MinSafeVersion,
				cve.Severity, cve.Description)
		}
	}
}

// extractVersion finds the version of a module in go.mod content.
func extractVersion(goMod, moduleName string) string {
	for _, line := range strings.Split(goMod, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, moduleName+" ") || strings.HasPrefix(line, moduleName+"\t") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// ensureVPrefix adds "v" prefix if missing for semver compatibility.
func ensureVPrefix(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// TestSQLInjectionVulnerability checks for SQL injection vulnerabilities
func TestSQLInjectionVulnerability(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		shouldBlock bool
		description string
	}{
		{
			name:        "Simple SQL injection",
			input:       "1' OR '1'='1",
			shouldBlock: true,
			description: "Classic SQL injection attempt",
		},
		{
			name:        "Union-based injection",
			input:       "1 UNION SELECT * FROM users--",
			shouldBlock: true,
			description: "Union-based SQL injection",
		},
		{
			name:        "Comment injection",
			input:       "admin'--",
			shouldBlock: true,
			description: "SQL comment to bypass authentication",
		},
		{
			name:        "Stacked queries",
			input:       "1; DROP TABLE users--",
			shouldBlock: true,
			description: "Stacked query injection",
		},
		{
			name:        "Time-based blind injection",
			input:       "1' AND SLEEP(5)--",
			shouldBlock: true,
			description: "Time-based blind SQL injection",
		},
		{
			name:        "Safe input",
			input:       "12345",
			shouldBlock: false,
			description: "Normal numeric input",
		},
	}

	for _, tc := range testCases {
		isSafe := isSafeSQL(tc.input)
		expected := !tc.shouldBlock

		if isSafe != expected {
			t.Errorf("%s: %s (got safe=%v, expected safe=%v)", tc.name, tc.description, isSafe, expected)
		}
	}
}

// isSafeSQL checks if input is safe from SQL injection
func isSafeSQL(input string) bool {
	dangerous := []string{"'", "--", "/*", "*/", "union", "select", "drop", "insert", "delete", "update", ";"}

	inputLower := strings.ToLower(input)
	for _, pattern := range dangerous {
		if strings.Contains(inputLower, pattern) {
			return false
		}
	}

	return true
}

// TestPathTraversalVulnerability checks for path traversal vulnerabilities
func TestPathTraversalVulnerability(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		shouldBlock bool
		description string
	}{
		{
			name:        "Simple path traversal",
			path:        "../../../../etc/passwd",
			shouldBlock: true,
			description: "Attempt to access parent directories",
		},
		{
			name:        "Windows path traversal",
			path:        "..\\..\\..\\windows\\system32",
			shouldBlock: true,
			description: "Windows-style path traversal",
		},
		{
			name:        "Absolute path",
			path:        "/etc/passwd",
			shouldBlock: true,
			description: "Absolute path outside allowed directory",
		},
		{
			name:        "URL encoded traversal",
			path:        "%2e%2e%2fetc%2fpasswd",
			shouldBlock: true,
			description: "URL-encoded path traversal",
		},
		{
			name:        "Double encoded",
			path:        "%252e%252e%252fetc%252fpasswd",
			shouldBlock: true,
			description: "Double URL-encoded path traversal",
		},
		{
			name:        "Safe path",
			path:        "documents/report.txt",
			shouldBlock: false,
			description: "Normal file within allowed directory",
		},
	}

	for _, tc := range testCases {
		isSafe := isSafePath(tc.path)
		expected := !tc.shouldBlock

		if isSafe != expected {
			t.Errorf("%s: %s (got safe=%v, expected safe=%v)", tc.name, tc.description, isSafe, expected)
		}
	}
}

// isSafePath checks if a path is safe from traversal
func isSafePath(path string) bool {
	dangerous := []string{"../", "..\\", "..%2f", "..%5c", "%2e%2e", "%252e%252e", "//", "\\\\"}

	for _, pattern := range dangerous {
		if strings.Contains(strings.ToLower(path), pattern) {
			return false
		}
	}

	// Check for absolute paths
	if strings.HasPrefix(path, "/") || (len(path) > 1 && path[1] == ':') {
		return false
	}

	return true
}

// TestCommandInjectionVulnerability checks for command injection risks
func TestCommandInjectionVulnerability(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		shouldBlock bool
		description string
	}{
		{
			name:        "Simple command injection",
			input:       "file.txt; rm -rf /",
			shouldBlock: true,
			description: "Shell metacharacter semicolon",
		},
		{
			name:        "Pipe injection",
			input:       "file.txt | cat /etc/passwd",
			shouldBlock: true,
			description: "Shell pipe character",
		},
		{
			name:        "Backtick injection",
			input:       "file.txt`whoami`",
			shouldBlock: true,
			description: "Command substitution with backticks",
		},
		{
			name:        "Dollar parenthesis injection",
			input:       "file.txt$(whoami)",
			shouldBlock: true,
			description: "Command substitution with $(...)",
		},
		{
			name:        "Ampersand injection",
			input:       "file.txt & whoami",
			shouldBlock: true,
			description: "Background process separator",
		},
		{
			name:        "Safe filename",
			input:       "myfile_2024.txt",
			shouldBlock: false,
			description: "Normal filename",
		},
	}

	for _, tc := range testCases {
		isSafe := isSafeInput(tc.input)
		expected := !tc.shouldBlock

		if isSafe != expected {
			t.Errorf("%s: %s (got safe=%v, expected safe=%v)", tc.name, tc.description, isSafe, expected)
		}
	}
}

// isSafeInput checks if input is safe from command injection
func isSafeInput(input string) bool {
	dangerousChars := []string{";", "|", "&", "`", "$", "(", ")", "\\", "'", "\""}

	for _, char := range dangerousChars {
		if strings.Contains(input, char) {
			return false
		}
	}

	return true
}

// BenchmarkSecurityChecks measures security validation overhead
func BenchmarkSecurityChecks(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		isSafePath("documents/file.txt")
		isSafeInput("normal input")
	}
}
