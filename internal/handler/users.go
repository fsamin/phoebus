package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
)

// UpdateUser allows admins to change a user's role or active status.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

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

	writeJSON(w, http.StatusOK, user)
}
