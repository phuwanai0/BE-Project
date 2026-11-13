package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
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

type ChangePasswordRequest struct {
	Password string `json:"password"`
}

// GET /api/admin/users
func (h *AdminHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.DB.Query(ctx, `
		SELECT user_id, username, email, role
		FROM public.users
		ORDER BY user_id DESC
	`)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "ไม่สามารถดึงข้อมูลผู้ใช้ได้"})
		return
	}
	defer rows.Close()

	users := []AdminUser{}

	for rows.Next() {
		var user AdminUser

		if err := rows.Scan(
			&user.UserID,
			&user.Username,
			&user.Email,
			&user.Role,
		); err != nil {
			respondJSON(w, http.StatusInternalServerError,
				map[string]string{"error": "อ่านข้อมูลผู้ใช้ไม่สำเร็จ"})
			return
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "เกิดข้อผิดพลาดในการอ่านข้อมูล"})
		return
	}

	respondJSON(w, http.StatusOK, users)
}

// PUT /api/admin/users/{id}/password
// Admin can change password for both Admin and Member
func (h *AdminHandler) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest,
			map[string]string{"error": "รหัสผู้ใช้ไม่ถูกต้อง"})
		return
	}

	var req ChangePasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest,
			map[string]string{"error": "ข้อมูลไม่ถูกต้อง"})
		return
	}

	if len(req.Password) < 8 {
		respondJSON(w, http.StatusBadRequest,
			map[string]string{"error": "รหัสผ่านต้องมีอย่างน้อย 8 ตัวอักษร"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "เข้ารหัสรหัสผ่านไม่สำเร็จ"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := h.DB.Exec(ctx, `
		UPDATE public.users
		SET password = $1
		WHERE user_id = $2
		  AND role IN ('admin', 'member')
	`, string(hashedPassword), id)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "เปลี่ยนรหัสผ่านไม่สำเร็จ"})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound,
			map[string]string{"error": "ไม่พบบัญชีผู้ใช้"})
		return
	}

	respondJSON(w, http.StatusOK,
		map[string]string{"message": "เปลี่ยนรหัสผ่านสำเร็จ"})
}

// DELETE /api/admin/users/{id}
// Admin can delete both Admin and Member
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest,
			map[string]string{"error": "รหัสผู้ใช้ไม่ถูกต้อง"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get current admin ID from JWT context
	currentUserID, ok := r.Context().Value("user_id").(int)
	if !ok {
		respondJSON(w, http.StatusUnauthorized,
			map[string]string{"error": "ไม่พบข้อมูลผู้ใช้ที่เข้าสู่ระบบ"})
		return
	}

	// Prevent deleting own account
	if id == currentUserID {
		respondJSON(w, http.StatusForbidden,
			map[string]string{"error": "ไม่สามารถลบบัญชีของตัวเองได้"})
		return
	}

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "เริ่มต้นการลบไม่สำเร็จ"})
		return
	}
	defer tx.Rollback(ctx)

	// Check target user and lock row
	var role string
	err = tx.QueryRow(ctx, `
		SELECT role
		FROM public.users
		WHERE user_id = $1
		FOR UPDATE
	`, id).Scan(&role)

	if err != nil {
		respondJSON(w, http.StatusNotFound,
			map[string]string{"error": "ไม่พบบัญชีผู้ใช้"})
		return
	}

	if role != "admin" && role != "member" {
		respondJSON(w, http.StatusBadRequest,
			map[string]string{"error": "ประเภทบัญชีไม่ถูกต้อง"})
		return
	}

	// Prevent deleting the last admin
	if role == "admin" {
		var adminCount int

		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM public.users
			WHERE role = 'admin'
		`).Scan(&adminCount)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError,
				map[string]string{"error": "ตรวจสอบบัญชี Admin ไม่สำเร็จ"})
			return
		}

		if adminCount <= 1 {
			respondJSON(w, http.StatusForbidden,
				map[string]string{"error": "ไม่สามารถลบ Admin คนสุดท้ายได้"})
			return
		}
	}

	// Delete user
	_, err = tx.Exec(ctx, `
		DELETE FROM public.users
		WHERE user_id = $1
	`, id)

	if err != nil {
		respondJSON(w, http.StatusConflict,
			map[string]string{
				"error": "ลบบัญชีไม่ได้ อาจมีประวัติคำสั่งซื้อที่อ้างอิงบัญชีนี้อยู่",
			})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		respondJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "บันทึกการลบไม่สำเร็จ"})
		return
	}

	respondJSON(w, http.StatusOK,
		map[string]string{"message": "ลบบัญชีผู้ใช้สำเร็จ"})
}
