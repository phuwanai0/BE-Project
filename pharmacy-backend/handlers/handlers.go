package handlers

import (
	"encoding/json"
	"net/http"
)

// GetProducts: API แสดงรายการสินค้า
func GetProducts(w http.ResponseWriter, r *http.Request) {

	products := []map[string]interface{}{
		{
			"id":           1,
			"product_name": "พาราเซตามอล",
			"price":        25,
			"stock":        100,
		},
		{
			"id":           2,
			"product_name": "พลาสเตอร์ยา",
			"price":        35,
			"stock":        50,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// HealthCheck: ตรวจสอบสถานะ Backend
func HealthCheck(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Pharmacy API is running",
	})
}
