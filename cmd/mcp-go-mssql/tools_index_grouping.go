package main

// Index aggregation without STRING_AGG.
//
// STRING_AGG only exists from SQL Server 2017, and this project supports
// SQL Server 2008 and 2012, where "inspect detail=indexes" failed with a
// syntax error. The columns of an index are therefore grouped in Go instead
// of in the server.
//
// The output keeps the shape the aggregated query produced: one row per index
// with its columns joined by ", ".
//
// The trade-off is more rows on the wire: the server returns one row per
// indexed column and the grouping happens here. For a single table that is a
// handful of rows, and it keeps the response free of XML entity escaping
// (FOR XML PATH would mangle names containing & or <).

import (
	"sort"
	"strings"
)

// indexColumn is one column of an index, with the order the catalog reports.
type indexColumn struct {
	name     string
	ordinal  int
	columnID int
}

// groupIndexRows turns the per-column rows of the catalog into one row per
// index.
//
// The rows arrive ordered by index (primary keys first, then by name), so the
// order of the indexes is preserved. The columns of each index are ordered by
// their key ordinal, then by their position in the index, which reproduces the
// order of the aggregated query and breaks ties deterministically.
//
// A truncation sentinel in the rows is kept and moved to the end of the
// result, so a partially listed set of indexes stays visible as such.
func groupIndexRows(rows []map[string]interface{}) []map[string]interface{} {
	type indexGroup struct {
		meta    map[string]interface{}
		columns []indexColumn
	}

	var order []string
	groups := make(map[string]*indexGroup)
	var sentinel map[string]interface{}

	for _, row := range rows {
		if truncated, ok := row["_truncated"]; ok {
			sentinel = map[string]interface{}{"_truncated": truncated}
			continue
		}

		name, _ := row["index_name"].(string)
		group, ok := groups[name]
		if !ok {
			group = &indexGroup{meta: map[string]interface{}{
				"index_name":     name,
				"index_type":     row["index_type"],
				"is_unique":      row["is_unique"],
				"is_primary_key": row["is_primary_key"],
			}}
			groups[name] = group
			order = append(order, name)
		}

		column, _ := row["column_name"].(string)
		group.columns = append(group.columns, indexColumn{
			name:     column,
			ordinal:  valueToInt(row["key_ordinal"]),
			columnID: valueToInt(row["index_column_id"]),
		})
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, name := range order {
		group := groups[name]

		sort.SliceStable(group.columns, func(i, j int) bool {
			if group.columns[i].ordinal != group.columns[j].ordinal {
				return group.columns[i].ordinal < group.columns[j].ordinal
			}
			return group.columns[i].columnID < group.columns[j].columnID
		})

		columns := make([]string, 0, len(group.columns))
		for _, column := range group.columns {
			columns = append(columns, column.name)
		}

		row := make(map[string]interface{}, len(group.meta)+1)
		for key, value := range group.meta {
			row[key] = value
		}
		row["columns"] = strings.Join(columns, ", ")
		out = append(out, row)
	}

	if sentinel != nil {
		out = append(out, sentinel)
	}
	return out
}

// valueToInt converts a catalog value to an int, whatever the driver
// returned for it.
func valueToInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}
