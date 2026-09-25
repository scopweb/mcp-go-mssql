package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// execute_procedure.

// toolExecuteProcedure handles the execute_procedure tool.
func (s *MCPMSSQLServer) toolExecuteProcedure(id interface{}, params CallToolParams) *MCPResponse {
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

	procName, ok := params.Arguments["procedure_name"].(string)
	if !ok || procName == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: "Error: Missing or invalid 'procedure_name' parameter",
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
						Type: "text",
						Text: "Error: No stored procedures are whitelisted. Set MSSQL_WHITELIST_PROCEDURES environment variable.",
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
						Type: "text",
						Text: fmt.Sprintf("Error: Stored procedure '%s' is not in the whitelist. Allowed: %s", procName, whitelistEnv),
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
						Type: "text",
						Text: fmt.Sprintf("Error: %v", err),
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
							Type: "text",
							Text: fmt.Sprintf("Error: Invalid JSON in parameters: %v", err),
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
			// Parameter NAMES are interpolated into the EXEC text (only the
			// values are bound as @pN placeholders), so an unvalidated name
			// is a SQL injection vector. Reject anything that is not a plain
			// T-SQL identifier (optionally '@'-prefixed).
			normalizedName := strings.TrimPrefix(paramName, "@")
			if !validParamNamePattern.MatchString(paramName) {
				s.secLogger.Printf("Rejected unsafe stored-procedure parameter name: %q", paramName)
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      id,
					Result: CallToolResult{
						Content: []ContentItem{
							{Type: "text", Text: fmt.Sprintf("Error: invalid parameter name '%s' (only letters, digits and underscore are allowed, optionally prefixed with '@')", paramName)},
						},
						IsError: true,
					},
				}
			}
			paramStrings = append(paramStrings, fmt.Sprintf("@%s = @p%d", normalizedName, i))
			args = append(args, paramValue)
			i++
		}
		queryBuilder.WriteString(strings.Join(paramStrings, ", "))
	}

	// Procedures can write even when their name suggests otherwise, so
	// READ_ONLY=true blocks them in both classic and dynamic modes.
	procEffective := s.getEffectiveConfig()
	s.dynamicMu.RLock()
	procAlias := s.activeAlias
	s.dynamicMu.RUnlock()
	if procEffective.readOnly {
		s.secLogger.Printf("SECURITY VIOLATION: execute_procedure '%s' blocked in read-only mode", procName)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: execute_procedure is blocked because READ_ONLY=true. Stored procedures may modify data."}},
				IsError: true,
			},
		}
	}
	if s.isDynamic && procAlias != "" {
		if !s.isOperationConfirmed("EXEC", []string{procNameLower}, procAlias, queryBuilder.String()) {
			confErr := s.requireConfirmationForModification("EXEC", []string{procNameLower}, procAlias, queryBuilder.String())
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: confErr.Error()},
					},
					IsError: true,
				},
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, _, err := s.executeSecureQuery(ctx, queryBuilder.String(), args...)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error executing procedure '%s': %v", procName, err),
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
					Text: fmt.Sprintf("Procedure '%s' executed successfully:\n%s", procName, string(resultBytes)),
				},
			},
		},
	}
}
