package routes

import (
	"net/http"
	"os"

	"pharmacy-backend/handlers"
	"pharmacy-backend/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(db *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	productHandler := handlers.ProductHandler{DB: db}
	categoryHandler := handlers.CategoryHandler{DB: db}
	authHandler := handlers.AuthHandler{DB: db}

	// Authentication API
	mux.HandleFunc("POST /api/register", authHandler.Register)
	mux.HandleFunc("POST /api/login", authHandler.Login)

	// Product API - Public
	mux.HandleFunc("GET /api/products", productHandler.GetProducts)
	mux.HandleFunc("GET /api/products/search", productHandler.SearchProducts)
	mux.HandleFunc("GET /api/products/category", productHandler.FilterProducts)
	mux.HandleFunc("GET /api/products/{id}", productHandler.GetProduct)

	// Product API - Admin Only
	mux.Handle("POST /api/products",
		middleware.RequireAdmin(http.HandlerFunc(productHandler.CreateProduct)))

	mux.Handle("PUT /api/products/{id}",
		middleware.RequireAdmin(http.HandlerFunc(productHandler.UpdateProduct)))

	mux.Handle("DELETE /api/products/{id}",
		middleware.RequireAdmin(http.HandlerFunc(productHandler.DeleteProduct)))

	// Category API - Public
	mux.HandleFunc("GET /api/categories", categoryHandler.GetCategories)
	mux.HandleFunc("GET /api/categories/{id}", categoryHandler.GetCategory)

	// Category API - Admin Only
	mux.Handle("POST /api/categories",
		middleware.RequireAdmin(http.HandlerFunc(categoryHandler.CreateCategory)))

	mux.Handle("PUT /api/categories/{id}",
		middleware.RequireAdmin(http.HandlerFunc(categoryHandler.UpdateCategory)))

	mux.Handle("DELETE /api/categories/{id}",
		middleware.RequireAdmin(http.HandlerFunc(categoryHandler.DeleteCategory)))

	// Health Check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	return cors(mux)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := os.Getenv("FRONTEND_URL")

		if origin == "" {
			origin = "http://localhost:3000"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
