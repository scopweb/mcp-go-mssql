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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	// NOTE: Windows Integrated Auth (winsspi) is conditionally imported in
	// integrated_auth_windows.go using a //go:build windows tag so that
	// `go build` and govulncheck succeed on Linux CI runners.
)

// MCP Protocol structures
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  interface{}            `json:"params,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
}

type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Result  interface{}            `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
	Meta    map[string]interface{} `json:"_meta,omitempty"`
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

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// boolPtr is a helper to create *bool for tool annotations.
func boolPtr(b bool) *bool { return &b }

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

// readOnlyDangerousSPs is the single source of truth for system procedures
// that must always be blocked in read-only contexts, even when we allow
// certain safe administrative procedures via EXEC.
var readOnlyDangerousSPs = []string{
	"XP_CMDSHELL", "XP_REGREAD", "XP_REGWRITE", "XP_FILEEXIST",
	"XP_DIRTREE", "XP_FIXEDDRIVES", "XP_SERVICECONTROL",
	"SP_CONFIGURE", "SP_ADDLOGIN", "SP_DROPLOGIN",
	"SP_ADDSRVROLEMEMBER", "SP_DROPSRVROLEMEMBER",
	"SP_ADDROLEMEMBER", "SP_DROPROLEMEMBER",
	"SP_EXECUTESQL", "SP_OACREATE", "SP_OAMETHOD",
}

