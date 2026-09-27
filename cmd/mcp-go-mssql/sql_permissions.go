package main

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Table whitelist and modification confirmation. Regex is defense in depth, not the security boundary.

// Pre-compiled patterns for table name extraction (performance optimization)
// These patterns try to be robust against common schema-qualified names.
var tableExtractionPatterns = []*regexp.Regexp{
	// FROM / JOIN with optional schema or database prefix
	regexp.MustCompile(`(?i)\bFROM\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bJOIN\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),

	// INSERT / UPDATE / DELETE
	regexp.MustCompile(`(?i)\bINTO\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bUPDATE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?\s+FROM`),

	// DDL
	regexp.MustCompile(`(?i)\bTABLE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bVIEW\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),

	// Extra safety for fully qualified names like db.schema.table or [db].[schema].[table]
	regexp.MustCompile(`(?i)\bFROM\s+[\w\.\[\]]+\.[\w\.\[\]]+\.?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bJOIN\s+[\w\.\[\]]+\.[\w\.\[\]]+\.?\[?([\w]+)\]?`),
}

// qualifiedTableExtractionPatterns mirror tableExtractionPatterns but capture
// the FULL 1-to-3-part object reference (e.g. "otro_schema.temp_ai",
// "otradb.dbo.temp_ai") instead of only the final segment. They exist so the
// whitelist check can compare schema-qualified names (M4) and stop treating
// "otro_schema.temp_ai" as the whitelisted, unqualified "temp_ai". Each pattern
// captures group 1 = a run of dotted, optionally bracketed identifiers.
var qualifiedTableExtractionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bFROM\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bJOIN\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bINTO\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bUPDATE\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bDELETE\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})\s+FROM`),
	regexp.MustCompile(`(?i)\bTABLE\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
	regexp.MustCompile(`(?i)\bVIEW\s+((?:\[[^\]]+\]|\w+)(?:\.(?:\[[^\]]+\]|\w+)){0,2})`),
}

// getWhitelistedTables returns the cached list of tables/views allowed for modification.
func (s *MCPMSSQLServer) getWhitelistedTables() []string {
	return s.config.whitelistTables
}

// getEffectiveConfig returns the security configuration that should be applied right now.
// If there is an active dynamic alias, it returns that alias's posture.
// Otherwise, it falls back to the global server config.
// This is critical for secure dynamic multi-database usage.
func (s *MCPMSSQLServer) getEffectiveConfig() serverConfig {
	s.dynamicMu.RLock()
	defer s.dynamicMu.RUnlock()

	if s.activeAlias != "" {
		if alias, ok := s.dynamicAliases[s.activeAlias]; ok {
			return serverConfig{
				readOnly:        alias.ReadOnly,
				whitelistTables: alias.WhitelistTables,
				whitelistProcs:  s.config.whitelistProcs, // global for now
			}
		}
	}

	// No active dynamic alias → use global config (with the safety guard already applied at startup)
	return s.config
}

// requireConfirmationForModification is called when a writable alias attempts a modification.
// It returns an error that tells the AI it must call confirm_operation first.
func queryHash(query string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(query)))
	return fmt.Sprintf("%x", sum[:])
}

func (s *MCPMSSQLServer) requireConfirmationForModification(operation string, tables []string, alias, query string) error {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	desc := fmt.Sprintf("%s on tables: %s (alias %s)", operation, strings.Join(tables, ", "), alias)

	s.pendingConfirmation = &PendingConfirmation{
		Operation:   operation,
		Tables:      tables,
		Description: desc,
		Alias:       alias,
		QueryHash:   queryHash(query),
		ExpiresAt:   time.Now().Add(90 * time.Second), // 90 seconds to confirm
	}

	return fmt.Errorf("CONFIRMATION REQUIRED: This is a modification operation (%s) on a writable dynamic alias.\n\nYou must first call the 'confirm_operation' tool with this exact description:\n\"%s\"\n\nOnly after receiving confirmation will the operation be allowed.", operation, desc) //nolint:staticcheck // multi-line user-facing message; capitalization + punctuation are intentional
}

// isOperationConfirmed checks if there is a valid pending confirmation that matches the current operation.
func (s *MCPMSSQLServer) isOperationConfirmed(operation string, tables []string, alias, query string) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	if s.pendingConfirmation == nil {
		return false
	}

	if time.Now().After(s.pendingConfirmation.ExpiresAt) {
		s.pendingConfirmation = nil
		return false
	}

	// A confirmation is only valid on the alias it was issued for. Without
	// this binding, confirming a modification on writable alias A and then
	// switching to writable alias B would let the confirmation be consumed
	// against B's databases (cross-alias confirmation portability).
	if !strings.EqualFold(s.pendingConfirmation.Alias, alias) {
		return false
	}
	if !s.pendingConfirmation.Approved || s.pendingConfirmation.QueryHash != queryHash(query) {
		return false
	}

	// Simple matching: same operation and at least one overlapping table
	if strings.EqualFold(s.pendingConfirmation.Operation, operation) {
		for _, t1 := range s.pendingConfirmation.Tables {
			for _, t2 := range tables {
				if t1 == t2 {
					// Consume the approved confirmation after one exact statement.
					s.pendingConfirmation = nil
					return true
				}
			}
		}
	}

	return false
}

