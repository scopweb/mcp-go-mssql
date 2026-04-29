package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

	"mcp-go-mssql/internal/sqlguard"
)

// serverConfig holds cached configuration read once at startup.
type serverConfig struct {
	readOnly             bool
	whitelistTables      []string
	whitelistProcs       string
	allowedDatabases     []string // additional databases this connector can query (cross-database)
	confirmDestructive   bool     // require confirmation for destructive DDL operations
	autopilot            bool     // skip schema validation for autonomous operation (destructive confirmation still enforced)
	skipSchemaValidation bool     // skip schema validation independently of autopilot (effective skip = autopilot OR skipSchemaValidation)
}

// pendingOperation represents a destructive operation awaiting user confirmation.
type pendingOperation struct {
	query     string
	createdAt time.Time
	expiresAt time.Time
}

// confirmationTokenTTL is how long a destructive operation token remains valid.
const confirmationTokenTTL = 5 * time.Minute

// pendingOpsGCInterval is the cadence at which expired confirmation tokens
// are purged from s.pendingOps. Independent from confirmationTokenTTL: a token
// can outlive its expiry by up to one interval before being collected.
const pendingOpsGCInterval = 60 * time.Second

// gcPendingOps removes confirmation tokens past their expiresAt. Returns
// (active, purged) counts for logging. Safe to call concurrently with
// checkDestructiveConfirmation / confirm_operation handlers because it
// holds pendingOpMu for the duration of the sweep.
func (s *MCPMSSQLServer) gcPendingOps() (active, purged int) {
	now := time.Now()
	s.pendingOpMu.Lock()
	defer s.pendingOpMu.Unlock()

	for token, op := range s.pendingOps {
		if !now.Before(op.expiresAt) {
			delete(s.pendingOps, token)
			purged++
		}
	}
	active = len(s.pendingOps)
	return active, purged
}

// startPendingOpsGC starts a background goroutine that periodically purges
// expired confirmation tokens. Runs for the lifetime of the server. Logs at
// each tick so operators can see token activity over time. Always logs at
// least the active count; only logs purged when there was something to purge
// (keeps logs quiet on idle servers).
func (s *MCPMSSQLServer) startPendingOpsGC() {
	go func() {
		ticker := time.NewTicker(pendingOpsGCInterval)
		defer ticker.Stop()
		for range ticker.C {
			active, purged := s.gcPendingOps()
			if purged > 0 {
				s.secLogger.Printf("pending_ops_gc: active=%d purged=%d", active, purged)
			} else if active > 0 {
				s.secLogger.Printf("pending_ops_gc: active=%d (none expired)", active)
			}
		}
	}()
}

// ConnectionInfo holds a named dynamic database connection together with the
// per-connection security policy. Each connection gets its own sqlguard.Guard
// so a single MCP server can hold (for example) a read-only PROD connection
// next to a permissive STAGING one without the policies bleeding between them.
type ConnectionInfo struct {
	Alias     string
	DB        *sql.DB
	Server    string
	Database  string
	User      string
	CreatedAt time.Time
	// Per-connection security config. The guard is built from these fields
	// in dynamic_connect; downstream handlers must use ConnectionInfo.guard
	// (NOT the server-wide s.guard) when running queries against this DB.
	readOnly             bool
	whitelistTables      []string
	autopilot            bool
	skipSchemaValidation bool
	guard                *sqlguard.Guard
}

// MSSQL Server
type MCPMSSQLServer struct {
	db          *sql.DB
	dbMu        sync.RWMutex
	secLogger   *SecurityLogger
	devMode     bool
	config      serverConfig
	pendingOps  map[string]pendingOperation // token -> operation awaiting confirmation
	pendingOpMu sync.Mutex
	rateLimiter struct {
		mu        sync.Mutex
		tokens    int
		maxTokens int
		lastReset time.Time
		interval  time.Duration
	}
	// Dynamic multi-connection support
	dynamicMode     bool
	connections     map[string]*ConnectionInfo
	connMu          sync.RWMutex
	maxDynamicConns int
	// Security guard for SQL query validation
	guard *sqlguard.Guard
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

// Dynamic connection management for multi-database mode.

func (s *MCPMSSQLServer) addDynamicConnectionInfo(alias string, connInfo *ConnectionInfo) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.connections[alias] = connInfo
}

