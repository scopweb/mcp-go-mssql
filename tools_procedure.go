package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// handleExecuteProcedure executes a whitelisted stored procedure
func (s *MCPMSSQLServer) handleExecuteProcedure(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
						Annotations: annAssistantHigh,
					},
				},
				IsError: true,
			},
		}
	}

	procName, ok := params.Arguments["procedure_name"].(string)
	if !ok || procName == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Missing or invalid 'procedure_name' parameter",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Check whitelist (cached at startup)
	whitelistEnv := s.config.whitelistProcs
	if whitelistEnv == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: No stored procedures are whitelisted. Set MSSQL_WHITELIST_PROCEDURES environment variable.",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	allowedProcs := strings.Split(whitelistEnv, ",")
	procAllowed := false
	procNameLower := strings.ToLower(strings.TrimSpace(procName))
	for _, allowed := range allowedProcs {
		if strings.ToLower(strings.TrimSpace(allowed)) == procNameLower {
			procAllowed = true
			break
		}
	}

	if !procAllowed {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error: Stored procedure '%s' is not in the whitelist. Allowed: %s", procName, whitelistEnv),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Validate procedure name contains only safe characters
	if err := s.validateProcedureName(procName); err != nil {
		s.secLogger.Printf("Rejected unsafe procedure name: %s", procName)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Parse parameters if provided
	var procParams map[string]interface{}
	if paramsJSON, ok := params.Arguments["parameters"].(string); ok && paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &procParams); err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error: Invalid JSON in parameters: %v", err),
							Annotations: annBothHigh,
						},
					},
					IsError: true,
				},
			}
		}
	}

	// Build EXEC statement with parameters
	var queryBuilder strings.Builder
	queryBuilder.WriteString("EXEC ")
	queryBuilder.WriteString(procName)

	var args []interface{}
	if len(procParams) > 0 {
		queryBuilder.WriteString(" ")
		paramStrings := make([]string, 0, len(procParams))
		i := 1
		for paramName, paramValue := range procParams {
			paramStrings = append(paramStrings, fmt.Sprintf("@%s = @p%d", paramName, i))
			args = append(args, paramValue)
			i++
		}
		queryBuilder.WriteString(strings.Join(paramStrings, ", "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := s.executeSecureQuery(ctx, queryBuilder.String(), args...)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Error executing procedure '%s': %v", procName, err),
						Annotations: annBothHigh,
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
						Type:        "text",
						Text:        fmt.Sprintf("Error formatting results: %v", err),
						Annotations: annBothHigh,
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
					Type:        "text",
					Text:        fmt.Sprintf("Procedure '%s' executed successfully:\n%s", procName, string(resultBytes)),
					Annotations: annBothProcedure,
				},
			},
		},
	}
}