// extractAllTablesFromQuery finds all table/view names referenced in the query
func (s *MCPMSSQLServer) extractAllTablesFromQuery(query string) []string {
	queryUpper := strings.ToUpper(query)
	tablesFound := make(map[string]bool) // Use map to avoid duplicates

	for _, pattern := range tableExtractionPatterns {
		matches := pattern.FindAllStringSubmatch(queryUpper, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := match[1]
				// Remove brackets if present [tablename] -> tablename
				tableName = strings.Trim(tableName, "[]")
				tableName = strings.ToLower(strings.TrimSpace(tableName))

				// Remove any remaining schema/database prefix that the regex might have captured
				// e.g. "dbo.users" -> "users"
				if idx := strings.LastIndex(tableName, "."); idx != -1 {
					tableName = tableName[idx+1:]
				}

				if tableName != "" {
					tablesFound[tableName] = true
				}
			}
		}
	}

	// Convert map keys to slice
	var tables []string
	for table := range tablesFound {
		tables = append(tables, table)
	}
	return tables
}

// splitObjectParts splits a SQL object reference on '.' while respecting
// bracketed identifiers, so "[my.schema].tbl" yields ["my.schema", "tbl"]
// rather than three parts.
func splitObjectParts(raw string) []string {
	var parts []string
	var cur strings.Builder
	inBracket := false
	for _, r := range raw {
		switch {
		case r == '[':
			inBracket = true
		case r == ']':
			inBracket = false
		case r == '.' && !inBracket:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	parts = append(parts, cur.String())
	return parts
}

// normalizeObjectName normalizes a 1-to-3-part SQL object reference into a
// canonical, lowercase, schema-qualified key for whitelist comparison (M4):
//   - 1 part  ("temp_ai")            -> "dbo.temp_ai"      (default schema)
//   - 2 parts ("app.temp_ai")        -> "app.temp_ai"
//   - 3+ parts ("db.dbo.temp_ai")    -> "db.dbo.temp_ai"   (last three parts)
//
// Because a database-qualified (3-part) name keeps its database segment, it
// will not match a 1- or 2-part whitelist entry — so cross-database writes fail
// closed unless whitelisted with the exact same 3-part name.
func normalizeObjectName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var cleaned []string
	for _, p := range splitObjectParts(raw) {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "[]")
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, strings.ToLower(p))
		}
	}
	switch len(cleaned) {
	case 0:
		return ""
	case 1:
		return "dbo." + cleaned[0]
	case 2:
		return cleaned[0] + "." + cleaned[1]
	default:
		n := len(cleaned)
		return strings.Join(cleaned[n-3:], ".")
	}
}

// extractAllQualifiedTablesFromQuery returns the schema-qualified, normalized
// names of every table/view referenced in the query (M4). Unlike
// extractAllTablesFromQuery (which strips the schema/database prefix), this
// preserves it so the whitelist check can distinguish "dbo.temp_ai" from
// "otro_schema.temp_ai".
func (s *MCPMSSQLServer) extractAllQualifiedTablesFromQuery(query string) []string {
	found := make(map[string]bool)
	for _, pattern := range qualifiedTableExtractionPatterns {
		for _, match := range pattern.FindAllStringSubmatch(query, -1) {
			if len(match) > 1 {
				if name := normalizeObjectName(match[1]); name != "" {
					found[name] = true
				}
			}
		}
	}
	var out []string
	for k := range found {
		out = append(out, k)
	}
	return out
}

// extractOperation determines the primary SQL operation (INSERT, UPDATE, DELETE, etc.)
func (s *MCPMSSQLServer) extractOperation(query string) string {
	queryUpper := stripLeadingComments(stripSQLComments(query))
	if isSelectInto(queryUpper) {
		return "SELECT_INTO"
	}

	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE"}
	for _, op := range modifyOps {
		if strings.HasPrefix(queryUpper, op) {
			return op
		}
	}

	// If WITH is found, check if there's a modify operation after the CTE.
	// Whole-word match: a substring search flagged any CTE touching a column such as
	// DataUltimUpdate / IDUsuariUltimUpdate as UPDATE (false positive that blocked plain
	// SELECTs in read-only mode). A keyword inside a string literal still matches, which
	// errs on the safe side.
	if strings.HasPrefix(queryUpper, "WITH") {
		if m := cteModifyKeywordRe.FindString(queryUpper); m != "" {
			return strings.ToUpper(m)
		}
	}

	return "SELECT" // Default to SELECT for read operations
}

