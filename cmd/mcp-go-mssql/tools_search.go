package main

// Unified search for the explore tool.
//
// The three search modes (object name, object definition, column name) used
// to answer with three different shapes: an array of rows for the name and
// definition modes, an object for the column mode, and none of them said why
// an object matched or what to do next.
//
// They now share one envelope: qualified objects, the reason of the match,
// the direct foreign keys of the matched tables and the next step to take.
//
// The ranking is deterministic: an exact name first, then a prefix, then a
// substring, and the name as the tie break. The kind of match is computed in
// SQL so the ranking happens before the limit.
//
// The queries only read catalog views of SQL Server 2005 and later, and the
// foreign keys are assembled in Go, so no 2017 only aggregate is needed.

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Limits of a search.
const (
	// defaultSearchLimit is the number of objects a search returns when the
	// caller does not ask for a limit.
	defaultSearchLimit = 50
	// maxSearchLimit is the hard cap of the number of objects returned.
	maxSearchLimit = 200
	// maxForeignKeyObjects is the number of matched tables whose foreign
	// keys are resolved. The rest are reported without them.
	maxForeignKeyObjects = 30
	// maxForeignKeysPerObject is the cap of foreign keys per object.
	maxForeignKeysPerObject = 10
	// maxNextSteps is the number of suggested next steps.
	maxNextSteps = 5
)

// The kinds of match, from the most to the least specific.
const (
	matchExact    = "exact"
	matchPrefix   = "prefix"
	matchContains = "contains"
)

// The fields a search can match on.
const (
	matchFieldName       = "name"
	matchFieldDefinition = "definition"
	matchFieldColumn     = "column"
)

// searchColumn is a column of a matched table.
type searchColumn struct {
	Name      string `json:"name"`
	DataType  string `json:"data_type"`
	MaxLength *int   `json:"max_length,omitempty"`
}

// searchForeignKey is a direct foreign key of a matched table.
type searchForeignKey struct {
	FromColumns []string `json:"from_columns"`
	ToTable     string   `json:"to_table"`
	ToColumns   []string `json:"to_columns"`
}

// searchMatchKind explains why an object matched.
type searchMatchKind struct {
	// Field is the field that matched: name, definition or column.
	Field string `json:"field"`
	// Kind is how it matched: exact, prefix or contains.
	Kind string `json:"kind"`
	// Term is the term that was searched.
	Term string `json:"term"`
}

// searchMatch is one object that matched.
type searchMatch struct {
	ObjectType    string             `json:"object_type"`
	Schema        string             `json:"schema"`
	Name          string             `json:"name"`
	QualifiedName string             `json:"qualified_name"`
	Match         searchMatchKind    `json:"match"`
	Columns       []searchColumn     `json:"columns,omitempty"`
	ForeignKeys   []searchForeignKey `json:"foreign_keys,omitempty"`
	Created       string             `json:"created,omitempty"`
	Modified      string             `json:"modified,omitempty"`
	Definition    string             `json:"definition_snippet,omitempty"`
}

// searchNextStep is the suggested next call.
type searchNextStep struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
	Reason    string                 `json:"reason"`
}

// searchLimits reports the limits that were applied to a search.
type searchLimits struct {
	// MaxObjects is the maximum number of objects returned.
	MaxObjects int `json:"max_objects"`
	// MaxForeignKeyObjects is the number of matched tables whose foreign
	// keys were resolved.
	MaxForeignKeyObjects int `json:"max_foreign_key_objects"`
	// MaxForeignKeysPerObject is the cap of foreign keys per object.
	MaxForeignKeysPerObject int `json:"max_foreign_keys_per_object"`
	// MaxCellBytes and MaxResponseBytes are the optional budgets of the
	// call, or null when there is none.
	MaxCellBytes     *int `json:"max_cell_bytes"`
	MaxResponseBytes *int `json:"max_response_bytes"`
}

