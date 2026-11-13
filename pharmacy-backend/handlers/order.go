package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderHandler struct {
	DB *pgxpool.Pool
}

type OrderItemResponse struct {
	ProductID int     `json:"product_id"`
	Name      string  `json:"product_name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Subtotal  float64 `json:"subtotal"`
}

type OrderResponse struct {
	OrderID int                 `json:"order_id"`
	Total   float64             `json:"total"`
	Items   []OrderItemResponse `json:"items"`
}

func orderWriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// อ่าน user_id จาก Context แบบเดียวกับ CartHandler
func getOrderUserID(r *http.Request) (int, bool) {
	userID, ok := r.Context().Value("user_id").(int)
	return userID, ok && userID > 0
}

// POST /api/orders
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := getOrderUserID(r)
	if !ok {
		orderWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	ctx := r.Context()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเริ่มคำสั่งซื้อได้",
		})
		return
	}
	defer tx.Rollback(ctx)

	// 1. ค้นหาตะกร้าของผู้ใช้และล็อกแถวไว้
	var cartID int

	err = tx.QueryRow(ctx, `
		SELECT cart_id
		FROM cart
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&cartID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			orderWriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "ไม่พบตะกร้าสินค้า",
			})
			return
		}

		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถอ่านตะกร้าได้",
		})
		return
	}

	// 2. อ่านสินค้าในตะกร้าและล็อกแถวสินค้า
	rows, err := tx.Query(ctx, `
		SELECT
			ci.product_id,
			p.product_name,
			ci.quantity,
			p.price,
			p.stock
		FROM cart_items ci
		JOIN products p ON p.product_id = ci.product_id
		WHERE ci.cart_id = $1
		ORDER BY ci.product_id
		FOR UPDATE OF p
	`, cartID)

	if err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถอ่านรายการสินค้าได้",
		})
		return
	}

	items := []OrderItemResponse{}
	var total float64

	for rows.Next() {
		var item OrderItemResponse
		var stock int

		err := rows.Scan(
			&item.ProductID,
			&item.Name,
			&item.Quantity,
			&item.Price,
			&stock,
		)

		if err != nil {
			rows.Close()
			orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "อ่านข้อมูลสินค้าไม่สำเร็จ",
			})
			return
		}

		if item.Quantity <= 0 {
			rows.Close()
			orderWriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "จำนวนสินค้าไม่ถูกต้อง",
			})
			return
		}

		if item.Quantity > stock {
			rows.Close()
			orderWriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "สินค้า " + item.Name + " มีจำนวนไม่เพียงพอ",
			})
			return
		}

		item.Subtotal = item.Price * float64(item.Quantity)
		total += item.Subtotal

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "อ่านรายการสินค้าไม่สำเร็จ",
		})
		return
	}
	rows.Close()

	if len(items) == 0 {
		orderWriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ไม่มีสินค้าในตะกร้า",
		})
		return
	}

	// 3. สร้างคำสั่งซื้อ
	var orderID int

	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, total)
		VALUES ($1, $2)
		RETURNING order_id
	`, userID, total).Scan(&orderID)

	if err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "สร้างคำสั่งซื้อไม่สำเร็จ",
		})
		return
	}

	// 4. บันทึกรายการสินค้าและหักสต็อก
	for _, item := range items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items
				(order_id, product_id, quantity, price)
			VALUES ($1, $2, $3, $4)
		`,
			orderID,
			item.ProductID,
			item.Quantity,
			item.Price,
		)

		if err != nil {
			orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "บันทึกรายการสินค้าไม่สำเร็จ",
			})
			return
		}

		result, err := tx.Exec(ctx, `
			UPDATE products
			SET stock = stock - $1
			WHERE product_id = $2
			  AND stock >= $1
		`, item.Quantity, item.ProductID)

		if err != nil || result.RowsAffected() != 1 {
			orderWriteJSON(w, http.StatusConflict, map[string]string{
				"error": "ตัดสต็อกไม่สำเร็จ กรุณาลองใหม่",
			})
			return
		}
	}

	// 5. ล้างรายการสินค้าในตะกร้า
	_, err = tx.Exec(ctx, `
		DELETE FROM cart_items
		WHERE cart_id = $1
	`, cartID)

	if err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ล้างตะกร้าไม่สำเร็จ",
		})
		return
	}

	// 6. ยืนยัน Transaction
	if err := tx.Commit(ctx); err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ยืนยันคำสั่งซื้อไม่สำเร็จ",
		})
		return
	}

	orderWriteJSON(w, http.StatusCreated, OrderResponse{
		OrderID: orderID,
		Total:   total,
		Items:   items,
	})
}

// GET /api/orders/my
func (h *OrderHandler) GetMyOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := getOrderUserID(r)
	if !ok {
		orderWriteJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT order_id, total, order_date
		FROM orders
		WHERE user_id = $1
		ORDER BY order_date DESC
	`, userID)

	if err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถโหลดประวัติคำสั่งซื้อได้",
		})
		return
	}
	defer rows.Close()

	type MyOrder struct {
		OrderID   int     `json:"order_id"`
		Total     float64 `json:"total"`
		OrderDate string  `json:"order_date"`
	}

	orders := []MyOrder{}

	for rows.Next() {
		var order MyOrder

		err := rows.Scan(
			&order.OrderID,
			&order.Total,
			&order.OrderDate,
		)

		if err != nil {
			orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "อ่านประวัติคำสั่งซื้อไม่สำเร็จ",
			})
			return
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		orderWriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "อ่านประวัติคำสั่งซื้อไม่สำเร็จ",
		})
		return
	}

	orderWriteJSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
	})
}
