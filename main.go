package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

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
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

type Capabilities struct {
	Tools ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
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
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Security Logger
type SecurityLogger struct {
	*log.Logger
}

func NewSecurityLogger() *SecurityLogger {
	return &SecurityLogger{
		Logger: log.New(os.Stderr, "[SECURITY] ", log.LstdFlags|log.Lshortfile),
	}
}

func (sl *SecurityLogger) LogConnectionAttempt(success bool) {
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	sl.Printf("Database connection attempt: %s", status)
}

func (sl *SecurityLogger) sanitizeForLogging(input string) string {
	sensitivePatterns := []string{
		`(?i)(password|pwd|secret|key|token)=[^;\\s]*`,
		`(?i)(password|pwd)\\s*=\\s*[^;\\s]*`,
	}
	
	result := input
	for _, pattern := range sensitivePatterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "${1}=***")
	}
	
	return result
}

// MSSQL Server
type MCPMSSQLServer struct {
	db        *sql.DB
	secLogger *SecurityLogger
	devMode   bool
}

func buildSecureConnectionString() (string, error) {
	server := os.Getenv("MSSQL_SERVER")
	database := os.Getenv("MSSQL_DATABASE")
	user := os.Getenv("MSSQL_USER")
	password := os.Getenv("MSSQL_PASSWORD")
	port := os.Getenv("MSSQL_PORT")
	
	if server == "" || database == "" || user == "" || password == "" {
		return "", fmt.Errorf("missing required environment variables: MSSQL_SERVER, MSSQL_DATABASE, MSSQL_USER, MSSQL_PASSWORD")
	}
	
	if port == "" {
		port = "1433"
	}
	
	// For development mode, allow untrusted certificates
	trustCert := "false"
	if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
		trustCert = "true"
	}
	
	return fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=true;trustservercertificate=%s;connection timeout=30;command timeout=30",
		server, database, user, password, port, trustCert,
	), nil
}

func (s *MCPMSSQLServer) validateBasicInput(input string) error {
	if len(input) > 4096 {
		return fmt.Errorf("input too large")
	}
	if len(input) == 0 {
		return fmt.Errorf("empty input")
	}
	return nil
}

func (s *MCPMSSQLServer) executeSecureQuery(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	
	if err := s.validateBasicInput(query); err != nil {
		return nil, err
	}
	
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		if s.devMode {
			s.secLogger.Printf("Failed to prepare statement: %v", err)
			return nil, fmt.Errorf("query preparation failed: %v", err)
		}
		s.secLogger.Printf("Failed to prepare statement: query preparation error")
		return nil, fmt.Errorf("query preparation failed")
	}
	defer stmt.Close()
	
	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		if s.devMode {
			s.secLogger.Printf("Failed to execute query: %v", err)
			return nil, fmt.Errorf("query execution failed: %v", err)
		}
		s.secLogger.Printf("Failed to execute query: execution error")
		return nil, fmt.Errorf("query execution failed")
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	
	return results, nil
}

func (s *MCPMSSQLServer) handleToolCall(id interface{}, params CallToolParams) *MCPResponse {
	switch params.Name {
	case "get_database_info":
		var info strings.Builder
		
		if s.db == nil {
			info.WriteString("Database Status: Disconnected\n")
			info.WriteString("Reason: No database connection established\n")
			if os.Getenv("MSSQL_SERVER") == "" {
				info.WriteString("Configuration: Missing MSSQL_SERVER environment variable\n")
			}
		} else {
			info.WriteString("Database Status: Connected\n")
			info.WriteString("Server: " + os.Getenv("MSSQL_SERVER") + "\n")
			info.WriteString("Database: " + os.Getenv("MSSQL_DATABASE") + "\n")
			info.WriteString("Encryption: Enabled (TLS)\n")
		}
		
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: info.String(),
					},
				},
			},
		}
		
	case "query_database":
		if s.db == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: "Error: Database not connected. Use get_database_info to check connection status.",
						},
					},
					IsError: true,
				},
			}
		}
		
		query, ok := params.Arguments["query"].(string)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: "Error: Missing or invalid 'query' parameter",
						},
					},
					IsError: true,
				},
			}
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		results, err := s.executeSecureQuery(ctx, query)
		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Query Error: %v", err),
						},
					},
					IsError: true,
				},
			}
		}
		
		// Format results as JSON
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
						Text: fmt.Sprintf("Query executed successfully. Results:\n%s", string(resultBytes)),
					},
				},
			},
		}
		
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

