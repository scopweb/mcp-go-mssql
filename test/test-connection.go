package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	// Test connection parameters from environment variables
	server := os.Getenv("MSSQL_SERVER")
	database := os.Getenv("MSSQL_DATABASE")
	user := os.Getenv("MSSQL_USER")
	password := os.Getenv("MSSQL_PASSWORD")
	port := os.Getenv("MSSQL_PORT")
	
	// Check required environment variables
	if server == "" || database == "" || user == "" || password == "" {
		log.Fatal("Missing required environment variables: MSSQL_SERVER, MSSQL_DATABASE, MSSQL_USER, MSSQL_PASSWORD")
	}
	
	if port == "" {
		port = "1433"
	}

	// Build connection string with appropriate certificate trust setting
	trustCert := "false"
	if os.Getenv("DEVELOPER_MODE") == "true" {
		trustCert = "true"
	}
	
	connStr := fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=true;trustservercertificate=%s;connection timeout=30;command timeout=30",
		server, database, user, password, port, trustCert)

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