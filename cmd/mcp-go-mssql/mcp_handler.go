package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// JSON-RPC method dispatch for the stdio transport.

func (s *MCPMSSQLServer) handleRequest(req MCPRequest) *MCPResponse {
	defer func() {
		if r := recover(); r != nil {
			s.secLogger.Printf("Recovered panic in handleRequest for method %s: %v (request dropped, server stays alive)", req.Method, r)
		}
	}()
	switch req.Method {
	case "initialize":
		dbStatus := "disconnected"
		if s.getDB() != nil {
			dbStatus = "connected"
		}

		// Extract client's protocolVersion and echo it back (spec MUST requirement)
		clientVersion := "2025-11-25" // default to latest spec version
		if req.Params != nil {
			if paramBytes, err := json.Marshal(req.Params); err == nil {
				var initParams InitializeParams
				if err := json.Unmarshal(paramBytes, &initParams); err == nil && initParams.ProtocolVersion != "" {
					supported := map[string]bool{"2024-11-05": true, "2025-06-18": true, "2025-11-25": true}
					if !supported[initParams.ProtocolVersion] {
						return &MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32602, Message: "Unsupported protocol version", Data: map[string]interface{}{"supported": []string{"2024-11-05", "2025-06-18", "2025-11-25"}}}}
					}
					clientVersion = initParams.ProtocolVersion
				}
			}
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: clientVersion,
				Capabilities: Capabilities{
					Tools: ToolsCapability{
						ListChanged: false,
					},
					Logging: map[string]interface{}{},
				},
				ServerInfo: ServerInfo{
					Name:    fmt.Sprintf("mcp-go-mssql (%s)", dbStatus),
					Title:   "MSSQL Database Connector",
					Version: "1.0.0",
				},
				Instructions: s.buildInstructions(),
			},
		}

	case "ping":
		// MCP spec MUST: respond promptly to ping with empty result
		if req.ID == nil {
			return nil
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}

	case "logging/setLevel":
		// MCP spec SHOULD: respect the minimum log level set by client
		if req.Params != nil {
			if paramBytes, err := json.Marshal(req.Params); err == nil {
				var levelParams struct {
					Level string `json:"level"`
				}
				if err := json.Unmarshal(paramBytes, &levelParams); err == nil {
					mcpLevel := strings.ToLower(levelParams.Level)
					var slogLevel slog.Level
					switch mcpLevel {
					case "debug":
						slogLevel = slog.LevelDebug
					case "info", "notice":
						slogLevel = slog.LevelInfo
					case "warning":
						slogLevel = slog.LevelWarn
					case "error", "critical", "alert", "emergency":
						slogLevel = slog.LevelError
					default:
						slogLevel = slog.LevelInfo
					}
					s.secLogger.levelVar.Set(slogLevel)
				}
			}
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}

	case "notifications/cancelled":
		// MCP spec: cancellation notification — no response needed
		// Future: could cancel in-flight query contexts
		return nil

	case "tools/list":
		return s.listTools(req.ID)

	case "tools/call":
		var params CallToolParams
		paramBytes, err := json.Marshal(req.Params)
		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32602,
					Message: "Invalid params: failed to parse request parameters",
				},
			}
		}
		if err2 := json.Unmarshal(paramBytes, &params); err2 != nil {
			s.secLogger.Printf("Failed to unmarshal call params: %v", err2)
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32602,
					Message: "Invalid params: " + err2.Error(),
				},
			}
		}

		return s.handleToolCall(req.ID, params)

	case "notifications/initialized":
		// Notifications don't need a response
		return nil

	default:
		// Only respond to requests with IDs (not notifications)
		if req.ID != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32601,
					Message: "Method not found",
				},
			}
		}
		return nil
	}
}
