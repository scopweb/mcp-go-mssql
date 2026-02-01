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
	osuser "os/user"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
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
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

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

// Compiled regex patterns for better performance
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|pwd|secret|key|token)=[^;\\s]*`),
	regexp.MustCompile(`(?i)(password|pwd)\\s*=\\s*[^;\\s]*`),
}

// Pre-compiled word-boundary patterns for read-only keyword detection
var dangerousKeywordPatterns = map[string]*regexp.Regexp{
	"INSERT":   regexp.MustCompile(`(?i)\bINSERT\b`),
	"UPDATE":   regexp.MustCompile(`(?i)\bUPDATE\b`),
	"DELETE":   regexp.MustCompile(`(?i)\bDELETE\b`),
	"DROP":     regexp.MustCompile(`(?i)\bDROP\b`),
	"CREATE":   regexp.MustCompile(`(?i)\bCREATE\b`),
	"ALTER":    regexp.MustCompile(`(?i)\bALTER\b`),
	"TRUNCATE": regexp.MustCompile(`(?i)\bTRUNCATE\b`),
	"MERGE":    regexp.MustCompile(`(?i)\bMERGE\b`),
	"EXEC":     regexp.MustCompile(`(?i)\bEXEC\b`),
	"EXECUTE":  regexp.MustCompile(`(?i)\bEXECUTE\b`),
	"CALL":     regexp.MustCompile(`(?i)\bCALL\b`),
	"BULK":     regexp.MustCompile(`(?i)\bBULK\b`),
	"BCP":      regexp.MustCompile(`(?i)\bBCP\b`),
}

// Pre-compiled patterns for table name extraction (performance optimization)
var tableExtractionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bFROM\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),             // FROM [schema.]table
	regexp.MustCompile(`(?i)\bJOIN\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),             // JOIN [schema.]table
	regexp.MustCompile(`(?i)\bINTO\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),             // INSERT INTO [schema.]table
	regexp.MustCompile(`(?i)\bUPDATE\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),           // UPDATE [schema.]table
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),    // DELETE FROM [schema.]table
	regexp.MustCompile(`(?i)\bDELETE\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?\s+FROM`),    // DELETE table FROM
	regexp.MustCompile(`(?i)\bTABLE\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),            // CREATE/DROP TABLE
	regexp.MustCompile(`(?i)\bVIEW\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`),             // CREATE/DROP VIEW
	regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\s+(?:\[?[\w]+\]?\.)?\[?([\w]+)\]?`), // TRUNCATE TABLE
}

// Pre-compiled pattern for procedure name validation
var validProcedureNamePattern = regexp.MustCompile(`^[\w.\[\]]+$`)

func (sl *SecurityLogger) sanitizeForLogging(input string) string {
	result := input
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "${1}=***")
	}

	return result
}

// MSSQL Server
type MCPMSSQLServer struct {
	db        *sql.DB
	dbMu      sync.RWMutex
	secLogger *SecurityLogger
	devMode   bool
}

func (s *MCPMSSQLServer) getDB() *sql.DB {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return s.db
}

func (s *MCPMSSQLServer) setDB(db *sql.DB) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	s.db = db
}

