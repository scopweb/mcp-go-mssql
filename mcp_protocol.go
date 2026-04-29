package main

// MCP Protocol structures
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ConfirmationRequiredError is returned when a destructive DDL operation
// requires user confirmation before execution.
type ConfirmationRequiredError struct {
	Token     string // token to pass to confirm_operation
	Operation string // human-readable operation type
	Target    string // schema.object target
	ExpiresIn string // time until token expires
}

func (e *ConfirmationRequiredError) Error() string {
	return "DESTRUCTIVE OPERATION REQUIRES CONFIRMATION: " + e.Operation + " on " + e.Target + ". Use confirm_operation tool with token " + e.Token
}

// DestructiveConfirmationCode is the JSON-RPC error code for destructive operation confirmation.
const DestructiveConfirmationCode = -32000

type InitializeParams struct {
	ProtocolVersion string   `json:"protocolVersion"`
	Capabilities    struct{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

type Capabilities struct {
	Tools   ToolsCapability        `json:"tools,omitempty"`
	Logging map[string]interface{} `json:"logging"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

type Tool struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description"`
	InputSchema InputSchema      `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolsListResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []ContentItem          `json:"content"`
	IsError bool                   `json:"isError,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
}

type ContentAnnotations struct {
	Audience []string `json:"audience,omitempty"` // "user", "assistant", or both
	Priority float64  `json:"priority,omitempty"` // 0.0 (least) to 1.0 (most important)
}

type ContentItem struct {
	Type        string              `json:"type"`
	Text        string              `json:"text"`
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// boolPtr is a helper to create *bool for tool annotations.
func boolPtr(b bool) *bool { return &b }

// Content annotation presets for MCP content items.
// Priority scale: 0.0 (least) → 1.0 (most important / effectively required).
var (
	// annAssistantLow marks low-priority content for the LLM (status checks, reference info).
	annAssistantLow = &ContentAnnotations{Audience: []string{"assistant"}, Priority: 0.3}
	// annAssistantHigh marks high-priority content for the LLM (critical diagnostics).
	annAssistantHigh = &ContentAnnotations{Audience: []string{"assistant"}, Priority: 1.0}
	// annBothExplore marks explore results — discovery context, lower priority.
	annBothExplore = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 0.4}
	// annBothInspect marks inspect results — structural reference.
	annBothInspect = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 0.5}
	// annBothQuery marks query results — directly requested data.
	annBothQuery = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 0.7}
	// annBothProcedure marks procedure results — action with side effects.
	annBothProcedure = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 0.8}
	// annBothExplain marks explain results — secondary analysis.
	annBothExplain = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 0.3}
	// annBothHigh marks high-priority content for both audiences (errors).
	annBothHigh = &ContentAnnotations{Audience: []string{"user", "assistant"}, Priority: 1.0}
)
