package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// DatabaseConfig holds connection configuration
type DatabaseConfig struct {
	Server   string `json:"server"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	Port     string `json:"port"`
	DevMode  bool   `json:"developer_mode"`
}

// QueryResult holds query execution results
type QueryResult struct {
	Success bool                     `json:"success"`
	Data    []map[string]interface{} `json:"data,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Info    string                   `json:"info,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run db-connector.go test                    # Test connection")
		fmt.Println("  go run db-connector.go info                    # Show database info")
		fmt.Println("  go run db-connector.go query \"SELECT ...\"      # Execute query")
		fmt.Println("  go run db-connector.go tables                  # List all tables")
		fmt.Println("  go run db-connector.go describe table_name     # Describe table structure")
		os.Exit(1)
	}

	command := os.Args[1]
	
	config, err := loadConfig()
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
		executeQuery(db, os.Args[2])
	case "tables":
		listTables(db)
	case "describe":
		if len(os.Args) < 3 {
			printError("Describe command requires table name")
			os.Exit(1)
		}
		describeTable(db, os.Args[2])
	default:
		printError("Unknown command: %s", command)
		os.Exit(1)
	}
}

func loadConfig() (*DatabaseConfig, error) {
	config := &DatabaseConfig{
		Server:   os.Getenv("MSSQL_SERVER"),
		Database: os.Getenv("MSSQL_DATABASE"),
		User:     os.Getenv("MSSQL_USER"),
		Password: os.Getenv("MSSQL_PASSWORD"),
		Port:     os.Getenv("MSSQL_PORT"),
		DevMode:  strings.ToLower(os.Getenv("DEVELOPER_MODE")) == "true",
	}

	if config.Server == "" || config.Database == "" || config.User == "" || config.Password == "" {
		return nil, fmt.Errorf("missing required environment variables: MSSQL_SERVER, MSSQL_DATABASE, MSSQL_USER, MSSQL_PASSWORD")
	}

	if config.Port == "" {
		config.Port = "1433"
	}

	return config, nil
}

func connectDatabase(config *DatabaseConfig) (*sql.DB, error) {
	trustCert := "false"
	if config.DevMode {
		trustCert = "true"
	}

	connStr := fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=true;trustservercertificate=%s;connection timeout=30;command timeout=30",
		config.Server, config.Database, config.User, config.Password, config.Port, trustCert)

	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
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
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return db, nil
}

func testConnection(db *sql.DB, config *DatabaseConfig) {
	result := QueryResult{Success: true}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var version string
	err := db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Test query failed: %v", err)
	} else {
		result.Info = fmt.Sprintf("✅ Connection successful!\nServer: %s:%s\nDatabase: %s\nUser: %s\nTLS: Enabled\nVersion: %s",
			config.Server, config.Port, config.Database, config.User, strings.TrimSpace(version))
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

func executeQuery(db *sql.DB, query string) {
	result := QueryResult{Success: true}
	
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

func listTables(db *sql.DB) {
	query := `
		SELECT 
			TABLE_SCHEMA as schema_name,
			TABLE_NAME as table_name,
			TABLE_TYPE as table_type
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_TYPE IN ('BASE TABLE', 'VIEW')
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`
	executeQuery(db, query)
}

func describeTable(db *sql.DB, tableName string) {
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

		rows.Scan(valuePtrs...)
		
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