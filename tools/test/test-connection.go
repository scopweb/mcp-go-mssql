package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Helper function to get env variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Load from .env file or environment variables
	server := getEnvOrDefault("MSSQL_SERVER", "10.203.3.10")
	database := getEnvOrDefault("MSSQL_DATABASE", "JJP_TRANSFER")
	user := getEnvOrDefault("MSSQL_USER", "userTRANSFER")
	password := getEnvOrDefault("MSSQL_PASSWORD", "jl3RN7o02g")
	auth := strings.ToLower(getEnvOrDefault("MSSQL_AUTH", "sql"))
	port := getEnvOrDefault("MSSQL_PORT", "1433")

	// Check required environment variables based on auth mode
	if server == "" || database == "" {
		log.Fatal("Missing required environment variables: MSSQL_SERVER, MSSQL_DATABASE")
	}
	if auth == "sql" {
		if user == "" || password == "" {
			log.Fatal("Missing required environment variables for SQL auth: MSSQL_USER, MSSQL_PASSWORD")
		}
	}

	// Build connection string with appropriate certificate trust setting
	trustCert := "false"
	if os.Getenv("DEVELOPER_MODE") == "true" {
		trustCert = "true"
	}

	var connStr string
	if auth == "integrated" || auth == "windows" {
		connStr = fmt.Sprintf("server=%s;database=%s;port=%s;encrypt=true;trustservercertificate=%s;integrated security=SSPI;connection timeout=30;command timeout=30",
			server, database, port, trustCert)
	} else {
		connStr = fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=true;trustservercertificate=%s;connection timeout=30;command timeout=30",
			server, database, user, password, port, trustCert)
	}

	fmt.Printf("Testing connection to: %s:%s\n", server, port)
	fmt.Printf("Database: %s\n", database)
	fmt.Printf("User: %s\n", user)

	// Try to connect
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		log.Fatalf("Error creating connection: %v", err)
	}
	defer db.Close()

	// Test ping
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Testing ping...")
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}

	fmt.Println("✅ Connection successful!")

	// Try a simple query
	rows, err := db.QueryContext(ctx, "SELECT @@VERSION as version")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
		fmt.Printf("SQL Server Version: %s\n", version)
	}

	fmt.Println("✅ Test completed successfully!")
}
