// Dashboard for MCP-Go-MSSQL configuration builder
package main

import (
	"embed"
	"log"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	fs := http.FileServer(http.FS(staticFiles))
	http.Handle("/", fs)

	log.Println("Dashboard disponible en http://localhost:8080")
	log.Println("Presiona Ctrl+C para detener")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
