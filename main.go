package main

import (
	"log"
	"net/http"
	"t2cbackend/api"
	"t2cbackend/database"
)

func main() {
	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	log.Println("Database initialized successfully")

	// Setup router
	router := api.SetupRouter()

	// Start server
	port := ":8080"
	log.Printf("Server starting on http://localhost%s", port)
	log.Printf("API endpoints available at http://localhost%s/api/", port)
	
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}