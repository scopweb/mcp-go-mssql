package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	osuser "os/user"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/microsoft/go-mssqldb/integratedauth/winsspi"
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

// Security Logger — structured logging via log/slog (stdlib Go 1.21+)
type SecurityLogger struct {
	logger   *slog.Logger
	levelVar *slog.LevelVar // dynamic level controlled by MCP logging/setLevel
}

func NewSecurityLogger() *SecurityLogger {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})
	return &SecurityLogger{
		logger:   slog.New(handler).With(slog.String("component", "security")),
		levelVar: lvl,
	}
}

// Printf provides backward-compatible formatted logging.
func (sl *SecurityLogger) Printf(format string, args ...interface{}) {
	sl.logger.Info(fmt.Sprintf(format, args...))
}

func (sl *SecurityLogger) LogConnectionAttempt(success bool) {
	sl.logger.Info("database connection attempt",
		slog.Bool("success", success),
	)
}

// Compiled regex patterns for better performance
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|pwd|secret|key|token)=[^;\s]*`),
	regexp.MustCompile(`(?i)(password|pwd)\s*=\s*[^;\s]*`),
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

// serverConfig holds cached configuration read once at startup.
type serverConfig struct {
	readOnly        bool
	whitelistTables []string
	whitelistProcs  string
}

// MSSQL Server
type MCPMSSQLServer struct {
	db          *sql.DB
	dbMu        sync.RWMutex
	secLogger   *SecurityLogger
	devMode     bool
	config      serverConfig
	rateLimiter struct {
		mu        sync.Mutex
		tokens    int
		maxTokens int
		lastReset time.Time
		interval  time.Duration
	}
}

// checkRateLimit implements a simple token-bucket rate limiter for tool invocations.
// Returns true if the request is allowed, false if rate limited.
func (s *MCPMSSQLServer) checkRateLimit() bool {
	s.rateLimiter.mu.Lock()
	defer s.rateLimiter.mu.Unlock()

	now := time.Now()
	if now.Sub(s.rateLimiter.lastReset) >= s.rateLimiter.interval {
		s.rateLimiter.tokens = s.rateLimiter.maxTokens
		s.rateLimiter.lastReset = now
	}

	if s.rateLimiter.tokens <= 0 {
		return false
	}
	s.rateLimiter.tokens--
	return true
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
				slog.Warn("custom connection string has encrypt=false in production mode")
			}
			if !strings.Contains(connStrLower, "encrypt=") {
				slog.Warn("custom connection string missing encrypt parameter in production mode")
			}
			if strings.Contains(connStrLower, "trustservercertificate=true") {
				slog.Warn("custom connection string has trustservercertificate=true in production mode")
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
			connStr = fmt.Sprintf("server=%s;port=%s;database=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, port, database, encrypt, trustCert,
			)
		} else {
			// No database specified - connect to master or default database
			connStr = fmt.Sprintf("server=%s;port=%s;integrated security=SSPI;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
				server, port, encrypt, trustCert,
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

// stripLeadingComments removes SQL comments and whitespace from the beginning of a query.
func stripLeadingComments(query string) string {
	q := strings.TrimSpace(strings.ToUpper(query))
	for strings.HasPrefix(q, "--") || strings.HasPrefix(q, "/*") || strings.HasPrefix(q, " ") || strings.HasPrefix(q, "\t") || strings.HasPrefix(q, "\n") || strings.HasPrefix(q, "\r") {
		if strings.HasPrefix(q, "--") {
			if idx := strings.Index(q, "\n"); idx != -1 {
				q = strings.TrimSpace(q[idx+1:])
			} else {
				return q
			}
		} else if strings.HasPrefix(q, "/*") {
			if idx := strings.Index(q, "*/"); idx != -1 {
				q = strings.TrimSpace(q[idx+2:])
			} else {
				return q
			}
		} else {
			q = strings.TrimSpace(q[1:])
		}
	}
	return q
}

func (s *MCPMSSQLServer) validateReadOnlyQuery(query string) error {
	// Check if read-only mode is enabled (cached at startup)
	if !s.config.readOnly {
		return nil // Read-only mode disabled, allow all queries
	}

	// If whitelist is configured, allow modifications to pass through to validateTablePermissions()
	// This enables the use case: READ_ONLY=true + WHITELIST=table1,table2
	// where only whitelisted tables can be modified
	whitelist := s.getWhitelistedTables()
	if len(whitelist) > 0 {
		// Whitelist is configured - let validateTablePermissions() handle modification permissions
		// We still need to block dangerous operations though
		normalizedQuery := stripLeadingComments(query)
		_ = normalizedQuery // used for future pattern matching if needed

		// Block dangerous system procedures even with whitelist
		dangerousSPs := []string{
			"XP_CMDSHELL", "XP_REGREAD", "XP_REGWRITE", "XP_FILEEXIST",
			"XP_DIRTREE", "XP_FIXEDDRIVES", "XP_SERVICECONTROL",
			"SP_CONFIGURE", "SP_ADDLOGIN", "SP_DROPLOGIN",
			"SP_ADDSRVROLEMEMBER", "SP_DROPSRVROLEMEMBER",
			"SP_ADDROLEMEMBER", "SP_DROPROLEMEMBER",
			"SP_EXECUTESQL", "SP_OACREATE", "SP_OAMETHOD",
		}

		queryUpper := strings.ToUpper(query)
		for _, sp := range dangerousSPs {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}

		// Allow query to proceed to validateTablePermissions() for whitelist check
		return nil
	}

	// No whitelist configured - enforce strict read-only mode
	normalizedQuery := stripLeadingComments(query)

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

// getWhitelistedTables returns the cached list of tables/views allowed for modification.
func (s *MCPMSSQLServer) getWhitelistedTables() []string {
	return s.config.whitelistTables
}

// parseWhitelistTables parses a comma-separated whitelist into normalized lowercase slice.
func parseWhitelistTables(env string) []string {
	if env == "" {
		return nil
	}
	tables := strings.Split(env, ",")
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
	queryUpper := stripLeadingComments(query)

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
	// Only validate if read-only mode is enabled (cached at startup)
	if !s.config.readOnly {
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

// maxQueryRows limits the number of rows returned by any query to prevent token overflow.
const maxQueryRows = 500

// executeSecureQuery runs a validated, prepared query and returns up to maxQueryRows rows.
// If the result is truncated, the last element contains a "_truncated" warning key.
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
		return nil, fmt.Errorf("query preparation failed: check SQL syntax, table/column names, and permissions. Use explore tool to verify table exists")
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		if s.devMode {
			s.secLogger.Printf("Failed to execute query: %v", err)
			return nil, fmt.Errorf("query execution failed: %v", err)
		}
		s.secLogger.Printf("Failed to execute query: execution error")
		return nil, fmt.Errorf("query execution failed: the query syntax is valid but execution was rejected by the server. Check permissions and data constraints")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	rowCount := 0
	truncated := false
	for rows.Next() {
		if rowCount >= maxQueryRows {
			truncated = true
			break
		}
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
		rowCount++
	}

	if truncated {
		results = append(results, map[string]interface{}{
			"_truncated": fmt.Sprintf("Results limited to %d rows. Use WHERE or TOP to narrow the query.", maxQueryRows),
		})
	}

	return results, nil
}

func (s *MCPMSSQLServer) handleToolCall(id interface{}, params CallToolParams) *MCPResponse {
	// MCP spec MUST: rate limit tool invocations
	if !s.checkRateLimit() {
		s.secLogger.Printf("Rate limit exceeded for tool: %s", params.Name)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Rate limit exceeded. Please wait before making more requests.", Annotations: annBothHigh}},
				IsError: true,
			},
		}
	}

	switch params.Name {
	case "get_database_info":
		var info strings.Builder

		if s.getDB() == nil {
			info.WriteString("Database Status: DISCONNECTED\n\n")

			// Show current configuration so Claude can diagnose
			info.WriteString("=== Current Configuration ===\n")
			if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
				info.WriteString("Connection: Custom connection string (MSSQL_CONNECTION_STRING)\n")
			} else {
				server := os.Getenv("MSSQL_SERVER")
				if server == "" {
					info.WriteString("MSSQL_SERVER: NOT SET (required)\n")
				} else {
					info.WriteString("MSSQL_SERVER: " + server + "\n")
				}
				database := os.Getenv("MSSQL_DATABASE")
				if database != "" {
					info.WriteString("MSSQL_DATABASE: " + database + "\n")
				} else {
					info.WriteString("MSSQL_DATABASE: not set\n")
				}
				auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
				if auth == "" {
					auth = "sql"
				}
				info.WriteString("MSSQL_AUTH: " + auth + "\n")
				if auth == "sql" {
					if os.Getenv("MSSQL_USER") == "" {
						info.WriteString("MSSQL_USER: NOT SET (required for SQL auth)\n")
					} else {
						info.WriteString("MSSQL_USER: " + os.Getenv("MSSQL_USER") + "\n")
					}
					if os.Getenv("MSSQL_PASSWORD") == "" {
						info.WriteString("MSSQL_PASSWORD: NOT SET (required for SQL auth)\n")
					} else {
						info.WriteString("MSSQL_PASSWORD: ***\n")
					}
				} else if auth == "integrated" || auth == "windows" {
					if u, err := osuser.Current(); err == nil {
						info.WriteString("Windows User: " + u.Username + "\n")
					}
				}
				port := os.Getenv("MSSQL_PORT")
				if port == "" {
					port = "1433"
				}
				info.WriteString("MSSQL_PORT: " + port + "\n")
				info.WriteString("DEVELOPER_MODE: " + os.Getenv("DEVELOPER_MODE") + "\n")
				encryptVal := os.Getenv("MSSQL_ENCRYPT")
				if encryptVal != "" {
					info.WriteString("MSSQL_ENCRYPT: " + encryptVal + "\n")
				}
			}

			// Diagnostic hints for Claude to suggest fixes
			info.WriteString("\n=== Possible Causes ===\n")
			if os.Getenv("MSSQL_SERVER") == "" && os.Getenv("MSSQL_CONNECTION_STRING") == "" {
				info.WriteString("- MSSQL_SERVER environment variable is not set\n")
			} else {
				auth := strings.ToLower(os.Getenv("MSSQL_AUTH"))
				devMode := strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true"
				encrypt := "true"
				if devMode {
					if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
						encrypt = strings.ToLower(envEncrypt)
					} else {
						encrypt = "false"
					}
				}

				if auth == "sql" || auth == "" {
					if os.Getenv("MSSQL_USER") == "" || os.Getenv("MSSQL_PASSWORD") == "" {
						info.WriteString("- Missing MSSQL_USER or MSSQL_PASSWORD for SQL authentication\n")
					}
				}
				if encrypt == "true" {
					info.WriteString("- TLS encryption is ENABLED. If the server is SQL Server 2008/2012 or doesn't have TLS certificates, set MSSQL_ENCRYPT=false with DEVELOPER_MODE=true\n")
				}
				if !devMode {
					info.WriteString("- Production mode requires valid TLS certificates. For internal/dev servers, set DEVELOPER_MODE=true\n")
				}
				if auth == "integrated" || auth == "windows" {
					info.WriteString("- Windows Integrated Auth: verify the Windows user has SQL Server login permissions\n")
					info.WriteString("- Check that SQL Server allows Windows Authentication mode\n")
					info.WriteString("- For remote servers, verify Active Directory connectivity\n")
				}
				info.WriteString("- Verify the server is reachable and SQL Server service is running\n")
				info.WriteString("- Check firewall rules allow connections on the configured port\n")
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

			// Show read-only status and whitelist (cached config)
			whitelist := s.getWhitelistedTables()
			if s.config.readOnly {
				if len(whitelist) > 0 {
					info.WriteString("Access Mode: READ-ONLY with whitelist exceptions\n")
					info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
					info.WriteString("Note: SELECT allowed on all tables. Modifications (INSERT/UPDATE/DELETE/CREATE/DROP) only allowed on whitelisted tables.\n")
				} else {
					info.WriteString("Access Mode: READ-ONLY (SELECT queries only)\n")
					info.WriteString("Whitelisted Tables: NONE (all modifications blocked)\n")
				}
			} else {
				if len(whitelist) > 0 {
					info.WriteString("Access Mode: Whitelist-protected (modifications restricted)\n")
					info.WriteString("Whitelisted Tables: " + strings.Join(whitelist, ", ") + "\n")
					info.WriteString("Note: Only whitelisted tables can be modified. All other tables are read-only.\n")
				} else {
					info.WriteString("Access Mode: Full access\n")
				}
			}
		}

		// Annotation: diagnostics for the LLM; high priority when disconnected
		ann := annAssistantLow
		if s.getDB() == nil {
			ann = annAssistantHigh
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        info.String(),
						Annotations: ann,
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
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
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
							Type:        "text",
							Text:        "Error: Missing or invalid 'query' parameter",
							Annotations: annBothHigh,
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
							Type:        "text",
							Text:        fmt.Sprintf("Query Error: %v", err),
							Annotations: annBothHigh,
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
						Text:        fmt.Sprintf("Query executed successfully. Results:\n%s", string(resultBytes)),
						Annotations: annBothQuery,
					},
				},
			},
		}

	case "explore":
		if s.getDB() == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
						},
					},
					IsError: true,
				},
			}
		}

		exploreType := "tables"
		if t, ok := params.Arguments["type"].(string); ok && t != "" {
			exploreType = t
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var results []map[string]interface{}
		var err error
		var label string

		switch exploreType {
		case "databases":
			label = "Databases found"
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
			results, err = s.executeSecureQuery(ctx, query)

		case "procedures":
			label = "Stored procedures found"
			schemaFilter, _ := params.Arguments["schema"].(string)
			filterVal, _ := params.Arguments["filter"].(string)
			if schemaFilter != "" && filterVal != "" {
				query := `
					SELECT
						SCHEMA_NAME(p.schema_id) as schema_name,
						p.name as procedure_name,
						p.create_date,
						p.modify_date
					FROM sys.procedures p
					WHERE SCHEMA_NAME(p.schema_id) = @p1 AND p.name LIKE @p2
					ORDER BY schema_name, procedure_name
				`
				results, err = s.executeSecureQuery(ctx, query, schemaFilter, "%"+filterVal+"%")
			} else if schemaFilter != "" {
				query := `
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
			} else if filterVal != "" {
				query := `
					SELECT
						SCHEMA_NAME(p.schema_id) as schema_name,
						p.name as procedure_name,
						p.create_date,
						p.modify_date
					FROM sys.procedures p
					WHERE p.name LIKE @p1
					ORDER BY schema_name, procedure_name
				`
				results, err = s.executeSecureQuery(ctx, query, "%"+filterVal+"%")
			} else {
				query := `
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

		case "search":
			pattern, ok := params.Arguments["pattern"].(string)
			if !ok || pattern == "" {
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      id,
					Result: CallToolResult{
						Content: []ContentItem{
							{Type: "text", Text: "Error: 'pattern' is required when type=search", Annotations: annBothHigh},
						},
						IsError: true,
					},
				}
			}
			searchIn, _ := params.Arguments["search_in"].(string)
			likePattern := "%" + pattern + "%"
			if searchIn == "definition" {
				label = fmt.Sprintf("Objects matching '%s' in definition", pattern)
				query := `
					SELECT
						o.type_desc      AS object_type,
						SCHEMA_NAME(o.schema_id) AS schema_name,
						o.name           AS object_name,
						m.definition     AS definition_snippet
					FROM sys.sql_modules m
					JOIN sys.objects     o ON o.object_id = m.object_id
					WHERE m.definition LIKE @p1
					ORDER BY o.type_desc, o.name
				`
				results, err = s.executeSecureQuery(ctx, query, likePattern)
			} else {
				label = fmt.Sprintf("Objects matching '%s' in name", pattern)
				query := `
					SELECT
						o.type_desc      AS object_type,
						SCHEMA_NAME(o.schema_id) AS schema_name,
						o.name           AS object_name,
						o.create_date    AS created,
						o.modify_date    AS modified
					FROM sys.objects o
					WHERE o.name LIKE @p1
					  AND o.type IN ('U','V','P','FN','IF','TF')
					ORDER BY o.type_desc, o.name
				`
				results, err = s.executeSecureQuery(ctx, query, likePattern)
			}

		case "views":
			label = "Views found"
			viewFilter, _ := params.Arguments["filter"].(string)
			if viewFilter != "" {
				query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v WHERE v.TABLE_NAME LIKE @p1 ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
				results, err = s.executeSecureQuery(ctx, query, "%"+viewFilter+"%")
			} else {
				query := "SELECT v.TABLE_SCHEMA AS schema_name, v.TABLE_NAME AS view_name, v.CHECK_OPTION AS check_option, v.IS_UPDATABLE AS is_updatable, LEFT(v.VIEW_DEFINITION, 300) AS definition_preview FROM INFORMATION_SCHEMA.VIEWS v ORDER BY v.TABLE_SCHEMA, v.TABLE_NAME"
				results, err = s.executeSecureQuery(ctx, query)
			}

		default: // "tables"
			label = "Tables and views found"
			if filterVal, ok := params.Arguments["filter"].(string); ok && filterVal != "" {
				filterPattern := "%" + filterVal + "%"
				query := `
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM INFORMATION_SCHEMA.TABLES
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					  AND TABLE_NAME LIKE @p1
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`
				results, err = s.executeSecureQuery(ctx, query, filterPattern)
			} else {
				query := `
					SELECT
						TABLE_SCHEMA as schema_name,
						TABLE_NAME as table_name,
						TABLE_TYPE as table_type
					FROM INFORMATION_SCHEMA.TABLES
					WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
					ORDER BY TABLE_SCHEMA, TABLE_NAME
				`
				results, err = s.executeSecureQuery(ctx, query)
			}
		}

		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error in explore: %v", err),
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
						Text:        fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
						Annotations: annBothExplore,
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
							Type: "text",
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

	case "inspect":
		if s.getDB() == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text:        "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
							Annotations: annAssistantHigh,
						},
					},
					IsError: true,
				},
			}
		}

		tableName, ok := params.Arguments["table_name"].(string)
		if !ok || tableName == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: "Error: Missing or invalid 'table_name' parameter", Annotations: annBothHigh},
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

		detail := "columns"
		if d, ok := params.Arguments["detail"].(string); ok && d != "" {
			detail = d
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		columnsQuery := `
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
		indexesQuery := `
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
		fkQuery := `
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

		if detail == "all" {
			colResults, err := s.executeSecureQuery(ctx, columnsQuery, schemaName, tableName)
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting columns: %v", err), Annotations: annBothHigh}}, IsError: true,
				}}
			}
			idxResults, err := s.executeSecureQuery(ctx, indexesQuery, tableName, schemaName)
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting indexes: %v", err), Annotations: annBothHigh}}, IsError: true,
				}}
			}
			fkResults, err := s.executeSecureQuery(ctx, fkQuery, tableName, schemaName)
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting foreign keys: %v", err), Annotations: annBothHigh}}, IsError: true,
				}}
			}
			depsAllQuery := `
				SELECT
					SCHEMA_NAME(o.schema_id)  AS referencing_schema,
					o.name                    AS referencing_object,
					o.type_desc               AS referencing_type,
					sed.is_caller_dependent,
					sed.is_ambiguous
				FROM sys.sql_expression_dependencies sed
				JOIN sys.objects o ON o.object_id = sed.referencing_id
				WHERE sed.referenced_entity_name = @p1
				  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
				ORDER BY o.type_desc, referencing_schema, referencing_object
			`
			depsResults, _ := s.executeSecureQuery(ctx, depsAllQuery, tableName, schemaName) // #nosec G104 - dependencies query is optional, errors handled gracefully
			combined := map[string]interface{}{
				"columns":      colResults,
				"indexes":      idxResults,
				"foreign_keys": fkResults,
				"dependencies": depsResults,
			}
			resultBytes, err := json.MarshalIndent(combined, "", "  ")
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error formatting results: %v", err), Annotations: annBothHigh}}, IsError: true,
				}}
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Full inspection of '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
							Annotations: annBothInspect,
						},
					},
				},
			}
		}

		var results []map[string]interface{}
		var err error
		var label string

		switch detail {
		case "indexes":
			label = fmt.Sprintf("Indexes for '%s.%s'", schemaName, tableName)
			results, err = s.executeSecureQuery(ctx, indexesQuery, tableName, schemaName)
		case "foreign_keys":
			label = fmt.Sprintf("Foreign keys for '%s.%s'", schemaName, tableName)
			results, err = s.executeSecureQuery(ctx, fkQuery, tableName, schemaName)
		case "dependencies":
			label = fmt.Sprintf("Objects that depend on '%s.%s'", schemaName, tableName)
			depsQuery := `
				SELECT
					SCHEMA_NAME(o.schema_id)  AS referencing_schema,
					o.name                    AS referencing_object,
					o.type_desc               AS referencing_type,
					sed.is_caller_dependent,
					sed.is_ambiguous
				FROM sys.sql_expression_dependencies sed
				JOIN sys.objects o ON o.object_id = sed.referencing_id
				WHERE sed.referenced_entity_name = @p1
				  AND (sed.referenced_schema_name = @p2 OR sed.referenced_schema_name IS NULL)
				ORDER BY o.type_desc, referencing_schema, referencing_object
			`
			results, err = s.executeSecureQuery(ctx, depsQuery, tableName, schemaName)
		default: // "columns"
			label = fmt.Sprintf("Table structure for '%s'", tableName)
			results, err = s.executeSecureQuery(ctx, columnsQuery, schemaName, tableName)
			if err == nil && len(results) == 0 {
				return &MCPResponse{
					JSONRPC: "2.0",
					ID:      id,
					Result: CallToolResult{
						Content: []ContentItem{
							{Type: "text", Text: fmt.Sprintf("Table '%s' not found", tableName), Annotations: annBothHigh},
						},
						IsError: true,
					},
				}
			}
		}

		if err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type:        "text",
							Text:        fmt.Sprintf("Error in inspect: %v", err),
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
						Text:        fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
						Annotations: annBothInspect,
					},
				},
			},
		}

	case "explain_query":
		if s.getDB() == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.", Annotations: annAssistantHigh}},
					IsError: true,
				},
			}
		}

		query, ok := params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: Missing or invalid 'query' parameter", Annotations: annBothHigh}},
					IsError: true,
				},
			}
		}

		// Only allow SELECT queries for safety (always enforced, regardless of MSSQL_READ_ONLY)
		if op := s.extractOperation(query); op != "SELECT" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: explain_query only accepts SELECT queries, got: " + op, Annotations: annBothHigh}},
					IsError: true,
				},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use a dedicated connection so SET SHOWPLAN_TEXT applies only to this query
		conn, err := s.getDB().Conn(ctx)
		if err != nil {
			connErrMsg := "Error acquiring connection"
			if s.devMode {
				connErrMsg += ": " + err.Error()
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: connErrMsg, Annotations: annBothHigh}},
					IsError: true,
				},
			}
		}
		defer conn.Close()

		// Enable showplan (does not execute the query, only returns the plan)
		if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_TEXT ON"); err != nil {
			showplanErrMsg := "Error enabling SHOWPLAN"
			if s.devMode {
				showplanErrMsg += ": " + err.Error()
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: showplanErrMsg, Annotations: annBothHigh}},
					IsError: true,
				},
			}
		}

		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup
			planErrMsg := "Error getting execution plan"
			if s.devMode {
				planErrMsg += ": " + err.Error()
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: planErrMsg, Annotations: annBothHigh}},
					IsError: true,
				},
			}
		}
		defer rows.Close()

		var planLines []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err == nil {
				planLines = append(planLines, line)
			}
		}
		_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup

		if len(planLines) == 0 {
			planLines = []string{"(no plan returned — query may be too simple or unsupported)"}
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type:        "text",
						Text:        "Execution plan:\n\n" + strings.Join(planLines, "\n"),
						Annotations: annBothExplain,
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
		// Extract client's protocolVersion and echo it back (spec MUST requirement)
		clientVersion := "2025-11-25" // default to latest spec version
		if req.Params != nil {
			if paramBytes, err := json.Marshal(req.Params); err == nil {
				var initParams InitializeParams
				if err := json.Unmarshal(paramBytes, &initParams); err == nil && initParams.ProtocolVersion != "" {
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
					Name:    "mcp-go-mssql",
					Title:   "MSSQL Database Connector",
					Version: "1.0.0",
				},
				Instructions: "This server provides secure access to a Microsoft SQL Server database. Use get_database_info to check connection status, explore to discover tables/views/procedures, inspect to examine table structure, query_database to run SQL queries, execute_procedure to call whitelisted stored procedures, and explain_query to analyze query execution plans.",
			},
		}

	case "ping":
		// MCP spec MUST: respond promptly to ping with empty result
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
		// JSON-RPC 2.0: if message has an ID it's a request and MUST get a response
		if req.ID != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]interface{}{},
			}
		}
		return nil

	case "tools/list":
		tools := []Tool{
			{
				Name:        "query_database",
				Title:       "Query Database",
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
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			},
			{
				Name:        "get_database_info",
				Title:       "Get Database Info",
				Description: "Get database connection status and basic information",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
					Required:   []string{},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			{
				Name:        "explore",
				Title:       "Explore Database",
				Description: "Explore database objects. type=tables (default) lists tables/views, type=views lists views with metadata (check_option, is_updatable, definition preview), type=databases lists all databases, type=procedures lists stored procedures, type=search searches objects by name or source definition (requires pattern).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"type": {
							Type:        "string",
							Description: "What to explore: 'tables' (default), 'views', 'databases', 'procedures', 'search'",
						},
						"filter": {
							Type:        "string",
							Description: "Name filter for tables/procedures (LIKE match, e.g. 'Pedido')",
						},
						"schema": {
							Type:        "string",
							Description: "Schema filter for procedures (optional)",
						},
						"pattern": {
							Type:        "string",
							Description: "Search pattern. Required when type=search.",
						},
						"search_in": {
							Type:        "string",
							Description: "Where to search: 'name' (default) or 'definition' (inside procedure/view source code)",
						},
					},
					Required: []string{},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			{
				Name:        "inspect",
				Title:       "Inspect Table",
				Description: "Inspect a table's structure. detail=columns (default) returns column info, detail=indexes returns indexes, detail=foreign_keys returns FK relationships, detail=dependencies returns objects (views, procedures, functions) that reference this table, detail=all returns everything in one call.",
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
						"detail": {
							Type:        "string",
							Description: "What to retrieve: 'columns' (default), 'indexes', 'foreign_keys', 'dependencies', 'all'",
						},
					},
					Required: []string{"table_name"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			},
			{
				Name:        "execute_procedure",
				Title:       "Execute Procedure",
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
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(true),
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			},
			{
				Name:        "explain_query",
				Title:       "Explain Query",
				Description: "Show the estimated execution plan for a SQL query without executing it. Useful for performance analysis and query optimization. Only SELECT queries are accepted.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {
							Type:        "string",
							Description: "SELECT query to analyze (must be a read-only query)",
						},
					},
					Required: []string{"query"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
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
		// JSON-RPC 2.0: if message has an ID it's a request and MUST get a response
		if req.ID != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]interface{}{},
			}
		}
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

	// Cache configuration once at startup (avoid os.Getenv on every request)
	cfg := serverConfig{
		readOnly:        strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) == "true",
		whitelistTables: parseWhitelistTables(os.Getenv("MSSQL_WHITELIST_TABLES")),
		whitelistProcs:  os.Getenv("MSSQL_WHITELIST_PROCEDURES"),
	}

	// Create MCP server without database initially
	server := &MCPMSSQLServer{
		db:        nil,
		secLogger: secLogger,
		devMode:   devMode,
		config:    cfg,
	}
	// Initialize rate limiter: 60 tool calls per minute
	server.rateLimiter.maxTokens = 60
	server.rateLimiter.tokens = 60
	server.rateLimiter.lastReset = time.Now()
	server.rateLimiter.interval = time.Minute

	// Try to establish database connection (non-fatal)
	// Use context for cancellation and WaitGroup for clean shutdown
	connCtx, connCancel := context.WithCancel(context.Background())
	var connWg sync.WaitGroup
	connWg.Add(1)
	go func() {
		defer connWg.Done()
		// Give MCP protocol time to initialize
		select {
		case <-time.After(2 * time.Second):
		case <-connCtx.Done():
			return
		}

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

		// Log security settings (using cached config)
		if server.config.readOnly {
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
	// Set explicit buffer limit (4MB) to prevent silent truncation and limit memory usage
	const maxScanBuf = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanBuf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			secLogger.Printf("Invalid JSON received: %v", err)
			// MCP spec MUST: respond with -32700 Parse error for invalid JSON
			parseErrResp := &MCPResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &MCPError{
					Code:    -32700,
					Message: "Parse error",
				},
			}
			if respBytes, err := json.Marshal(parseErrResp); err == nil {
				fmt.Println(string(respBytes))
			}
			continue
		}

		// MCP spec: all messages MUST be JSON-RPC 2.0
		if req.JSONRPC != "2.0" {
			invalidResp := &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32600,
					Message: "Invalid Request: missing or incorrect jsonrpc version, must be \"2.0\"",
				},
			}
			if respBytes, err := json.Marshal(invalidResp); err == nil {
				fmt.Println(string(respBytes))
			}
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

	// Clean shutdown: cancel connection goroutine and wait for it
	connCancel()
	connWg.Wait()

	// Close database connection if it was established
	if db := server.getDB(); db != nil {
		if err := db.Close(); err != nil {
			secLogger.Printf("Error closing database connection: %v", err)
		} else {
			secLogger.Printf("Database connection closed")
		}
	}
}