// cteModifyKeywordRe detects a modifying statement after a CTE as a WHOLE WORD.
var cteModifyKeywordRe = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|CREATE|ALTER|TRUNCATE|MERGE)\b`)

// validateTablePermissions validates that all tables in a modify operation are whitelisted
func (s *MCPMSSQLServer) validateTablePermissions(query string) error {
	// Use effective config (respects active dynamic alias posture)
	effective := s.getEffectiveConfig()
	if !effective.readOnly && len(effective.whitelistTables) == 0 {
		s.dynamicMu.RLock()
		isWritableDynamicAlias := s.isDynamic && s.activeAlias != ""
		s.dynamicMu.RUnlock()
		if isWritableDynamicAlias {
			return fmt.Errorf("permission denied: writable dynamic aliases require an explicit table whitelist")
		}
		return nil // No restrictions configured for the current context, allow all operations
	}

	whitelist := effective.whitelistTables
	operation := s.extractOperation(query)

	// Determine if this is a modification operation
	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE", "SELECT_INTO"}
	isModifyOp := false
	for _, op := range modifyOps {
		if operation == op {
			isModifyOp = true
			break
		}
	}

	// If not a modify operation (e.g., SELECT), allow it
	if !isModifyOp {
		return nil
	}
	if effective.readOnly {
		return fmt.Errorf("permission denied: read-only mode blocks %s operations", operation)
	}

	// Extract ALL tables referenced in the query. tablesInQuery holds the bare
	// (schema-stripped) names, kept for confirmation prompts, logging and the
	// fail-closed check so their behavior is unchanged. qualifiedTables holds
	// the schema-qualified names used for the actual whitelist decision (M4).
	tablesInQuery := s.extractAllTablesFromQuery(query)
	qualifiedTables := s.extractAllQualifiedTablesFromQuery(query)

	// === CONFIRMATION REQUIREMENT FOR WRITABLE DYNAMIC ALIASES ===
	// We only enforce explicit confirmation when using dynamic mode with a writable alias.
	// This protects the high-risk "multiple databases" scenario without breaking classic single-connection usage.
	s.dynamicMu.RLock()
	activeAlias := s.activeAlias
	hasActiveWritableAlias := activeAlias != "" && !effective.readOnly
	s.dynamicMu.RUnlock()

	s.secLogger.Printf("Permission check - Operation: %s, Tables found: %v, Whitelist: %v",
		operation, tablesInQuery, whitelist)

	// If whitelist is empty, deny all modifications
	if len(whitelist) == 0 {
		return fmt.Errorf("permission denied: no tables are whitelisted for %s operations", operation)
	}

	// FAIL CLOSED: if this is a modification operation but the regex-based
	// extractor could not identify any target table, we must deny rather than
	// allow. Otherwise a modify query whose table the regex fails to parse
	// would slip through the per-table whitelist loop below (which simply does
	// not iterate) and be permitted.
	if len(tablesInQuery) == 0 || len(qualifiedTables) == 0 {
		s.secLogger.Printf("SECURITY VIOLATION: %s operation with no extractable table name - denying (fail-closed)", operation)
		return fmt.Errorf("permission denied: could not determine the target table(s) for this %s operation; rewrite it using a simple, explicit table reference", operation)
	}
	// If the schema-qualified extractor accounted for fewer references than the
	// bare one, a referenced table may have gone unchecked — deny (fail-closed).
	if len(qualifiedTables) < len(tablesInQuery) {
		s.secLogger.Printf("SECURITY VIOLATION: %s operation - qualified extractor (%d) disagrees with bare extractor (%d); denying (fail-closed)", operation, len(qualifiedTables), len(tablesInQuery))
		return fmt.Errorf("permission denied: could not reliably qualify all target table(s) for this %s operation; rewrite it using simple, explicit table references", operation)
	}

	// Normalize the whitelist to the same schema-qualified form once (M4). Bare
	// entries default to the dbo schema, so "temp_ai" matches "dbo.temp_ai" but
	// not "otro_schema.temp_ai" or "otradb.dbo.temp_ai".
	normalizedWhitelist := make(map[string]bool, len(whitelist))
	for _, allowed := range whitelist {
		if nw := normalizeObjectName(allowed); nw != "" {
			normalizedWhitelist[nw] = true
		}
	}

	// Check if ALL (schema-qualified) tables in the query are whitelisted.
	for _, table := range qualifiedTables {
		if !normalizedWhitelist[table] {
			s.secLogger.Printf("SECURITY VIOLATION: Attempted %s operation on non-whitelisted table '%s'",
				operation, table)
			return fmt.Errorf("permission denied: table '%s' is not whitelisted for %s operations",
				table, operation)
		}
	}

	if hasActiveWritableAlias && !s.isOperationConfirmed(operation, tablesInQuery, activeAlias, query) {
		return s.requireConfirmationForModification(operation, tablesInQuery, activeAlias, query)
	}

	// All tables are whitelisted
	s.secLogger.Printf("Permission granted: %s operation on whitelisted table(s) %v",
		operation, qualifiedTables)
	return nil
}
