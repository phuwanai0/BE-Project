
package middleware

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

// RequireAuth ตรวจสอบ JWT ก่อนเข้าใช้งาน API
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")

		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			http.Error(
				w,
				`{"error":"กรุณาเข้าสู่ระบบ"}`,
				http.StatusUnauthorized,
			)
			return
		}

		tokenString := authHeader[7:]
		secret := os.Getenv("JWT_SECRET")

		if secret == "" {
			http.Error(
				w,
				`{"error":"Server configuration error"}`,
				http.StatusInternalServerError,
			)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}

			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			http.Error(
				w,
				`{"error":"Token ไม่ถูกต้องหรือหมดอายุ"}`,
				http.StatusUnauthorized,
			)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(
				w,
				`{"error":"ข้อมูล Token ไม่ถูกต้อง"}`,
				http.StatusUnauthorized,
			)
			return
		}

		rawID, exists := claims["user_id"]
		if !exists {
			http.Error(
				w,
				`{"error":"ไม่พบ user_id ใน Token"}`,
				http.StatusUnauthorized,
			)
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
			http.Error(
				w,
				`{"error":"user_id ไม่ถูกต้อง"}`,
				http.StatusUnauthorized,
			)
			return
		}

		ctx := contextWithUserID(r.Context(), userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ส่ง user_id เข้า Context
func contextWithUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}
