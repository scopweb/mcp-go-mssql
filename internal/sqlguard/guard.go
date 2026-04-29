package sqlguard

import (
	"fmt"
	"strings"
)

// dangerousSystemProcedures are SQL Server system procedures that must never
// be invoked under read-only mode regardless of whitelist configuration.
// Categorised loosely: shell access, registry, login/role admin, dynamic
// SQL, and OLE Automation. Compared case-insensitively against the upper-
// cased query.
var dangerousSystemProcedures = []string{
	"XP_CMDSHELL", "XP_REGREAD", "XP_REGWRITE", "XP_FILEEXIST",
	"XP_DIRTREE", "XP_FIXEDDRIVES", "XP_SERVICECONTROL",
	"SP_CONFIGURE", "SP_ADDLOGIN", "SP_DROPLOGIN",
	"SP_ADDSRVROLEMEMBER", "SP_DROPSRVROLEMEMBER",
	"SP_ADDROLEMEMBER", "SP_DROPROLEMEMBER",
	"SP_EXECUTESQL", "SP_OACREATE", "SP_OAMETHOD",
}

// safeReadOnlyProcedures are read-only system procedures that callers may
// legitimately invoke even under read-only mode. Anything outside this list
// containing SP_ or XP_ is rejected.
var safeReadOnlyProcedures = []string{
	"SP_HELP", "SP_HELPTEXT", "SP_HELPINDEX", "SP_HELPCONSTRAINT",
	"SP_COLUMNS", "SP_TABLES", "SP_STORED_PROCEDURES",
	"SP_FKEYS", "SP_PKEYS", "SP_STATISTICS",
	"SP_DATABASES", "SP_HELPDB",
}

// allowedReadOnlyPrefixes are the SQL verbs allowed under strict read-only
// mode (when no whitelist is configured). WITH covers Common Table
// Expressions; the body is then re-validated for forbidden keywords.
var allowedReadOnlyPrefixes = []string{
	"SELECT",
	"WITH",
	"SHOW",
	"DESCRIBE",
	"DESC",
	"EXPLAIN",
}

// ValidateReadOnly enforces the read-only policy.
//
//   - If read-only is disabled, returns nil (any query allowed).
//   - If a whitelist is configured, dangerous system procedures are blocked
//     but everything else is allowed through; the per-table check is
//     performed by ValidateTablePermissions.
//   - With no whitelist, only queries beginning with SELECT / WITH / SHOW /
//     DESCRIBE / DESC / EXPLAIN are accepted, and even those are screened
//     for embedded modify keywords and unsafe system procedures.
//
// The error returned names the offending keyword or procedure so callers
// can surface a useful message to the user.
func (g *Guard) ValidateReadOnly(query string) error {
	if !g.cfg.ReadOnly {
		return nil
	}

	whitelist := g.cfg.Whitelist
	if len(whitelist) > 0 {
		return validateReadOnlyWithWhitelist(query)
	}

	return validateReadOnlyStrict(query)
}

// validateReadOnlyWithWhitelist applies the relaxed read-only check used
// when a whitelist is configured: only the dangerous-procedures filter
// runs, and the per-table whitelist check is the caller's responsibility.
func validateReadOnlyWithWhitelist(query string) error {
	queryUpper := strings.ToUpper(query)
	for _, sp := range dangerousSystemProcedures {
		if strings.Contains(queryUpper, sp) {
			return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
		}
	}
	return nil
}

// validateReadOnlyStrict applies the strict read-only check used when no
// whitelist is configured: only SELECT-class verbs are allowed, dangerous
// keywords are rejected wherever they appear, and system procedures are
// limited to a small allow list.
func validateReadOnlyStrict(query string) error {
	normalizedQuery := StripLeadingComments(query)

	for _, prefix := range allowedReadOnlyPrefixes {
		if !strings.HasPrefix(normalizedQuery, prefix) {
			continue
		}

		// Even a SELECT/WITH-prefixed query may embed a modify
		// operation later (e.g. WITH x AS (DELETE ...) SELECT 1).
		for keyword, pattern := range dangerousKeywordPatterns {
			if pattern.MatchString(query) {
				return fmt.Errorf("read-only mode: query contains forbidden operation '%s'", keyword)
			}
		}

		queryUpper := strings.ToUpper(query)
		for _, sp := range dangerousSystemProcedures {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}

		// If SP_ or XP_ appears at all, require it to be in the safe
		// list. This is a conservative belt-and-braces check.
		if strings.Contains(queryUpper, "SP_") || strings.Contains(queryUpper, "XP_") {
			isSafe := false
			for _, safeSP := range safeReadOnlyProcedures {
				if strings.Contains(queryUpper, safeSP) {
					isSafe = true
					break
				}
			}
			if !isSafe {
				return fmt.Errorf("read-only mode: system procedure not in allowed list")
			}
		}

		return nil
	}

	return fmt.Errorf("read-only mode: only SELECT and read operations are allowed")
}