func buildSecureConnectionString() (string, error) {
	// Check for custom connection string first
	if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
		connStrLower := strings.ToLower(customConnStr)
		isProduction := strings.ToLower(os.Getenv("DEVELOPER_MODE")) != "true"

		if isProduction {
			// Warn about insecure settings in production
			if strings.Contains(connStrLower, "encrypt=false") {
				log.New(os.Stderr, "[SECURITY] ", log.LstdFlags).Printf("WARNING: Custom connection string has encrypt=false in production mode")
			}
			if !strings.Contains(connStrLower, "encrypt=") {
				log.New(os.Stderr, "[SECURITY] ", log.LstdFlags).Printf("WARNING: Custom connection string missing encrypt parameter in production mode")
			}
			if strings.Contains(connStrLower, "trustservercertificate=true") {
				log.New(os.Stderr, "[SECURITY] ", log.LstdFlags).Printf("WARNING: Custom connection string has trustservercertificate=true in production mode")
			}
		}

		// Ensure timeout settings are present; append defaults if missing
		if !strings.Contains(connStrLower, "connection timeout") {
			customConnStr += ";connection timeout=30"
		}
		if !strings.Contains(connStrLower, "command timeout") {
			customConnStr += ";command timeout=30"
		}

		return customConnStr, nil
	}

	server := os.Getenv("MSSQL_SERVER")
	database := os.Getenv("MSSQL_DATABASE")
	user := os.Getenv("MSSQL_USER")
	password := os.Getenv("MSSQL_PASSWORD")
	port := os.Getenv("MSSQL_PORT")
	auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))

	if auth == "" {
		auth = "sql"
	}

	if server == "" {
		return "", fmt.Errorf("missing required environment variable: MSSQL_SERVER")
	}

	// For Windows Auth, database is optional (allows exploring all databases)
	// For SQL Auth, database is required
	if auth == "sql" {
		if database == "" {
			return "", fmt.Errorf("missing required environment variable for SQL auth: MSSQL_DATABASE")
		}
		if user == "" || password == "" {
			return "", fmt.Errorf("missing required environment variables for SQL auth: MSSQL_USER, MSSQL_PASSWORD")
		}
	}

	if port == "" {
		port = "1433"
	}

	// For development mode, allow disabling encryption and untrusted certificates
	encrypt := "true"
	trustCert := "false"
	if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
		// In development mode, allow disabling encryption for local SQL Server instances
		if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
			encrypt = strings.ToLower(envEncrypt)
		} else {
			// Default to false for development mode to match local SQL Server setups
			encrypt = "false"
		}
		trustCert = "true"
	}

	// Build connection string depending on requested authentication mode
	switch auth {
	case "integrated", "windows":
		// Windows Integrated Authentication (SSPI)
		// The process will use the credentials of the Windows user running it
		// Database is optional - if not specified, connects to default database
		var connStr string
		if database != "" {
			connStr = fmt.Sprintf("server=%s;database=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, database, encrypt, trustCert,
			)
		} else {
			// No database specified - connect to master or default database
			connStr = fmt.Sprintf("server=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, encrypt, trustCert,
			)
		}
		return connStr, nil
	case "azure":
		// Azure AD auth needs an additional implementation to obtain tokens
		return "", fmt.Errorf("Azure AD authentication not implemented in buildSecureConnectionString; use MSSQL_CONNECTION_STRING or set MSSQL_AUTH=sql")
	default:
		// Default to SQL Server authentication
		return fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
			server, port, database, user, password, encrypt, trustCert,
		), nil
	}
}

func (s *MCPMSSQLServer) validateProcedureName(name string) error {
	if name == "" {
		return fmt.Errorf("empty procedure name")
	}
	if !validProcedureNamePattern.MatchString(name) {
		return fmt.Errorf("invalid procedure name: contains disallowed characters")
	}
	return nil
}

func (s *MCPMSSQLServer) validateBasicInput(input string) error {
	// Allow larger queries - up to 1MB (1048576 characters)
	maxSize := 1048576
	if customMax := os.Getenv("MSSQL_MAX_QUERY_SIZE"); customMax != "" {
		if size, err := strconv.Atoi(customMax); err == nil && size > 0 {
			maxSize = size
		}
	}

	if len(input) > maxSize {
		return fmt.Errorf("input too large (max %d characters)", maxSize)
	}
	if len(input) == 0 {
		return fmt.Errorf("empty input")
	}
	return nil
}

