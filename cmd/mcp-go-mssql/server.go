package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

// Process-local server state, rate limit, and connection lifecycle.

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

// MCPMSSQLServer is one stdio process and its active SQL connection.
// Read-only, whitelist, and confirmation state are process-local. Run one
// process per agent session; the active dynamic alias is not an isolation boundary.
type MCPMSSQLServer struct {
	db        *sql.DB // current active connection (for backward compat + single-connection mode)
	dbMu      sync.RWMutex
	secLogger *SecurityLogger
	devMode   bool
	config    serverConfig
	isDynamic bool // frozen at startup from isDynamicMode(); controls tool surface + connection behavior

	// Dynamic connections support (for "one app, multiple related DBs" use case)
	dynamicAliases map[string]DynamicAlias
	connections    map[string]*sql.DB // alias -> open connection
	dynamicMu      sync.RWMutex
	activeAlias    string // currently selected dynamic alias (if any)

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
	Operation   string // e.g. "DELETE", "UPDATE", "DROP"
	Tables      []string
	Description string
	Alias       string // dynamic alias this confirmation was issued for; confirmations are not portable across aliases
	QueryHash   string // exact normalized statement that the approval covers
	Approved    bool   // set only by confirm_operation; a pending request is never approval
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

	// Open and validate the NEW connection first. Only after the ping
	// succeeds do we close the previous active connection — otherwise a
	// failed connect attempt would silently drop the working session.
	// We use mssql.NewConnector + sql.OpenDB so the driver can deliver
	// informational messages (PRINT, SET STATISTICS TIME/IO output) via
	// the sqlexp.ReturnMessage queue.
	connector, err := mssql.NewConnector(connStr)
	if err != nil {
		return fmt.Errorf("failed to build connector for alias '%s': %w", aliasName, err)
	}
	db := sql.OpenDB(connector)

	// Apply the same pool limits as the classic startup path so a dynamic
	// connection cannot exhaust server-side resources.
	db.SetMaxOpenConns(10)                  // More concurrent connections
	db.SetMaxIdleConns(5)                   // More idle connections for reuse
	db.SetConnMaxLifetime(30 * time.Minute) // Shorter lifetime for fresher connections
	db.SetConnMaxIdleTime(5 * time.Minute)  // Quick cleanup of unused connections

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// #nosec G104 -- close error ignored when ping fails; db state is already broken
		db.Close()
		return fmt.Errorf("failed to connect to alias '%s': %w", aliasName, err)
	}

	// New connection works: close the previous active one (if any) and swap.
	if s.activeAlias != "" {
		if oldConn, exists := s.connections[s.activeAlias]; exists && oldConn != nil {
			_ = oldConn.Close()
			delete(s.connections, s.activeAlias)
		}
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

	// Any pending confirmation belongs to the previous alias (or to no
	// alias); confirmations are bound to a single alias and must not
	// survive a connection switch.
	s.confirmMu.Lock()
	s.pendingConfirmation = nil
	s.confirmMu.Unlock()

	s.secLogger.Printf("Dynamic connection switched to alias '%s' (readOnly=%v)", aliasName, alias.ReadOnly)
	return nil
}

// closeAllConnections closes every open database handle (all dynamic aliases
// plus the classic/active connection) exactly once. Called at process
// shutdown so SQL Server sessions are released promptly instead of lingering
// until the server-side idle timeout.
func (s *MCPMSSQLServer) closeAllConnections() {
	s.dynamicMu.Lock()
	for alias, conn := range s.connections {
		if conn != nil {
			_ = conn.Close()
		}
		delete(s.connections, alias)
	}
	s.activeAlias = ""
	s.dynamicMu.Unlock()

	// s.db may point to the same handle as a dynamic connection already
	// closed above; sql.DB.Close is safe to call more than once.
	s.dbMu.Lock()
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	s.dbMu.Unlock()
}