// searchResult is the answer of a search.
type searchResult struct {
	Matches  []searchMatch `json:"matches"`
	Returned int           `json:"returned"`
	// Truncated is true when more objects matched than the limit returned.
	Truncated bool `json:"truncated"`
	// RowsTruncated is the explicit name of Truncated.
	RowsTruncated bool `json:"rows_truncated"`
	// CellsTruncated is true when a cell was shortened.
	CellsTruncated bool `json:"cells_truncated"`
	// ResponseTruncated is true when the response budget stopped the result.
	ResponseTruncated bool `json:"response_truncated"`
	// Limits reports the limits that were applied.
	Limits searchLimits `json:"limits"`
	// NextSteps are the suggested calls to continue the task.
	NextSteps []searchNextStep `json:"next_steps"`
}

// clampSearchLimit applies the default and the hard cap of the limit.
func clampSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

// objectTypeFilter selects the object types a search looks at.
//
// It cannot use o.type: in SQL Server 2012 to 2019 the column is a char(2)
// code ('U ', 'V ', 'P ') and in 2022 it is a tinyint, so comparing it with
// text either fails with error 245 or behaves differently per version.
// The type_desc is a friendly name in 2012 and later, and a char(2) code
// before that, so both spellings are accepted and the codes are matched with
// their padded form.
const objectTypeFilter = `(
			o.type_desc IN ('U', 'V', 'P', 'FN', 'IF', 'TF')
			OR o.type_desc IN (
				'USER_TABLE', 'VIEW', 'SQL_STORED_PROCEDURE',
				'SQL_SCALAR_FUNCTION', 'SQL_INLINE_TABLE_VALUED_FUNCTION',
				'SQL_TABLE_VALUED_FUNCTION'
			)
		)`

// friendlyObjectType maps the type of an object to a stable name.
//
// The catalog reports the code ('U', 'P ') in the older versions and the
// friendly name ('USER_TABLE', 'SQL_STORED_PROCEDURE') in the newer ones.
func friendlyObjectType(typeDesc string) string {
	switch strings.ToUpper(strings.TrimSpace(typeDesc)) {
	case "U", "USER_TABLE":
		return "TABLE"
	case "V", "VIEW":
		return "VIEW"
	case "P", "SQL_STORED_PROCEDURE":
		return "PROCEDURE"
	case "FN", "SQL_SCALAR_FUNCTION":
		return "FUNCTION"
	case "IF", "SQL_INLINE_TABLE_VALUED_FUNCTION":
		return "FUNCTION"
	case "TF", "SQL_TABLE_VALUED_FUNCTION":
		return "FUNCTION"
	default:
		return strings.TrimSpace(typeDesc)
	}
}

// buildObjectSearchQuery builds the query that searches objects by name.
//
// The kind of match is computed in the query, so the ranking happens before
// the limit and the result is deterministic: exact, then prefix, then
// substring, and the name as the tie break.
func buildObjectSearchQuery(pattern string, limit int) (string, []interface{}) {
	escaped := escapeLikePattern(pattern)
	args := []interface{}{
		limit + 1,
		pattern,
		escaped + "%",
		"%" + escaped + "%",
	}

	query := fmt.Sprintf(`SELECT TOP (@p1)
			o.type_desc      AS object_type,
			SCHEMA_NAME(o.schema_id) AS schema_name,
			o.name           AS object_name,
			o.create_date    AS created,
			o.modify_date    AS modified,
			CASE
				WHEN o.name = @p2 THEN '%s'
				WHEN o.name LIKE @p3 ESCAPE '\' THEN '%s'
				ELSE '%s'
			END AS match_kind
		FROM sys.objects o
		WHERE o.name LIKE @p4 ESCAPE '\'
		  AND %s
		ORDER BY CASE
				WHEN o.name = @p2 THEN 0
				WHEN o.name LIKE @p3 ESCAPE '\' THEN 1
				ELSE 2
			END, o.name`,
		matchExact, matchPrefix, matchContains, objectTypeFilter)

	return query, args
}

