package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CartHandler struct {
	DB *pgxpool.Pool
}

type CartItemRequest struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// ส่ง JSON response
func cartWriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// อ่าน user_id จาก Context
func getCartUserID(r *http.Request) (int, bool) {
	userID, ok := r.Context().Value("user_id").(int)
	return userID, ok && userID > 0
}

// สร้างตะกร้าหากยังไม่มี
func (h *CartHandler) getOrCreateCart(ctx context.Context, userID int) (int, error) {
	var cartID int

	err := h.DB.QueryRow(
		ctx,
		`INSERT INTO cart (user_id)
		 VALUES ($1)
		 ON CONFLICT (user_id)
		 DO UPDATE SET user_id = EXCLUDED.user_id
		 RETURNING cart_id`,
		userID,
	).Scan(&cartID)

	return cartID, err
}

// GET /api/cart
func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := getCartUserID(r)
	if !ok {
		cartWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	cartID, err := h.getOrCreateCart(r.Context(), userID)
	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเปิดตะกร้าได้",
		})
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT
			ci.cart_item_id,
			ci.product_id,
			p.product_name,
			ci.quantity,
			p.price,
			p.stock
		FROM cart_items ci
		JOIN products p ON p.product_id = ci.product_id
		WHERE ci.cart_id = $1
		ORDER BY ci.cart_item_id DESC
	`, cartID)

	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถโหลดตะกร้าได้",
		})
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}

	for rows.Next() {
		var (
			itemID      int
			productID   int
			productName string
			quantity    int
			price       float64
			stock       int
		)

		err := rows.Scan(
			&itemID,
			&productID,
			&productName,
			&quantity,
			&price,
			&stock,
		)

		if err != nil {
			cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "อ่านข้อมูลสินค้าไม่สำเร็จ",
			})
			return
		}

		items = append(items, map[string]interface{}{
			"cart_item_id": itemID,
			"product_id":   productID,
			"product_name": productName,
			"quantity":     quantity,
			"price":        price,
			"stock":        stock,
		})
	}

	if err := rows.Err(); err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "อ่านรายการตะกร้าไม่สำเร็จ",
		})
		return
	}

	cartWriteJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

// POST /api/cart/items
func (h *CartHandler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getCartUserID(r)
	if !ok {
		cartWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	var req CartItemRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ข้อมูลไม่ถูกต้อง",
		})
		return
	}

	if req.ProductID <= 0 {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "กรุณาระบุสินค้า",
		})
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	ctx := r.Context()

	cartID, err := h.getOrCreateCart(ctx, userID)
	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเปิดตะกร้าได้",
		})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเริ่มรายการได้",
		})
		return
	}
	defer tx.Rollback(ctx)

	var stock int

	err = tx.QueryRow(ctx,
		`SELECT stock FROM products WHERE product_id = $1 FOR UPDATE`,
		req.ProductID,
	).Scan(&stock)

	if err != nil {
		if err == pgx.ErrNoRows {
			cartWriteJSON(w, http.StatusNotFound, map[string]string{
				"error": "ไม่พบสินค้า",
			})
			return
		}

		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ตรวจสอบสต็อกสินค้าไม่สำเร็จ",
		})
		return
	}

	var currentQty int

	err = tx.QueryRow(ctx, `
		SELECT quantity
		FROM cart_items
		WHERE cart_id = $1 AND product_id = $2
		FOR UPDATE
	`, cartID, req.ProductID).Scan(&currentQty)

	if err != nil && err != pgx.ErrNoRows {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ตรวจสอบตะกร้าไม่สำเร็จ",
		})
		return
	}

	if currentQty+req.Quantity > stock {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "จำนวนสินค้าเกินสต็อกที่มี",
		})
		return
	}

	if err == pgx.ErrNoRows {
		_, err = tx.Exec(ctx, `
			INSERT INTO cart_items (cart_id, product_id, quantity)
			VALUES ($1, $2, $3)
		`, cartID, req.ProductID, req.Quantity)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE cart_items
			SET quantity = quantity + $1
			WHERE cart_id = $2 AND product_id = $3
		`, req.Quantity, cartID, req.ProductID)
	}

	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "เพิ่มสินค้าลงตะกร้าไม่สำเร็จ",
		})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "บันทึกตะกร้าไม่สำเร็จ",
		})
		return
	}

	cartWriteJSON(w, http.StatusCreated, map[string]string{
		"message": "เพิ่มสินค้าลงตะกร้าแล้ว",
	})
}

// PUT /api/cart/items/{id}
func (h *CartHandler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getCartUserID(r)
	if !ok {
		cartWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	itemID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || itemID <= 0 {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "รหัสสินค้าไม่ถูกต้อง",
		})
		return
	}

	var req CartItemRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Quantity <= 0 {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "จำนวนสินค้าไม่ถูกต้อง",
		})
		return
	}

	tag, err := h.DB.Exec(r.Context(), `
		UPDATE cart_items ci
		SET quantity = $1
		FROM cart c, products p
		WHERE ci.cart_item_id = $2
		  AND ci.cart_id = c.cart_id
		  AND c.user_id = $3
		  AND ci.product_id = p.product_id
		  AND $1 <= p.stock
	`, req.Quantity, itemID, userID)

	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "แก้ไขจำนวนสินค้าไม่สำเร็จ",
		})
		return
	}

	if tag.RowsAffected() == 0 {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ไม่พบสินค้าในตะกร้าหรือจำนวนเกินสต็อก",
		})
		return
	}

	cartWriteJSON(w, http.StatusOK, map[string]string{
		"message": "แก้ไขจำนวนสินค้าแล้ว",
	})
}

// DELETE /api/cart/items/{id}
func (h *CartHandler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := getCartUserID(r)
	if !ok {
		cartWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	itemID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || itemID <= 0 {
		cartWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "รหัสสินค้าไม่ถูกต้อง",
		})
		return
	}

	tag, err := h.DB.Exec(r.Context(), `
		DELETE FROM cart_items ci
		USING cart c
		WHERE ci.cart_id = c.cart_id
		  AND ci.cart_item_id = $1
		  AND c.user_id = $2
	`, itemID, userID)

	if err != nil {
		cartWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ลบสินค้าไม่สำเร็จ",
		})
		return
	}

	if tag.RowsAffected() == 0 {
		cartWriteJSON(w, http.StatusNotFound, map[string]string{
			"error": "ไม่พบสินค้าในตะกร้า",
		})
		return
	}

	cartWriteJSON(w, http.StatusOK, map[string]string{
		"message": "ลบสินค้าแล้ว",
	})
}
