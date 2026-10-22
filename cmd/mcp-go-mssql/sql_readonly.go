package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"mcp-go-mssql/internal/sqlguard"
)

// Read-only SQL guard: stacked batches, dangerous keywords, and safe admin procedures.

// Pre-compiled word-boundary patterns for read-only keyword detection
var dangerousKeywordPatterns = map[string]*regexp.Regexp{
	"INSERT":   regexp.MustCompile(`(?i)\bINSERT\b`),
	"UPDATE":   regexp.MustCompile(`(?i)\bUPDATE\b`),
	"DELETE":   regexp.MustCompile(`(?i)\bDELETE\b`),
	"DROP":     regexp.MustCompile(`(?i)\bDROP\b`),
	"CREATE":   regexp.MustCompile(`(?i)\bCREATE\b`),
	"ALTER":    regexp.MustCompile(`(?i)\bALTER\b`),
	"TRUNCATE": regexp.MustCompile(`(?i)\bTRUNCATE\b`),
	"MERGE":    regexp.MustCompile(`(?i)\bMERGE\b`),
	"EXEC":     regexp.MustCompile(`(?i)\bEXEC\b`),
	"EXECUTE":  regexp.MustCompile(`(?i)\bEXECUTE\b`),
	"CALL":     regexp.MustCompile(`(?i)\bCALL\b`),
	"BULK":     regexp.MustCompile(`(?i)\bBULK\b`),
	"BCP":      regexp.MustCompile(`(?i)\bBCP\b`),
}

// readOnlyDangerousSPs is the single source of truth for system procedures
// that must always be blocked in read-only contexts, even when we allow
// certain safe administrative procedures via EXEC.
var readOnlyDangerousSPs = []string{
	"XP_CMDSHELL", "XP_REGREAD", "XP_REGWRITE", "XP_FILEEXIST",
	"XP_DIRTREE", "XP_FIXEDDRIVES", "XP_SERVICECONTROL",
	"SP_CONFIGURE", "SP_ADDLOGIN", "SP_DROPLOGIN",
	"SP_ADDSRVROLEMEMBER", "SP_DROPSRVROLEMEMBER",
	"SP_ADDROLEMEMBER", "SP_DROPROLEMEMBER",
	"SP_EXECUTESQL", "SP_OACREATE", "SP_OAMETHOD",
}

// Pre-compiled pattern for procedure name validation
var validProcedureNamePattern = regexp.MustCompile(`^[\w.\[\]]+$`)

// Pre-compiled pattern for stored-procedure parameter names. T-SQL parameter
// names are regular identifiers, optionally prefixed with '@'. Restricting to
// this shape prevents SQL injection through the parameter-name side of the
// EXEC statement built in execute_procedure (values are always parameterized,
// but names were previously interpolated verbatim).
var validParamNamePattern = regexp.MustCompile(`^@?[A-Za-z_][A-Za-z0-9_]*$`)

func (s *MCPMSSQLServer) validateProcedureName(name string) error {
	if name == "" {
		return fmt.Errorf("empty procedure name")
	}
	if !validProcedureNamePattern.MatchString(name) {
		return fmt.Errorf("invalid procedure name: contains disallowed characters")
	}
	return nil
}

func (s *MCPMSSQLServer) validateBasicInput(input string) error {
	// Allow larger queries - up to 1MB (1048576 characters)
	maxSize := 1048576
	if customMax := os.Getenv("MSSQL_MAX_QUERY_SIZE"); customMax != "" {
		if size, err := strconv.Atoi(customMax); err == nil && size > 0 {
			maxSize = size
		}
	}

	if len(input) > maxSize {
		return fmt.Errorf("input too large (max %d characters)", maxSize)
	}
	if len(input) == 0 {
		return fmt.Errorf("empty input")
	}
	return nil
}

