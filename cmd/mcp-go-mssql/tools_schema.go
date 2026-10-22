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

	var results []map[string]interface{}
	var err error
	var label string

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
		results, _, err = s.executeSecureQuery(ctx, query)

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
			results, _, err = s.executeSecureQuery(ctx, query, schemaFilter, "%"+filterVal+"%")
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
			results, _, err = s.executeSecureQuery(ctx, query, schemaFilter)
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
			results, _, err = s.executeSecureQuery(ctx, query, "%"+filterVal+"%")
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
			results, _, err = s.executeSecureQuery(ctx, query)
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
		likePattern := "%" + pattern + "%"
		if searchIn == "column" {
			// Column discovery: which table owns this column.
			schemaFilter, _ := params.Arguments["schema"].(string)
			tableFilter, _ := params.Arguments["table"].(string)
			limit, _ := params.Arguments["limit"].(float64)

			label = fmt.Sprintf("Columns matching '%s'", pattern)
			search, searchErr := s.searchColumns(ctx, schemaFilter, tableFilter, pattern, int(limit))
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

			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: fmt.Sprintf("%s:\n%s", label, string(encoded))},
					},
				},
			}
		}
		if searchIn == "definition" {
			label = fmt.Sprintf("Objects matching '%s' in definition", pattern)
			query := `
				SELECT
					o.type_desc      AS object_type,
					SCHEMA_NAME(o.schema_id) AS schema_name,
					o.name           AS object_name,
					m.definition     AS definition_snippet
				FROM sys.sql_modules m
				JOIN sys.objects     o ON o.object_id = m.object_id
				WHERE m.definition LIKE @p1
				ORDER BY o.type_desc, o.name
			`
			results, _, err = s.executeSecureQuery(ctx, query, likePattern)
		} else {
			label = fmt.Sprintf("Objects matching '%s' in name", pattern)
			query := `
				SELECT
					o.type_desc      AS object_type,
					SCHEMA_NAME(o.schema_id) AS schema_name,
					o.name           AS object_name,
					o.create_date    AS created,
					o.modify_date    AS modified
				FROM sys.objects o
				WHERE o.name LIKE @p1
				  AND o.type IN ('U','V','P','FN','IF','TF')
				ORDER BY o.type_desc, o.name
			`
			results, _, err = s.executeSecureQuery(ctx, query, likePattern)
		}

	case "views":
		label = "Views found"
		viewFilter, _ := params.Arguments["filter"].(string)
		if viewFilter != "" {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v WHERE v.TABLE_NAME LIKE @p1 ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			results, _, err = s.executeSecureQuery(ctx, query, "%"+viewFilter+"%")
		} else {
			query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
			results, _, err = s.executeSecureQuery(ctx, query)
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
			results, _, err = s.executeSecureQuery(ctx, query, filterPattern)
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
			results, _, err = s.executeSecureQuery(ctx, query)
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

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
				},
			},
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

	if detail == "all" {
		colResults, _, err := s.executeSecureQuery(ctx, columnsQuery, schemaName, tableName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting columns: %v", err)}}, IsError: true,
			}}
		}
		idxResults, _, err := s.executeSecureQuery(ctx, indexesQuery, tableName, schemaName)
		if err != nil {
			return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting indexes: %v", err)}}, IsError: true,
			}}
		}
		idxResults = groupIndexRows(idxResults)
		fkResults, _, err := s.executeSecureQuery(ctx, fkQuery, tableName, schemaName)
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
		depsResults, _, _ := s.executeSecureQuery(ctx, depsAllQuery, tableName, schemaName) // #nosec G104 - dependencies query is optional, errors handled gracefully
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
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Full inspection of '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
					},
				},
			},
		}
	}

	var results []map[string]interface{}
	var err error
	var label string

	switch detail {
	case "indexes":
		label = fmt.Sprintf("Indexes for '%s.%s'", schemaName, tableName)
		results, _, err = s.executeSecureQuery(ctx, indexesQuery, tableName, schemaName)
		if err == nil {
			results = groupIndexRows(results)
		}
	case "foreign_keys":
		label = fmt.Sprintf("Foreign keys for '%s.%s'", schemaName, tableName)
		results, _, err = s.executeSecureQuery(ctx, fkQuery, tableName, schemaName)
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
		results, _, err = s.executeSecureQuery(ctx, depsQuery, tableName, schemaName)
	default: // "columns"
		label = fmt.Sprintf("Table structure for '%s'", tableName)
		results, _, err = s.executeSecureQuery(ctx, columnsQuery, schemaName, tableName)
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

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
				},
			},
		},
	}
}
