package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	osuser "os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/microsoft/go-mssqldb/integratedauth/winsspi"

	"mcp-go-mssql/internal/sqlguard"
)

// fileExists reports whether the named file exists.
func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// getExecutableDir returns the directory containing the current executable.
func getExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// securityLogger implements sqlguard.Logger for CLI output
type securityLogger struct{}

func (securityLogger) Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[SECURITY] "+format+"\n", args...)
}

// loadEnvFile reads a .env file and sets environment variables from KEY=VALUE lines.
// Empty lines and lines starting with # are skipped.
func loadEnvFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	return scanner.Err()
}

// ConnectionInfo holds information about an active database connection
type ConnectionInfo struct {
	Alias    string
	DB       *sql.DB
	Server   string
	Database string
	User     string
	guard    *sqlguard.Guard
}

// CLIState holds the CLI's runtime state including dynamic connections
type CLIState struct {
	dynamicMode     bool
	connections     map[string]*ConnectionInfo
	activeAlias     string
	devMode         bool
	maxDynamicConns int
	stateFile       string
}

// Global CLI state
var cliState = &CLIState{
	connections: make(map[string]*ConnectionInfo),
}

const stateFileName = ".db-connector-state"

// loadState loads persisted state from disk and reconnects to active alias
func loadState() {
	stateFile := filepath.Join(getExecutableDir(), stateFileName)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}
	alias := strings.TrimSpace(string(data))
	if alias == "" {
		return
	}
	// Reconnect to persisted alias
	cliState.activeAlias = alias
	connectDynamicSilent(alias)
}

// saveState persists active alias to disk
func saveState() {
	stateFile := filepath.Join(getExecutableDir(), stateFileName)
	os.WriteFile(stateFile, []byte(cliState.activeAlias), 0600)
}

// DatabaseConfig holds connection configuration
type DatabaseConfig struct {
	Server   string `json:"server"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	Port     string `json:"port"`
	DevMode  bool   `json:"developer_mode"`
	Auth     string `json:"auth"` // auth mode: sql (default), integrated/windows, azure
}

// QueryResult holds query execution results
type QueryResult struct {
	Success bool                     `json:"success"`
	Data    []map[string]interface{} `json:"data,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Info    string                   `json:"info,omitempty"`
}

