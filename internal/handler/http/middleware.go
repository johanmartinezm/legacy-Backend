package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

type contextKey string

const UserIDKey contextKey = "userID"

// UserRoleKey lleva el claim "role" del token. AdminOnly ya lo comprobaba por su
// cuenta, pero las rutas que estan bajo AuthMiddleware y aceptan campos
// reservados a administradores necesitan distinguir tambien: sin esto, un
// handler no tiene forma de saber quien le esta llamando.
const UserRoleKey contextKey = "userRole"

// RoleAdmin es el valor que AdminLogin pone en el claim "role"
// (auth_service.go:301). Los usuarios normales llevan su tipo de cuenta:
// familia, empresa o profesional.
const RoleAdmin = "admin"

// IsAdmin indica si quien hace la peticion tiene rol de administrador.
func IsAdmin(ctx context.Context) bool {
	role, _ := ctx.Value(UserRoleKey).(string)
	return role == RoleAdmin
}

func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString := ""

			if authHeader != "" {
				bearerToken := strings.Split(authHeader, " ")
				if len(bearerToken) == 2 && strings.ToLower(bearerToken[0]) == "bearer" {
					tokenString = bearerToken[1]
				}
			}

			if tokenString == "" {
				tokenString = r.URL.Query().Get("token")
			}
			tokenString = strings.TrimSpace(tokenString)

			if tokenString == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}
			// WithValidMethods fija el algoritmo antes de mirar la firma. Sin
			// esto, quien decide que algoritmo se usa es el propio token, que
			// es el clasico "alg: none" / confusion de algoritmo. Hoy no era
			// explotable —la keyfunc devuelve []byte, y con eso la libreria
			// rechaza tanto RS256 como none—, pero eso es un detalle de
			// implementacion de la dependencia, no una decision nuestra.
			// El validador de Apple ya lo hacia asi (apple/validator.go).
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

			userID, ok := claims["sub"].(string)
			if !ok {
				http.Error(w, "User ID not found in token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			// El rol viaja en el token desde el login; los handlers que aceptan
			// campos reservados a administradores lo consultan con IsAdmin.
			if role, ok := claims["role"].(string); ok {
				ctx = context.WithValue(ctx, UserRoleKey, role)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString := ""

			if authHeader != "" {
				bearerToken := strings.Split(authHeader, " ")
				if len(bearerToken) == 2 && strings.ToLower(bearerToken[0]) == "bearer" {
					tokenString = bearerToken[1]
				}
			}

			if tokenString == "" {
				tokenString = r.URL.Query().Get("token")
			}
			tokenString = strings.TrimSpace(tokenString)

			if tokenString == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Mismo motivo que en AuthMiddleware: el algoritmo lo fijamos
			// nosotros, no el token.
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			}, jwt.WithValidMethods([]string{"HS256"}))

			if err != nil || !token.Valid {
				next.ServeHTTP(w, r) // Invalid token, still proceed as anonymous
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			userID, ok := claims["sub"].(string)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			// El rol también, igual que en AuthMiddleware. Sin esto IsAdmin
			// devuelve falso en las rutas opcionales aunque quien pregunte sea
			// administrador con un token válido, y el panel —que consume el
			// listado público de eventos— perdería el enlace de acceso al
			// editarlos.
			if role, ok := claims["role"].(string); ok {
				ctx = context.WithValue(ctx, UserRoleKey, role)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