// buildDefinitionSearchQuery builds the query that searches the source of
// views, procedures and functions.
func buildDefinitionSearchQuery(pattern string, limit int) (string, []interface{}) {
	escaped := escapeLikePattern(pattern)
	args := []interface{}{
		limit + 1,
		"%" + escaped + "%",
	}

	query := fmt.Sprintf(`SELECT TOP (@p1)
			o.type_desc      AS object_type,
			SCHEMA_NAME(o.schema_id) AS schema_name,
			o.name           AS object_name,
			m.definition     AS definition_snippet
		FROM sys.sql_modules m
		JOIN sys.objects     o ON o.object_id = m.object_id
		WHERE m.definition LIKE @p2 ESCAPE '\'
		ORDER BY o.type_desc, o.name`)

	return query, args
}

// buildForeignKeysQuery builds the query that resolves the direct foreign
// keys of the given tables.
//
// The tables come from the catalog, and they are still passed as parameters.
func buildForeignKeysQuery(schema string, tables []string) (string, []interface{}) {
	if len(tables) == 0 {
		return "", nil
	}

	placeholders := make([]string, 0, len(tables))
	args := []interface{}{schema}
	for _, table := range tables {
		args = append(args, table)
		placeholders = append(placeholders, fmt.Sprintf("@p%d", len(args)))
	}

	query := fmt.Sprintf(`SELECT
			OBJECT_SCHEMA_NAME(fk.parent_object_id) AS from_schema,
			OBJECT_NAME(fk.parent_object_id) AS from_table,
			COL_NAME(fkc.parent_object_id, fkc.parent_column_id) AS from_column,
			OBJECT_SCHEMA_NAME(fk.referenced_object_id) AS to_schema,
			OBJECT_NAME(fk.referenced_object_id) AS to_table,
			COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) AS to_column
		FROM sys.foreign_keys fk
		INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		WHERE OBJECT_SCHEMA_NAME(fk.parent_object_id) = @p1
		  AND OBJECT_NAME(fk.parent_object_id) IN (%s)
		ORDER BY from_table, to_table, fkc.constraint_column_id`,
		strings.Join(placeholders, ", "))

	return query, args
}

// groupForeignKeys turns the per column rows of the catalog into one entry
// per foreign key, keeping the columns of a composite key together.
func groupForeignKeys(rows []map[string]interface{}) []searchForeignKey {
	type key struct {
		from string
		to   string
	}
	var order []key
	grouped := make(map[key]*searchForeignKey, len(rows))

	for _, row := range rows {
		fromTable := str(row["from_table"])
		toSchema := str(row["to_schema"])
		toTable := str(row["to_table"])

		identifier := key{from: fromTable, to: qualified(toSchema, toTable)}
		entry, ok := grouped[identifier]
		if !ok {
			entry = &searchForeignKey{ToTable: identifier.to}
			grouped[identifier] = entry
			order = append(order, identifier)
		}
		entry.FromColumns = append(entry.FromColumns, str(row["from_column"]))
		entry.ToColumns = append(entry.ToColumns, str(row["to_column"]))
	}

	keys := make([]searchForeignKey, 0, len(order))
	for _, identifier := range order {
		keys = append(keys, *grouped[identifier])
	}
	return keys
}

// searchForeignKeysByTable resolves the direct foreign keys of the given
// tables, grouped by table.
func (s *MCPMSSQLServer) searchForeignKeysByTable(ctx context.Context, schema string, tables []string, limits queryLimits) (map[string][]searchForeignKey, error) {
	result := make(map[string][]searchForeignKey)
	if len(tables) == 0 {
		return result, nil
	}

	query, args := buildForeignKeysQuery(schema, tables)
	outcome, err := s.executeSecureQueryWithLimits(ctx, query, limits, args...)
	if err != nil {
		return result, err
	}

	// The rows are grouped per table, so the columns of a composite key
	// stay together.
	rowsByTable := make(map[string][]map[string]interface{})
	for _, row := range outcome.Rows {
		fromTable := str(row["from_table"])
		rowsByTable[fromTable] = append(rowsByTable[fromTable], row)
	}
	for table, rows := range rowsByTable {
		result[table] = groupForeignKeys(rows)
	}
	return result, nil
}

