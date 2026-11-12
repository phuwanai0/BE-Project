
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
	// Load .env (ถ้าไม่มี ให้ใช้ Environment Variables)
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	} else {
		log.Println(".env loaded successfully")
	}

	// Check JWT Secret
	if os.Getenv("JWT_SECRET") == "" {
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
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
