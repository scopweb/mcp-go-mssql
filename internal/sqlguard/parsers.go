package sqlguard

import (
	"regexp"
	"strings"
)

// ParseWhitelistTables parses a comma-separated whitelist into a normalised
// (lowercase, trimmed, deduplicated by formatting) slice. The special value
// "*" means all tables — returned as []string{"*"}.
//
// Empty input returns nil so callers can treat "no whitelist" and "empty
// whitelist" identically with len(...) == 0.
func ParseWhitelistTables(env string) []string {
	if env == "" {
		return nil
	}
	if strings.TrimSpace(env) == "*" {
		return []string{"*"}
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

// ParseAllowedDatabases parses a comma-separated list of allowed cross-
// database names. Returns nil for empty input. Names are lowercased.
func ParseAllowedDatabases(env string) []string {
	if env == "" {
		return nil
	}
	dbs := strings.Split(env, ",")
	var normalized []string
	for _, db := range dbs {
		db = strings.TrimSpace(db)
		if db != "" {
			normalized = append(normalized, strings.ToLower(db))
		}
	}
	return normalized
}

// ExtractOperation determines the primary SQL verb of a query: INSERT,
// UPDATE, DELETE, DROP, CREATE, ALTER, TRUNCATE, MERGE — or SELECT for
// anything else. Handles WITH-prefixed CTEs by scanning the body for a
// modify operation.
//
// The query is upper-cased and stripped of leading comments before
// inspection so cosmetic prefixes don't fool the detector.
func ExtractOperation(query string) string {
	queryUpper := StripLeadingComments(query)

	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE"}
	for _, op := range modifyOps {
		if strings.HasPrefix(queryUpper, op) {
			return op
		}
	}

	if strings.HasPrefix(queryUpper, "WITH") {
		for _, op := range modifyOps {
			if strings.Contains(queryUpper, op) {
				return op
			}
		}
	}

	return "SELECT"
}

// ExtractDestructiveOpType returns a human-readable description of the
// destructive DDL operation in the query (e.g. "DROP TABLE"). Falls back to
// "<FIRST_WORD> (detected)" when the query matches no known pattern, or
// "UNKNOWN" for an empty query.
func ExtractDestructiveOpType(query string) string {
	upper := strings.ToUpper(query)
	for op, pattern := range destructiveOpPatterns {
		if pattern.MatchString(query) {
			return strings.ReplaceAll(op, "_", " ")
		}
	}
	parts := strings.Fields(upper)
	if len(parts) > 0 {
		return parts[0] + " (detected)"
	}
	return "UNKNOWN"
}

// ExtractDDLTargetObjects extracts the target schema/name/type tuples from a
// DDL query. Returns one DDLTarget per recognised statement (DROP TABLE,
// DROP VIEW, DROP PROCEDURE, DROP FUNCTION, ALTER VIEW, ALTER TABLE,
// TRUNCATE TABLE). The list is empty for non-DDL queries.
//
// Schema defaults to "dbo" when not present. Names are lowercased. Brackets
// and quotes are stripped so consumers don't need to normalise themselves.
func ExtractDDLTargetObjects(query string) []DDLTarget {
	var targets []DDLTarget

	add := func(matches []string, t ObjectType) {
		if len(matches) >= 3 {
			schema, objName := ParseTableRef(matches[1], matches[2])
			targets = append(targets, DDLTarget{Schema: schema, Name: objName, ObjType: t})
		}
	}

	// The patterns are inlined here (rather than promoted to package-level
	// vars) because they are only used in this single function and keeping
	// them local makes the per-statement intent obvious.
	add(regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeTable)
	add(regexp.MustCompile(`(?i)\bDROP\s+VIEW\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeView)
	add(regexp.MustCompile(`(?i)\bDROP\s+PROCEDURE\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeProcedure)
	add(regexp.MustCompile(`(?i)\bDROP\s+FUNCTION\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeFunction)
	add(regexp.MustCompile(`(?i)\bALTER\s+VIEW\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeView)
	add(regexp.MustCompile(`(?i)\bALTER\s+TABLE\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeTable)
	add(regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\s+((?:\[?[\w]+\]?\.)?\[?([\w]+)\]?)`).FindStringSubmatch(query), ObjectTypeTable)

	return targets
}

// ExtractTableRefs finds all table/view references in the query, including
// cross-database qualifiers (DatabaseName.dbo.TableName) AND tables inside
// subqueries. The subquery sweep prevents evasion via nested SELECTs like
//
//	SELECT * FROM (SELECT name FROM sys.objects) AS x
//
// where a restricted system table would otherwise be hidden from a
// top-level scan.
//
// References to the system schemas sys and information_schema are
// returned with the schema prefix preserved in the Table field
// ("sys.objects", "information_schema.columns") so whitelist comparisons
// can match them as restricted.
func ExtractTableRefs(query string) []TableRef {
	queryUpper := strings.ToUpper(query)
	type refKey struct{ db, table string }
	seen := make(map[refKey]bool)
	var refs []TableRef

	// Subquery sweep first — must happen before the top-level pass so
	// nested table references are not lost when the parent regex matches
	// only the outermost FROM.
	for _, t := range extractTablesFromSubqueries(query) {
		key := refKey{t.Database, t.Table}
		if !seen[key] {
			seen[key] = true
			refs = append(refs, t)
		}
	}

	for _, pattern := range tableExtractionPatterns {
		matches := pattern.FindAllStringSubmatch(queryUpper, -1)
		for _, match := range matches {
			if len(match) <= 2 {
				continue
			}
			database, schemaPrefix := ParseTablePrefix(match[1])

			tableName := match[2]
			tableName = strings.Trim(tableName, "[]")
			tableName = strings.ToLower(strings.TrimSpace(tableName))
			if tableName == "" || sqlReservedWords[tableName] {
				continue
			}

			// Re-attach system schema prefixes so the whitelist
			// validator can deny e.g. sys.objects.
			if systemSchemas[schemaPrefix] {
				tableName = schemaPrefix + "." + tableName
			} else if systemSchemas[database] {
				tableName = database + "." + tableName
				database = ""
			}

			key := refKey{database, tableName}
			if !seen[key] {
				seen[key] = true
				refs = append(refs, TableRef{Database: database, Table: tableName})
			}
		}
	}
	return refs
}

// extractTablesFromSubqueries returns the table references that appear
// inside parenthesised SELECT ... FROM ... bodies. Matched as a separate
// pass so the sub-references are guaranteed to be in the result of
// ExtractTableRefs.
func extractTablesFromSubqueries(query string) []TableRef {
	type refKey struct{ db, table string }
	seen := make(map[refKey]bool)
	var refs []TableRef

	queryClean := StripStringLiterals(query)
	matches := subqueryWithFromPattern.FindAllString(queryClean, -1)

	for _, subquery := range matches {
		subqueryUpper := strings.ToUpper(subquery)
		for _, pattern := range tableExtractionPatterns {
			subMatches := pattern.FindAllStringSubmatch(subqueryUpper, -1)
			for _, match := range subMatches {
				if len(match) <= 2 {
					continue
				}
				database, schemaPrefix := ParseTablePrefix(match[1])

				tableName := match[2]
				tableName = strings.Trim(tableName, "[]")
				tableName = strings.ToLower(strings.TrimSpace(tableName))
				if tableName == "" || sqlReservedWords[tableName] {
					continue
				}

				if systemSchemas[schemaPrefix] {
					tableName = schemaPrefix + "." + tableName
				} else if systemSchemas[database] {
					tableName = database + "." + tableName
					database = ""
				}

				key := refKey{database, tableName}
				if !seen[key] {
					seen[key] = true
					refs = append(refs, TableRef{Database: database, Table: tableName})
				}
			}
		}
	}

	return refs
}
