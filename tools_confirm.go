package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mcp-go-mssql/internal/sqlguard"
)

// handleConfirmOperation confirms and executes a pending destructive operation
func (s *MCPMSSQLServer) handleConfirmOperation(id interface{}, params CallToolParams) *MCPResponse {
	token, ok := params.Arguments["token"].(string)
	if !ok || token == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Missing or invalid 'token' parameter. Provide the token from the destructive operation warning.",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Look up the pending operation
	s.pendingOpMu.Lock()
	op, exists := s.pendingOps[token]
	if !exists {
		s.pendingOpMu.Unlock()
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Invalid or expired confirmation token. The token may have expired (valid for 5 minutes) or has already been used.",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Check if expired
	if time.Now().After(op.expiresAt) {
		delete(s.pendingOps, token)
		s.pendingOpMu.Unlock()
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Error: Confirmation token has expired. Please retry the original destructive operation.",
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Remove token (one-time use)
	delete(s.pendingOps, token)
	s.pendingOpMu.Unlock()

	// Log that confirmation was received
	s.secLogger.Printf("DESTRUCTIVE OPERATION CONFIRMED: executing %s", op.query)

	// Execute the pending operation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := s.executeSecureQuery(ctx, op.query)
	if err != nil {
		// If execution fails, return the error
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("Operation failed: %v", err),
						Annotations: annBothHigh,
					},
				},
				IsError: true,
			},
		}
	}

	// Operation succeeded
	opType := sqlguard.ExtractDestructiveOpType(op.query)
	if results == nil || len(results) == 0 {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        fmt.Sprintf("%s executed successfully after confirmation. No rows returned.", opType),
						Annotations: annBothQuery,
					},
				},
			},
		}
	}

	resultBytes, _ := json.MarshalIndent(results, "", "  ")
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type:        "text",
					Text:        fmt.Sprintf("%s executed successfully after confirmation. %d rows returned:\n%s", opType, len(results), string(resultBytes)),
					Annotations: annBothQuery,
				},
			},
		},
	}
}