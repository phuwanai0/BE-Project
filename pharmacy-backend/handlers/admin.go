
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	DB *pgxpool.Pool
}

type AdminUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

func (h *AdminHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT user_id, username, email, role
		FROM public.users
		ORDER BY user_id DESC
	`)
	if err != nil {
		http.Error(w, "ไม่สามารถดึงข้อมูลสมาชิกได้", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []AdminUser{}

	for rows.Next() {
		var user AdminUser

		err := rows.Scan(
			&user.UserID,
			&user.Username,
			&user.Email,
			&user.Role,
		)
		if err != nil {
			http.Error(w, "ไม่สามารถอ่านข้อมูลสมาชิกได้", http.StatusInternalServerError)
			return
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "เกิดข้อผิดพลาดในการอ่านข้อมูลสมาชิก", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
