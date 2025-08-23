package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	// Test connection parameters
	server := "10.203.3.10"
	database := "JJP_TRANSFER"
	user := "userTRANSFER"
	password := "jl3RN7o02g"
	port := "1433"

	// Build connection string (with trust certificate for development)
	connStr := fmt.Sprintf("server=%s;database=%s;user id=%s;password=%s;port=%s;encrypt=true;trustservercertificate=true;connection timeout=30;command timeout=30",
		server, database, user, password, port)

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