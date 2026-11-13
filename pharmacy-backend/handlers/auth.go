package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"pharmacy-backend/authcontext"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB *pgxpool.Pool
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ใช้ชื่อ MemberChangePasswordRequest เพื่อไม่ให้ซ้ำกับ admin.go
type MemberChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UserClaims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ข้อมูลไม่ถูกต้อง",
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "กรุณากรอกข้อมูลให้ครบ",
		})
		return
	}

	if len(req.Password) < 8 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "รหัสผ่านต้องมีอย่างน้อย 8 ตัวอักษร",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเข้ารหัสรหัสผ่านได้",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var userID int

	err = h.DB.QueryRow(ctx, `
		INSERT INTO users (username, email, password)
		VALUES ($1, $2, $3)
		RETURNING user_id
	`, req.Username, req.Email, string(hashedPassword)).Scan(&userID)

	if err != nil {
		respondJSON(w, http.StatusConflict, map[string]string{
			"error": "Username หรือ Email นี้ถูกใช้งานแล้ว",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"message": "สมัครสมาชิกสำเร็จ",
		"user_id": userID,
	})
}

// Login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ข้อมูลไม่ถูกต้อง",
		})
		return
	}

	var userID int
	var username, hashedPassword, role string

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.DB.QueryRow(ctx, `
		SELECT user_id, username, password, role
		FROM users
		WHERE email = $1
	`, strings.TrimSpace(req.Email)).Scan(
		&userID,
		&username,
		&hashedPassword,
		&role,
	)

	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Email หรือ Password ไม่ถูกต้อง",
		})
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(req.Password),
	)

	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Email หรือ Password ไม่ถูกต้อง",
		})
		return
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ยังไม่ได้ตั้งค่า JWT_SECRET",
		})
		return
	}

	claims := UserClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถสร้าง Token ได้",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":  "เข้าสู่ระบบสำเร็จ",
		"token":    tokenString,
		"user_id":  userID,
		"username": username,
		"role":     role,
	})
}

// ChangePassword: สมาชิกเปลี่ยนรหัสผ่านของบัญชีตัวเอง
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(authcontext.UserIDKey).(int)

	if !ok || userID <= 0 {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "กรุณาเข้าสู่ระบบ",
		})
		return
	}

	var req MemberChangePasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ข้อมูลไม่ถูกต้อง",
		})
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "กรุณากรอกรหัสผ่านให้ครบ",
		})
		return
	}

	if len(req.NewPassword) < 8 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "รหัสผ่านใหม่ต้องมีอย่างน้อย 8 ตัวอักษร",
		})
		return
	}

	if req.CurrentPassword == req.NewPassword {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "รหัสผ่านใหม่ต้องไม่ซ้ำกับรหัสผ่านเดิม",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var hashedPassword string

	err := h.DB.QueryRow(ctx, `
		SELECT password
		FROM users
		WHERE user_id = $1
	`, userID).Scan(&hashedPassword)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถตรวจสอบบัญชีได้",
		})
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(req.CurrentPassword),
	)

	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "รหัสผ่านเดิมไม่ถูกต้อง",
		})
		return
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.NewPassword),
		bcrypt.DefaultCost,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเข้ารหัสรหัสผ่านใหม่ได้",
		})
		return
	}

	result, err := h.DB.Exec(ctx, `
		UPDATE users
		SET password = $1
		WHERE user_id = $2
	`, string(newHashedPassword), userID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ไม่สามารถเปลี่ยนรหัสผ่านได้",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "ไม่พบบัญชีสมาชิก",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "เปลี่ยนรหัสผ่านสำเร็จ",
	})
}
