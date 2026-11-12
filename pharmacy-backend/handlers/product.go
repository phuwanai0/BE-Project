package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductHandler struct {
	DB *pgxpool.Pool
}

type Product struct {
	ProductID    int     `json:"product_id"`
	ProductName  string  `json:"product_name"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	Stock        int     `json:"stock"`
	CategoryID   int     `json:"category_id"`
	CategoryName string  `json:"category_name,omitempty"`
}

type ProductInput struct {
	ProductName string  `json:"product_name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	CategoryID  int     `json:"category_id"`
}

func validProduct(p ProductInput) bool {
	return strings.TrimSpace(p.ProductName) != "" &&
		strings.TrimSpace(p.Description) != "" &&
		p.Price >= 0 &&
		p.Stock >= 0 &&
		p.CategoryID > 0
}

// GET /api/products
func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `
		SELECT p.product_id, p.product_name, p.description,
		       p.price, p.stock, p.category_id, c.category_name
		FROM products p
		JOIN categories c ON p.category_id = c.category_id
		ORDER BY p.product_id
	`

	rows, err := h.DB.Query(ctx, query)
	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()

	products := []Product{}

	for rows.Next() {
		var p Product

		if err := rows.Scan(
			&p.ProductID,
			&p.ProductName,
			&p.Description,
			&p.Price,
			&p.Stock,
			&p.CategoryID,
			&p.CategoryName,
		); err != nil {
			writeDBError(w, err)
			return
		}

		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, products)
}

// GET /api/products/{id}
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var p Product

	err = h.DB.QueryRow(ctx, `
		SELECT p.product_id, p.product_name, p.description,
		       p.price, p.stock, p.category_id, c.category_name
		FROM products p
		JOIN categories c ON p.category_id = c.category_id
		WHERE p.product_id = $1
	`, id).Scan(
		&p.ProductID,
		&p.ProductName,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.CategoryID,
		&p.CategoryName,
	)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// POST /api/products
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var p ProductInput

	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if !validProduct(p) {
		writeError(w, http.StatusBadRequest, "Invalid product data")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var result Product

	err := h.DB.QueryRow(ctx, `
		INSERT INTO products
			(product_name, description, price, stock, category_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING product_id, product_name, description,
		          price, stock, category_id
	`,
		strings.TrimSpace(p.ProductName),
		strings.TrimSpace(p.Description),
		p.Price,
		p.Stock,
		p.CategoryID,
	).Scan(
		&result.ProductID,
		&result.ProductName,
		&result.Description,
		&result.Price,
		&result.Stock,
		&result.CategoryID,
	)

	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// PUT /api/products/{id}
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var p ProductInput

	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if !validProduct(p) {
		writeError(w, http.StatusBadRequest, "Invalid product data")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var result Product

	err = h.DB.QueryRow(ctx, `
		UPDATE products
		SET product_name = $1,
		    description = $2,
		    price = $3,
		    stock = $4,
		    category_id = $5
		WHERE product_id = $6
		RETURNING product_id, product_name, description,
		          price, stock, category_id
	`,
		strings.TrimSpace(p.ProductName),
		strings.TrimSpace(p.Description),
		p.Price,
		p.Stock,
		p.CategoryID,
		id,
	).Scan(
		&result.ProductID,
		&result.ProductName,
		&result.Description,
		&result.Price,
		&result.Stock,
		&result.CategoryID,
	)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DELETE /api/products/{id}
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tag, err := h.DB.Exec(ctx,
		"DELETE FROM products WHERE product_id = $1", id)

	if err != nil {
		writeDBError(w, err)
		return
	}

	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Product deleted successfully",
	})
}

// GET /api/products/search?q=คำค้น
func (h *ProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	if q == "" {
		writeError(w, http.StatusBadRequest, "Search keyword is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT p.product_id, p.product_name, p.description,
		       p.price, p.stock, p.category_id, c.category_name
		FROM products p
		JOIN categories c ON p.category_id = c.category_id
		WHERE p.product_name ILIKE $1
		ORDER BY p.product_id
	`, "%"+q+"%")

	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()

	products := []Product{}

	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ProductID,
			&p.ProductName,
			&p.Description,
			&p.Price,
			&p.Stock,
			&p.CategoryID,
			&p.CategoryName,
		); err != nil {
			writeDBError(w, err)
			return
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, products)
}

// GET /api/products?category_id=1
func (h *ProductHandler) FilterProducts(w http.ResponseWriter, r *http.Request) {
	categoryText := r.URL.Query().Get("category_id")
	categoryID, err := strconv.Atoi(categoryText)

	if err != nil || categoryID <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT p.product_id, p.product_name, p.description,
		       p.price, p.stock, p.category_id, c.category_name
		FROM products p
		JOIN categories c ON p.category_id = c.category_id
		WHERE p.category_id = $1
		ORDER BY p.product_id
	`, categoryID)

	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()

	products := []Product{}

	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ProductID,
			&p.ProductName,
			&p.Description,
			&p.Price,
			&p.Stock,
			&p.CategoryID,
			&p.CategoryName,
		); err != nil {
			writeDBError(w, err)
			return
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, products)
}

// ป้องกัน unused import ในกรณีปรับโค้ดต่อ
var _ = strconv.Itoa