func (s *MCPMSSQLServer) validateReadOnlyQuery(query string) error {
	// Check if read-only mode is enabled
	if strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) != "true" {
		return nil // Read-only mode disabled, allow all queries
	}

	// Normalize query for checking
	normalizedQuery := strings.TrimSpace(strings.ToUpper(query))

	// Remove leading comments and whitespace
	for strings.HasPrefix(normalizedQuery, "--") || strings.HasPrefix(normalizedQuery, "/*") || strings.HasPrefix(normalizedQuery, " ") || strings.HasPrefix(normalizedQuery, "\t") || strings.HasPrefix(normalizedQuery, "\n") || strings.HasPrefix(normalizedQuery, "\r") {
		if strings.HasPrefix(normalizedQuery, "--") {
			// Skip until end of line
			if idx := strings.Index(normalizedQuery, "\n"); idx != -1 {
				normalizedQuery = strings.TrimSpace(normalizedQuery[idx+1:])
			} else {
				return fmt.Errorf("read-only mode: only SELECT queries are allowed")
			}
		} else if strings.HasPrefix(normalizedQuery, "/*") {
			// Skip until end of block comment
			if idx := strings.Index(normalizedQuery, "*/"); idx != -1 {
				normalizedQuery = strings.TrimSpace(normalizedQuery[idx+2:])
			} else {
				return fmt.Errorf("read-only mode: only SELECT queries are allowed")
			}
		} else {
			normalizedQuery = strings.TrimSpace(normalizedQuery[1:])
		}
	}

	// List of allowed read-only operations
	allowedPrefixes := []string{
		"SELECT",
		"WITH", // Common Table Expressions that start with WITH
		"SHOW",
		"DESCRIBE",
		"DESC",
		"EXPLAIN",
	}

	// Check if query starts with an allowed prefix
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(normalizedQuery, prefix) {
			// Additional check: ensure no dangerous keywords are present (using word boundaries)
			for keyword, pattern := range dangerousKeywordPatterns {
				if pattern.MatchString(query) {
					return fmt.Errorf("read-only mode: query contains forbidden operation '%s'", keyword)
				}
			}

			// Dangerous system procedures (block these)
			dangerousSPs := []string{
				"XP_CMDSHELL", "XP_REGREAD", "XP_REGWRITE", "XP_FILEEXIST",
				"XP_DIRTREE", "XP_FIXEDDRIVES", "XP_SERVICECONTROL",
				"SP_CONFIGURE", "SP_ADDLOGIN", "SP_DROPLOGIN",
				"SP_ADDSRVROLEMEMBER", "SP_DROPSRVROLEMEMBER",
				"SP_ADDROLEMEMBER", "SP_DROPROLEMEMBER",
				"SP_EXECUTESQL", "SP_OACREATE", "SP_OAMETHOD",
			}

			// Safe read-only system procedures (allow these)
			safeSPs := []string{
				"SP_HELP", "SP_HELPTEXT", "SP_HELPINDEX", "SP_HELPCONSTRAINT",
				"SP_COLUMNS", "SP_TABLES", "SP_STORED_PROCEDURES",
				"SP_FKEYS", "SP_PKEYS", "SP_STATISTICS",
				"SP_DATABASES", "SP_HELPDB",
			}

			queryUpper := strings.ToUpper(query)
			// Check for dangerous SPs
			for _, sp := range dangerousSPs {
				if strings.Contains(queryUpper, sp) {
					return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
				}
			}

			// If query contains SP_ or XP_, verify it's in the safe list
			if strings.Contains(queryUpper, "SP_") || strings.Contains(queryUpper, "XP_") {
				isSafe := false
				for _, safeSP := range safeSPs {
					if strings.Contains(queryUpper, safeSP) {
						isSafe = true
						break
					}
				}
				if !isSafe {
					return fmt.Errorf("read-only mode: system procedure not in allowed list")
				}
			}

			return nil // Query is allowed
		}
	}

	return fmt.Errorf("read-only mode: only SELECT and read operations are allowed")
}