// buildNextSteps suggests the calls that continue the task.
func buildNextSteps(matches []searchMatch) []searchNextStep {
	steps := make([]searchNextStep, 0, maxNextSteps)
	for _, match := range matches {
		if len(steps) >= maxNextSteps {
			break
		}
		if match.ObjectType != "TABLE" && match.ObjectType != "VIEW" {
			continue
		}
		steps = append(steps, searchNextStep{
			Tool: "inspect",
			Arguments: map[string]interface{}{
				"table_name": match.QualifiedName,
				"detail":     "all",
			},
			Reason: fmt.Sprintf("read the full structure of %s", match.QualifiedName),
		})
	}
	return steps
}

// sortMatches keeps a deterministic order even when the query could not
// guarantee one.
func sortMatches(matches []searchMatch) {
	order := map[string]int{matchExact: 0, matchPrefix: 1, matchContains: 2}
	sort.SliceStable(matches, func(i, j int) bool {
		left, right := order[matches[i].Match.Kind], order[matches[j].Match.Kind]
		if left != right {
			return left < right
		}
		if matches[i].QualifiedName != matches[j].QualifiedName {
			return matches[i].QualifiedName < matches[j].QualifiedName
		}
		return matches[i].ObjectType < matches[j].ObjectType
	})
}

// str returns the value as a string, or an empty string.
func str(value interface{}) string {
	text, _ := value.(string)
	return text
}

