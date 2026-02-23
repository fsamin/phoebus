package handler

import (
	"net/http"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/model"
)

// RefreshToken issues a new JWT if the current one is still valid.
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Fetch current user to ensure still active and get latest role
	var user model.User
	err := h.db.GetContext(r.Context(), &user, `
		SELECT id, username, display_name, role, active
		FROM users WHERE id = $1
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	if !user.Active {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "user account is inactive"})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg.JWT.Secret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
		},
	})
}

// AuthProviders returns which auth providers are enabled (for the login page).
func (h *Handler) AuthProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"local": h.cfg.Auth.LocalEnabled,
		"oidc":  h.cfg.Auth.OIDC.Enabled,
		"ldap":  h.cfg.Auth.LDAP.Enabled,
		"proxy": h.cfg.Auth.ProxyAuth.Enabled,
	})
}