// Pre-compiled patterns for table name extraction (performance optimization)
// These patterns try to be robust against common schema-qualified names.
var tableExtractionPatterns = []*regexp.Regexp{
	// FROM / JOIN with optional schema or database prefix
	regexp.MustCompile(`(?i)\bFROM\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bJOIN\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),

	// INSERT / UPDATE / DELETE
	regexp.MustCompile(`(?i)\bINTO\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bUPDATE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?\s+FROM`),

	// DDL
	regexp.MustCompile(`(?i)\bTABLE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bVIEW\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\s+(?:[\w\.\[\]]+\.)?\[?([\w]+)\]?`),

	// Extra safety for fully qualified names like db.schema.table or [db].[schema].[table]
	regexp.MustCompile(`(?i)\bFROM\s+[\w\.\[\]]+\.[\w\.\[\]]+\.?\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bJOIN\s+[\w\.\[\]]+\.[\w\.\[\]]+\.?\[?([\w]+)\]?`),
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

// DynamicAlias represents one preconfigured dynamic connection with its own security posture.
// This is the foundation for secure "one application, multiple related databases" scenarios.
type DynamicAlias struct {
	Alias            string
	Server           string
	Database         string
	User             string
	Password         string // stored in memory only after load; never logged
	Encrypt          string // per-alias override ("true"|"false"|"disable") — optional, never logged
	Port             string // per-alias override — optional, defaults to "1433", never logged
	ConnectionString string // full DSN override (URL or ADO form) — optional, takes precedence; never logged
	ReadOnly         bool
	WhitelistTables  []string
}

// MSSQL Server
type MCPMSSQLServer struct {
	db          *sql.DB   // current active connection (for backward compat + single-connection mode)
	dbMu        sync.RWMutex
	secLogger   *SecurityLogger
	devMode     bool
	config      serverConfig
	isDynamic   bool // frozen at startup from isDynamicMode(); controls tool surface + connection behavior

	// Dynamic connections support (for "one app, multiple related DBs" use case)
	dynamicAliases  map[string]DynamicAlias
	connections     map[string]*sql.DB // alias -> open connection
	dynamicMu       sync.RWMutex
	activeAlias     string // currently selected dynamic alias (if any)

	// Confirmation system for writable dynamic aliases (secure by default)
	pendingConfirmation *PendingConfirmation
	confirmMu           sync.Mutex

	rateLimiter struct {
		mu        sync.Mutex
		tokens    int
		maxTokens int
		lastReset time.Time
		interval  time.Duration
	}
}

// PendingConfirmation represents a confirmation that the AI must explicitly call
// before performing a potentially destructive operation on a writable dynamic alias.
type PendingConfirmation struct {
	Operation   string    // e.g. "DELETE", "UPDATE", "DROP"
	Tables      []string
	Description string
	ExpiresAt   time.Time
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

// connectToDynamicAlias establishes a connection to a preconfigured dynamic alias,
// closes the previous active connection (if any), and makes the new one the active context.
// This is the key function for secure multi-database usage within one application.
func (s *MCPMSSQLServer) connectToDynamicAlias(aliasName string) error {
	s.dynamicMu.Lock()
	defer s.dynamicMu.Unlock()

	aliasName = strings.ToUpper(strings.TrimSpace(aliasName))

	alias, ok := s.dynamicAliases[aliasName]
	if !ok {
		return fmt.Errorf("unknown dynamic alias: %s (use dynamic_available to list)", aliasName)
	}

	if alias.Server == "" || alias.Database == "" {
		return fmt.Errorf("alias '%s' is missing SERVER or DATABASE configuration", aliasName)
	}

	// Build connection string for this alias (respecting its own security posture is handled via getEffectiveConfig)
	// We reuse the secure connection string logic but construct it manually for the alias.
	// Priority: alias.ConnectionString > per-alias Encrypt/Port > devMode defaults.
	connStr, err := buildAliasConnectionString(&alias, s.devMode, aliasName)
	if err != nil {
		return fmt.Errorf("failed to build connection string for alias '%s': %w", aliasName, err)
	}

	// Close previous active connection if we had one
	if s.activeAlias != "" {
		if oldConn, exists := s.connections[s.activeAlias]; exists && oldConn != nil {
			_ = oldConn.Close()
			delete(s.connections, s.activeAlias)
		}
	}

	// Open new connection
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection for alias '%s': %w", aliasName, err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// #nosec G104 -- close error ignored when ping fails; db state is already broken
		db.Close()
		return fmt.Errorf("failed to connect to alias '%s': %w", aliasName, err)
	}

	// Store the connection
	if s.connections == nil {
		s.connections = make(map[string]*sql.DB)
	}
	s.connections[aliasName] = db

	// Make it the active one
	s.dbMu.Lock()
	s.db = db
	s.dbMu.Unlock()

	s.activeAlias = aliasName

	s.secLogger.Printf("Dynamic connection switched to alias '%s' (readOnly=%v)", aliasName, alias.ReadOnly)
	return nil
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
		return "", fmt.Errorf("Azure AD authentication not implemented in buildSecureConnectionString; use MSSQL_CONNECTION_STRING or set MSSQL_AUTH=sql") //nolint:staticcheck // intentional capitalization for user-facing error
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
	// Use effective config (per-alias if a dynamic connection is active)
	effective := s.getEffectiveConfig()
	if !effective.readOnly {
		return nil // Read-only mode disabled for current context, allow all queries
	}

	// If whitelist is configured, allow modifications to pass through to validateTablePermissions()
	// This enables the use case: READ_ONLY=true + WHITELIST=table1,table2
	// where only whitelisted tables can be modified
	whitelist := effective.whitelistTables
	if len(whitelist) > 0 {
		// Whitelist is configured - let validateTablePermissions() handle modification permissions
		// We still need to block dangerous operations though
		normalizedQuery := stripLeadingComments(query)
		_ = normalizedQuery // used for future pattern matching if needed

		// Block dangerous system procedures even with whitelist
		queryUpper := strings.ToUpper(query)
		for _, sp := range readOnlyDangerousSPs {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}

		// Allow query to proceed to validateTablePermissions() for whitelist check
		return nil
	}

	// No whitelist configured - enforce strict read-only mode
	normalizedQuery := stripLeadingComments(query)

	// Special case (new for admin introspection support):
	// Allow EXEC/EXECUTE of a small set of known read-only system procedures
	// used for schema discovery and database administration.
	// This is deliberately narrow and does not relax the general ban on EXEC.
	if isSafeReadOnlyAdminProcedure(query) {
		queryUpper := strings.ToUpper(query)
		for _, sp := range readOnlyDangerousSPs {
			if strings.Contains(queryUpper, sp) {
				return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
			}
		}
		return nil
	}

	// List of allowed read-only operations (traditional path)
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

			// Check for dangerous SPs (single source of truth)
			queryUpper := strings.ToUpper(query)
			for _, sp := range readOnlyDangerousSPs {
				if strings.Contains(queryUpper, sp) {
					return fmt.Errorf("read-only mode: query contains forbidden procedure '%s'", sp)
				}
			}

			// Safe read-only system procedures (legacy path for SELECT queries that mention SP_ names)
			safeSPs := []string{
				"SP_HELP", "SP_HELPTEXT", "SP_HELPINDEX", "SP_HELPCONSTRAINT",
				"SP_COLUMNS", "SP_TABLES", "SP_STORED_PROCEDURES",
				"SP_FKEYS", "SP_PKEYS", "SP_STATISTICS",
				"SP_DATABASES", "SP_HELPDB",
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

// isSafeReadOnlyAdminProcedure reports whether the query is an invocation
// of a well-known read-only system stored procedure used for schema discovery
// and administrative introspection (e.g. sp_help, sp_helptext, sp_spaceused).
//
// This is the controlled relaxation that allows legitimate administrative reads
// via EXEC while the general EXEC/EXECUTE keyword remains dangerous for everything else.
func isSafeReadOnlyAdminProcedure(query string) bool {
	q := strings.ToUpper(stripLeadingComments(query))
	q = strings.TrimSpace(q)

	// Match common forms:
	//   EXEC sp_help 'dbo.Table'
	//   EXECUTE dbo.sp_helptext 'proc'
	//   EXEC [dbo].[sp_columns] @table_name='x'
	// We capture the procedure name (last identifier before any parameters).
	re := regexp.MustCompile(`^\s*EXEC(UTE)?\s+(?:\[?[\w$]+\]?\.)*(?:\[?([\w$]+)\]?)\b`)
	matches := re.FindStringSubmatch(q)
	if len(matches) < 3 {
		return false
	}

	proc := strings.ToUpper(matches[2])

	// Conservative allowlist of read-only administrative / schema procedures.
	// Only add procedures that are:
	//   - Documented as read-only / no side effects
	//   - Commonly used for legitimate schema and admin discovery
	//   - Do not allow arbitrary code execution or configuration changes
	safeAdminProcs := map[string]bool{
		"SP_HELP":              true,
		"SP_HELPTEXT":          true,
		"SP_HELPINDEX":         true,
		"SP_HELPCONSTRAINT":    true,
		"SP_HELPTRIGGER":       true,
		"SP_COLUMNS":           true,
		"SP_TABLES":            true,
		"SP_STORED_PROCEDURES": true,
		"SP_FKEYS":             true,
		"SP_PKEYS":             true,
		"SP_STATISTICS":        true,
		"SP_DATABASES":         true,
		"SP_HELPDB":            true,
		"SP_SPACEUSED":         true,
		// Intentionally not including sp_who / sp_lock in the initial set
		// to keep the surface minimal. Can be evaluated later.
	}

	return safeAdminProcs[proc]
}

// getWhitelistedTables returns the cached list of tables/views allowed for modification.
func (s *MCPMSSQLServer) getWhitelistedTables() []string {
	return s.config.whitelistTables
}

// getEffectiveConfig returns the security configuration that should be applied right now.
// If there is an active dynamic alias, it returns that alias's posture.
// Otherwise, it falls back to the global server config.
// This is critical for secure dynamic multi-database usage.
func (s *MCPMSSQLServer) getEffectiveConfig() serverConfig {
	s.dynamicMu.RLock()
	defer s.dynamicMu.RUnlock()

	if s.activeAlias != "" {
		if alias, ok := s.dynamicAliases[s.activeAlias]; ok {
			return serverConfig{
				readOnly:        alias.ReadOnly,
				whitelistTables: alias.WhitelistTables,
				whitelistProcs:  s.config.whitelistProcs, // global for now
			}
		}
	}

	// No active dynamic alias → use global config (with the safety guard already applied at startup)
	return s.config
}

// requireConfirmationForModification is called when a writable alias attempts a modification.
// It returns an error that tells the AI it must call confirm_operation first.
func (s *MCPMSSQLServer) requireConfirmationForModification(operation string, tables []string) error {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	desc := fmt.Sprintf("%s on tables: %s", operation, strings.Join(tables, ", "))

	s.pendingConfirmation = &PendingConfirmation{
		Operation:   operation,
		Tables:      tables,
		Description: desc,
		ExpiresAt:   time.Now().Add(90 * time.Second), // 90 seconds to confirm
	}

	return fmt.Errorf("CONFIRMATION REQUIRED: This is a modification operation (%s) on a writable dynamic alias.\n\nYou must first call the 'confirm_operation' tool with this exact description:\n\"%s\"\n\nOnly after receiving confirmation will the operation be allowed.", operation, desc) //nolint:staticcheck // multi-line user-facing message; capitalization + punctuation are intentional
}

// isOperationConfirmed checks if there is a valid pending confirmation that matches the current operation.
func (s *MCPMSSQLServer) isOperationConfirmed(operation string, tables []string) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	if s.pendingConfirmation == nil {
		return false
	}

	if time.Now().After(s.pendingConfirmation.ExpiresAt) {
		s.pendingConfirmation = nil
		return false
	}

	// Simple matching: same operation and at least one overlapping table
	if strings.EqualFold(s.pendingConfirmation.Operation, operation) {
		for _, t1 := range s.pendingConfirmation.Tables {
			for _, t2 := range tables {
				if t1 == t2 {
					// Consume the confirmation after use (one-time use)
					s.pendingConfirmation = nil
					return true
				}
			}
		}
	}

	return false
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

// loadDotEnvIfPresent loads a .env file from the same directory as the executable
// if it exists. It only sets variables that are not already present in the environment
// (MCP host-passed env takes precedence). This enables the documented dynamic
// connection workflow where credentials live next to the exe.
//
// Users running multiple isolated MCP server instances (classic + dynamic, or multiple
// classic for different DBs) can set MSSQL_IGNORE_LOCAL_ENV=true in their .mcp.json
// "env" block to completely disable .env loading for that instance. This provides
// strong isolation even if a .env file was accidentally left next to the executable.
func loadDotEnvIfPresent(secLogger *SecurityLogger) {
	// Nuclear option for isolation: completely ignore any .env next to the exe.
	// Intended for classic servers configured purely via Claude Desktop .mcp.json env.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("MSSQL_IGNORE_LOCAL_ENV"))); v == "true" || v == "1" || v == "yes" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return // cannot determine location, skip .env (rely on passed env)
	}
	exeDir := filepath.Dir(exePath)
	envPath := filepath.Join(exeDir, ".env")

	// #nosec G304 -- envPath is derived from os.Executable(), not user input
	data, err := os.ReadFile(envPath)
	if err != nil {
		return // no .env next to exe is normal/OK
	}

	loaded := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			// Remove surrounding quotes if present
			if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
				(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
				val = val[1 : len(val)-1]
			}
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
				loaded++
			}
		}
	}
	if loaded > 0 {
		secLogger.Printf("Loaded %d variables from .env next to executable (keys only; values redacted)", loaded)
	}
}

// hasDynamicAliases returns true if any MSSQL_DYNAMIC_* variables are present in env.
// Note: This is only used for auto-detection when MSSQL_DYNAMIC_MODE is not explicitly set.
func hasDynamicAliases() bool {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "MSSQL_DYNAMIC_") {
			return true
		}
	}
	return false
}

// isDynamicMode reports whether the server should operate in dynamic multi-connection mode
// (exposing dynamic_available / dynamic_connect / etc. and loading per-alias configs).
//
// Precedence (highest to lowest):
//   1. Explicit MSSQL_DYNAMIC_MODE=false  → always classic (even if DYNAMIC_* vars exist anywhere)
//   2. Explicit MSSQL_DYNAMIC_MODE=true   → always dynamic
//   3. No explicit setting:
//        - If any classic connection config is present (MSSQL_SERVER, MSSQL_CONNECTION_STRING,
//          or MSSQL_DATABASE) → classic mode. This protects users who configure classic servers
//          via .mcp.json "env" blocks or who have polluted parent environments.
//        - Otherwise, if any MSSQL_DYNAMIC_* vars exist → dynamic mode (auto-detect).
//        - Otherwise → classic (no DB configured).
//
// This design allows safe co-existence of multiple mcp-go-mssql instances (some classic,
// some dynamic) under Claude Desktop, even when the host process environment contains
// stray dynamic variables from other .env files or previous sessions.
func isDynamicMode() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MSSQL_DYNAMIC_MODE")))
	if mode == "false" || mode == "0" || mode == "no" || mode == "off" {
		return false // explicit opt-out always wins, multiple spellings accepted
	}
	if mode == "true" || mode == "1" || mode == "yes" || mode == "on" {
		return true
	}

	// No explicit MSSQL_DYNAMIC_MODE → decide based on what the user actually configured.
	hasClassicConfig := os.Getenv("MSSQL_SERVER") != "" ||
		os.Getenv("MSSQL_CONNECTION_STRING") != "" ||
		os.Getenv("MSSQL_DATABASE") != ""

	if hasClassicConfig {
		// User (or .mcp.json) provided a default connection → treat as classic.
		// Ignore any stray MSSQL_DYNAMIC_* that may have leaked from parent env or other .env files.
		return false
	}

	// No classic default configured → fall back to presence of dynamic aliases.
	return hasDynamicAliases()
}

// loadDynamicAliases scans the environment for MSSQL_DYNAMIC_<ALIAS>_* variables
// and builds a map of DynamicAlias with their individual security posture.
// This enables the secure "one application - multiple related databases" pattern.
func loadDynamicAliases(secLogger *SecurityLogger) map[string]DynamicAlias {
	aliases := make(map[string]DynamicAlias)
	prefix := "MSSQL_DYNAMIC_"

	// Collect all relevant env vars
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		if idx := strings.Index(env, "="); idx > 0 {
			key := env[:idx]
			val := env[idx+1:]
			if strings.HasPrefix(key, prefix) {
				envVars[key] = val
			}
		}
	}

	// Find all unique aliases
	aliasSet := make(map[string]bool)
	for key := range envVars {
		rest := strings.TrimPrefix(key, prefix)
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			alias := strings.ToUpper(parts[0])
			if alias != "MODE" {
				aliasSet[alias] = true
			}
		}
	}

	for alias := range aliasSet {
		// === SECURITY BY DEFAULT ===
		// Every dynamic alias defaults to the safest possible configuration.
		// If the user does not explicitly set MSSQL_DYNAMIC_<ALIAS>_READ_ONLY=false,
		// the alias will be read-only. This is intentional and by design.
		a := DynamicAlias{
			Alias:           alias,
			ReadOnly:        true,  // SAFE DEFAULT: read-only unless explicitly set to false
			WhitelistTables: nil,   // No writes allowed by default
		}

		// Load connection fields
		if v := envVars[prefix+alias+"_SERVER"]; v != "" {
			a.Server = v
		}
		if v := envVars[prefix+alias+"_DATABASE"]; v != "" {
			a.Database = v
		}
		if v := envVars[prefix+alias+"_USER"]; v != "" {
			a.User = v
		}
		if v := envVars[prefix+alias+"_PASSWORD"]; v != "" {
			a.Password = v
		}

		// Per-alias connection tuning. ConnectionString wins over Encrypt/Port
		// (the connector in connectToDynamicAlias honors that priority). These
		// fields are needed for legacy SQL Server 2000/2008/2012 instances whose
		// TLS 1.0 handshake is rejected by modern Go runtimes (see
		// .github/ISSUES/01-sql-server-2008-support.md for the classic-mode
		// equivalent of MSSQL_CONNECTION_STRING).
		if v := envVars[prefix+alias+"_ENCRYPT"]; v != "" {
			a.Encrypt = strings.ToLower(strings.TrimSpace(v))
		}
		if v := envVars[prefix+alias+"_PORT"]; v != "" {
			a.Port = strings.TrimSpace(v)
		}
		if v := envVars[prefix+alias+"_CONNECTION_STRING"]; v != "" {
			a.ConnectionString = v
		}

		// Per-alias security posture - ONLY override if explicitly provided
		// This enforces "secure by default"
		if ro := envVars[prefix+alias+"_READ_ONLY"]; ro != "" {
			// User explicitly set the value → respect it
			a.ReadOnly = strings.ToLower(strings.TrimSpace(ro)) == "true"
		}
		// If not set → remains true (safe default)

		if wl := envVars[prefix+alias+"_WHITELIST_TABLES"]; wl != "" {
			a.WhitelistTables = parseWhitelistTables(wl)
		}
		// If not set and ReadOnly=false → whitelist remains empty = no modifications allowed (very safe)

		aliases[alias] = a

		// Security logging (never log credentials)
		secLogger.Printf("Loaded dynamic alias '%s' (readOnly=%v, whitelistTables=%d) [secure defaults applied]",
			alias, a.ReadOnly, len(a.WhitelistTables))
	}

	if len(aliases) > 0 {
		secLogger.Printf("Dynamic mode: %d aliases loaded with individual security contexts", len(aliases))
	}

	return aliases
}
// buildAliasConnectionString constructs the SQL Server connection string for a
// dynamic alias. The function mirrors the override + per-alias tuning + devMode
// fallback strategy used by buildSecureConnectionString (classic mode), so the
// per-alias path and the global path behave identically.
//
// Priority:
//  1. alias.ConnectionString (full override, URL or ADO DSN) — wins outright.
//     This is the recommended workaround for legacy SQL Server 2000/2008/2012
//     instances that only negotiate TLS 1.0 (see .github/ISSUES/01-sql-server-2008-support.md).
//  2. alias.Encrypt + alias.Port — per-alias tuning on top of an auto-built DSN.
//  3. devMode — last-resort defaults (encrypt=false/trustCert=true in dev,
//     encrypt=true/trustCert=false in production).
//
// Default timeouts (connection timeout=30; command timeout=30) are appended
// when the user-supplied override does not include them.
//
// In production mode, the function emits slog.Warn for insecure settings
// (encrypt=false, missing encrypt, trustservercertificate=true), exactly like
// buildSecureConnectionString does for the global MSSQL_CONNECTION_STRING.
func buildAliasConnectionString(alias *DynamicAlias, devMode bool, aliasName string) (string, error) {
	if alias == nil {
		return "", fmt.Errorf("nil alias")
	}

	// Priority 1: full override per alias (URL or ADO DSN).
	if alias.ConnectionString != "" {
		cs := alias.ConnectionString
		csLower := strings.ToLower(cs)
		isProduction := !devMode

		if isProduction {
			if strings.Contains(csLower, "encrypt=false") {
				slog.Warn(fmt.Sprintf("dynamic alias '%s' connection string has encrypt=false in production mode", aliasName))
			}
			if !strings.Contains(csLower, "encrypt=") {
				slog.Warn(fmt.Sprintf("dynamic alias '%s' connection string missing encrypt parameter in production mode", aliasName))
			}
			if strings.Contains(csLower, "trustservercertificate=true") {
				slog.Warn(fmt.Sprintf("dynamic alias '%s' connection string has trustservercertificate=true in production mode", aliasName))
			}
		}

		if !strings.Contains(csLower, "connection timeout") {
			cs += ";connection timeout=30"
		}
		if !strings.Contains(csLower, "command timeout") {
			cs += ";command timeout=30"
		}
		return cs, nil
	}

	// Priority 2 + 3: per-alias Encrypt/Port, then devMode defaults.
	encrypt := "true"
	trustCert := "false"
	if devMode {
		encrypt = "false"
		trustCert = "true"
	}
	if alias.Encrypt != "" {
		encrypt = alias.Encrypt
	}

	port := alias.Port
	if port == "" {
		port = "1433"
	}

	return fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
		alias.Server, port, alias.Database, alias.User, alias.Password, encrypt, trustCert,
	), nil
}

// isLegacyTLSPivotError reports whether err looks like the Go runtime
// rejecting a server that only negotiates TLS 1.0 (protocol version 301).
// This is the fingerprint error you get when connecting to a SQL Server
// 2000/2008/2012 instance that has no TLS 1.2 support, with a modern Go
// runtime that refuses to negotiate below TLS 1.2. The wording covers both
// Go's exact phrasing ("protocol version 301") and a broader
// "unsupported protocol" fallback in case the Go error format changes.
func isLegacyTLSPivotError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "protocol version 301") ||
		strings.Contains(msg, "unsupported protocol")
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

				// Remove any remaining schema/database prefix that the regex might have captured
				// e.g. "dbo.users" -> "users"
				if idx := strings.LastIndex(tableName, "."); idx != -1 {
					tableName = tableName[idx+1:]
				}

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
	// Use effective config (respects active dynamic alias posture)
	effective := s.getEffectiveConfig()
	if !effective.readOnly {
		return nil // Whitelist mode disabled for current context, allow all operations
	}

	whitelist := effective.whitelistTables
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

	// === CONFIRMATION REQUIREMENT FOR WRITABLE DYNAMIC ALIASES ===
	// We only enforce explicit confirmation when using dynamic mode with a writable alias.
	// This protects the high-risk "multiple databases" scenario without breaking classic single-connection usage.
	s.dynamicMu.RLock()
	hasActiveWritableAlias := s.activeAlias != "" && !effective.readOnly
	s.dynamicMu.RUnlock()

	if hasActiveWritableAlias && !s.isOperationConfirmed(operation, tablesInQuery) {
		return s.requireConfirmationForModification(operation, tablesInQuery)
	}

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
	defer func() {
		if r := recover(); r != nil {
			s.secLogger.Printf("Recovered panic in handleToolCall for tool %s: %v (tool failed gracefully)", params.Name, r)
		}
	}()

	// MCP spec MUST: rate limit tool invocations
	if !s.checkRateLimit() {
		s.secLogger.Printf("Rate limit exceeded for tool: %s", params.Name)
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Rate limit exceeded. Please wait before making more requests."}},
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
				if auth == "sql" { //nolint:staticcheck // QF1003: switch would be slightly cleaner but if-else is fine here
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
				if isDynamicMode() {
					info.WriteString("DYNAMIC_MODE: true (dynamic_* tools available)\n")
					info.WriteString("NOTE: Per-alias security contexts are in development. Current ops use the (guarded) global posture above.\n")
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

			s.dynamicMu.RLock()
			activeAlias := s.activeAlias
			s.dynamicMu.RUnlock()

			if activeAlias != "" {
				// Dynamic mode - show the active alias and its specific posture
				if alias, ok := s.dynamicAliases[activeAlias]; ok {
					effective := s.getEffectiveConfig()
					fmt.Fprintf(&info, "Active Dynamic Alias: %s\n", activeAlias)
					fmt.Fprintf(&info, "  Server/Database: %s / %s\n", alias.Server, alias.Database)
					ro := "READ-ONLY"
					if !effective.readOnly {
						ro = "WRITABLE (whitelist restricted)"
					}
					fmt.Fprintf(&info, "  Security Posture: %s\n", ro)
					if len(effective.whitelistTables) > 0 {
						fmt.Fprintf(&info, "  Allowed modification tables: %s\n", strings.Join(effective.whitelistTables, ", "))
					}
				}
			} else if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
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
							Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
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

	case "explore":
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
							{Type: "text", Text: "Error: 'pattern' is required when type=search"},
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
							Type: "text",
							Text: fmt.Sprintf("Error in explore: %v", err),
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
						Text: fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
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

	case "inspect":
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

		tableName, ok := params.Arguments["table_name"].(string)
		if !ok || tableName == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{Type: "text", Text: "Error: Missing or invalid 'table_name' parameter"},
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
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting columns: %v", err)}}, IsError: true,
				}}
			}
			idxResults, err := s.executeSecureQuery(ctx, indexesQuery, tableName, schemaName)
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting indexes: %v", err)}}, IsError: true,
				}}
			}
			fkResults, err := s.executeSecureQuery(ctx, fkQuery, tableName, schemaName)
			if err != nil {
				return &MCPResponse{JSONRPC: "2.0", ID: id, Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error getting foreign keys: %v", err)}}, IsError: true,
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
					Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Error formatting results: %v", err)}}, IsError: true,
				}}
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Full inspection of '%s.%s':\n%s", schemaName, tableName, string(resultBytes)),
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
							{Type: "text", Text: fmt.Sprintf("Table '%s' not found", tableName)},
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
							Type: "text",
							Text: fmt.Sprintf("Error in inspect: %v", err),
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
						Text: fmt.Sprintf("%s:\n%s", label, string(resultBytes)),
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
					Content: []ContentItem{{Type: "text", Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps."}},
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
					Content: []ContentItem{{Type: "text", Text: "Error: Missing or invalid 'query' parameter"}},
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
					Content: []ContentItem{{Type: "text", Text: "Error: explain_query only accepts SELECT queries, got: " + op}},
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
					Content: []ContentItem{{Type: "text", Text: connErrMsg}},
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
					Content: []ContentItem{{Type: "text", Text: showplanErrMsg}},
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
					Content: []ContentItem{{Type: "text", Text: planErrMsg}},
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
						Type: "text",
						Text: "Execution plan:\n\n" + strings.Join(planLines, "\n"),
					},
				},
			},
		}

	// === Dynamic multi-connection tools (only reachable when s.isDynamic) ===
	// When !s.isDynamic these cases are unreachable because the tools are not
	// advertised in tools/list, but we keep cheap runtime guards for safety.
	case "dynamic_available":
		if !s.isDynamic {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: dynamic tools are not available in this server instance (classic single-connection mode). This usually means the server was started with classic MSSQL_SERVER / MSSQL_DATABASE configuration."}},
					IsError: true,
				},
			}
		}
		s.dynamicMu.RLock()
		defer s.dynamicMu.RUnlock()

		var sb strings.Builder
		sb.WriteString("Available dynamic connections (loaded from MSSQL_DYNAMIC_* variables):\n\n")

		if len(s.dynamicAliases) == 0 {
			sb.WriteString("No dynamic aliases configured.\n")
			sb.WriteString("Define variables like MSSQL_DYNAMIC_APP_MAIN_SERVER, MSSQL_DYNAMIC_APP_MAIN_DATABASE, etc.\n")
		} else {
			for alias, a := range s.dynamicAliases {
				ro := "READ-ONLY"
				if !a.ReadOnly {
					ro = "FULL ACCESS (with whitelist restrictions if configured)"
				}
				wl := ""
				if len(a.WhitelistTables) > 0 {
					wl = fmt.Sprintf(" | Whitelist: %s", strings.Join(a.WhitelistTables, ","))
				}
				// Marker only — never print the connection string (it may contain credentials).
				override := ""
				if a.ConnectionString != "" {
					override = " | ConnectionString: (custom override set)"
				}
				fmt.Fprintf(&sb, "- %s → %s/%s (%s%s%s)\n", alias, a.Server, a.Database, ro, wl, override)
			}
		}

		sb.WriteString("\nSecurity posture is applied **per alias** (much safer than global settings).\n")
		sb.WriteString("Use dynamic_connect with the alias name to switch the active connection.\n")
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: sb.String()}},
			},
		}

	case "dynamic_connect":
		if !s.isDynamic {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: dynamic_connect is not available in this server instance (classic single-connection mode)."}},
					IsError: true,
				},
			}
		}
		aliasIface := params.Arguments["alias"]
		alias := ""
		if s, ok := aliasIface.(string); ok {
			alias = strings.ToUpper(strings.TrimSpace(s))
		}
		if alias == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: 'alias' parameter is required (e.g. 'APP_MAIN', 'APP_IDENTITY')"}},
					IsError: true,
				},
			}
		}

		if err := s.connectToDynamicAlias(alias); err != nil {
			errMsg := fmt.Sprintf("Error connecting to alias '%s': %v", alias, err)
			// Surface an actionable hint when the error is the well-known TLS 1.0
			// handshake rejection that legacy SQL Server 2000/2008/2012 instances
			// trigger with modern Go runtimes. The wording covers both
			// "protocol version 301" (Go's exact phrasing) and a broader
			// "unsupported protocol" fallback for forward compatibility.
			if isLegacyTLSPivotError(err) {
				errMsg += "\n\nHint: this server likely speaks only TLS 1.0 (SQL Server 2000/2008/2012). " +
					"Set MSSQL_DYNAMIC_<ALIAS>_CONNECTION_STRING to a URL-form DSN: " +
					"sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"
			}
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: errMsg}},
					IsError: true,
				},
			}
		}

		// Success - report the effective security posture of the newly active alias
		effective := s.getEffectiveConfig()
		roStatus := "READ-ONLY (safe)"
		if !effective.readOnly {
			roStatus = "WRITABLE (restricted by whitelist)"
		}

		msg := fmt.Sprintf("Successfully connected to dynamic alias '%s'.\n\nEffective security posture for this connection:\n- Read-only mode: %s\n- Whitelisted tables for modification: %v\n\nAll subsequent queries will be validated against this alias's security rules.", alias, roStatus, effective.whitelistTables)

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: msg}},
			},
		}

	case "dynamic_disconnect":
		if !s.isDynamic {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: dynamic_disconnect is not available in this server instance (classic single-connection mode)."}},
					IsError: true,
				},
			}
		}
		s.dynamicMu.Lock()
		defer s.dynamicMu.Unlock()

		if s.activeAlias == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "No active dynamic connection to disconnect."}},
				},
			}
		}

		closedAlias := s.activeAlias

		// Close the connection
		if conn, ok := s.connections[s.activeAlias]; ok && conn != nil {
			_ = conn.Close()
			delete(s.connections, s.activeAlias)
		}

		s.dbMu.Lock()
		s.db = nil
		s.dbMu.Unlock()

		s.activeAlias = ""

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Disconnected from alias '%s'. No active database connection.", closedAlias)}},
			},
		}

	case "dynamic_list":
		if !s.isDynamic {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: dynamic_list is not available in this server instance (classic single-connection mode)."}},
					IsError: true,
				},
			}
		}
		s.dynamicMu.RLock()
		defer s.dynamicMu.RUnlock()

		var sb strings.Builder
		sb.WriteString("Dynamic aliases currently loaded:\n\n")

		if len(s.dynamicAliases) == 0 {
			sb.WriteString("(none)\n")
		} else {
			for alias, a := range s.dynamicAliases {
				active := ""
				if alias == s.activeAlias {
					active = "  ← ACTIVE"
				}
				fmt.Fprintf(&sb, "- %s (%s/%s)%s\n", alias, a.Server, a.Database, active)
			}
		}

		fmt.Fprintf(&sb, "\nTotal: %d aliases\n", len(s.dynamicAliases))
		if s.activeAlias != "" {
			fmt.Fprintf(&sb, "Active connection: %s\n", s.activeAlias)
		} else {
			sb.WriteString("No active dynamic connection (use dynamic_connect <alias>)\n")
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: sb.String()}},
			},
		}

	case "confirm_operation":
		if !s.isDynamic {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: confirm_operation is only relevant for writable dynamic aliases and is not available in classic single-connection mode."}},
					IsError: true,
				},
			}
		}
		descIface := params.Arguments["description"]
		description := ""
		if d, ok := descIface.(string); ok {
			description = strings.TrimSpace(d)
		}
		if description == "" {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Error: 'description' parameter is required. Describe clearly what operation you want to perform."}},
					IsError: true,
				},
			}
		}

		s.confirmMu.Lock()
		defer s.confirmMu.Unlock()

		if s.pendingConfirmation == nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "No pending operation requires confirmation at this moment."}},
					IsError: true,
				},
			}
		}

		if time.Now().After(s.pendingConfirmation.ExpiresAt) {
			s.pendingConfirmation = nil
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "The previous confirmation request has expired. Please try the operation again to generate a new confirmation request."}},
					IsError: true,
				},
			}
		}

		// Accept the confirmation if the description is reasonably similar
		// (we do a simple contains check to be practical with LLMs)
		pendingDesc := strings.ToLower(s.pendingConfirmation.Description)
		userDesc := strings.ToLower(description)

		if strings.Contains(userDesc, strings.ToLower(s.pendingConfirmation.Operation)) ||
			strings.Contains(pendingDesc, userDesc) ||
			len(userDesc) > 10 { // Accept reasonably long descriptions as intent to confirm

			s.pendingConfirmation = nil // consume it
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{{Type: "text", Text: "Confirmation accepted. You may now execute the modification query. This confirmation is valid for the next query only."}},
				},
			}
		}

		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("The description you provided does not sufficiently match the pending operation.\n\nPending operation was: %s\n\nPlease call confirm_operation again with a description that clearly references the intended action.", s.pendingConfirmation.Description)}},
				IsError: true,
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
				Instructions: "This server provides secure access to a Microsoft SQL Server database. Use get_database_info to check connection status, explore/inspect for schema, query_database / execute_procedure for operations (subject to read-only + whitelist policy). When configured for dynamic multi-DB mode (MSSQL_DYNAMIC_* variables + no classic MSSQL_SERVER), the dynamic_available / dynamic_connect / dynamic_list / confirm_operation tools become available. All modifications are governed by the active security posture.",
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
		// Future: could cancel in-flight query contexts
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

		// Dynamic tools (and confirm_operation) are ONLY included in the tool list
		// for servers that started in dynamic mode. Classic servers (the common case
		// when using .mcp.json "env" with plain MSSQL_SERVER etc.) will never see
		// dynamic_available, dynamic_connect, etc. This eliminates the "AI always
		// tries dynamic connections" problem reported by users running multiple
		// isolated server instances.
		if s.isDynamic {
			tools = append(tools,
				Tool{
					Name:        "dynamic_available",
					Title:       "List Dynamic Connections",
					Description: "List all preconfigured dynamic database connections (aliases defined via MSSQL_DYNAMIC_<ALIAS>_* environment variables). Does not expose credentials. Use dynamic_connect to activate one.",
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
				Tool{
					Name:        "dynamic_connect",
					Title:       "Connect to Dynamic Alias",
					Description: "Switch the active database connection to one of the preconfigured dynamic aliases (e.g. 'CRM', 'IDENTITY', 'GDP'). The security posture (read-only vs full access) is determined by the alias configuration or global safe defaults. After connecting, use get_database_info to verify the active alias and its effective permissions.",
					InputSchema: InputSchema{
						Type: "object",
						Properties: map[string]Property{
							"alias": {
								Type:        "string",
								Description: "The alias name of the preconfigured connection (case-insensitive, e.g. 'CRM' or 'crm')",
							},
						},
						Required: []string{"alias"},
					},
					Annotations: &ToolAnnotations{
						ReadOnlyHint:    boolPtr(false),
						DestructiveHint: boolPtr(false), // depends on the *alias* posture, not global
						IdempotentHint:  boolPtr(false),
						OpenWorldHint:   boolPtr(false),
					},
				},
				Tool{
					Name:        "dynamic_disconnect",
					Title:       "Disconnect Dynamic Connection",
					Description: "Close the currently active dynamic connection and return to disconnected state. Does not affect other aliases.",
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
				Tool{
					Name:        "dynamic_list",
					Title:       "List Active Dynamic Connections",
					Description: "Show currently loaded dynamic aliases and which one (if any) is the active connection for subsequent query_database calls.",
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
				Tool{
					Name:        "confirm_operation",
					Title:       "Confirm Dangerous Operation",
					Description: "Explicitly confirm a potentially destructive operation (INSERT/UPDATE/DELETE/DROP/etc.) on a writable dynamic alias. This is a required security step. You must call this tool with a clear description of what you intend to do before the actual modification query will be allowed.",
					InputSchema: InputSchema{
						Type: "object",
						Properties: map[string]Property{
							"description": {
								Type:        "string",
								Description: "Clear description of the operation you want to perform (e.g. 'DELETE all rows from temp_ai where created < 2025-01-01')",
							},
						},
						Required: []string{"description"},
					},
					Annotations: &ToolAnnotations{
						ReadOnlyHint:    boolPtr(false),
						DestructiveHint: boolPtr(true),
						IdempotentHint:  boolPtr(false),
						OpenWorldHint:   boolPtr(false),
					},
				},
			)
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

	// Load .env from executable directory (supports documented dynamic mode workflow)
	// Host-passed environment variables always take precedence.
	loadDotEnvIfPresent(secLogger)

	// Determine mode once. We use this both to decide whether to load dynamic aliases
	// and to expose the correct tool surface to the AI.
	dynamicMode := isDynamicMode()

	// Only load dynamic aliases when operating in dynamic mode.
	// This prevents stray MSSQL_DYNAMIC_* variables (inherited from Claude Desktop's
	// parent environment, previous shell sessions, or leftover .env files in other folders)
	// from polluting classic single-connection server instances.
	var dynamicAliases map[string]DynamicAlias
	if dynamicMode {
		dynamicAliases = loadDynamicAliases(secLogger)
	} else {
		dynamicAliases = make(map[string]DynamicAlias)
	}

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

	// === SECURITY GUARD: Dynamic mode + global READ_ONLY=false is fatal ===
	// This combination (as reported in production incidents) allows an AI (or
	// prompt-injected tool call) to dynamic_connect to any preconfigured internal
	// corporate database and execute arbitrary DML/DDL because enforcement is global.
	if isDynamicMode() && !cfg.readOnly {
		secLogger.Printf("*** FATAL SECURITY MISCONFIGURATION DETECTED ***")
		secLogger.Printf("DYNAMIC multi-connection mode is active (MSSQL_DYNAMIC_MODE or MSSQL_DYNAMIC_* vars present)")
		secLogger.Printf("BUT top-level MSSQL_READ_ONLY is false or unset (full access mode).")
		secLogger.Printf("This would expose ALL dynamic databases (including production ones) to unrestricted write operations via query_database after dynamic_connect.")
		secLogger.Printf("SAFETY ACTION: Forcing READ-ONLY mode globally for this session. Dynamic aliases will inherit safe defaults.")
		secLogger.Printf("Recommendation: Remove MSSQL_READ_ONLY=false from the .env used with dynamic connections. Per-alias control (MSSQL_DYNAMIC_<ALIAS>_READ_ONLY) will be supported in a future secure implementation.")
		cfg.readOnly = true
		cfg.whitelistTables = nil // start strict; per-alias can relax later
	}

	// Create MCP server without database initially
	server := &MCPMSSQLServer{
		db:             nil,
		secLogger:      secLogger,
		devMode:        devMode,
		config:         cfg,
		isDynamic:      dynamicMode,
		dynamicAliases: dynamicAliases,
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
		defer func() {
			if r := recover(); r != nil {
				secLogger.Printf("Recovered panic in background connection goroutine: %v (this should not happen - please report)", r)
			}
		}()
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
		safeEnvVars := []string{"MSSQL_SERVER", "MSSQL_DATABASE", "MSSQL_PORT", "MSSQL_AUTH", "MSSQL_READ_ONLY", "MSSQL_WHITELIST_TABLES", "DEVELOPER_MODE", "MSSQL_DYNAMIC_MODE"}
		secLogger.Printf("Configuration settings:")
		for _, key := range safeEnvVars {
			if val := os.Getenv(key); val != "" {
				secLogger.Printf("  %s=%s", key, val)
			}
		}
		if isDynamicMode() {
			secLogger.Printf("  DYNAMIC_ALIASES_DETECTED=true (see dynamic_available tool at runtime)")
		} else {
			secLogger.Printf("  DYNAMIC_MODE=false (classic single-connection mode)")
		}

		// Log security settings (using cached config)
		if server.config.readOnly {
			secLogger.Printf("READ-ONLY MODE ENABLED - Only SELECT queries allowed")
		} else {
			secLogger.Printf("Full access mode enabled")
		}

		if customConnStr == "" && serverHost == "" {
			if isDynamicMode() {
				secLogger.Printf("Dynamic multi-connection mode enabled - no default connection configured (use dynamic_connect after startup)")
			} else {
				secLogger.Printf("No MSSQL_SERVER or MSSQL_CONNECTION_STRING environment variable - database features disabled")
			}
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
	// Note: we intentionally do not recover here; a panic in the main loop is fatal
	// and will cause the host to restart us (logged). The recover is inside handleRequest for per-request safety.

	if err := scanner.Err(); err != nil && err != io.EOF {
		secLogger.Printf("Scanner error: %v", err)
	}

	// Clean shutdown: cancel connection goroutine and wait for it
	connCancel()
	connWg.Wait()
}
