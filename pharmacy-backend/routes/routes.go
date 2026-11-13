
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

	// INITIALIZE HANDLERS
	productHandler := handlers.ProductHandler{DB: db}
	categoryHandler := handlers.CategoryHandler{DB: db}
	authHandler := handlers.AuthHandler{DB: db}
	cartHandler := handlers.CartHandler{DB: db}
	orderHandler := handlers.OrderHandler{DB: db}
	adminHandler := handlers.AdminHandler{DB: db}

	// AUTHENTICATION
	mux.HandleFunc("POST /api/register", authHandler.Register)
	mux.HandleFunc("POST /api/login", authHandler.Login)

	// PRODUCTS - PUBLIC
	mux.HandleFunc("GET /api/products", productHandler.GetProducts)
	mux.HandleFunc("GET /api/products/search", productHandler.SearchProducts)
	mux.HandleFunc("GET /api/products/category", productHandler.FilterProducts)
	mux.HandleFunc("GET /api/products/{id}", productHandler.GetProduct)

	// PRODUCTS - ADMIN
	mux.Handle(
		"POST /api/products",
		middleware.RequireAdmin(
			http.HandlerFunc(productHandler.CreateProduct),
		),
	)

	mux.Handle(
		"PUT /api/products/{id}",
		middleware.RequireAdmin(
			http.HandlerFunc(productHandler.UpdateProduct),
		),
	)

	mux.Handle(
		"DELETE /api/products/{id}",
		middleware.RequireAdmin(
			http.HandlerFunc(productHandler.DeleteProduct),
		),
	)

	// CATEGORIES - PUBLIC
	mux.HandleFunc("GET /api/categories", categoryHandler.GetCategories)
	mux.HandleFunc("GET /api/categories/{id}", categoryHandler.GetCategory)

	// CATEGORIES - ADMIN
	mux.Handle(
		"POST /api/categories",
		middleware.RequireAdmin(
			http.HandlerFunc(categoryHandler.CreateCategory),
		),
	)

	mux.Handle(
		"PUT /api/categories/{id}",
		middleware.RequireAdmin(
			http.HandlerFunc(categoryHandler.UpdateCategory),
		),
	)

	mux.Handle(
		"DELETE /api/categories/{id}",
		middleware.RequireAdmin(
			http.HandlerFunc(categoryHandler.DeleteCategory),
		),
	)

	// CART - AUTHENTICATED
	mux.Handle(
		"GET /api/cart",
		middleware.RequireAuth(
			http.HandlerFunc(cartHandler.GetCart),
		),
	)

	mux.Handle(
		"POST /api/cart/items",
		middleware.RequireAuth(
			http.HandlerFunc(cartHandler.AddCartItem),
		),
	)

	mux.Handle(
		"PUT /api/cart/items/{id}",
		middleware.RequireAuth(
			http.HandlerFunc(cartHandler.UpdateCartItem),
		),
	)

	mux.Handle(
		"DELETE /api/cart/items/{id}",
		middleware.RequireAuth(
			http.HandlerFunc(cartHandler.RemoveCartItem),
		),
	)

	// ORDERS - AUTHENTICATED
	mux.Handle(
		"POST /api/orders",
		middleware.RequireAuth(
			http.HandlerFunc(orderHandler.CreateOrder),
		),
	)

	mux.Handle(
		"GET /api/orders/my",
		middleware.RequireAuth(
			http.HandlerFunc(orderHandler.GetMyOrders),
		),
	)

	// ADMIN USERS
	mux.Handle(
		"GET /api/admin/users",
		middleware.RequireAdmin(
			http.HandlerFunc(adminHandler.GetUsers),
		),
	)

	// Admin can still change a member's password
	mux.Handle(
		"PUT /api/admin/users/{id}/password",
		middleware.RequireAdmin(
			http.HandlerFunc(adminHandler.UpdateUserPassword),
		),
	)

	mux.Handle(
		"DELETE /api/admin/users/{id}",
		middleware.RequireAdmin(
			http.HandlerFunc(adminHandler.DeleteUser),
		),
	)

	// ADMIN ORDERS
	mux.Handle(
		"GET /api/admin/orders",
		middleware.RequireAdmin(
			http.HandlerFunc(orderHandler.GetAllOrders),
		),
	)

	// HEALTH CHECK
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"status":"ok","orders_api":true}`))
	})

	return cors(mux)
}

// CORS
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := os.Getenv("FRONTEND_URL")

		if origin == "" {
			origin = "http://localhost:5173"
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
