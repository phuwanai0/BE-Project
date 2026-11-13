package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

func authError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// RequireAuth ตรวจสอบ JWT ก่อนเข้าใช้งาน API
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")

		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			authError(w, http.StatusUnauthorized, "กรุณาเข้าสู่ระบบ")
			return
		}

		tokenString := authHeader[7:]
		secret := os.Getenv("JWT_SECRET")

		if secret == "" {
			authError(w, http.StatusInternalServerError, "Server configuration error")
			return
		}

		token, err := jwt.Parse(
			tokenString,
			func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
		)

		if err != nil || token == nil || !token.Valid {
			authError(w, http.StatusUnauthorized, "Token ไม่ถูกต้องหรือหมดอายุ")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			authError(w, http.StatusUnauthorized, "ข้อมูล Token ไม่ถูกต้อง")
			return
		}

		rawID, exists := claims["user_id"]
		if !exists {
			authError(w, http.StatusUnauthorized, "ไม่พบ user_id ใน Token")
			return
		}

		var userID int

		switch value := rawID.(type) {
		case float64:
			userID = int(value)
		case string:
			userID, err = strconv.Atoi(value)
			if err != nil {
				userID = 0
			}
		}

		if userID <= 0 {
			authError(w, http.StatusUnauthorized, "user_id ไม่ถูกต้อง")
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