func main() {
	// Load .env from executable directory first, fallback to current directory
	exeDir := getExecutableDir()
	if exeDir != "" {
		if envPath := filepath.Join(exeDir, ".env"); fileExists(envPath) {
			loadEnvFile(envPath)
		}
	} else {
		loadEnvFile(".env")
	}

	// Initialize CLI state
	cliState.devMode = strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true"
	cliState.dynamicMode = strings.ToLower(os.Getenv("MSSQL_DYNAMIC_MODE")) == "true" && os.Getenv("MSSQL_SERVER") == ""

	if maxConns := os.Getenv("MSSQL_DYNAMIC_MAX_CONNECTIONS"); maxConns != "" {
		if parsed, err := strconv.Atoi(maxConns); err == nil && parsed > 0 {
			cliState.maxDynamicConns = parsed
		}
	}
	if cliState.maxDynamicConns == 0 {
		cliState.maxDynamicConns = 10
	}

	// Load persisted state (active connection alias)
	if cliState.dynamicMode {
		loadState()
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Handle dynamic mode commands
	if cliState.dynamicMode {
		if handled := handleDynamicCommand(command); handled {
			os.Exit(0)
		}
	}

	// Direct connection mode (or fallback)
	config, guard, err := loadConfig()
	if err != nil {
		printError("Configuration error: %v", err)
		os.Exit(1)
	}

	db, err := connectDatabase(config)
	if err != nil {
		printError("Connection failed: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	switch command {
	case "test":
		testConnection(db, config)
	case "info":
		showDatabaseInfo(db, config)
	case "query":
		if len(os.Args) < 3 {
			printError("Query command requires SQL statement")
			os.Exit(1)
		}
		executeQuery(db, os.Args[2], guard)
	case "tables":
		listTables(db, guard)
	case "describe":
		if len(os.Args) < 3 {
			printError("Describe command requires table name")
			os.Exit(1)
		}
		describeTable(db, os.Args[2], guard)
	default:
		if cliState.dynamicMode {
			fmt.Printf("Unknown command '%s'. In dynamic mode, use: dynamic_available, dynamic_connect, dynamic_list, dynamic_disconnect\n", command)
		} else {
			printError("Unknown command: %s", command)
		}
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run db-connector.go test                    # Test connection")
	fmt.Println("  go run db-connector.go info                    # Show database info")
	fmt.Println("  go run db-connector.go query \"SELECT ...\"      # Execute query")
	fmt.Println("  go run db-connector.go tables                  # List all tables")
	fmt.Println("  go run db-connector.go describe table_name     # Describe table structure")
	if cliState.dynamicMode {
		fmt.Println("  go run db-connector.go dynamic_available      # List available dynamic connections")
		fmt.Println("  go run db-connector.go dynamic_connect <alias> # Connect to a dynamic connection")
		fmt.Println("  go run db-connector.go dynamic_list           # List active dynamic connections")
		fmt.Println("  go run db-connector.go dynamic_disconnect <alias> # Disconnect a dynamic connection")
	}
}

// handleDynamicCommand processes dynamic-mode specific commands.
// Returns true if the command was handled, false otherwise.
func handleDynamicCommand(command string) bool {
	switch command {
	case "dynamic_available":
		listAvailableDynamicConnections()
		return true
	case "dynamic_connect":
		if len(os.Args) < 3 {
			printError("dynamic_connect requires alias argument")
			os.Exit(1)
		}
		connectDynamic(os.Args[2])
		return true
	case "dynamic_list":
		listActiveDynamicConnections()
		return true
	case "dynamic_disconnect":
		if len(os.Args) < 3 {
			printError("dynamic_disconnect requires alias argument")
			os.Exit(1)
		}
		disconnectDynamic(os.Args[2])
		return true
	case "test", "info", "query", "tables", "describe":
		// These require an active connection in dynamic mode
		if cliState.activeAlias == "" {
			printError("No active dynamic connection. Use 'dynamic_connect <alias>' first")
			os.Exit(1)
		}
		// Get active connection and run command
		conn := cliState.connections[cliState.activeAlias]
		if conn == nil {
			printError("Active connection '%s' not found. Use 'dynamic_connect <alias>'", cliState.activeAlias)
			os.Exit(1)
		}
		runCommandOnConnection(command, conn)
		return true
	}
	return false
}

func runCommandOnConnection(command string, conn *ConnectionInfo) {
	switch command {
	case "test":
		testDynamicConnection(conn)
	case "info":
		showDynamicInfo(conn)
	case "query":
		if len(os.Args) < 3 {
			printError("Query command requires SQL statement")
			os.Exit(1)
		}
		executeQuery(conn.DB, os.Args[2], conn.guard)
	case "tables":
		listTables(conn.DB, conn.guard)
	case "describe":
		if len(os.Args) < 3 {
			printError("Describe command requires table name")
			os.Exit(1)
		}
		describeTable(conn.DB, os.Args[2], conn.guard)
	}
}

func getEnvPath() string {
	exeDir := getExecutableDir()
	if exeDir != "" {
		if envPath := filepath.Join(exeDir, ".env"); fileExists(envPath) {
			return envPath
		}
	}
	return ".env"
}

func listAvailableDynamicConnections() {
	envPath := getEnvPath()
	envFile, err := os.Open(envPath)
	if err != nil {
		fmt.Println("No .env file found")
		return
	}
	defer envFile.Close()

	scanner := bufio.NewScanner(envFile)
	knownAliases := make(map[string]struct {
		server   string
		database string
	})
	for scanner.Scan() {
		line := scanner.Text()
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.HasPrefix(key, "MSSQL_DYNAMIC_") && strings.HasSuffix(key, "_SERVER") && value != "" {
				alias := strings.TrimPrefix(strings.TrimSuffix(key, "_SERVER"), "MSSQL_DYNAMIC_")
				knownAliases[alias] = struct {
					server   string
					database string
				}{server: value}
			}
			if strings.HasPrefix(key, "MSSQL_DYNAMIC_") && strings.HasSuffix(key, "_DATABASE") && value != "" {
				alias := strings.TrimPrefix(strings.TrimSuffix(key, "_DATABASE"), "MSSQL_DYNAMIC_")
				if entry, ok := knownAliases[alias]; ok {
					entry.database = value
					knownAliases[alias] = entry
				}
			}
		}
	}

	fmt.Println("Available dynamic connections:")
	for alias, entry := range knownAliases {
		active := ""
		if cliState.activeAlias == alias {
			active = " (active)"
		}
		fmt.Printf("  - %s (%s/%s)%s\n", alias, entry.server, entry.database, active)
	}
	if len(knownAliases) == 0 {
		fmt.Println("  No dynamic connections configured")
	}
}

func connectDynamic(alias string) {
	if err := connectDynamicDB(alias); err != nil {
		printError("%v", err)
		os.Exit(1)
	}
	cliState.activeAlias = alias
	saveState()
	fmt.Printf("Connected to '%s' (%s/%s)\n", alias, cliState.connections[alias].Server, cliState.connections[alias].Database)
}

// connectDynamicDB establishes a connection without modifying active alias or saving state
func connectDynamicDB(alias string) error {
	if len(cliState.connections) >= cliState.maxDynamicConns {
		return fmt.Errorf("maximum dynamic connections (%d) reached", cliState.maxDynamicConns)
	}

	prefix := "MSSQL_DYNAMIC_" + strings.ToUpper(alias) + "_"
	server := os.Getenv(prefix + "SERVER")
	database := os.Getenv(prefix + "DATABASE")
	user := os.Getenv(prefix + "USER")
	password := os.Getenv(prefix + "PASSWORD")
	portStr := os.Getenv(prefix + "PORT")
	auth := os.Getenv(prefix + "AUTH")

	if server == "" || database == "" {
		return fmt.Errorf("dynamic connection '%s' not found or incomplete", alias)
	}

	isIntegratedAuth := strings.ToLower(auth) == "integrated" || strings.ToLower(auth) == "windows"
	if !isIntegratedAuth && (user == "" || password == "") {
		return fmt.Errorf("dynamic connection '%s' missing credentials", alias)
	}

	port := 1433
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	encrypt := "true"
	trustCert := "false"
	if cliState.devMode {
		encrypt = "false"
		trustCert = "true"
	}

	var connStr string
	if isIntegratedAuth {
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;encrypt=%s;trustservercertificate=%s;integrated security=SSPI;connection timeout=30;command timeout=30",
			server, port, database, encrypt, trustCert)
	} else {
		connStr = fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
			server, port, database, user, password, encrypt, trustCert)
	}

	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	readOnly := strings.ToLower(os.Getenv(prefix+"READ_ONLY")) == "true"
	guard := sqlguard.New(sqlguard.Config{
		ReadOnly:  readOnly,
		Whitelist: sqlguard.ParseWhitelistTables(os.Getenv(prefix + "WHITELIST_TABLES")),
		Logger:    securityLogger{},
	})

	cliState.connections[alias] = &ConnectionInfo{
		Alias:    alias,
		DB:       db,
		Server:   server,
		Database: database,
		User:     user,
		guard:    guard,
	}

	return nil
}

