package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryHandler struct {
	DB *pgxpool.Pool
}

type Category struct {
	CategoryID   int    `json:"category_id"`
	CategoryName string `json:"category_name"`
}

type CategoryInput struct {
	CategoryName string `json:"category_name"`
}

// GET /api/categories
func (h *CategoryHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT category_id, category_name
		FROM categories
		ORDER BY category_id
	`)

	if err != nil {
		writeDBError(w, err)
		return
	}
	defer rows.Close()

	categories := []Category{}

	for rows.Next() {
		var c Category

		if err := rows.Scan(&c.CategoryID, &c.CategoryName); err != nil {
			writeDBError(w, err)
			return
		}

		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, categories)
}

// GET /api/categories/{id}
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var c Category

	err = h.DB.QueryRow(ctx, `
		SELECT category_id, category_name
		FROM categories
		WHERE category_id = $1
	`, id).Scan(&c.CategoryID, &c.CategoryName)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// POST /api/categories
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var input CategoryInput

	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	input.CategoryName = strings.TrimSpace(input.CategoryName)

	if input.CategoryName == "" {
		writeError(w, http.StatusBadRequest, "Category name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var c Category

	err := h.DB.QueryRow(ctx, `
		INSERT INTO categories (category_name)
		VALUES ($1)
		RETURNING category_id, category_name
	`, input.CategoryName).Scan(&c.CategoryID, &c.CategoryName)

	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

// PUT /api/categories/{id}
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	var input CategoryInput

	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	input.CategoryName = strings.TrimSpace(input.CategoryName)

	if input.CategoryName == "" {
		writeError(w, http.StatusBadRequest, "Category name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var c Category

	err = h.DB.QueryRow(ctx, `
		UPDATE categories
		SET category_name = $1
		WHERE category_id = $2
		RETURNING category_id, category_name
	`, input.CategoryName, id).Scan(&c.CategoryID, &c.CategoryName)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}
	if err != nil {
		writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// DELETE /api/categories/{id}
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid category ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tag, err := h.DB.Exec(ctx,
		"DELETE FROM categories WHERE category_id = $1", id)

	if err != nil {
		writeDBError(w, err)
		return
	}

	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Category deleted successfully",
	})
}