// stripLeadingComments removes SQL comments and whitespace from the beginning of a query.
func stripLeadingComments(query string) string {
	q := strings.TrimSpace(strings.ToUpper(query))
	for strings.HasPrefix(q, "--") || strings.HasPrefix(q, "/*") || strings.HasPrefix(q, " ") || strings.HasPrefix(q, "\t") || strings.HasPrefix(q, "\n") || strings.HasPrefix(q, "\r") {
		if strings.HasPrefix(q, "--") {
			if idx := strings.Index(q, "\n"); idx != -1 {
				q = strings.TrimSpace(q[idx+1:])
			} else {
				return q
			}
		} else if strings.HasPrefix(q, "/*") {
			if idx := strings.Index(q, "*/"); idx != -1 {
				q = strings.TrimSpace(q[idx+2:])
			} else {
				return q
			}
		} else {
			q = strings.TrimSpace(q[1:])
		}
	}
	return q
}

// hasMultipleStatements reports whether the query contains more than one SQL
// statement (i.e. a statement separator followed by additional executable
// content). It is aware of single-quoted string literals, bracketed
// identifiers, line comments (--) and block comments (/* */) so that a
// semicolon inside any of those does NOT count as a separator, and a trailing
// semicolon with only whitespace/comments after it is allowed.
//
// This closes the stacked-query bypass where a query like
// "SELECT 1; DELETE FROM prod" would be classified as a harmless SELECT by the
// prefix-based operation detector while SQL Server happily executes the whole
// batch.
func hasMultipleStatements(query string) bool {
	if hasKeywordStackedStatements(query) || hasGOBatchSeparator(query) {
		return true
	}
	const (
		stNormal = iota
		stString
		stBracket
		stLineComment
		stBlockComment
	)
	state := stNormal
	runes := []rune(query)
	n := len(runes)
	for i := 0; i < n; i++ {
		c := runes[i]
		switch state {
		case stNormal:
			switch {
			case c == '\'':
				state = stString
			case c == '[':
				state = stBracket
			case c == '-' && i+1 < n && runes[i+1] == '-':
				state = stLineComment
				i++
			case c == '/' && i+1 < n && runes[i+1] == '*':
				state = stBlockComment
				i++
			case c == ';':
				// A separator counts only if non-whitespace, non-comment
				// content follows it.
				if hasExecutableRemainder(runes[i+1:]) {
					return true
				}
			}
		case stString:
			if c == '\'' {
				// Handle escaped '' inside a string literal.
				if i+1 < n && runes[i+1] == '\'' {
					i++
				} else {
					state = stNormal
				}
			}
		case stBracket:
			if c == ']' {
				if i+1 < n && runes[i+1] == ']' {
					i++
				} else {
					state = stNormal
				}
			}
		case stLineComment:
			if c == '\n' {
				state = stNormal
			}
		case stBlockComment:
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				state = stNormal
				i++
			}
		}
	}
	return false
}

// hasExecutableRemainder reports whether the given tail contains any
// executable SQL after stripping leading whitespace and comments. Used to
// allow a harmless trailing semicolon while still catching real stacked
// statements.
func hasExecutableRemainder(tail []rune) bool {
	s := strings.TrimSpace(string(tail))
	for s != "" {
		switch {
		case strings.HasPrefix(s, "--"):
			if idx := strings.IndexByte(s, '\n'); idx != -1 {
				s = strings.TrimSpace(s[idx+1:])
			} else {
				return false
			}
		case strings.HasPrefix(s, "/*"):
			if idx := strings.Index(s, "*/"); idx != -1 {
				s = strings.TrimSpace(s[idx+2:])
			} else {
				return false
			}
		case strings.HasPrefix(s, ";"):
			s = strings.TrimSpace(s[1:])
		default:
			return true
		}
	}
	return false
}

