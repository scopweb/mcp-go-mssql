package main

// tools/list catalog. The set of tools depends on classic vs dynamic mode.

const cellBudgetDescription = "Cell size budget in bytes (16..1048576; default and server ceiling 1048576). Longer values are shortened and marked. The SQL is never rewritten."
const responseBudgetDescription = "Approximate result budget in bytes (512..8388608; default and server ceiling 8388608). The result stops there and is marked as truncated."

// listTools returns the tools/list catalog for the active server mode.
func (s *MCPMSSQLServer) listTools(id interface{}) *MCPResponse {
	tools := []Tool{
		{
			Name:        "query_database",
			Title:       "Query Database",
			Description: "Execute a SQL query subject to the active database policy",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "SQL query to execute. The server validates the active read-only, whitelist and approval policy.",
					},
					"max_cell_bytes": {
						Type:        "integer",
						Description: cellBudgetDescription,
					},
					"max_response_bytes": {
						Type:        "integer",
						Description: responseBudgetDescription,
					},
					"format": {
						Type:        "string",
						Description: "How the rows are rendered: 'map' (default, one object per row) or 'columnar' (the column names once and every row an array of values in that order).",
					},
				},
				Required: []string{"query"},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			Name:        "get_database_info",
			Title:       "Get Database Info",
			Description: "Get database connection status and basic information",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			Name:        "explore",
			Title:       "Explore Database",
			Description: "Explore database objects. type=tables (default) lists tables/views, type=views lists views with metadata (check_option, is_updatable, definition preview), type=databases lists all databases, type=procedures lists stored procedures, type=search searches objects by name, by source definition (requires pattern) or by column name (requires pattern).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"type": {
						Type:        "string",
						Description: "What to explore: 'tables' (default), 'views', 'databases', 'procedures', 'search'",
					},
					"filter": {
						Type:        "string",
						Description: "Name filter for tables/procedures (LIKE match, e.g. 'Pedido')",
					},
					"schema": {
						Type:        "string",
						Description: "Schema filter for procedures, and for a column search (exact schema name)",
					},
					"table": {
						Type:        "string",
						Description: "Table filter for a column search (literal substring match)",
					},
					"pattern": {
						Type:        "string",
						Description: "Search term. Required when type=search.",
					},
					"search_in": {
						Type:        "string",
						Description: "Where to search: 'name' (default), 'definition' (inside procedure/view source code), 'column' (columns whose name contains the term)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results for type=search (default 50, max 200). A truncated result is reported with truncated=true.",
					},
					"max_cell_bytes": {
						Type:        "integer",
						Description: cellBudgetDescription,
					},
					"max_response_bytes": {
						Type:        "integer",
						Description: responseBudgetDescription,
					},
				},
				Required: []string{},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			Name:        "inspect",
			Title:       "Inspect Table",
			Description: "Inspect a table's structure. detail=columns (default) returns column info, detail=indexes returns indexes, detail=foreign_keys returns FK relationships, detail=dependencies returns objects (views, procedures, functions) that reference this table, detail=all returns everything in one call.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"table_name": {
						Type:        "string",
						Description: "Name of the table (can include schema: 'dbo.TableName')",
					},
					"schema": {
						Type:        "string",
						Description: "Schema name (optional, defaults to 'dbo')",
					},
					"detail": {
						Type:        "string",
						Description: "What to retrieve: 'columns' (default), 'indexes', 'foreign_keys', 'dependencies', 'all'",
					},
					"max_cell_bytes": {
						Type:        "integer",
						Description: cellBudgetDescription,
					},
					"max_response_bytes": {
						Type:        "integer",
						Description: responseBudgetDescription,
					},
				},
				Required: []string{"table_name"},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			Name:        "execute_procedure",
			Title:       "Execute Procedure",
			Description: "Execute a whitelisted stored procedure (requires MSSQL_WHITELIST_PROCEDURES env var)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"procedure_name": {
						Type:        "string",
						Description: "Name of the stored procedure to execute",
					},
					"parameters": {
						Type:        "string",
						Description: "JSON object with parameter names and values (optional)",
					},
				},
				Required: []string{"procedure_name"},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			Name:        "explain_query",
			Title:       "Explain Query",
			Description: "Show the estimated execution plan for a SQL query without executing it. Useful for performance analysis and query optimization. Only SELECT queries are accepted.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "SELECT query to analyze (must be a read-only query)",
					},
				},
				Required: []string{"query"},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:    boolPtr(true),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			},
		},
	}

	// Dynamic tools (and confirm_operation) are ONLY included in the tool list
	// for servers that started in dynamic mode. Classic servers (the common case
	// when using .mcp.json "env" with plain MSSQL_SERVER etc.) will never see
	// dynamic_available, dynamic_connect, etc. This eliminates the "AI always
	// tries dynamic connections" problem reported by users running multiple
	// isolated server instances.
	if s.isDynamic {
		tools = append(tools,
			Tool{
				Name:        "dynamic_available",
				Title:       "List Dynamic Connections",
				Description: "List all preconfigured dynamic database connections (aliases defined via MSSQL_DYNAMIC_<ALIAS>_* environment variables). Does not expose credentials. Use dynamic_connect to activate one.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			Tool{
				Name:        "dynamic_connect",
				Title:       "Connect to Dynamic Alias",
				Description: "Switch the active database connection to one of the preconfigured dynamic aliases (e.g. 'CRM', 'IDENTITY', 'GDP'). The security posture (read-only vs full access) is determined by the alias configuration or global safe defaults. After connecting, use get_database_info to verify the active alias and its effective permissions.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"alias": {
							Type:        "string",
							Description: "The alias name of the preconfigured connection (case-insensitive, e.g. 'CRM' or 'crm')",
						},
					},
					Required: []string{"alias"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(false), // depends on the *alias* posture, not global
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			},
			Tool{
				Name:        "dynamic_disconnect",
				Title:       "Disconnect Dynamic Connection",
				Description: "Close the currently active dynamic connection and return to disconnected state. Does not affect other aliases.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			Tool{
				Name:        "dynamic_list",
				Title:       "List Active Dynamic Connections",
				Description: "Show currently loaded dynamic aliases and which one (if any) is the active connection for subsequent query_database calls.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			Tool{
				Name:        "confirm_operation",
				Title:       "Confirm Dangerous Operation",
				Description: "Explicitly confirm a potentially destructive operation (INSERT/UPDATE/DELETE/DROP/etc.) on a writable dynamic alias. This is a required security step. You must call this tool with a clear description of what you intend to do before the actual modification query will be allowed.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"description": {
							Type:        "string",
							Description: "Clear description of the operation you want to perform (e.g. 'DELETE all rows from temp_ai where created < 2025-01-01')",
						},
					},
					Required: []string{"description"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(true),
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			},
		)
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  ToolsListResult{Tools: tools},
	}
}
