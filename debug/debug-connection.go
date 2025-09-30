package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	fmt.Println("=== MCP Go MSSQL Connection Debug Tool ===")

	// Read environment variables or use defaults for testing
	server := getEnvOrDefault("MSSQL_SERVER", "SERVER-GDP")
	database := getEnvOrDefault("MSSQL_DATABASE", "GDPA")
	user := getEnvOrDefault("MSSQL_USER", "sa")
	password := getEnvOrDefault("MSSQL_PASSWORD", "aidima")
	port := getEnvOrDefault("MSSQL_PORT", "1433")
	encrypt := getEnvOrDefault("MSSQL_ENCRYPT", "false")
	devMode := getEnvOrDefault("DEVELOPER_MODE", "true")

	fmt.Printf("Environment Variables:\n")
	fmt.Printf("  MSSQL_SERVER: %s\n", server)
	fmt.Printf("  MSSQL_DATABASE: %s\n", database)
	fmt.Printf("  MSSQL_USER: %s\n", user)
	fmt.Printf("  MSSQL_PASSWORD: %s\n", func() string { if password != "" { return "***SET***" } else { return "NOT SET" } }())
	fmt.Printf("  MSSQL_PORT: %s\n", port)
	fmt.Printf("  MSSQL_ENCRYPT: %s\n", encrypt)
	fmt.Printf("  DEVELOPER_MODE: %s\n", devMode)
	fmt.Println()

	if port == "" {
		port = "1433"
	}

	// Check for custom connection string first
	if customConnStr := os.Getenv("MSSQL_CONNECTION_STRING"); customConnStr != "" {
		fmt.Printf("Testing custom connection string:\n")
		fmt.Printf("Connection string: %s\n", strings.Replace(customConnStr, password, "***", -1))

		db, err := sql.Open("sqlserver", customConnStr)
		if err != nil {
			fmt.Printf("❌ sql.Open failed: %v\n\n", err)
			return
		}

		db.SetConnMaxLifetime(time.Minute * 3)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err != nil {
			fmt.Printf("❌ Ping failed: %v\n", err)
		} else {
			fmt.Printf("✅ Connection successful!\n")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var result string
			err = db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&result)
			cancel()

			if err != nil {
				fmt.Printf("❌ Query failed: %v\n", err)
			} else {
				fmt.Printf("✅ Query successful!\n")
				fmt.Printf("SQL Server Version: %s\n", result[:50]+"...")
			}
		}

		db.Close()
		fmt.Printf("\n=== Custom Connection String Test Complete ===\n")
		return
	}

	// Test different connection string formats
	connectionStrings := []string{
		// Format 1: Current MCP format
		fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;encrypt=false;trustservercertificate=true;connection timeout=30;command timeout=30",
			server, port, database, user, password),

		// Format 2: SSMS-like format without port in server
		fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=false;trustservercertificate=true;connection timeout=30",
			server, database, user, password, port),

		// Format 3: Data Source format (like SSMS)
		fmt.Sprintf("data source=%s;initial catalog=%s;user id=%s;password=%s;encrypt=false;trustservercertificate=true",
			server, database, user, password),

		// Format 4: SQL Server URL format
		fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&encrypt=disable&trustservercertificate=true",
			user, password, server, port, database),

		// Format 5: Exact SSMS format
		fmt.Sprintf("Data Source=%s;Persist Security Info=True;User ID=%s;Password=%s;Pooling=False;MultipleActiveResultSets=False;Encrypt=False;TrustServerCertificate=False;Command Timeout=0",
			server, user, password),
	}

	for i, connStr := range connectionStrings {
		fmt.Printf("Testing connection string format %d:\n", i+1)
		fmt.Printf("Connection string: %s\n", strings.Replace(connStr, password, "***", -1))

		db, err := sql.Open("sqlserver", connStr)
		if err != nil {
			fmt.Printf("❌ sql.Open failed: %v\n\n", err)
			continue
		}

		// Set timeouts
		db.SetConnMaxLifetime(time.Minute * 3)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err != nil {
			fmt.Printf("❌ Ping failed: %v\n", err)
		} else {
			fmt.Printf("✅ Connection successful!\n")

			// Test simple query
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var result string
			err = db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&result)
			cancel()

			if err != nil {
				fmt.Printf("❌ Query failed: %v\n", err)
			} else {
				fmt.Printf("✅ Query successful!\n")
				fmt.Printf("SQL Server Version: %s\n", result[:50]+"...")
			}
		}

		db.Close()
		fmt.Println()
	}

	fmt.Println("=== Debug Complete ===")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}