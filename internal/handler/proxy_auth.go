package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/google/uuid"
)

// ProxyAuthMiddleware handles transparent authentication via reverse proxy headers.
// If the configured user header is present and no valid JWT cookie exists, it upserts
// the user and sets a JWT session cookie. If a valid cookie already exists, it skips.
func (h *Handler) ProxyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.cfg.Auth.ProxyAuth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		cfg := h.cfg.Auth.ProxyAuth
		remoteUser := r.Header.Get(cfg.HeaderUser)
		if remoteUser == "" {
			next.ServeHTTP(w, r)
			return
		}

		// If there's already a valid JWT cookie, inject claims and continue
		if cookie, err := r.Cookie("phoebus_session"); err == nil {
			if claims, err := auth.ValidateToken(cookie.Value, h.cfg.JWT.Secret); err == nil {
				ctx := context.WithValue(r.Context(), claimsKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Resolve role from groups header
		role := model.Role(cfg.DefaultRole)
		if cfg.HeaderGroups != "" {
			if groupsHeader := r.Header.Get(cfg.HeaderGroups); groupsHeader != "" {
				role = h.resolveProxyRole(groupsHeader, cfg.GroupToRole, role)
			}
		}

		// Override role for forced admins
		if h.cfg.IsForcedAdmin(remoteUser) {
			role = model.RoleAdmin
		}

		// Extract optional headers
		email := ""
		if cfg.HeaderEmail != "" {
			email = r.Header.Get(cfg.HeaderEmail)
		}
		displayName := remoteUser
		if cfg.HeaderDisplayName != "" {
			if dn := r.Header.Get(cfg.HeaderDisplayName); dn != "" {
				displayName = dn
			}
		}

		// Upsert user
		user, err := h.upsertProxyUser(r, remoteUser, email, displayName, role)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy auth: failed to provision user"})
			return
		}

		if !user.Active {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "user account is inactive"})
			return
		}

		// Generate JWT and set cookie
		token, err := auth.GenerateToken(user, h.cfg.JWT.Secret)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy auth: failed to generate token"})
			return
		}

		h.db.ExecContext(r.Context(), "UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)

		http.SetCookie(w, &http.Cookie{
			Name:     "phoebus_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   8 * 60 * 60,
		})

		// Inject claims into context
		claims := &auth.Claims{
			UserID:   user.ID.String(),
			Username: user.Username,
			Role:     user.Role,
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveProxyRole parses comma-separated groups and returns the highest-privilege matching role.
func (h *Handler) resolveProxyRole(groupsHeader string, groupToRole map[string]string, defaultRole model.Role) model.Role {
	if len(groupToRole) == 0 {
		return defaultRole
	}

	bestRole := defaultRole
	groups := strings.Split(groupsHeader, ",")
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if mappedRole, ok := groupToRole[g]; ok {
			r := model.Role(mappedRole)
			if r == model.RoleAdmin {
				return model.RoleAdmin
			}
			if r == model.RoleInstructor && bestRole == model.RoleLearner {
				bestRole = model.RoleInstructor
			}
		}
	}
	return bestRole
}

// upsertProxyUser creates or updates a user provisioned via reverse proxy headers.
func (h *Handler) upsertProxyUser(r *http.Request, username, email, displayName string, role model.Role) (*model.User, error) {
	var user model.User

	err := h.db.GetContext(r.Context(), &user, `
		SELECT id, username, email, display_name, role, password_hash, external_id, auth_provider, active, last_login_at, created_at, updated_at
		FROM users WHERE username = $1 AND auth_provider = 'proxy'
	`, username)
	if err == nil {
		// Update on every login (sync attributes + role from groups)
		h.db.ExecContext(r.Context(), `
			UPDATE users SET display_name = $1, email = NULLIF($2, ''), role = $3, updated_at = now() WHERE id = $4
		`, displayName, email, role, user.ID)
		user.DisplayName = displayName
		if email != "" {
			user.Email = &email
		} else {
			user.Email = nil
		}
		user.Role = role
		return &user, nil
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	// Create new user
	newUser := &model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        emailPtr,
		DisplayName:  displayName,
		Role:         role,
		AuthProvider: "proxy",
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO users (id, username, email, display_name, role, auth_provider, active, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9)
	`, newUser.ID, newUser.Username, email, newUser.DisplayName, newUser.Role, newUser.AuthProvider, newUser.Active, newUser.CreatedAt, newUser.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert proxy user: %w", err)
	}

	return newUser, nil
}