// qualified returns the qualified name of an object.
func qualified(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

// groupColumnsByTable turns the per column rows of the catalog into one
// match per table, with the columns that matched.
func groupColumnsByTable(rows []map[string]interface{}, pattern string) []searchMatch {
	var order []string
	grouped := make(map[string]*searchMatch)

	for _, row := range rows {
		schema := str(row["schema_name"])
		table := str(row["table_name"])
		name := qualified(schema, table)

		match, ok := grouped[name]
		if !ok {
			match = &searchMatch{
				ObjectType:    "TABLE",
				Schema:        schema,
				Name:          table,
				QualifiedName: name,
				Match:         searchMatchKind{Field: matchFieldColumn, Kind: matchContains, Term: pattern},
			}
			grouped[name] = match
			order = append(order, name)
		}

		column := searchColumn{Name: str(row["column_name"]), DataType: str(row["data_type"])}
		switch length := row["max_length"].(type) {
		case int64:
			value := int(length)
			column.MaxLength = &value
		case int:
			value := length
			column.MaxLength = &value
		}
		match.Columns = append(match.Columns, column)
	}

	matches := make([]searchMatch, 0, len(order))
	for _, name := range order {
		matches = append(matches, *grouped[name])
	}
	return matches
}

// runSearch answers a search of the explore tool.
//
// The three modes share one envelope: the matched objects with their reason,
// the direct foreign keys of the matched tables and the next step to take.
func (s *MCPMSSQLServer) runSearch(ctx context.Context, searchIn, pattern, schemaFilter, tableFilter string, limit int, limits queryLimits) (searchResult, error) {
	applied := limits.appliedLimits()
	result := searchResult{
		Limits: searchLimits{
			MaxObjects:              clampSearchLimit(limit),
			MaxForeignKeyObjects:    maxForeignKeyObjects,
			MaxForeignKeysPerObject: maxForeignKeysPerObject,
			MaxCellBytes:            applied.MaxCellBytes,
			MaxResponseBytes:        applied.MaxResponseBytes,
		},
	}
	maxObjects := result.Limits.MaxObjects

	var matches []searchMatch
	var outcome queryOutcome
	var err error

	switch searchIn {
	case matchFieldDefinition:
		var query string
		var args []interface{}
		query, args = buildDefinitionSearchQuery(pattern, maxObjects)
		outcome, err = s.executeSecureQueryWithLimits(ctx, query, limits, args...)
		if err != nil {
			return result, err
		}
		for _, row := range outcome.Rows {
			schema := str(row["schema_name"])
			name := str(row["object_name"])
			matches = append(matches, searchMatch{
				ObjectType:    friendlyObjectType(str(row["object_type"])),
				Schema:        schema,
				Name:          name,
				QualifiedName: qualified(schema, name),
				Match:         searchMatchKind{Field: matchFieldDefinition, Kind: matchContains, Term: pattern},
				Definition:    str(row["definition_snippet"]),
			})
		}

	case matchFieldColumn:
		var query string
		var args []interface{}
		query, args = buildColumnSearchQuery(schemaFilter, tableFilter, pattern, maxObjects)
		outcome, err = s.executeSecureQueryWithLimits(ctx, query, limits, args...)
		if err != nil {
			return result, err
		}
		matches = groupColumnsByTable(outcome.Rows, pattern)

	default: // matchFieldName
		var query string
		var args []interface{}
		query, args = buildObjectSearchQuery(pattern, maxObjects)
		outcome, err = s.executeSecureQueryWithLimits(ctx, query, limits, args...)
		if err != nil {
			return result, err
		}
		for _, row := range outcome.Rows {
			schema := str(row["schema_name"])
			name := str(row["object_name"])
			kind := str(row["match_kind"])
			if kind == "" {
				kind = matchContains
			}
			matches = append(matches, searchMatch{
				ObjectType:    friendlyObjectType(str(row["object_type"])),
				Schema:        schema,
				Name:          name,
				QualifiedName: qualified(schema, name),
				Match:         searchMatchKind{Field: matchFieldName, Kind: kind, Term: pattern},
				Created:       str(row["created"]),
				Modified:      str(row["modified"]),
			})
		}
	}

	result.CellsTruncated = outcome.CellsTruncated
	result.ResponseTruncated = outcome.ResponseTruncated

	// The query asked for one row more than the limit: that extra row is
	// the signal that the result is partial.
	if len(matches) > maxObjects {
		result.Truncated = true
		matches = matches[:maxObjects]
	}
	result.RowsTruncated = result.Truncated

	sortMatches(matches)

	// The direct foreign keys of the matched tables, resolved per schema.
	tablesBySchema := make(map[string][]string)
	for _, match := range matches {
		if match.ObjectType != "TABLE" {
			continue
		}
		if len(tablesBySchema[match.Schema]) >= maxForeignKeyObjects {
			continue
		}
		tablesBySchema[match.Schema] = append(tablesBySchema[match.Schema], match.Name)
	}

	schemas := make([]string, 0, len(tablesBySchema))
	for schema := range tablesBySchema {
		schemas = append(schemas, schema)
	}
	sort.Strings(schemas)

	foreignKeys := make(map[string][]searchForeignKey)
	for _, schema := range schemas {
		keys, err := s.searchForeignKeysByTable(ctx, schema, tablesBySchema[schema], limits)
		if err != nil {
			return result, err
		}
		for table, entries := range keys {
			foreignKeys[qualified(schema, table)] = entries
		}
	}

	for i := range matches {
		entries, ok := foreignKeys[matches[i].QualifiedName]
		if !ok {
			continue
		}
		if len(entries) > maxForeignKeysPerObject {
			entries = entries[:maxForeignKeysPerObject]
		}
		matches[i].ForeignKeys = entries
	}

	result.Matches = matches
	result.Returned = len(matches)
	result.NextSteps = buildNextSteps(matches)
	return result, nil
}