// getWhitelistedTables returns the list of tables/views allowed for modification
func (s *MCPMSSQLServer) getWhitelistedTables() []string {
	whitelistEnv := os.Getenv("MSSQL_WHITELIST_TABLES")
	if whitelistEnv == "" {
		return []string{} // Empty whitelist means no tables allowed for modification
	}

	// Parse comma-separated list and normalize to lowercase
	tables := strings.Split(whitelistEnv, ",")
	var normalized []string
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table != "" {
			normalized = append(normalized, strings.ToLower(table))
		}
	}
	return normalized
}

// extractAllTablesFromQuery finds all table/view names referenced in the query
func (s *MCPMSSQLServer) extractAllTablesFromQuery(query string) []string {
	queryUpper := strings.ToUpper(query)
	tablesFound := make(map[string]bool) // Use map to avoid duplicates

	for _, pattern := range tableExtractionPatterns {
		matches := pattern.FindAllStringSubmatch(queryUpper, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := match[1]
				// Remove brackets if present [tablename] -> tablename
				tableName = strings.Trim(tableName, "[]")
				tableName = strings.ToLower(strings.TrimSpace(tableName))
				if tableName != "" {
					tablesFound[tableName] = true
				}
			}
		}
	}

	// Convert map keys to slice
	var tables []string
	for table := range tablesFound {
		tables = append(tables, table)
	}
	return tables
}

// extractOperation determines the primary SQL operation (INSERT, UPDATE, DELETE, etc.)
func (s *MCPMSSQLServer) extractOperation(query string) string {
	queryUpper := strings.ToUpper(strings.TrimSpace(query))

	// Remove leading comments
	for strings.HasPrefix(queryUpper, "--") || strings.HasPrefix(queryUpper, "/*") {
		if strings.HasPrefix(queryUpper, "--") {
			if idx := strings.Index(queryUpper, "\n"); idx != -1 {
				queryUpper = strings.TrimSpace(queryUpper[idx+1:])
			} else {
				break
			}
		} else if strings.HasPrefix(queryUpper, "/*") {
			if idx := strings.Index(queryUpper, "*/"); idx != -1 {
				queryUpper = strings.TrimSpace(queryUpper[idx+2:])
			} else {
				break
			}
		}
	}

	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE"}
	for _, op := range modifyOps {
		if strings.HasPrefix(queryUpper, op) {
			return op
		}
	}

	// If WITH is found, check if there's a modify operation after the CTE
	if strings.HasPrefix(queryUpper, "WITH") {
		for _, op := range modifyOps {
			if strings.Contains(queryUpper, op) {
				return op
			}
		}
	}

	return "SELECT" // Default to SELECT for read operations
}

// validateTablePermissions validates that all tables in a modify operation are whitelisted
func (s *MCPMSSQLServer) validateTablePermissions(query string) error {
	// Only validate if read-only mode is enabled
	if strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) != "true" {
		return nil // Whitelist mode disabled, allow all operations
	}

	whitelist := s.getWhitelistedTables()
	operation := s.extractOperation(query)

	// Determine if this is a modification operation
	modifyOps := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "MERGE"}
	isModifyOp := false
	for _, op := range modifyOps {
		if operation == op {
			isModifyOp = true
			break
		}
	}

	// If not a modify operation (e.g., SELECT), allow it
	if !isModifyOp {
		return nil
	}

	// Extract ALL tables referenced in the query
	tablesInQuery := s.extractAllTablesFromQuery(query)

	s.secLogger.Printf("Permission check - Operation: %s, Tables found: %v, Whitelist: %v",
		operation, tablesInQuery, whitelist)

	// If whitelist is empty, deny all modifications
	if len(whitelist) == 0 {
		return fmt.Errorf("permission denied: no tables are whitelisted for %s operations", operation)
	}

	// Check if ALL tables in the query are whitelisted
	for _, table := range tablesInQuery {
		isWhitelisted := false
		for _, allowedTable := range whitelist {
			if table == allowedTable {
				isWhitelisted = true
				break
			}
		}

		if !isWhitelisted {
			s.secLogger.Printf("SECURITY VIOLATION: Attempted %s operation on non-whitelisted table '%s'",
				operation, table)
			return fmt.Errorf("permission denied: table '%s' is not whitelisted for %s operations",
				table, operation)
		}
	}

	// All tables are whitelisted
	s.secLogger.Printf("Permission granted: %s operation on whitelisted table(s) %v",
		operation, tablesInQuery)
	return nil
}

