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

var forbiddenReadWords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
	"MERGE", "EXEC", "EXECUTE", "CALL", "BULK", "BCP",
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
type restrictedClass int

const (
	classSingle restrictedClass = iota
	classMultiple
	classGO
	classLexical
)

func classifyRestricted(query string) (sqlguard.Analysis, restrictedClass) {
	a, err := sqlguard.Analyze(query)
	if err != nil {
		return sqlguard.Analysis{}, classLexical
	}
	if a.GO {
		return a, classGO
	}
	if a.Statements > 1 || stackedWords(a.TopLevel) {
		return a, classMultiple
	}
	return a, classSingle
}

func restrictedBatchError(class restrictedClass) error {
	switch class {
	case classLexical:
		return fmt.Errorf("read-only mode: unterminated SQL literal, identifier, or comment")
	case classGO:
		return fmt.Errorf("read-only mode: GO batch separators are not supported")
	case classMultiple:
		return fmt.Errorf("read-only mode: multiple SQL statements are not allowed in a single request")
	default:
		return nil
	}
}

func hasMultipleStatements(query string) bool {
	_, class := classifyRestricted(query)
	return class != classSingle
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
	// A lexical error is never treated as a single statement.
	analysis, class := classifyRestricted(query)
	if err := restrictedBatchError(class); err != nil {
		return err
	}

	if isSelectInto(query) && effective.readOnly {
		return fmt.Errorf("read-only mode: SELECT INTO is a modification")
	}

	// A whitelist is a write policy for an explicitly writable context. It
	// never overrides READ_ONLY=true, which is intentionally absolute.
	whitelist := effective.whitelistTables
	if !effective.readOnly && len(whitelist) > 0 {
		// Procedures have a separate allowlist and must never be reached via
		// query_database merely because a table whitelist exists.
		if len(analysis.TopLevel) > 0 && (analysis.TopLevel[0] == "EXEC" || analysis.TopLevel[0] == "EXECUTE") {
			return fmt.Errorf("read-only mode: execute_procedure must be used for stored procedures")
		}

		if sp := forbiddenProcedure(analysis.Words); sp != "" {
			return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
		}

		// Allow query to proceed to validateTablePermissions() for whitelist check
		return nil
	}

	// No whitelist configured - enforce strict read-only mode.
	// Allow EXEC/EXECUTE of a small set of known read-only system procedures
	// used for schema discovery. This does not relax the general ban on EXEC.
	if isSafeReadOnlyAdminProcedure(query) {
		if sp := forbiddenProcedure(analysis.Words); sp != "" {
			return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
		}
		return nil
	}

	allowedPrefixes := []string{
		"SELECT",
		"WITH",
		"SHOW",
		"DESCRIBE",
		"DESC",
		"EXPLAIN",
	}
	started := ""
	if len(analysis.TopLevel) > 0 {
		started = analysis.TopLevel[0]
	}
	for _, prefix := range allowedPrefixes {
		if started != prefix {
			continue
		}
		if word := firstForbiddenWord(analysis.Words); word != "" {
			return fmt.Errorf("read-only mode: query contains forbidden operation '%s'", word)
		}
		if sp := forbiddenProcedure(analysis.Words); sp != "" {
			return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
		}
		if unsafeSystemProc(analysis.Words) {
			return fmt.Errorf("read-only mode: system procedure not in allowed list")
		}
		return nil
	}

	return fmt.Errorf("read-only mode: only SELECT and read operations are allowed")
}

func firstForbiddenWord(words []string) string {
	for _, w := range words {
		for _, bad := range forbiddenReadWords {
			if w == bad {
				return bad
			}
		}
	}
	return ""
}

func forbiddenProcedure(words []string) string {
	for _, sp := range readOnlyDangerousSPs {
		if sqlguard.ExecutableContains(words, sp) {
			return sp
		}
	}
	return ""
}

func unsafeSystemProc(words []string) bool {
	safeSPs := []string{
		"SP_HELP", "SP_HELPTEXT", "SP_HELPINDEX", "SP_HELPCONSTRAINT",
		"SP_COLUMNS", "SP_TABLES", "SP_STORED_PROCEDURES",
		"SP_FKEYS", "SP_PKEYS", "SP_STATISTICS",
		"SP_DATABASES", "SP_HELPDB",
	}
	for _, w := range words {
		if !strings.Contains(w, "SP_") && !strings.Contains(w, "XP_") {
			continue
		}
		safe := false
		for _, name := range safeSPs {
			if strings.Contains(w, name) {
				safe = true
				break
			}
		}
		if !safe {
			return true
		}
	}
	return false
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