// connectDynamicSilent reconnects without printing errors or output
func connectDynamicSilent(alias string) {
	connectDynamicDB(alias)
}

func listActiveDynamicConnections() {
	if len(cliState.connections) == 0 {
		fmt.Println("No active dynamic connections")
		return
	}
	fmt.Println("Active dynamic connections:")
	for alias, conn := range cliState.connections {
		active := ""
		if cliState.activeAlias == alias {
			active = " (active)"
		}
		fmt.Printf("  - %s (%s/%s)%s\n", alias, conn.Server, conn.Database, active)
	}
}

func disconnectDynamic(alias string) {
	conn, ok := cliState.connections[alias]
	if !ok {
		printError("Dynamic connection '%s' not found", alias)
		os.Exit(1)
	}
	conn.DB.Close()
	delete(cliState.connections, alias)
	if cliState.activeAlias == alias {
		// Switch to another active connection if available
		for a := range cliState.connections {
			cliState.activeAlias = a
			break
		}
		if len(cliState.connections) == 0 {
			cliState.activeAlias = ""
		}
	}
	saveState()
	fmt.Printf("Disconnected '%s'\n", alias)
}

func testDynamicConnection(conn *ConnectionInfo) {
	result := QueryResult{Success: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var version string
	userDisplay := conn.User
	if conn.User == "" {
		userDisplay = "Integrated"
	}
	err := conn.DB.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Test query failed: %v", err)
	} else {
		result.Info = fmt.Sprintf("✅ Connection successful!\nServer: %s\nDatabase: %s\nUser: %s\nTLS: Enabled\nVersion: %s",
			conn.Server, conn.Database, userDisplay, strings.TrimSpace(version))
	}

	printResult(result)
}

