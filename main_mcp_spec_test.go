package main

import (
	"encoding/json"
	"testing"
)

// TestMCPPingHandler verifies the server responds to ping with empty result (MUST per spec).
func TestMCPPingHandler(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "ping-1",
		Method:  "ping",
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatal("Expected response to ping, got nil")
	}
	if response.Error != nil {
		t.Fatalf("Ping should not return error, got: %v", response.Error)
	}

	// Result must be an empty object {}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("Failed to marshal ping result: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("Ping result should be a JSON object: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Ping result should be empty object, got %d keys", len(result))
	}
}

// TestMCPInitializeCapabilities verifies the server declares required capabilities.
func TestMCPInitializeCapabilities(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "init-1",
		Method:  "initialize",
		Params:  InitializeParams{ProtocolVersion: "2025-11-25"},
	}

	response := server.handleRequest(req)
	if response == nil || response.Error != nil {
		t.Fatal("Expected successful initialize response")
	}

	resultBytes, _ := json.Marshal(response.Result)
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify capabilities are declared by checking the raw JSON
	var rawResult map[string]interface{}
	json.Unmarshal(resultBytes, &rawResult)
	caps, ok := rawResult["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected capabilities in result")
	}
	if _, ok := caps["logging"]; !ok {
		t.Error("Expected logging capability to be declared")
	}
	if _, ok := caps["tools"]; !ok {
		t.Error("Expected tools capability to be declared")
	}
}

// TestMCPInitializeServerInfo verifies ServerInfo has name, title, and version.
func TestMCPInitializeServerInfo(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "init-2",
		Method:  "initialize",
		Params:  InitializeParams{ProtocolVersion: "2025-11-25"},
	}

	response := server.handleRequest(req)
	resultBytes, _ := json.Marshal(response.Result)
	var initResult InitializeResult
	json.Unmarshal(resultBytes, &initResult)

	if initResult.ServerInfo.Name == "" {
		t.Error("ServerInfo.Name must not be empty")
	}
	if initResult.ServerInfo.Title == "" {
		t.Error("ServerInfo.Title should be set for display in client UIs")
	}
	if initResult.ServerInfo.Version == "" {
		t.Error("ServerInfo.Version must not be empty")
	}
}

// TestMCPInitializeInstructions verifies the instructions field is present.
func TestMCPInitializeInstructions(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "init-3",
		Method:  "initialize",
		Params:  InitializeParams{ProtocolVersion: "2025-11-25"},
	}

	response := server.handleRequest(req)
	resultBytes, _ := json.Marshal(response.Result)
	var initResult InitializeResult
	json.Unmarshal(resultBytes, &initResult)

	if initResult.Instructions == "" {
		t.Error("Instructions field should be set to help LLM understand server usage")
	}
}

// TestMCPToolAnnotations verifies all tools have annotations.
func TestMCPToolAnnotations(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "tools-ann",
		Method:  "tools/list",
	}

	response := server.handleRequest(req)
	resultBytes, _ := json.Marshal(response.Result)
	var toolsResult ToolsListResult
	json.Unmarshal(resultBytes, &toolsResult)

	readOnlyTools := map[string]bool{
		"get_database_info": true,
		"explore":           true,
		"inspect":           true,
		"explain_query":     true,
	}

	for _, tool := range toolsResult.Tools {
		if tool.Annotations == nil {
			t.Errorf("Tool %q missing annotations", tool.Name)
			continue
		}

		// Verify read-only tools are marked correctly
		if readOnlyTools[tool.Name] {
			if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
				t.Errorf("Tool %q should have readOnlyHint=true", tool.Name)
			}
			if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Errorf("Tool %q should have destructiveHint=false", tool.Name)
			}
		}
	}

	// Verify execute_procedure is marked destructive
	for _, tool := range toolsResult.Tools {
		if tool.Name == "execute_procedure" {
			if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Error("execute_procedure should have destructiveHint=true")
			}
		}
	}
}

// TestMCPToolsCallInvalidParams verifies -32602 error for malformed params.
func TestMCPToolsCallInvalidParams(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "call-bad",
		Method:  "tools/call",
		Params:  "not-a-valid-object", // String instead of object
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	if response.Error == nil {
		t.Fatal("Expected error for invalid params")
	}
	if response.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", response.Error.Code)
	}
}

// TestMCPLoggingSetLevel verifies the server accepts logging/setLevel.
func TestMCPLoggingSetLevel(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "log-1",
		Method:  "logging/setLevel",
		Params:  map[string]interface{}{"level": "warning"},
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatal("Expected response to logging/setLevel, got nil")
	}
	if response.Error != nil {
		t.Fatalf("logging/setLevel should not return error, got: %v", response.Error)
	}
}

// TestMCPCancelledNotification verifies cancellation notifications are handled without response.
func TestMCPCancelledNotification(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/cancelled",
		Params:  map[string]interface{}{"requestId": "some-id", "reason": "user cancelled"},
	}

	response := server.handleRequest(req)
	if response != nil {
		t.Error("Cancellation notification should not produce a response")
	}
}

// TestMCPNotificationsInitialized verifies no response for initialized notification.
func TestMCPNotificationsInitialized(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	response := server.handleRequest(req)
	if response != nil {
		t.Error("notifications/initialized should not produce a response")
	}
}

// TestMCPMethodNotFound verifies -32601 for unknown methods.
func TestMCPMethodNotFound(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "unknown-1",
		Method:  "nonexistent/method",
	}

	response := server.handleRequest(req)
	if response == nil {
		t.Fatal("Expected error response for unknown method")
	}
	if response.Error == nil || response.Error.Code != -32601 {
		t.Errorf("Expected -32601 Method not found, got: %v", response.Error)
	}
}

// TestMCPUnknownNotificationIgnored verifies unknown notifications without ID produce no response.
func TestMCPUnknownNotificationIgnored(t *testing.T) {
	server := newTestMCPServer()

	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/unknown",
		// No ID = notification
	}

	response := server.handleRequest(req)
	if response != nil {
		t.Error("Unknown notification should be silently ignored (no response)")
	}
}
