
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
	// Load .env (Render ใช้ Environment Variables)
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	} else {
		log.Println(".env loaded successfully")
	}

	// Check JWT_SECRET
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is empty. Please check Environment Variables")
	}

	log.Println("JWT_SECRET loaded successfully")

	// Connect Database
	db, err := config.Connect()
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// Setup Routes
	mux := routes.New(db)

	// Server Port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on port", port)

	// Start Server
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Server stopped:", err)
	}
}