func (s *MCPMSSQLServer) executeSecureQuery(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	if err := s.validateBasicInput(query); err != nil {
		return nil, err
	}

	// Validate read-only restrictions
	if err := s.validateReadOnlyQuery(query); err != nil {
		s.secLogger.Printf("Read-only violation blocked: %s", err)
		return nil, err
	}

	// Validate granular table permissions (whitelist)
	if err := s.validateTablePermissions(query); err != nil {
		s.secLogger.Printf("Permission violation blocked: %s", err)
		return nil, err
	}

	stmt, err := db.PrepareContext(ctx, query)
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

		if s.getDB() == nil {
			info.WriteString("Database Status: Disconnected\n")
			info.WriteString("Reason: No database connection established\n")
			if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
				info.WriteString("Configuration: Using custom connection string\n")
			} else if os.Getenv("MSSQL_SERVER") == "" {
				info.WriteString("Configuration: Missing MSSQL_SERVER environment variable\n")
			}
		} else {
			info.WriteString("Database Status: Connected\n")
			if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
				info.WriteString("Connection: Custom connection string\n")
				info.WriteString("Mode: " + func() string {
					if os.Getenv("DEVELOPER_MODE") == "true" {
						return "Development"
					} else {
						return "Production"
					}
				}() + "\n")
			} else {
				info.WriteString("Server: " + os.Getenv("MSSQL_SERVER") + "\n")
				info.WriteString("Database: " + os.Getenv("MSSQL_DATABASE") + "\n")
				encrypt := "Enabled (TLS)"
				if os.Getenv("DEVELOPER_MODE") == "true" && os.Getenv("MSSQL_ENCRYPT") != "true" {
					encrypt = "Disabled (Development)"
				}
				info.WriteString("Encryption: " + encrypt + "\n")
			}

			// Show read-only status and whitelist
			if strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) == "true" {
				info.WriteString("Access Mode: READ-ONLY (SELECT queries only)\n")

				// Show whitelist if configured
				whitelist := s.getWhitelistedTables()
				if len(whitelist) > 0 {
					info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
					info.WriteString("Note: Only whitelisted tables can be modified (INSERT/UPDATE/DELETE/CREATE/DROP)\n")
				} else {
					info.WriteString("Whitelisted Tables: NONE (all modifications blocked)\n")
				}
			} else {
				info.WriteString("Access Mode: Full access\n")
			}
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
		if s.getDB() == nil {
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

	case "list_tables":
		if s.getDB() == nil {
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

		query := `
			SELECT
				TABLE_SCHEMA as schema_name,
				TABLE_NAME as table_name,
				TABLE_TYPE as table_type
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
			ORDER BY TABLE_SCHEMA, TABLE_NAME
		`

		// Use shorter timeout for metadata queries (faster)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
							Text: fmt.Sprintf("Error listing tables: %v", err),
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
						Text: fmt.Sprintf("Tables and views found:\n%s", string(resultBytes)),
					},
				},
			},
		}

	case "describe_table":
		if s.getDB() == nil {
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

		tableName, ok := params.Arguments["table_name"].(string)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: "Error: Missing or invalid 'table_name' parameter",
						},
					},
					IsError: true,
				},
			}
		}

		// Parse schema.table format or use schema parameter
		schemaName := "dbo" // default schema
		if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
			schemaName = schema
		}

		// Check if table_name contains schema (e.g., "dbo.Clients" or "[dbo].[Clients]")
		if strings.Contains(tableName, ".") {
			parts := strings.Split(tableName, ".")
			if len(parts) == 2 {
				schemaName = strings.Trim(parts[0], "[]")
				tableName = strings.Trim(parts[1], "[]")
			}
		}
		tableName = strings.Trim(tableName, "[]")

		query := `
			SELECT
				COLUMN_NAME as column_name,
				DATA_TYPE as data_type,
				IS_NULLABLE as is_nullable,
				COLUMN_DEFAULT as default_value,
				CHARACTER_MAXIMUM_LENGTH as max_length,
				ORDINAL_POSITION as position
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = @p1 AND TABLE_NAME = @p2
			ORDER BY ORDINAL_POSITION
		`

		// Use shorter timeout for metadata queries (faster)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := s.executeSecureQuery(ctx, query, schemaName, tableName)
		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error describing table '%s': %v", tableName, err),
						},
					},
					IsError: true,
				},
			}
		}

		if len(results) == 0 {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Table '%s' not found", tableName),
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
						Text: fmt.Sprintf("Table structure for '%s':\n%s", tableName, string(resultBytes)),
					},
				},
			},
		}

	case "list_databases":
		if s.getDB() == nil {
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

		query := `
			SELECT 
				name as database_name,
				database_id,
				create_date,
				state_desc as state
			FROM sys.databases
			WHERE database_id > 4
			ORDER BY name
		`

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
							Text: fmt.Sprintf("Error listing databases: %v", err),
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
						Text: fmt.Sprintf("Databases found:\n%s", string(resultBytes)),
					},
				},
			},
		}

	case "get_indexes":
		if s.getDB() == nil {
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

		tableName, ok := params.Arguments["table_name"].(string)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: "Error: Missing or invalid 'table_name' parameter",
						},
					},
					IsError: true,
				},
			}
		}

		schemaName := "dbo"
		if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
			schemaName = schema
		}
		if strings.Contains(tableName, ".") {
			parts := strings.Split(tableName, ".")
			if len(parts) == 2 {
				schemaName = strings.Trim(parts[0], "[]")
				tableName = strings.Trim(parts[1], "[]")
			}
		}
		tableName = strings.Trim(tableName, "[]")

		query := `
			SELECT 
				i.name as index_name,
				i.type_desc as index_type,
				i.is_unique,
				i.is_primary_key,
				STRING_AGG(c.name, ', ') WITHIN GROUP (ORDER BY ic.key_ordinal) as columns
			FROM sys.indexes i
			INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
			INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
			INNER JOIN sys.tables t ON i.object_id = t.object_id
			INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
			WHERE t.name = @p1 AND s.name = @p2 AND i.name IS NOT NULL
			GROUP BY i.name, i.type_desc, i.is_unique, i.is_primary_key
			ORDER BY i.is_primary_key DESC, i.name
		`

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := s.executeSecureQuery(ctx, query, tableName, schemaName)
		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error getting indexes: %v", err),
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
						Text: fmt.Sprintf("Indexes for '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
					},
				},
			},
		}

	case "get_foreign_keys":
		if s.getDB() == nil {
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

		tableName, ok := params.Arguments["table_name"].(string)
		if !ok {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: "Error: Missing or invalid 'table_name' parameter",
						},
					},
					IsError: true,
				},
			}
		}

		schemaName := "dbo"
		if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
			schemaName = schema
		}
		if strings.Contains(tableName, ".") {
			parts := strings.Split(tableName, ".")
			if len(parts) == 2 {
				schemaName = strings.Trim(parts[0], "[]")
				tableName = strings.Trim(parts[1], "[]")
			}
		}
		tableName = strings.Trim(tableName, "[]")

		query := `
			SELECT 
				fk.name as constraint_name,
				OBJECT_SCHEMA_NAME(fk.parent_object_id) as from_schema,
				OBJECT_NAME(fk.parent_object_id) as from_table,
				COL_NAME(fkc.parent_object_id, fkc.parent_column_id) as from_column,
				OBJECT_SCHEMA_NAME(fk.referenced_object_id) as to_schema,
				OBJECT_NAME(fk.referenced_object_id) as to_table,
				COL_NAME(fkc.referenced_object_id, fkc.referenced_column_id) as to_column,
				fk.delete_referential_action_desc as on_delete,
				fk.update_referential_action_desc as on_update
			FROM sys.foreign_keys fk
			INNER JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
			INNER JOIN sys.tables t ON fk.parent_object_id = t.object_id
			INNER JOIN sys.schemas s ON t.schema_id = s.schema_id
			WHERE (t.name = @p1 AND s.name = @p2)
			   OR (OBJECT_NAME(fk.referenced_object_id) = @p1 AND OBJECT_SCHEMA_NAME(fk.referenced_object_id) = @p2)
			ORDER BY fk.name
		`

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := s.executeSecureQuery(ctx, query, tableName, schemaName)
		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error getting foreign keys: %v", err),
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
						Text: fmt.Sprintf("Foreign keys for '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
					},
				},
			},
		}

	case "list_stored_procedures":
		if s.getDB() == nil {
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

		schemaFilter := ""
		if schema, ok := params.Arguments["schema"].(string); ok && schema != "" {
			schemaFilter = schema
		}

		var query string
		var results []map[string]interface{}
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if schemaFilter != "" {
			query = `
				SELECT 
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				WHERE SCHEMA_NAME(p.schema_id) = @p1
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQuery(ctx, query, schemaFilter)
		} else {
			query = `
				SELECT 
					SCHEMA_NAME(p.schema_id) as schema_name,
					p.name as procedure_name,
					p.create_date,
					p.modify_date
				FROM sys.procedures p
				ORDER BY schema_name, procedure_name
			`
			results, err = s.executeSecureQuery(ctx, query)
		}

		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error listing stored procedures: %v", err),
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
						Text: fmt.Sprintf("Stored procedures found:\n%s", string(resultBytes)),
					},
				},
			},
		}

	case "execute_procedure":
		if s.getDB() == nil {
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

		// Check whitelist
		whitelistEnv := os.Getenv("MSSQL_WHITELIST_PROCEDURES")
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
		if s.getDB() != nil {
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
			{
				Name:        "list_tables",
				Description: "List all tables and views in the database",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
			},
			{
				Name:        "describe_table",
				Description: "Get the structure and schema information for a specific table",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"table_name": {
							Type:        "string",
							Description: "Name of the table to describe (can include schema: 'dbo.TableName' or just 'TableName')",
						},
						"schema": {
							Type:        "string",
							Description: "Schema name (optional, defaults to 'dbo')",
						},
					},
					Required: []string{"table_name"},
				},
			},
			{
				Name:        "list_databases",
				Description: "List all databases on the SQL Server instance",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
			},
			{
				Name:        "get_indexes",
				Description: "Get indexes for a specific table",
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
					},
					Required: []string{"table_name"},
				},
			},
			{
				Name:        "get_foreign_keys",
				Description: "Get foreign key relationships for a specific table",
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
					},
					Required: []string{"table_name"},
				},
			},
			{
				Name:        "list_stored_procedures",
				Description: "List all stored procedures in the database",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"schema": {
							Type:        "string",
							Description: "Filter by schema (optional)",
						},
					},
					Required: []string{},
				},
			},
			{
				Name:        "execute_procedure",
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
			if err2 := json.Unmarshal(paramBytes, &params); err2 != nil {
				s.secLogger.Printf("Failed to unmarshal call params: %v", err2)
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
		authMode := os.Getenv("MSSQL_AUTH")

		customConnStr := os.Getenv("MSSQL_CONNECTION_STRING")
		if customConnStr != "" {
			secLogger.Printf("Using custom connection string: %s", secLogger.sanitizeForLogging(customConnStr))
		} else {
			auth := strings.ToLower(authMode)
			if auth == "" {
				auth = "sql"
			}

			if auth == "integrated" || auth == "windows" {
				secLogger.Printf("Environment variables - Server: %s, Database: %s, Auth: INTEGRATED (Windows), DevMode: %s",
					serverHost,
					func() string {
						if database != "" {
							return database
						}
						return "(not specified - will use default)"
					}(),
					os.Getenv("DEVELOPER_MODE"))

				// Show current Windows user
				if u, err := osuser.Current(); err == nil {
					secLogger.Printf("Running as Windows user: %s", u.Username)
				}
			} else {
				secLogger.Printf("Environment variables - Server: %s, Database: %s, Auth: SQL, User: %s, Password: %s, DevMode: %s",
					serverHost, database, user,
					func() string {
						if password != "" {
							return "***"
						} else {
							return "MISSING"
						}
					}(),
					os.Getenv("DEVELOPER_MODE"))
			}
		}

		// Log only non-sensitive configuration settings
		safeEnvVars := []string{"MSSQL_SERVER", "MSSQL_DATABASE", "MSSQL_PORT", "MSSQL_AUTH", "MSSQL_READ_ONLY", "MSSQL_WHITELIST_TABLES", "DEVELOPER_MODE"}
		secLogger.Printf("Configuration settings:")
		for _, key := range safeEnvVars {
			if val := os.Getenv(key); val != "" {
				secLogger.Printf("  %s=%s", key, val)
			}
		}

		// Log security settings
		if strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) == "true" {
			secLogger.Printf("READ-ONLY MODE ENABLED - Only SELECT queries allowed")
		} else {
			secLogger.Printf("Full access mode enabled")
		}

		if customConnStr == "" && serverHost == "" {
			secLogger.Printf("No MSSQL_SERVER or MSSQL_CONNECTION_STRING environment variable - database features disabled")
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

		// Configure optimized connection pool
		db.SetMaxOpenConns(10)                  // More concurrent connections
		db.SetMaxIdleConns(5)                   // More idle connections for reuse
		db.SetConnMaxLifetime(30 * time.Minute) // Shorter lifetime for fresher connections
		db.SetConnMaxIdleTime(5 * time.Minute)  // Quick cleanup of unused connections

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

				auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
				if auth == "" {
					auth = "sql"
				}

				if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
					secLogger.Printf("Using custom connection string format")
				} else if auth == "integrated" || auth == "windows" {
					secLogger.Printf("Using Windows Integrated Authentication (SSPI)")
					secLogger.Printf("Troubleshooting tips for integrated auth:")
					secLogger.Printf("  1. Ensure your Windows user has permission in SQL Server")
					secLogger.Printf("  2. Check if SQL Server is configured to allow Windows Authentication")
					secLogger.Printf("  3. Try using server='.' or server='localhost' or server='(local)'")
					secLogger.Printf("  4. Verify TCP/IP or Named Pipes are enabled in SQL Server Configuration Manager")

					// Get current Windows user
					if u, err := osuser.Current(); err == nil {
						secLogger.Printf("  Running as Windows user: %s\\%s", u.Username, u.Name)
					}
				} else {
					encrypt := "true"
					if strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true" {
						if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
							encrypt = strings.ToLower(envEncrypt)
						} else {
							encrypt = "false"
						}
					}
					secLogger.Printf("Connection string format: server=SERVER;port=PORT;database=DB;user id=USER;password=***;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30", encrypt, trustCert)
				}
			} else {
				secLogger.Printf("Failed to ping database: connection test failed")
			}
			if cerr := db.Close(); cerr != nil {
				secLogger.Printf("Error closing DB after failed ping: %v", cerr)
			}
			return
		}

		secLogger.LogConnectionAttempt(true)
		secLogger.Printf("Database connection established successfully")

		// Update server with working database connection
		server.setDB(db)
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
