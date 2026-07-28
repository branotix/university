package routes

import (
	"context"
	"net/http"
	"strings"

	"varsity-network/internal/config"
	"varsity-network/pkg/utils"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user_claims"

// AuthMiddleware JWT টোকেন ভ্যালিডেট করে
func AuthMiddleware(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			trimmed := strings.TrimPrefix(authHeader, "Bearer ")
			if trimmed == authHeader {
				http.Error(w, "Invalid token format (Bearer token required)", http.StatusUnauthorized)
				return
			}
			tokenString = trimmed
		} else if cookie, err := r.Cookie("vn_token"); err == nil {
			// Fall back to the persistent login cookie set at login/register time.
			tokenString = cookie.Value
		}

		if tokenString == "" {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		claims := &utils.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Request Context-এ Claims স্টোর করা
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// RequireRole wraps an already-authenticated handler (must be used after AuthMiddleware)
// and only allows the request through if the JWT claims contain one of the allowed roles.
func RequireRole(allowedRoles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(*utils.Claims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			for _, role := range allowedRoles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
		}
	}
}