func (s *MCPMSSQLServer) handleRequest(req MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		dbStatus := "disconnected"
		if s.db != nil {
			dbStatus = "connected"
		}
		
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: "2025-06-18",
				Capabilities: Capabilities{
					Tools: ToolsCapability{
						ListChanged: false,
					},
				},
				ServerInfo: ServerInfo{
					Name:    fmt.Sprintf("mcp-go-mssql (%s)", dbStatus),
					Version: "1.0.0",
				},
			},
		}
	
	case "tools/list":
		tools := []Tool{
			{
				Name:        "query_database",
				Description: "Execute a secure SQL query against the MSSQL database",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {
							Type:        "string",
							Description: "SQL query to execute (uses prepared statements for security)",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "get_database_info",
				Description: "Get database connection status and basic information",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
			},
		}
		
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  ToolsListResult{Tools: tools},
		}
	
	case "tools/call":
		var params CallToolParams
		if paramBytes, err := json.Marshal(req.Params); err == nil {
			json.Unmarshal(paramBytes, &params)
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

func main() {
	// Initialize security logger
	secLogger := NewSecurityLogger()
	secLogger.Printf("Starting secure MCP-MSSQL server")
	
	// Check for developer mode
	devMode := strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true"
	if devMode {
		secLogger.Printf("DEVELOPER MODE ENABLED - Detailed errors will be shown")
	}
	
	// Create MCP server without database initially
	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: secLogger,
		devMode:   devMode,
	}
	
	// Try to establish database connection (non-fatal)
	go func() {
		// Give MCP protocol time to initialize
		time.Sleep(2 * time.Second)
		
		// Check if we have required environment variables
		serverHost := os.Getenv("MSSQL_SERVER")
		database := os.Getenv("MSSQL_DATABASE") 
		user := os.Getenv("MSSQL_USER")
		password := os.Getenv("MSSQL_PASSWORD")
		
		secLogger.Printf("Environment variables - Server: %s, Database: %s, User: %s, Password: %s, DevMode: %s", 
			serverHost, database, user, 
			func() string { if password != "" { return "***" } else { return "MISSING" } }(),
			os.Getenv("DEVELOPER_MODE"))
		
		if serverHost == "" {
			secLogger.Printf("No MSSQL_SERVER environment variable - database features disabled")
			return
		}
		
		// Build secure connection string
		connStr, err := buildSecureConnectionString()
		if err != nil {
			if devMode {
				secLogger.Printf("Failed to build connection string: %v", err)
			} else {
				secLogger.Printf("Failed to build connection string: configuration error")
			}
			return
		}
		
		// Connect to MSSQL
		secLogger.Printf("Attempting to connect to MSSQL server...")
		db, err := sql.Open("sqlserver", connStr)
		if err != nil {
			if devMode {
				secLogger.Printf("sql.Open failed: %v", err)
			} else {
				secLogger.Printf("Failed to connect: connection error")
			}
			return
		}
		secLogger.Printf("sql.Open successful, testing connection...")
		
		// Configure connection pool for security
		db.SetMaxOpenConns(5)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(time.Hour)
		db.SetConnMaxIdleTime(time.Minute * 15)
		
		// Test connection with longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		secLogger.Printf("Testing database connection with ping...")
		if err := db.PingContext(ctx); err != nil {
			secLogger.LogConnectionAttempt(false)
			if devMode {
				secLogger.Printf("Database ping failed: %v", err)
				trustCert := "false"
				if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
					trustCert = "true"
				}
				secLogger.Printf("Connection string format: server=SERVER;database=DB;user id=USER;password=***;port=PORT;encrypt=true;trustservercertificate=%s;connection timeout=30;command timeout=30", trustCert)
			} else {
				secLogger.Printf("Failed to ping database: connection test failed")
			}
			db.Close()
			return
		}
		
		secLogger.LogConnectionAttempt(true)
		secLogger.Printf("Database connection established successfully")
		
		// Update server with working database connection
		server.db = db
	}()
	
	// Start MCP protocol handler
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			secLogger.Printf("Invalid JSON received: %v", err)
			continue
		}
		
		sanitizedReq := secLogger.sanitizeForLogging(line)
		secLogger.Printf("Processing request: %s", sanitizedReq)
		
		response := server.handleRequest(req)
		
		// Only send response if one is needed (not for notifications)
		if response != nil {
			responseBytes, err := json.Marshal(response)
			if err != nil {
				secLogger.Printf("Failed to marshal response: %v", err)
				continue
			}
			
			fmt.Println(string(responseBytes))
		}
	}
	
	if err := scanner.Err(); err != nil && err != io.EOF {
		secLogger.Printf("Scanner error: %v", err)
	}
}