
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"pharmacy-backend/config"
	"pharmacy-backend/routes"
)

func main() {
	// Load Environment Variables
	if err := godotenv.Load(); err != nil {
		log.Fatal("Cannot load .env file:", err)
	}

	log.Println(".env loaded successfully")

	// Check JWT Secret
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET is empty. Please check your .env file")
	}

	log.Println("JWT_SECRET loaded successfully")

	// Connect Database
	db, err := config.Connect()
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer db.Close()

	// Setup Routes
	mux := routes.New(db)

	// Server Port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on port", port)

	// Start Server
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