func (s *MCPMSSQLServer) validateReadOnlyQuery(query string) error {
	// Use effective config (per-alias if a dynamic connection is active)
	effective := s.getEffectiveConfig()
	if !effective.readOnly && len(effective.whitelistTables) == 0 {
		return nil // No restrictions configured for the current context, allow all queries
	}

	// Diagnostic wrapper allowlist: SET STATISTICS (TIME|IO) (ON|OFF) is a
	// pure observability statement — it neither reads nor writes user
	// data. It is the ONLY multi-statement form we accept in read-only
	// mode. The wrapper parser is checked FIRST so a valid wrapper
	// recurses through validateReadOnlyQuery on its base statement
	// before the multi-statement rejection fires. Anything that is not
	// a valid wrapper (including "SET STATISTICS TIME ON; DROP TABLE x;
	// SELECT 1") still falls through to the multi-statement reject
	// below.
	if w, err := parseStatisticsWrapper(query); err == nil {
		return s.validateReadOnlyQuery(w.BaseStatement)
	}

	// Reject multi-statement batches outright in any restricted (read-only or
	// whitelist) context. Stacked statements are the primary way to smuggle a
	// modification past the prefix-based operation detector, e.g.
	// "SELECT 1; DELETE FROM prod". A single trailing semicolon is still fine.
	if hasMultipleStatements(query) {
		return fmt.Errorf("read-only mode: multiple SQL statements are not allowed in a single request")
	}

	if isSelectInto(query) && effective.readOnly {
		return fmt.Errorf("read-only mode: SELECT INTO is a modification")
	}

	// A whitelist is a write policy for an explicitly writable context. It
	// never overrides READ_ONLY=true, which is intentionally absolute.
	whitelist := effective.whitelistTables
	if !effective.readOnly && len(whitelist) > 0 {
		// Whitelist is configured - let validateTablePermissions() handle modification permissions
		// We still need to block dangerous operations though
		normalizedQuery := stripLeadingComments(query)
		_ = normalizedQuery // used for future pattern matching if needed

		// Procedures have a separate allowlist and must never be reached via
		// query_database merely because a table whitelist exists.
		if strings.HasPrefix(strings.ToUpper(stripLeadingComments(query)), "EXEC") {
			return fmt.Errorf("read-only mode: execute_procedure must be used for stored procedures")
		}

		// Block dangerous system procedures even with whitelist
		queryUpper := strings.ToUpper(query)
		for _, sp := range readOnlyDangerousSPs {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}

		// Allow query to proceed to validateTablePermissions() for whitelist check
		return nil
	}

	// No whitelist configured - enforce strict read-only mode
	normalizedQuery := stripLeadingComments(query)

	// Special case (new for admin introspection support):
	// Allow EXEC/EXECUTE of a small set of known read-only system procedures
	// used for schema discovery and database administration.
	// This is deliberately narrow and does not relax the general ban on EXEC.
	if isSafeReadOnlyAdminProcedure(query) {
		queryUpper := strings.ToUpper(query)
		for _, sp := range readOnlyDangerousSPs {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}
		return nil
	}

	// List of allowed read-only operations (traditional path)
	allowedPrefixes := []string{
		"SELECT",
		"WITH", // Common Table Expressions that start with WITH
		"SHOW",
		"DESCRIBE",
		"DESC",
		"EXPLAIN",
	}

	// Check if query starts with an allowed prefix
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(normalizedQuery, prefix) {
			// Additional check: ensure no dangerous keywords are present (using word boundaries)
			for keyword, pattern := range dangerousKeywordPatterns {
				if pattern.MatchString(query) {
					return fmt.Errorf("read-only mode: query contains forbidden operation '%s'", keyword)
				}
			}

			// Check for dangerous SPs (single source of truth)
			queryUpper := strings.ToUpper(query)
			for _, sp := range readOnlyDangerousSPs {
				if strings.Contains(queryUpper, sp) {
					return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
				}
			}

			// Safe read-only system procedures (legacy path for SELECT queries that mention SP_ names)
			safeSPs := []string{
				"SP_HELP", "SP_HELPTEXT", "SP_HELPINDEX", "SP_HELPCONSTRAINT",
				"SP_COLUMNS", "SP_TABLES", "SP_STORED_PROCEDURES",
				"SP_FKEYS", "SP_PKEYS", "SP_STATISTICS",
				"SP_DATABASES", "SP_HELPDB",
			}

			// If query contains SP_ or XP_, verify it's in the safe list
			if strings.Contains(queryUpper, "SP_") || strings.Contains(queryUpper, "XP_") {
				isSafe := false
				for _, safeSP := range safeSPs {
					if strings.Contains(queryUpper, safeSP) {
						isSafe = true
						break
					}
				}
				if !isSafe {
					return fmt.Errorf("read-only mode: system procedure not in allowed list")
				}
			}

			return nil // Query is allowed
		}
	}

	return fmt.Errorf("read-only mode: only SELECT and read operations are allowed")
}

