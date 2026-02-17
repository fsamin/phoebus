package handler

import (
	"context"
	"net/http"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/model"
)

type contextKey string

const claimsKey contextKey = "claims"

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("phoebus_session")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		claims, err := auth.ValidateToken(cookie.Value, h.cfg.JWTSecret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) RequireRole(roles ...model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
				// Admin has access to everything
				if claims.Role == model.RoleAdmin {
					next.ServeHTTP(w, r)
					return
				}
				// Instructor has access to instructor routes
				if claims.Role == model.RoleInstructor && role == model.RoleInstructor {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		})
	}
}

func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsKey).(*auth.Claims)
	return claims
}
