package main

// Tool dispatch. The rate limit is enforced here, once, before the tool method runs.

func (s *MCPMSSQLServer) handleToolCall(id interface{}, params CallToolParams) *MCPResponse {
	defer func() {
		if r := recover(); r != nil {
			s.secLogger.Printf("Recovered panic in handleToolCall for tool %s: %v (tool failed gracefully)", params.Name, r)
		}
	}()

	// MCP spec MUST: rate limit tool invocations
	if !s.checkRateLimit() {
		s.secLogger.Printf("Rate limit exceeded for tool: %s", params.Name)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Rate limit exceeded. Please wait before making more requests."}},
				IsError: true,
			},
		}
	}

	switch params.Name {
	case "get_database_info":
		return s.toolGetDatabaseInfo(id, params)
	case "query_database":
		return s.toolQueryDatabase(id, params)
	case "explore":
		return s.toolExplore(id, params)
	case "execute_procedure":
		return s.toolExecuteProcedure(id, params)
	case "inspect":
		return s.toolInspect(id, params)
	case "explain_query":
		return s.toolExplainQuery(id, params)
	case "dynamic_available":
		return s.toolDynamicAvailable(id, params)
	case "dynamic_connect":
		return s.toolDynamicConnect(id, params)
	case "dynamic_disconnect":
		return s.toolDynamicDisconnect(id, params)
	case "dynamic_list":
		return s.toolDynamicList(id, params)
	case "confirm_operation":
		return s.toolConfirmOperation(id, params)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &MCPError{
				Code:    -32602,
				Message: "Unknown tool: " + params.Name,
			},
		}
	}
}