func isSelectInto(query string) bool {
	return sqlguard.HasSelectInto(query)
}

// isSafeReadOnlyAdminProcedure reports whether the query is an invocation
// of a well-known read-only system stored procedure used for schema discovery
// and administrative introspection (e.g. sp_help, sp_helptext, sp_spaceused).
//
// This is the controlled relaxation that allows legitimate administrative reads
// via EXEC while the general EXEC/EXECUTE keyword remains dangerous for everything else.
func isSafeReadOnlyAdminProcedure(query string) bool {
	q := strings.ToUpper(stripLeadingComments(stripSQLComments(query)))
	q = strings.TrimSpace(q)

	// Match common forms:
	//   EXEC sp_help 'dbo.Table'
	//   EXECUTE dbo.sp_helptext 'proc'
	//   EXEC [sys].[sp_columns] @table_name='x'
	// Schema is captured so app.sp_help cannot impersonate a system procedure.
	re := regexp.MustCompile(`^\s*EXEC(UTE)?\s+(?:\[?([\w$]+)\]?\.)?\[?([\w$]+)\]?\b`)
	matches := re.FindStringSubmatch(q)
	if len(matches) < 4 {
		return false
	}

	schema := strings.ToUpper(matches[2])
	if schema != "" && schema != "DBO" && schema != "SYS" {
		return false
	}

	proc := strings.ToUpper(matches[3])

	// Conservative allowlist of read-only administrative / schema procedures.
	// Only add procedures that are:
	//   - Documented as read-only / no side effects
	//   - Commonly used for legitimate schema and admin discovery
	//   - Do not allow arbitrary code execution or configuration changes
	safeAdminProcs := map[string]bool{
		"SP_HELP":              true,
		"SP_HELPTEXT":          true,
		"SP_HELPINDEX":         true,
		"SP_HELPCONSTRAINT":    true,
		"SP_HELPTRIGGER":       true,
		"SP_COLUMNS":           true,
		"SP_TABLES":            true,
		"SP_STORED_PROCEDURES": true,
		"SP_FKEYS":             true,
		"SP_PKEYS":             true,
		"SP_STATISTICS":        true,
		"SP_DATABASES":         true,
		"SP_HELPDB":            true,
		"SP_SPACEUSED":         true,
		// Intentionally not including sp_who / sp_lock in the initial set
		// to keep the surface minimal. Can be evaluated later.
	}

	if !safeAdminProcs[proc] {
		return false
	}
	if proc == "SP_SPACEUSED" && spaceUsedUpdatesCatalog(q) {
		return false
	}
	return true
}

func spaceUsedUpdatesCatalog(queryUpper string) bool {
	if regexp.MustCompile(`@UPDATEUSAGE\s*=\s*(N)?'?(TRUE|YES|T|1)'?`).MatchString(queryUpper) {
		return true
	}
	return regexp.MustCompile(`SP_SPACEUSED\s+(?:N)?'[^']*'\s*,`).MatchString(queryUpper) ||
		regexp.MustCompile(`SP_SPACEUSED\s+[A-Z0-9_$.\[\]]+\s*,`).MatchString(queryUpper)
}