func showDynamicInfo(conn *ConnectionInfo) {
	result := QueryResult{Success: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queries := map[string]string{
		"version":    "SELECT @@VERSION as version",
		"servername": "SELECT @@SERVERNAME as server_name",
		"dbname":     "SELECT DB_NAME() as database_name",
		"user":       "SELECT SYSTEM_USER as current_user",
	}

	info := make(map[string]interface{})
	for key, query := range queries {
		var value string
		err := conn.DB.QueryRowContext(ctx, query).Scan(&value)
		if err != nil {
			info[key] = fmt.Sprintf("Error: %v", err)
		} else {
			info[key] = value
		}
	}

	result.Data = []map[string]interface{}{info}
	printResult(result)
}

func loadConfig() (*DatabaseConfig, *sqlguard.Guard, error) {
	config := &DatabaseConfig{
		Server:   os.Getenv("MSSQL_SERVER"),
		Database: os.Getenv("MSSQL_DATABASE"),
		User:     os.Getenv("MSSQL_USER"),
		Password: os.Getenv("MSSQL_PASSWORD"),
		Port:     os.Getenv("MSSQL_PORT"),
		DevMode:  strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true",
		Auth:     strings.ToLower(os.Getenv("MSSQL_AUTH")),
	}

	if config.Auth == "" {
		config.Auth = "sql" // default to SQL authentication
	}

	if config.Server == "" {
		return nil, nil, fmt.Errorf("missing required environment variable: MSSQL_SERVER")
	}

	// For Windows Auth, database is optional (allows exploring all databases)
	// For SQL Auth, database is required
	if config.Auth == "sql" {
		if config.Database == "" {
			return nil, nil, fmt.Errorf("missing required environment variable for SQL auth: MSSQL_DATABASE")
		}
		if config.User == "" || config.Password == "" {
			return nil, nil, fmt.Errorf("missing required environment variables for SQL auth: MSSQL_USER, MSSQL_PASSWORD")
		}
	}

	if config.Port == "" {
		config.Port = "1433"
	}

	// Create security guard
	guard := sqlguard.New(sqlguard.Config{
		ReadOnly:  strings.ToLower(os.Getenv("MSSQL_READ_ONLY")) == "true",
		Whitelist: sqlguard.ParseWhitelistTables(os.Getenv("MSSQL_WHITELIST_TABLES")),
		Logger:    securityLogger{},
	})

	return config, guard, nil
}

func connectDatabase(config *DatabaseConfig) (*sql.DB, error) {
	trustCert := "false"
	encrypt := "true"
	if config.DevMode {
		trustCert = "true"
		// In development mode, allow disabling encryption for older SQL Server instances
		if envEncrypt := os.Getenv("MSSQL_ENCRYPT"); envEncrypt != "" {
			encrypt = strings.ToLower(envEncrypt)
		} else {
			encrypt = "false"
		}
	}

	// Check if a complete custom connection string is provided
	if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
		connStrLower := strings.ToLower(customConnStr)
		if !strings.Contains(connStrLower, "connection timeout") {
			customConnStr += ";connection timeout=30"
		}
		if !strings.Contains(connStrLower, "command timeout") {
			customConnStr += ";command timeout=30"
		}
		db, err := sql.Open("sqlserver", customConnStr)
		if err != nil {
			return nil, fmt.Errorf("failed to open connection: %w", err)
		}
		return db, nil
	}

	// Build connection string depending on requested authentication mode
	var connStr string
	switch strings.ToLower(config.Auth) {
	case "integrated", "windows":
		// Windows Integrated Authentication (SSPI) — only works on Windows.
		if config.Database != "" {
			connStr = fmt.Sprintf("server=%s;port=%s;database=%s;encrypt=%s;trustservercertificate=%s;integrated security=SSPI;connection timeout=30;command timeout=30",
				config.Server, config.Port, config.Database, encrypt, trustCert)
		} else {
			connStr = fmt.Sprintf("server=%s;port=%s;encrypt=%s;trustservercertificate=%s;integrated security=SSPI;connection timeout=30;command timeout=30",
				config.Server, config.Port, encrypt, trustCert)
		}
	default:
		// Default to SQL Server authentication (user/password)
		connStr = fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=%s;trustservercertificate=%s;connection timeout=30;command timeout=30",
			config.Server, config.Port, config.Database, config.User, config.Password, encrypt, trustCert)
	}

	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		if cerr := db.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close db after ping failure: %v\n", cerr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func testConnection(db *sql.DB, config *DatabaseConfig) {
	result := QueryResult{Success: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var version string
	userDisplay := config.User
	if strings.ToLower(config.Auth) == "integrated" || strings.ToLower(config.Auth) == "windows" {
		if u, err := osuser.Current(); err == nil {
			userDisplay = fmt.Sprintf("Integrated (%s)", u.Username)
		} else {
			userDisplay = "Integrated"
		}
	}
	err := db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Test query failed: %v", err)
	} else {
		dbDisplay := config.Database
		if dbDisplay == "" {
			dbDisplay = "(default/all available)"
		}
		result.Info = fmt.Sprintf("✅ Connection successful!\nServer: %s:%s\nDatabase: %s\nUser: %s\nTLS: Enabled\nVersion: %s",
			config.Server, config.Port, dbDisplay, userDisplay, strings.TrimSpace(version))
	}

	printResult(result)
}