// getDynamicConnection returns the *sql.DB for an active dynamic connection.
// Most callers should prefer getDynamicConnectionInfo, which also returns the
// per-connection security policy needed to validate queries correctly.
func (s *MCPMSSQLServer) getDynamicConnection(alias string) (*sql.DB, bool) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	if conn, ok := s.connections[alias]; ok {
		return conn.DB, true
	}
	return nil, false
}

// getDynamicConnectionInfo returns the full ConnectionInfo (DB + per-connection
// guard + policy flags) for an active alias, or (nil, false) when missing.
// query_database uses this so each alias enforces its own policy instead of
// falling back to the server-wide guard.
func (s *MCPMSSQLServer) getDynamicConnectionInfo(alias string) (*ConnectionInfo, bool) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	conn, ok := s.connections[alias]
	return conn, ok
}

func (s *MCPMSSQLServer) listDynamicConnections() []*ConnectionInfo {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	result := make([]*ConnectionInfo, 0, len(s.connections))
	for _, conn := range s.connections {
		result = append(result, conn)
	}
	return result
}

func (s *MCPMSSQLServer) removeDynamicConnection(alias string) error {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if conn, ok := s.connections[alias]; ok {
		conn.DB.Close()
		delete(s.connections, alias)
		return nil
	}
	return fmt.Errorf("connection '%s' not found", alias)
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

// getWhitelistedTables returns the cached list of tables/views allowed for modification.
// Kept as a thin accessor because callers in main.go still reach for it; new code
// should use s.guard.Whitelist() instead.
func (s *MCPMSSQLServer) getWhitelistedTables() []string {
	return s.config.whitelistTables
}

// extractAllTablesFromQuery finds all table/view names referenced in the query (flat list).
// This is a backward-compatible wrapper around sqlguard.ExtractTableRefs that returns only table names.
func (s *MCPMSSQLServer) extractAllTablesFromQuery(query string) []string {
	refs := sqlguard.ExtractTableRefs(query)
	seen := make(map[string]bool)
	var tables []string
	for _, ref := range refs {
		if !seen[ref.Table] {
			seen[ref.Table] = true
			tables = append(tables, ref.Table)
		}
	}
	return tables
}

// isAllowedDatabase checks if a database name is in the allowed cross-database list.
func (s *MCPMSSQLServer) isAllowedDatabase(db string) bool {
	for _, allowed := range s.config.allowedDatabases {
		if allowed == db {
			return true
		}
	}
	return false
}

// loadTablesForDatabase returns the set of table names in a given database.
// If database is empty, queries the current database. Uses INFORMATION_SCHEMA.
func (s *MCPMSSQLServer) loadTablesForDatabase(ctx context.Context, database string) (map[string]bool, error) {
	var checkQuery string
	if database == "" {
		checkQuery = `SELECT LOWER(TABLE_NAME) AS table_name FROM INFORMATION_SCHEMA.TABLES`
	} else {
		// Cross-database query: [OtherDB].INFORMATION_SCHEMA.TABLES
		// database name is from our allowedDatabases list (not user input), but sanitize anyway
		if !regexp.MustCompile(`^[\w]+$`).MatchString(database) {
			return nil, fmt.Errorf("invalid database name")
		}
		checkQuery = fmt.Sprintf(`SELECT LOWER(TABLE_NAME) AS table_name FROM [%s].INFORMATION_SCHEMA.TABLES`, database)
	}
	results, err := s.executeSecureQuery(ctx, checkQuery)
	if err != nil {
		return nil, err
	}
	tables := make(map[string]bool)
	for _, row := range results {
		if name, ok := row["table_name"].(string); ok {
			tables[strings.ToLower(name)] = true
		}
	}
	return tables, nil
}

// validateTablesExist performs best-effort validation that tables referenced in a query
// actually exist in the database. Supports cross-database references for allowed databases.
// If INFORMATION_SCHEMA is not accessible (permissions), it silently skips validation.
// Returns nil if all tables exist or validation was skipped,
// or an error message with suggestions if unknown tables are found.
func (s *MCPMSSQLServer) validateTablesExist(ctx context.Context, query string) *string {
	refs := sqlguard.ExtractTableRefs(query)
	if len(refs) == 0 {
		return nil
	}

	// Cache loaded tables per database (empty string = current database)
	tableCache := make(map[string]map[string]bool)

	// Load current database tables
	currentTables, err := s.loadTablesForDatabase(ctx, "")
	if err != nil {
		// No permissions to read schema — skip validation silently
		s.secLogger.Printf("Schema validation skipped: cannot load tables from current database (err: %v)", err)
		return nil
	}
	if len(currentTables) == 0 {
		s.secLogger.Printf("Schema validation warning: current database has 0 tables in INFORMATION_SCHEMA.TABLES")
	}
	s.secLogger.Printf("Schema validation: loaded %d tables from current database", len(currentTables))
	tableCache[""] = currentTables

	// Check each table reference
	var missing []string
	for _, ref := range refs {
		// Skip system schema-qualified tables (sys.objects, information_schema.columns, etc.)
		// These are virtual/system tables not listed in INFORMATION_SCHEMA.TABLES
		if sqlguard.IsSystemSchemaTable(ref.Table) {
			continue
		}
		if ref.Database != "" {
			// Cross-database reference — check if database is allowed
			if !s.isAllowedDatabase(ref.Database) {
				missing = append(missing, fmt.Sprintf("%s.%s", ref.Database, ref.Table))
				continue
			}
			// Load tables for that database if not cached
			if _, ok := tableCache[ref.Database]; !ok {
				dbTables, err := s.loadTablesForDatabase(ctx, ref.Database)
				if err != nil {
					// Can't read that database's schema — skip validation for it
					s.secLogger.Printf("Schema validation skipped: cannot load tables from cross-database '%s' (err: %v)", ref.Database, err)
					continue
				}
				s.secLogger.Printf("Schema validation: loaded %d tables from cross-database '%s'", len(dbTables), ref.Database)
				tableCache[ref.Database] = dbTables
			}
			if !tableCache[ref.Database][ref.Table] {
				missing = append(missing, fmt.Sprintf("%s.%s", ref.Database, ref.Table))
			}
		} else {
			// Current database reference
			if !currentTables[ref.Table] {
				missing = append(missing, ref.Table)
			}
		}
	}

	if len(missing) == 0 {
		return nil
	}

	s.secLogger.Printf("Schema validation: %d tables not found in cache (this may indicate permissions issue or empty INFORMATION_SCHEMA)", len(missing))

	// Build suggestions using current database tables (most common case)
	var parts []string
	for _, m := range missing {
		// For cross-database refs (db.table), suggest from that database if available
		if strings.Contains(m, ".") {
			dotParts := strings.SplitN(m, ".", 2)
			dbName, tableName := dotParts[0], dotParts[1]
			if !s.isAllowedDatabase(dbName) {
				allowedList := strings.Join(s.config.allowedDatabases, ", ")
				if allowedList == "" {
					allowedList = "(none configured)"
				}
				parts = append(parts, fmt.Sprintf("'%s' — database '%s' is not in MSSQL_ALLOWED_DATABASES. Allowed: %s",
					m, dbName, allowedList))
			} else if dbTables, ok := tableCache[dbName]; ok {
				suggestions := findSimilarNames(tableName, dbTables)
				if len(suggestions) > 0 {
					parts = append(parts, fmt.Sprintf("'%s' not found in database '%s'. Did you mean: %s?",
						m, dbName, strings.Join(suggestions, ", ")))
				} else {
					parts = append(parts, fmt.Sprintf("'%s' not found in database '%s' (no similar tables found)", m, dbName))
				}
			} else {
				parts = append(parts, fmt.Sprintf("'%s' — could not access database '%s'", m, dbName))
			}
		} else {
			suggestions := findSimilarNames(m, currentTables)
			if len(suggestions) > 0 {
				parts = append(parts, fmt.Sprintf("'%s' not found. Did you mean: %s?", m, strings.Join(suggestions, ", ")))
			} else {
				parts = append(parts, fmt.Sprintf("'%s' not found (no similar tables found)", m))
			}
		}
	}

	msg := "Schema validation error — the following tables/views do not exist:\n" + strings.Join(parts, "\n") +
		"\n\nUse the 'explore' tool to list available tables, or 'inspect' to see column details."
	return &msg
}

// objectExists checks if a database object (table, view, proc, func) already exists.
// schema and name should be lowercase for consistency.
func (s *MCPMSSQLServer) objectExists(ctx context.Context, schema, name string, objType sqlguard.ObjectType) (bool, error) {
	db := s.getDB()
	if db == nil {
		return false, fmt.Errorf("database not connected")
	}

	// Map ObjectType to sys.objects type codes
	var typeCodes []string
	switch objType {
	case sqlguard.ObjectTypeTable:
		typeCodes = []string{"U"}
	case sqlguard.ObjectTypeView:
		typeCodes = []string{"V"}
	case sqlguard.ObjectTypeProcedure:
		typeCodes = []string{"P", "PC"} // also "PC" for CLR procedure
	case sqlguard.ObjectTypeFunction:
		typeCodes = []string{"FN", "IF", "TF", "FT"} // scalar, inline table, table-valued, assembly
	default:
		return false, fmt.Errorf("unknown object type: %s", objType)
	}

	// Build query to check object existence
	query := `
		SELECT TOP 1 1 FROM sys.objects
		WHERE LOWER(SCHEMA_NAME(schema_id)) = @p1
		  AND LOWER(name) = @p2
		  AND type IN (` + placeholders(len(typeCodes)) + `)`

	args := make([]interface{}, 0, len(typeCodes)+2)
	args = append(args, schema, name)
	for _, tc := range typeCodes {
		args = append(args, tc)
	}

	results, err := s.executeSecureQuery(ctx, query, args...)
	if err != nil {
		// If query fails (permissions, etc.), assume object doesn't exist to be safe
		return false, nil
	}

	return len(results) > 0, nil
}

// placeholders generates SQL Server parameter placeholders like @p1, @p2, @p3.
func placeholders(count int) string {
	var parts []string
	for i := 1; i <= count; i++ {
		parts = append(parts, fmt.Sprintf("@p%d", i))
	}
	return strings.Join(parts, ", ")
}

// checkDestructiveConfirmation checks if a query targets an existing object and requires
// user confirmation before execution. Returns (token, error) if confirmation is needed.
func (s *MCPMSSQLServer) checkDestructiveConfirmation(ctx context.Context, query string) (string, error) {
	// Only check if confirmDestructive is enabled
	if !s.config.confirmDestructive {
		return "", nil
	}

	// Extract target objects from the DDL query
	targets := sqlguard.ExtractDDLTargetObjects(query)
	if len(targets) == 0 {
		return "", nil // Not a destructive operation we track
	}

	// Check if any target object exists
	for _, target := range targets {
		exists, err := s.objectExists(ctx, target.Schema, target.Name, target.ObjType)
		if err != nil {
			s.secLogger.Printf("Error checking object existence for %s.%s: %v", target.Schema, target.Name, err)
			continue // Skip on error, let the query execute
		}
		if exists {
			// Generate confirmation token
			token := generateOpToken()
			s.pendingOpMu.Lock()
			s.pendingOps[token] = pendingOperation{
				query:     query,
				createdAt: time.Now(),
				expiresAt: time.Now().Add(confirmationTokenTTL),
			}
			s.pendingOpMu.Unlock()

			// Log the warning
			s.secLogger.Printf("DESTRUCTIVE OPERATION WARNING: %s on existing %s.%s (%s) - token: %s",
				sqlguard.ExtractDestructiveOpType(query), target.Schema, target.Name, target.ObjType, token)

			return token, nil
		}
	}

	return "", nil
}

// generateOpToken generates a random confirmation token using crypto/rand.
func generateOpToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
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
	if err := s.guard.ValidateReadOnly(query); err != nil {
		s.secLogger.Printf("Read-only violation blocked: %s", err)
		return nil, err
	}

	// Validate granular table permissions (whitelist)
	if err := s.guard.ValidateTablePermissions(query); err != nil {
		s.secLogger.Printf("Permission violation blocked: %s", err)
		return nil, err
	}

	// Validate subqueries don't reference restricted tables
	if err := s.guard.ValidateSubqueriesForRestrictedTables(query); err != nil {
		s.secLogger.Printf("Subquery safety violation blocked: %s", err)
		return nil, err
	}

	// Check for destructive operation confirmation requirement
	// AUTOPILOT does NOT skip destructive confirmation — each dangerous operation requires explicit confirmation
	s.secLogger.Printf("AUTOPILOT=%v — %s", s.config.autopilot, sqlguard.ExtractOperation(query))
	if token, err := s.checkDestructiveConfirmation(ctx, query); err != nil {
		// If check fails with confirmation requirement, return confirmation error
		if token != "" {
			targets := sqlguard.ExtractDDLTargetObjects(query)
			var targetStr string
			if len(targets) > 0 {
				targetStr = targets[0].Schema + "." + targets[0].Name
			}
			opType := sqlguard.ExtractDestructiveOpType(query)
			return nil, &ConfirmationRequiredError{
				Token:     token,
				Operation: opType,
				Target:    targetStr,
				ExpiresIn: "5 minutes",
			}
		}
		// Other errors from checkDestructiveConfirmation are logged but don't block
	} else if token != "" {
		// Confirmation required
		targets := sqlguard.ExtractDDLTargetObjects(query)
		var targetStr string
		if len(targets) > 0 {
			targetStr = targets[0].Schema + "." + targets[0].Name
		}
		opType := sqlguard.ExtractDestructiveOpType(query)
		return nil, &ConfirmationRequiredError{
			Token:     token,
			Operation: opType,
			Target:    targetStr,
			ExpiresIn: "5 minutes",
		}
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

// executeSecureQueryWithDB is a wrapper that accepts an explicit db connection
// and a per-call sqlguard.Guard. This enables dynamic multi-database
// connections in MSSQL_DYNAMIC_MODE: each ConnectionInfo carries its own
// guard, so queries against alias "prod" enforce prod's policy and queries
// against "staging" enforce staging's, even from the same MCP server.
//
// guard may be nil — in that case the server-wide s.guard is used. Callers
// hitting the default DB should pass nil; callers hitting a dynamic alias
// should pass connInfo.guard.
func (s *MCPMSSQLServer) executeSecureQueryWithDB(ctx context.Context, db *sql.DB, guard *sqlguard.Guard, query string, args ...interface{}) ([]map[string]interface{}, error) {
	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	if guard == nil {
		guard = s.guard
	}

	if err := s.validateBasicInput(query); err != nil {
		return nil, err
	}

	// Validate read-only restrictions
	if err := guard.ValidateReadOnly(query); err != nil {
		s.secLogger.Printf("Read-only violation blocked: %s", err)
		return nil, err
	}

	// Validate granular table permissions (whitelist)
	if err := guard.ValidateTablePermissions(query); err != nil {
		s.secLogger.Printf("Permission violation blocked: %s", err)
		return nil, err
	}

	// Validate subqueries don't reference restricted tables
	if err := guard.ValidateSubqueriesForRestrictedTables(query); err != nil {
		s.secLogger.Printf("Subquery safety violation blocked: %s", err)
		return nil, err
	}

	// Check for destructive operation confirmation requirement
	// AUTOPILOT does NOT skip destructive confirmation — each dangerous operation requires explicit confirmation
	s.secLogger.Printf("AUTOPILOT=%v — %s", s.config.autopilot, sqlguard.ExtractOperation(query))
	if token, err := s.checkDestructiveConfirmation(ctx, query); err != nil {
		// If check fails with confirmation requirement, return confirmation error
		if token != "" {
			targets := sqlguard.ExtractDDLTargetObjects(query)
			var targetStr string
			if len(targets) > 0 {
				targetStr = targets[0].Schema + "." + targets[0].Name
			}
			opType := sqlguard.ExtractDestructiveOpType(query)
			return nil, &ConfirmationRequiredError{
				Token:     token,
				Operation: opType,
				Target:    targetStr,
				ExpiresIn: "5 minutes",
			}
		}
		// Other errors from checkDestructiveConfirmation are logged but don't block
	} else if token != "" {
		// Confirmation required
		targets := sqlguard.ExtractDDLTargetObjects(query)
		var targetStr string
		if len(targets) > 0 {
			targetStr = targets[0].Schema + "." + targets[0].Name
		}
		opType := sqlguard.ExtractDestructiveOpType(query)
		return nil, &ConfirmationRequiredError{
			Token:     token,
			Operation: opType,
			Target:    targetStr,
			ExpiresIn: "5 minutes",
		}
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
		return s.handleGetDatabaseInfo(id, params)
	case "query_database":
		return s.handleQueryDatabase(id, params)
	case "explore":
		return s.handleExplore(id, params)
	case "execute_procedure":
		return s.handleExecuteProcedure(id, params)
	case "inspect":
		return s.handleInspect(id, params)
	case "explain_query":
		return s.handleExplainQuery(id, params)
	case "confirm_operation":
		return s.handleConfirmOperation(id, params)
	case "dynamic_connect":
		return s.handleDynamicConnect(id, params)
	case "dynamic_list":
		return s.handleDynamicList(id, params)
	case "dynamic_available":
		return s.handleDynamicAvailable(id, params)
	case "dynamic_disconnect":
		return s.handleDynamicDisconnect(id, params)
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
				Description: "Execute any SQL query (SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP). Aliases: run_sql, execute_sql, db_query, sql_execute, sql_query, run_query, exec_query. Returns up to 500 rows. For execution plan only (no execution), use 'explain_query' instead.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {
							Type:        "string",
							Description: "SQL query: SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP. Example: UPDATE mytable SET col='value' WHERE id=1",
						},
						"connection": {
							Type:        "string",
							Description: "Dynamic connection alias (from dynamic_connect). If not specified, uses the default database connection.",
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
				Description: "Get database connection status, server info, and current configuration. Aliases: server_info, db_status, db_info, connection_status. Use this first to verify connectivity before running queries.",
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
				Description: "Explore database objects (tables, views, procedures, databases). Aliases: list_tables, list_views, list_procedures, show_tables, show_views, db_explore, find_tables, search_tables. type=tables (default), type=views, type=databases, type=procedures, type=search.",
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
						"database": {
							Type:        "string",
							Description: "Explore tables in a specific allowed cross-database (requires MSSQL_ALLOWED_DATABASES). Example: 'JJP_Carregues'",
						},
						"connection": {
							Type:        "string",
							Description: "Dynamic connection alias to use (e.g. 'TEST'). Use dynamic_list to see available connections.",
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
				Description: "Inspect a table's structure (columns, indexes, foreign keys, dependencies). Aliases: describe_table, table_structure, schema_info, show_columns, table_info, column_info, index_info. detail=columns (default), detail=indexes, detail=foreign_keys, detail=dependencies, detail=all.",
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
						"connection": {
							Type:        "string",
							Description: "Dynamic connection alias to use (e.g. 'TEST'). Use dynamic_list to see available connections.",
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
				Title:       "Show Query Execution Plan",
				Description: "Display the estimated execution plan for a SELECT query WITHOUT executing it. Aliases: show_plan, explain_plan, sql_explain, analyze_query, query_plan, plan_analysis. ONLY accepts SELECT queries. For INSERT/UPDATE/DELETE or query execution, use 'query_database' instead.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {
							Type:        "string",
							Description: "SELECT query only — shows execution plan without running the query. For INSERT/UPDATE/DELETE use 'query_database' instead.",
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
			{
				Name:        "confirm_operation",
				Title:       "Confirm Destructive Operation",
				Description: "Confirm a pending destructive operation that requires explicit user confirmation. Use the token from the destructive operation warning.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"token": {
							Type:        "string",
							Description: "Confirmation token received from a destructive operation warning",
						},
					},
					Required: []string{"token"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(true),
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			},
		}

		// Append dynamic tools only when dynamic mode is enabled
		if s.dynamicMode {
			tools = append(tools, Tool{
				Name:        "dynamic_connect",
				Title:       "Dynamic Connect",
				Description: "Activate a pre-configured dynamic database connection. Credentials are read from .env (MSSQL_DYNAMIC_<ALIAS>_* vars). Use dynamic_list to see available connections. Requires MSSQL_DYNAMIC_MODE=true.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"alias": {
							Type:        "string",
							Description: "Connection alias from .env configuration (e.g., 'identity', 'ferratge')",
						},
					},
					Required: []string{"alias"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(false),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(true),
					OpenWorldHint:   boolPtr(false),
				},
			}, Tool{
				Name:        "dynamic_list",
				Title:       "Dynamic List Connections",
				Description: "List all active (connected) dynamic database connections. Use dynamic_available to see all configured connections.",
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
			}, Tool{
				Name:        "dynamic_available",
				Title:       "Dynamic Available Connections",
				Description: "List all pre-configured dynamic connections available in .env. Use this first to discover which aliases you can connect with via dynamic_connect.",
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
			}, Tool{
				Name:        "dynamic_disconnect",
				Title:       "Dynamic Disconnect",
				Description: "Close a named dynamic database connection. Aliases: disconnect_db, db_disconnect.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"alias": {
							Type:        "string",
							Description: "Name of the connection to close",
						},
					},
					Required: []string{"alias"},
				},
				Annotations: &ToolAnnotations{
					ReadOnlyHint:    boolPtr(true),
					DestructiveHint: boolPtr(false),
					IdempotentHint:  boolPtr(false),
					OpenWorldHint:   boolPtr(false),
				},
			})
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