// ValidateSubqueriesForRestrictedTables prevents data exfiltration via
// nested queries when read-only + whitelist mode is active. Without this
// check an attacker could write
//
//	SELECT * FROM (SELECT secret_col FROM restricted_table) x
//
// and bypass top-level table validation. We extract any (SELECT ...)
// subqueries and refuse the whole query if a subquery references a
// non-whitelisted or cross-database table.
//
// No-op when read-only is off or no whitelist is configured (in which
// case there's nothing meaningful to compare against).
func (g *Guard) ValidateSubqueriesForRestrictedTables(query string) error {
	if !g.cfg.ReadOnly {
		return nil
	}
	whitelist := g.cfg.Whitelist
	if len(whitelist) == 0 {
		return nil
	}

	matches := subqueryExtractPattern.FindAllString(query, -1)
	for _, subquery := range matches {
		subqueryRefs := ExtractTableRefs(subquery)
		for _, ref := range subqueryRefs {
			if ref.Database != "" {
				return fmt.Errorf("query contains subquery referencing cross-database table '%s.%s' which is not allowed", ref.Database, ref.Table)
			}

			isWhitelisted := false
			for _, allowedTable := range whitelist {
				if allowedTable == "*" || ref.Table == allowedTable {
					isWhitelisted = true
					break
				}
			}

			if !isWhitelisted {
				return fmt.Errorf("query contains subquery that references non-whitelisted table '%s' — this pattern may be used for data exfiltration", ref.Table)
			}
		}
	}

	return nil
}

// ValidateTablePermissions checks that every table referenced by a modify
// operation is in the whitelist, and that no modification touches a
// cross-database table (which is always forbidden, even with the wildcard
// whitelist).
//
//   - Off when read-only is disabled.
//   - SELECT and other read operations bypass the per-table check.
//   - Empty whitelist → all modifications denied.
//   - Whitelist == ["*"] → all current-database tables allowed.
//   - Cross-database refs (Database != "") always rejected.
func (g *Guard) ValidateTablePermissions(query string) error {
	if !g.cfg.ReadOnly {
		return nil
	}

	whitelist := g.cfg.Whitelist
	operation := ExtractOperation(query)

	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE"}
	isModifyOp := false
	for _, op := range modifyOps {
		if operation == op {
			isModifyOp = true
			break
		}
	}
	if !isModifyOp {
		return nil
	}

	refsInQuery := ExtractTableRefs(query)

	var tableNames []string
	for _, ref := range refsInQuery {
		if ref.Database != "" {
			tableNames = append(tableNames, ref.Database+"."+ref.Table)
		} else {
			tableNames = append(tableNames, ref.Table)
		}
	}

	g.log.Printf("Permission check - Operation: %s, Tables found: %v, Whitelist: %v",
		operation, tableNames, whitelist)

	for _, ref := range refsInQuery {
		if ref.Database != "" {
			g.log.Printf("SECURITY VIOLATION: Attempted %s operation on cross-database table '%s.%s'",
				operation, ref.Database, ref.Table)
			return fmt.Errorf("permission denied: cross-database modification not allowed — table '%s.%s' is in another database",
				ref.Database, ref.Table)
		}

		isWhitelisted := false
		for _, allowedTable := range whitelist {
			if allowedTable == "*" || ref.Table == allowedTable {
				isWhitelisted = true
				break
			}
		}

		if !isWhitelisted {
			g.log.Printf("SECURITY VIOLATION: Attempted %s operation on non-whitelisted table '%s'",
				operation, ref.Table)
			return fmt.Errorf("permission denied: table '%s' is not whitelisted for %s operations",
				ref.Table, operation)
		}
	}

	g.log.Printf("Permission granted: %s operation on whitelisted table(s) %v",
		operation, tableNames)
	return nil
}