func showDatabaseInfo(db *sql.DB, config *DatabaseConfig) {
	result := QueryResult{Success: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queries := map[string]string{
		"version":    "SELECT @@VERSION as version",
		"servername": "SELECT @@SERVERNAME as server_name",
		"dbname":     "SELECT DB_NAME() as database_name",
		"user":       "SELECT SYSTEM_USER as current_user",
	}

	info := make(map[string]interface{})
	for key, query := range queries {
		var value string
		err := db.QueryRowContext(ctx, query).Scan(&value)
		if err != nil {
			info[key] = fmt.Sprintf("Error: %v", err)
		} else {
			info[key] = value
		}
	}

	result.Data = []map[string]interface{}{info}
	printResult(result)
}

func executeQuery(db *sql.DB, query string, guard *sqlguard.Guard) {
	result := QueryResult{Success: true}

	// Security validation before execution
	if err := guard.ValidateReadOnly(query); err != nil {
		result.Success = false
		result.Error = err.Error()
		printResult(result)
		return
	}

	if err := guard.ValidateTablePermissions(query); err != nil {
		result.Success = false
		result.Error = err.Error()
		printResult(result)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Determine if it's a SELECT query or other
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(queryUpper, "SELECT") || strings.HasPrefix(queryUpper, "WITH") {
		executeSelectQuery(db, ctx, query, &result)
	} else {
		executeNonSelectQuery(db, ctx, query, &result)
	}

	printResult(result)
}

func executeSelectQuery(db *sql.DB, ctx context.Context, query string, result *QueryResult) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Query failed: %v", err)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to get columns: %v", err)
		return
	}

	var data []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to scan row: %v", err)
			return
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
		data = append(data, row)
	}

	result.Data = data
	result.Info = fmt.Sprintf("Query executed successfully. Rows returned: %d", len(data))
}

func executeNonSelectQuery(db *sql.DB, ctx context.Context, query string, result *QueryResult) {
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Query failed: %v", err)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		result.Info = "Query executed successfully (rows affected unknown)"
	} else {
		result.Info = fmt.Sprintf("Query executed successfully. Rows affected: %d", rowsAffected)
	}
}

func listTables(db *sql.DB, guard *sqlguard.Guard) {
	query := `
		SELECT
			TABLE_SCHEMA as schema_name,
			TABLE_NAME as table_name,
			TABLE_TYPE as table_type
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`
	executeQuery(db, query, guard)
}

func describeTable(db *sql.DB, tableName string, guard *sqlguard.Guard) {
	query := `
		SELECT
			COLUMN_NAME as column_name,
			DATA_TYPE as data_type,
			IS_NULLABLE as is_nullable,
			COLUMN_DEFAULT as default_value,
			CHARACTER_MAXIMUM_LENGTH as max_length
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = @p1
		ORDER BY ORDINAL_POSITION
	`

	result := QueryResult{Success: true}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Query failed: %v", err)
		printResult(result)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to get columns: %v", err)
		printResult(result)
		return
	}

	var data []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to scan row: %v", err)
			printResult(result)
			return
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
		data = append(data, row)
	}

	if len(data) == 0 {
		result.Success = false
		result.Error = fmt.Sprintf("Table '%s' not found", tableName)
	} else {
		result.Data = data
		result.Info = fmt.Sprintf("Table structure for '%s'", tableName)
	}

	printResult(result)
}

func printResult(result QueryResult) {
	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(output))
}

func printError(format string, args ...interface{}) {
	result := QueryResult{
		Success: false,
		Error:   fmt.Sprintf(format, args...),
	}
	printResult(result)
}