// getenvBool reads an env var and treats it as a bool. Comparison is
// case-insensitive against "true". Anything else (including unset, empty,
func main() {
	// Load .env file ONLY if no direct server is configured via environment variables
	// If MSSQL_SERVER or MSSQL_CONNECTION_STRING is set (e.g., from Claude Desktop JSON config), skip .env entirely
	// This allows two modes: direct connection (MSSQL_SERVER/MSSQL_CONNECTION_STRING set) vs dynamic multi-connection (.env)
	if os.Getenv("MSSQL_SERVER") == "" && os.Getenv("MSSQL_CONNECTION_STRING") == "" {
		loadEnvFile(".env")
	}

	// Initialize security logger
	secLogger := NewSecurityLogger()
	secLogger.Printf("Starting secure MCP-MSSQL server")

	// Check for developer mode
	devMode := getenvBool("DEVELOPER_MODE")
	if devMode {
		secLogger.Printf("DEVELOPER MODE ENABLED - Detailed errors will be shown")
	}

	// Load and validate configuration. Warnings are logged but never abort.
	cfg, warnings, err := loadConfig()
	if err != nil {
		secLogger.Printf("FATAL: invalid configuration: %v", err)
		os.Exit(1)
	}
	for _, msg := range warnings {
		secLogger.Printf("CONFIG WARNING: %s", msg)
	}

	// Dynamic multi-connection mode configuration
	// Only enable if no direct server is configured (MSSQL_SERVER not set)
	// AND MSSQL_DYNAMIC_MODE=true
	dynamicMode := os.Getenv("MSSQL_SERVER") == "" && strings.ToLower(os.Getenv("MSSQL_DYNAMIC_MODE")) == "true"
	maxDynamicConns := 10
	if envMax := os.Getenv("MSSQL_DYNAMIC_MAX_CONNECTIONS"); envMax != "" {
		if parsed, err := strconv.Atoi(envMax); err == nil && parsed > 0 {
			maxDynamicConns = parsed
		}
	}

	// Create MCP server without database initially
	server := &MCPMSSQLServer{
		db:              nil,
		secLogger:       secLogger,
		devMode:         devMode,
		config:          cfg,
		pendingOps:      make(map[string]pendingOperation),
		dynamicMode:     dynamicMode,
		connections:     make(map[string]*ConnectionInfo),
		maxDynamicConns: maxDynamicConns,
	}

	// Initialize sqlguard with security configuration
	server.guard = sqlguard.New(sqlguard.Config{
		ReadOnly:         cfg.readOnly,
		Whitelist:        cfg.whitelistTables,
		AllowedDatabases: cfg.allowedDatabases,
		Logger:           secLogger,
	})

	// Log final configuration
	secLogger.Printf("=== SERVER CONFIGURATION ===")
	secLogger.Printf("DEVELOPER_MODE=%v", devMode)
	secLogger.Printf("AUTOPILOT=%v (skips schema validation; destructive confirmation always enforced)", cfg.autopilot)
	secLogger.Printf("SKIP_SCHEMA_VALIDATION=%v (independent flag; effective skip = AUTOPILOT OR SKIP_SCHEMA_VALIDATION)", cfg.skipSchemaValidation)
	secLogger.Printf("CONFIRM_DESTRUCTIVE=%v", cfg.confirmDestructive)
	secLogger.Printf("READ_ONLY=%v", cfg.readOnly)
	wl := strings.Join(cfg.whitelistTables, ",")
	if wl == "" {
		wl = "(none)"
	}
	secLogger.Printf("WHITELIST_TABLES=%s", wl)
	secLogger.Printf("=============================")
	// Initialize rate limiter: 60 tool calls per minute
	server.rateLimiter.maxTokens = 60
	server.rateLimiter.tokens = 60
	server.rateLimiter.lastReset = time.Now()
	server.rateLimiter.interval = time.Minute

	// Start the background GC for expired confirmation tokens. Runs for the
	// lifetime of the process; logs activity through secLogger.
	server.startPendingOpsGC()

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

		// Log only non-sensitive configuration settings. Sensitive vars
		// (MSSQL_PASSWORD, MSSQL_CONNECTION_STRING) are NEVER added here.
		safeEnvVars := []string{
			"MSSQL_SERVER", "MSSQL_DATABASE", "MSSQL_PORT", "MSSQL_AUTH",
			"MSSQL_READ_ONLY", "MSSQL_WHITELIST_TABLES", "MSSQL_WHITELIST_PROCEDURES",
			"MSSQL_ALLOWED_DATABASES",
			"MSSQL_AUTOPILOT", "MSSQL_SKIP_SCHEMA_VALIDATION", "MSSQL_CONFIRM_DESTRUCTIVE",
			"MSSQL_DYNAMIC_MODE", "MSSQL_DYNAMIC_MAX_CONNECTIONS",
			"DEVELOPER_MODE",
		}
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

		// Auto-connect for dynamic mode: if no direct connection but dynamic connections exist, auto-connect to first
		if customConnStr == "" && serverHost == "" && dynamicMode {
			if firstAlias := discoverFirstDynamicConnection(); firstAlias != "" {
				secLogger.Printf("Auto-connecting to first dynamic connection: %s", firstAlias)
				// Build connection string for the first dynamic alias and connect
				alias := firstAlias
				prefix := "MSSQL_DYNAMIC_" + strings.ToUpper(alias) + "_"
				os.Setenv("MSSQL_SERVER", os.Getenv(prefix+"SERVER"))
				os.Setenv("MSSQL_DATABASE", os.Getenv(prefix+"DATABASE"))
				os.Setenv("MSSQL_USER", os.Getenv(prefix+"USER"))
				os.Setenv("MSSQL_PASSWORD", os.Getenv(prefix+"PASSWORD"))
				os.Setenv("MSSQL_PORT", os.Getenv(prefix+"PORT"))
				os.Setenv("MSSQL_AUTH", os.Getenv(prefix+"AUTH"))
				os.Setenv("MSSQL_READ_ONLY", os.Getenv(prefix+"READ_ONLY"))
				os.Setenv("MSSQL_WHITELIST_TABLES", os.Getenv(prefix+"WHITELIST_TABLES"))
				os.Setenv("MSSQL_AUTOPILOT", os.Getenv(prefix+"AUTOPILOT"))
				os.Setenv("MSSQL_SKIP_SCHEMA_VALIDATION", os.Getenv(prefix+"SKIP_SCHEMA_VALIDATION"))
				os.Setenv("MSSQL_DYNAMIC_ACTIVE_ALIAS", alias)
				// Update server.config to match the dynamic connection's settings
				// This is critical because loadConfig() ran BEFORE auto-connect set these env vars
				server.config.readOnly = getenvBool(prefix+"READ_ONLY")
				server.config.whitelistTables = sqlguard.ParseWhitelistTables(os.Getenv(prefix+"WHITELIST_TABLES"))
				server.config.autopilot = getenvBool(prefix+"AUTOPILOT")
				server.config.skipSchemaValidation = getenvBool(prefix+"SKIP_SCHEMA_VALIDATION")
				// Rebuild guard with updated config (critical for security)
				server.guard = sqlguard.New(sqlguard.Config{
					ReadOnly:         server.config.readOnly,
					Whitelist:        server.config.whitelistTables,
					AllowedDatabases: server.config.allowedDatabases,
					Logger:           secLogger,
				})
				// Update serverHost for logging below
				serverHost = os.Getenv("MSSQL_SERVER")
				database = os.Getenv("MSSQL_DATABASE")
				secLogger.Printf("Auto-connected to %s (%s/%s) with readOnly=%v, autopilot=%v, whitelist=%v",
					alias, serverHost, database, server.config.readOnly, server.config.autopilot, server.config.whitelistTables)
			} else {
				secLogger.Printf("Dynamic multi-connection mode enabled - no connections configured in .env")
				return
			}
		} else if customConnStr == "" && serverHost == "" {
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
