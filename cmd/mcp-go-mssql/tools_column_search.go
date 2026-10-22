package main

// Column discovery: find the tables that own a column.
//
// explore already searches object names and object definitions. It could not
// answer the most common discovery question an agent has: "which table has
// the column ClienteId?". Answering it meant listing the catalog and calling
// inspect once per table.
//
// The query is built by a pure function so it can be tested without a
// database, and it is always parameterized: the search term, the optional
// schema and table filters and the row limit never reach the query as text.
//
// The column search is one of the modes of the unified search of the explore
// tool (runSearch), which groups the columns by table and adds the foreign
// keys and the next step. This file keeps the query that finds the columns
// and the escaping rules it shares with the search by name.

import (
	"fmt"
	"strings"
)

// escapeLikePattern escapes the LIKE metacharacters of a literal substring.
//
// The pattern is matched literally: an agent that searches for "100%" finds
// the column "100%" and not every column ending in "100". The escape
// character itself is escaped first.
func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
		`[`, `\[`,
	)
	return replacer.Replace(value)
}

// buildColumnSearchQuery builds the parameterized query that finds columns.
//
// The limit is passed as a parameter and the query asks for one row more than
// the limit, so the caller can tell "exactly the limit" from "there are
// more" without running a second COUNT query over the catalog.
//
// The optional schema and table filters change the WHERE clause and the
// argument list, so the filters stay sargable instead of becoming an
// "IS NULL OR ..." predicate that would force a full scan of the catalog.
func buildColumnSearchQuery(schema, table, pattern string, limit int) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// @p1 is the limit (+1 to detect more results).
	args = append(args, limit+1)
	conditions = append(conditions, "c.COLUMN_NAME LIKE @p2 ESCAPE '\\'")
	args = append(args, "%"+escapeLikePattern(pattern)+"%")

	if schema != "" {
		args = append(args, schema)
		conditions = append(conditions, fmt.Sprintf("c.TABLE_SCHEMA = @p%d", len(args)))
	}
	if table != "" {
		args = append(args, "%"+escapeLikePattern(table)+"%")
		conditions = append(conditions, fmt.Sprintf("c.TABLE_NAME LIKE @p%d ESCAPE '\\'", len(args)))
	}

	query := fmt.Sprintf(`SELECT TOP (@p1)
			c.TABLE_SCHEMA AS schema_name,
			c.TABLE_NAME AS table_name,
			c.COLUMN_NAME AS column_name,
			c.DATA_TYPE AS data_type,
			c.CHARACTER_MAXIMUM_LENGTH AS max_length,
			c.IS_NULLABLE AS is_nullable
		FROM INFORMATION_SCHEMA.COLUMNS c
		WHERE %s
		ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`,
		strings.Join(conditions, " AND "))

	return query, args
}
