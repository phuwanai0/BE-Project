
package models

type Product struct {
    ID           int     `json:"id"`
    ProductName  string  `json:"product_name"`
    Description  string  `json:"description"`
    Price        float64 `json:"price"`
    Stock        int     `json:"stock"`
    CategoryID   int     `json:"category_id"`
    CategoryName string  `json:"category_name"`
}
