package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func isDuplicateKey(err error) bool {
	return strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint")
}

// CreateUser allows admins to create a local user.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.LocalEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "local auth is disabled"})
		return
	}

	var req struct {
		Username    string     `json:"username"`
		DisplayName string     `json:"display_name"`
		Email       string     `json:"email"`
		Role        model.Role `json:"role"`
		Password    string     `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.Username) < 4 || len(req.Username) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 4-32 characters"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}
	switch req.Role {
	case model.RoleLearner, model.RoleInstructor, model.RoleAdmin:
	default:
		req.Role = model.RoleLearner
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	var user model.User
	err = h.db.QueryRowxContext(r.Context(), `
		INSERT INTO users (id, username, display_name, email, password_hash, role, auth_provider, active)
		VALUES (gen_random_uuid(), $1, $2, NULLIF($3, ''), $4, $5, 'local', true)
		RETURNING id, username, display_name, email, role, auth_provider, active, created_at, updated_at
	`, req.Username, req.DisplayName, req.Email, hash, req.Role).StructScan(&user)
	if err != nil {
		if isDuplicateKey(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already taken"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	claims := ClaimsFromContext(r.Context())
	h.auditLog(r.Context(), claims, "create", "user", user.ID.String(), map[string]any{"username": user.Username, "role": user.Role})

	writeJSON(w, http.StatusCreated, user)
}

// UpdateUser allows admins to change a user's role or active status.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if _, err := uuid.Parse(userID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var req struct {
		Role   *model.Role `json:"role,omitempty"`
		Active *bool       `json:"active,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Role == nil && req.Active == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nothing to update"})
		return
	}

	// Validate role
	if req.Role != nil {
		switch *req.Role {
		case model.RoleLearner, model.RoleInstructor, model.RoleAdmin:
			// valid
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role (must be learner, instructor, or admin)"})
			return
		}
	}

	// Prevent self-deactivation
	claims := ClaimsFromContext(r.Context())
	if claims != nil && claims.UserID == userID {
		if req.Active != nil && !*req.Active {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot deactivate yourself"})
			return
		}
	}

	// Prevent role change for forced admins
	if req.Role != nil {
		var username string
		if err := h.db.GetContext(r.Context(), &username, "SELECT username FROM users WHERE id = $1", userID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		if h.cfg.IsForcedAdmin(username) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "this user's role is managed by configuration and cannot be changed"})
			return
		}
	}

	// Build update query
	if req.Role != nil && req.Active != nil {
		_, err := h.db.ExecContext(r.Context(),
			"UPDATE users SET role = $1, active = $2, updated_at = now() WHERE id = $3",
			*req.Role, *req.Active, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
			return
		}
	} else if req.Role != nil {
		_, err := h.db.ExecContext(r.Context(),
			"UPDATE users SET role = $1, updated_at = now() WHERE id = $2",
			*req.Role, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
			return
		}
	} else {
		_, err := h.db.ExecContext(r.Context(),
			"UPDATE users SET active = $1, updated_at = now() WHERE id = $2",
			*req.Active, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user"})
			return
		}
	}

	// Fetch updated user
	var user model.User
	err := h.db.GetContext(r.Context(), &user, `
		SELECT id, username, email, display_name, role, auth_provider, active, last_login_at, created_at, updated_at
		FROM users WHERE id = $1
	`, userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	h.auditLog(r.Context(), claims, "update", "user", userID, map[string]any{"role": user.Role, "active": user.Active})

	writeJSON(w, http.StatusOK, user)
}
