package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

// AdminOnly middleware ensures the request has a valid JWT with role "admin".
func AdminOnly(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}
			tokenString := parts[1]
			// Mismo motivo que en AuthMiddleware (middleware.go): el algoritmo
			// lo fijamos nosotros, no el token. Aqui pesa mas, porque lo que
			// hay detras son las rutas de administracion.
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}
			role, ok := claims["role"].(string)
			if !ok || role != "admin" {
				http.Error(w, "Admin role required", http.StatusForbidden)
				return
			}
			adminID, _ := claims["sub"].(string)
			ctx := context.WithValue(r.Context(), UserIDKey, adminID)
			// token is valid and role is admin; proceed
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
