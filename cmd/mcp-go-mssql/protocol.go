package main

// MCP JSON-RPC types for the stdio transport.
// Names and JSON tags follow the MCP specification and must stay stable.

// MCPRequest is one JSON-RPC request read from stdin.
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  interface{}            `json:"params,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
}

// MCPResponse is one JSON-RPC response written to stdout.
// Notifications leave ID nil and are not written.
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Result  interface{}            `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
}

// MCPError is a JSON-RPC error object. Messages sent to the client stay generic.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams is the MCP initialize request payload.
type InitializeParams struct {
	ProtocolVersion string   `json:"protocolVersion"`
	Capabilities    struct{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResult is the MCP initialize response payload.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// Capabilities advertises the server features this process implements.
type Capabilities struct {
	Tools   ToolsCapability        `json:"tools,omitempty"`
	Logging map[string]interface{} `json:"logging"`
}

// ToolsCapability reports whether the tool list can change after initialize.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolAnnotations are MCP hints. Pointers distinguish "unset" from false.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

// Tool is one entry in tools/list.
type Tool struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description"`
	InputSchema InputSchema      `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// InputSchema is the JSON Schema object for a tool's arguments.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

// Property describes one tool argument.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolsListResult is the tools/list response.
type ToolsListResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolParams is the tools/call request payload.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CallToolResult is the tools/call response payload.
type CallToolResult struct {
	Content           []ContentItem          `json:"content"`
	StructuredContent interface{}            `json:"structuredContent,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
	Meta              map[string]interface{} `json:"_meta,omitempty"`
}

// ContentItem is one text block inside a tool result.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ServerInfo identifies this MCP server to the client.
type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// boolPtr returns a pointer so tool annotations can omit a field when unset.
func boolPtr(b bool) *bool { return &b }
