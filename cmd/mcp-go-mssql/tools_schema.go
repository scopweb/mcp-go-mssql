package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// explore and inspect.

// toolExplore handles the explore tool.
func (s *MCPMSSQLServer) toolExplore(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
					},
				},
				IsError: true,
			},
		}
	}

	exploreType := "tables"
	if t, ok := params.Arguments["type"].(string); ok && t != "" {
		exploreType = t
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Optional response budgets. They are applied in Go while the rows are
	// read; the SQL of the catalog is never rewritten.
	limits, limitsErr := parseQueryLimits(params.Arguments)
	if limitsErr != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{Type: "text", Text: fmt.Sprintf("Error in explore: %v", limitsErr)},
				},
				IsError: true,
			},
		}
	}

	var results []map[string]interface{}
	var outcome queryOutcome
	var err error
	var label string

	// run executes a catalog query and keeps the shape the catalog tools
	// have always had: the rows, plus the "_truncated" sentinel when the
	// row cap was reached.
	run := func(query string, args ...interface{}) error {
		outcome, err = s.executeSecureQueryWithLimits(ctx, query, limits, args...)
		if err != nil {
			return err
		}
		results = withTruncationSentinel(outcome.Rows, outcome.RowsTruncated)
		return nil
	}

	switch exploreType {
	case "databases":
		label = "Databases found"
		query := `
			SELECT
				name as database_name,
				database_id,
				create_date,
				state_desc as state
			FROM sys.databases
			WHERE database_id > 4
			ORDER BY name
		`
		err = run(query)

	case "procedures":
		label = "Stored procedures found"
		schemaFilter, _ := params.Arguments["schema"].(string)
		filterVal, _ := params.Arguments["filter"].(string)
		if schemaFilter != "" && filterVal != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE SCHEMA_NAME(p.schema_id) = @p1 AND p.name LIKE @p2
				ORDER BY schema_name, procedure_name
			`
			err = run(query, schemaFilter, "%"+filterVal+"%")
		} else if schemaFilter != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE SCHEMA_NAME(p.schema_id) = @p1
				ORDER BY schema_name, procedure_name
			`
			err = run(query, schemaFilter)
		} else if filterVal != "" {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE p.name LIKE @p1
				ORDER BY schema_name, procedure_name
			`
			err = run(query, "%"+filterVal+"%")
		} else {
			query := `
				SELECT
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				ORDER BY schema_name, procedure_name
			`
			err = run(query)
		}

	case "search":
		pattern, ok := params.Arguments["pattern"].(string)
		if !ok || pattern == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: "Error: 'pattern' is required when type=search"},
					},
					IsError: true,
				},
			}
		}
		searchIn, _ := params.Arguments["search_in"].(string)
		switch searchIn {
		case matchFieldColumn, matchFieldDefinition:
		default:
			searchIn = matchFieldName
		}

		schemaFilter, _ := params.Arguments["schema"].(string)
		tableFilter, _ := params.Arguments["table"].(string)
		limit, _ := params.Arguments["limit"].(float64)

		label = fmt.Sprintf("Objects matching '%s' in %s", pattern, searchIn)
		search, searchErr := s.runSearch(ctx, searchIn, pattern, schemaFilter, tableFilter, int(limit), limits)
		if searchErr != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: fmt.Sprintf("Error in explore: %v", searchErr)},
					},
					IsError: true,
				},
			}
		}

		encoded, encodeErr := json.MarshalIndent(search, "", "  ")
		if encodeErr != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: fmt.Sprintf("Error formatting results: %v", encodeErr)},
					},
					IsError: true,
				},
			}
		}

		text := fmt.Sprintf("%s:\n%s", label, string(encoded))
		if notice := truncationNotice(search.RowsTruncated, search.CellsTruncated, search.ResponseTruncated, 0); notice != "" {
			text += "\n" + notice
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{Type: "text", Text: text},
				},
				StructuredContent: search,
			},
		}

	case "views":
		label = "Views found"
		viewFilter, _ := params.Arguments["filter"].(string)
		if viewFilter != "" {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v WHERE v.TABLE_NAME LIKE @p1 ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			err = run(query, "%"+viewFilter+"%")
		} else {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			err = run(query)
		}

	default: // "tables"
		label = "Tables and views found"
		if filterVal, ok := params.Arguments["filter"].(string); ok && filterVal != "" {
			filterPattern := "%" + filterVal + "%"
			query := `
				SELECT
					TABLE_SCHEMA as schema_name,
					TABLE_NAME as table_name,
					TABLE_TYPE as table_type
				FROM INFORMATION_SCHEMA.TABLES
				WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
				  AND TABLE_NAME LIKE @p1
				ORDER BY TABLE_SCHEMA, TABLE_NAME
			`
			err = run(query, filterPattern)
		} else {
			query := `
				SELECT
					TABLE_SCHEMA as schema_name,
					TABLE_NAME as table_name,
					TABLE_TYPE as table_type
				FROM INFORMATION_SCHEMA.TABLES
				WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
				ORDER BY TABLE_SCHEMA, TABLE_NAME
			`
			err = run(query)
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error in explore: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	resultBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error formatting results: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	envelope := ResultEnvelope{
		Rows:              outcome.Rows,
		RowCount:          len(outcome.Rows),
		Truncated:         outcome.RowsTruncated,
		RowsTruncated:     outcome.RowsTruncated,
		CellsTruncated:    outcome.CellsTruncated,
		ResponseTruncated: outcome.ResponseTruncated,
		TruncatedCells:    outcome.TruncatedCells,
		Limits:            outcome.Limits,
	}

	// The text keeps the shape it has always had, plus an explicit notice
	// when the answer is not complete.
	text := fmt.Sprintf("%s:\n%s", label, string(resultBytes))
	if notice := truncationNotice(outcome.RowsTruncated, outcome.CellsTruncated, outcome.ResponseTruncated, outcome.CellsCut); notice != "" {
		text += "\n" + notice
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: text,
				},
			},
			StructuredContent: envelope,
		},
	}
}

// toolInspect handles the inspect tool.
func (s *MCPMSSQLServer) toolInspect(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
					},
				},
				IsError: true,
			},
		}
	}

	tableName, ok := params.Arguments["table_name"].(string)
	if !ok || tableName == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{Type: "text", Text: "Error: Missing or invalid 'table_name' parameter"},
				},
				IsError: true,
			},
		}
	}

	schemaName := "dbo"
	if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
		schemaName = schema
	}
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		if len(parts) == 2 {
			schemaName = strings.Trim(parts[0], "[]")
			tableName = strings.Trim(parts[1], "[]")
		}
	}
	tableName = strings.Trim(tableName, "[]")

	detail := "columns"
	if d, ok := params.Arguments["detail"].(string); ok && d != "" {
		detail = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	columnsQuery := `
		SELECT
			COLUMN_NAME as column_name,
			DATA_TYPE as data_type,
			IS_NULLABLE as is_nullable,
			COLUMN_DEFAULT as default_value,
			CHARACTER_MAXIMUM_LENGTH as max_length,
			ORDINAL_POSITION as position
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = @p1 AND TABLE_NAME = @p2
		ORDER BY ORDINAL_POSITION
	`
	// One row per indexed column: the columns of an index are grouped in Go
	// (see tools_index_grouping.go) because STRING_AGG only exists from
	// SQL Server 2017 and this project supports 2008 and 2012.
	indexesQuery := `
		SELECT
			i.name as index_name,
			i.type_desc as index_type,
			i.is_unique,
			i.is_primary_key,
			c.name as column_name,
			ic.key_ordinal,
			ic.index_column_id
		FROM sys.indexes i
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		INNER JOIN sys.tables t ON i.object_id = t.object_id
		INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE t.name = @p1 AND s.name = @p2 AND i.name IS NOT NULL
		ORDER BY i.is_primary_key DESC, i.name, ic.key_ordinal, ic.index_column_id
	`
	fkQuery := `
		SELECT
			fk.name as constraint_name,
			OBJECT_SCHEMA_NAME(fk.parent_object_id) as from_schema,
			OBJECT_NAME(fk.parent_object_id) as from_table,
			COL_NAME(fkc.parent_object_id, fkc.parent_column_id) as from_column,
			OBJECT_SCHEMA_NAME(fk.referenced_object_id) as to_schema,
			OBJECT_NAME(fk.referenced_object_id) as to_table,
			COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) as to_column,
			fk.delete_referential_action_desc as on_delete,
			fk.update_referential_action_desc as on_update
		FROM sys.foreign_keys fk
		INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		INNER JOIN sys.tables t ON fk.parent_object_id = t.object_id
		INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE (t.name = @p1 AND s.name = @p2)
		   OR (OBJECT_NAME(fk.referenced_object_id) = @p1 AND OBJECT_SCHEMA_NAME(fk.referenced_object_id) = @p2)
		ORDER BY fk.name
	`

	limits, limitsErr := parseQueryLimits(params.Arguments)
	if limitsErr != nil {
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error in inspect: %v", limitsErr)}}, IsError: true,
		}}
	}

	if detail == "all" {
		// Every section is read with the same budgets, and the facts of the
		// truncation are accumulated for the structured result.
		all := InspectAllResult{}
		read := func(query string, args ...interface{}) ([]map[string]interface{}, error) {
			outcome, err := s.executeSecureQueryWithLimits(ctx, query, limits, args...)
			if err != nil {
				return nil, err
			}
			all.RowsTruncated = all.RowsTruncated || outcome.RowsTruncated
			all.CellsTruncated = all.CellsTruncated || outcome.CellsTruncated
			all.ResponseTruncated = all.ResponseTruncated || outcome.ResponseTruncated
			all.CellsCut += outcome.CellsCut
			all.TruncatedCells = append(all.TruncatedCells, outcome.TruncatedCells...)
			all.Limits = outcome.Limits
			return outcome.Rows, nil
		}

		colResults, err := read(columnsQuery, schemaName, tableName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting columns: %v", err)}}, IsError: true,
			}}
		}
		idxResults, err := read(indexesQuery, tableName, schemaName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting indexes: %v", err)}}, IsError: true,
			}}
		}
		idxResults = groupIndexRows(idxResults)
		fkResults, err := read(fkQuery, tableName, schemaName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting foreign keys: %v", err)}}, IsError: true,
			}}
		}
		depsAllQuery := `
			SELECT
				SCHEMA_NAME(o.schema_id)  AS referencing_schema,
				o.name                    AS referencing_object,
				o.type_desc               AS referencing_type,
				sed.is_caller_dependent,
				sed.is_ambiguous
			FROM sys.sql_expression_dependencies sed
			JOIN sys.objects o ON o.object_id = sed.referencing_id
			WHERE sed.referenced_entity_name = @p1
			  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
			ORDER BY o.type_desc, referencing_schema, referencing_object
		`
		depsResults, _ := read(depsAllQuery, tableName, schemaName) // #nosec G104 - dependencies query is optional, errors handled gracefully

		all.Columns = colResults
		all.Indexes = idxResults
		all.ForeignKeys = fkResults
		all.Dependencies = depsResults
		all.Truncated = all.RowsTruncated
		all.RowCount = len(colResults) + len(idxResults) + len(fkResults) + len(depsResults)

		combined := map[string]interface{}{
			"columns":      colResults,
			"indexes":      idxResults,
			"foreign_keys": fkResults,
			"dependencies": depsResults,
		}
		resultBytes, err := json.MarshalIndent(combined, "", "  ")
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error formatting results: %v", err)}}, IsError: true,
			}}
		}

		text := fmt.Sprintf("Full inspection of '%s.%s':\n%s", schemaName, tableName, string(resultBytes))
		if notice := truncationNotice(all.RowsTruncated, all.CellsTruncated, all.ResponseTruncated, all.CellsCut); notice != "" {
			text += "\n" + notice
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: text,
					},
				},
				StructuredContent: all,
			},
		}
	}

	var results []map[string]interface{}
	var outcome queryOutcome
	var err error
	var label string

	// run executes a catalog query and keeps the shape the catalog tools
	// have always had: the rows, plus the "_truncated" sentinel when the
	// row cap was reached.
	run := func(query string, args ...interface{}) error {
		outcome, err = s.executeSecureQueryWithLimits(ctx, query, limits, args...)
		if err != nil {
			return err
		}
		results = withTruncationSentinel(outcome.Rows, outcome.RowsTruncated)
		return nil
	}

	switch detail {
	case "indexes":
		label = fmt.Sprintf("Indexes for '%s.%s'", schemaName, tableName)
		err = run(indexesQuery, tableName, schemaName)
		if err == nil {
			// The grouped rows are the ones the caller receives, in the
			// text and in the structured result.
			grouped := groupIndexRows(outcome.Rows)
			outcome.Rows = grouped
			results = withTruncationSentinel(grouped, outcome.RowsTruncated)
		}
	case "foreign_keys":
		label = fmt.Sprintf("Foreign keys for '%s.%s'", schemaName, tableName)
		err = run(fkQuery, tableName, schemaName)
	case "dependencies":
		label = fmt.Sprintf("Objects that depend on '%s.%s'", schemaName, tableName)
		depsQuery := `
			SELECT
				SCHEMA_NAME(o.schema_id)  AS referencing_schema,
				o.name                    AS referencing_object,
				o.type_desc               AS referencing_type,
				sed.is_caller_dependent,
				sed.is_ambiguous
			FROM sys.sql_expression_dependencies sed
			JOIN sys.objects o ON o.object_id = sed.referencing_id
			WHERE sed.referenced_entity_name = @p1
			  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
			ORDER BY o.type_desc, referencing_schema, referencing_object
		`
		err = run(depsQuery, tableName, schemaName)
	default: // "columns"
		label = fmt.Sprintf("Table structure for '%s'", tableName)
		err = run(columnsQuery, schemaName, tableName)
		if err == nil && len(results) == 0 {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: fmt.Sprintf("Table '%s' not found", tableName)},
					},
					IsError: true,
				},
			}
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error in inspect: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	resultBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error formatting results: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	envelope := ResultEnvelope{
		Rows:              outcome.Rows,
		RowCount:          len(outcome.Rows),
		Truncated:         outcome.RowsTruncated,
		RowsTruncated:     outcome.RowsTruncated,
		CellsTruncated:    outcome.CellsTruncated,
		ResponseTruncated: outcome.ResponseTruncated,
		TruncatedCells:    outcome.TruncatedCells,
		Limits:            outcome.Limits,
	}

	// The text keeps the shape it has always had, plus an explicit notice
	// when the answer is not complete.
	text := fmt.Sprintf("%s:\n%s", label, string(resultBytes))
	if notice := truncationNotice(outcome.RowsTruncated, outcome.CellsTruncated, outcome.ResponseTruncated, outcome.CellsCut); notice != "" {
		text += "\n" + notice
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: text,
				},
			},
			StructuredContent: envelope,
		},
	}
}
