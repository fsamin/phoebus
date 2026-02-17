package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	db  *sql.DB
	cfg *config.Config
}

func New(db *sql.DB, cfg *config.Config) *Handler {
	return &Handler{db: db, cfg: cfg}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public
	r.Get("/api/health", h.Health)
	r.Post("/api/auth/login", h.Login)

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(h.AuthMiddleware)
		r.Get("/api/me", h.Me)

		// Admin only
		r.Group(func(r chi.Router) {
			r.Use(h.RequireRole(model.RoleAdmin))
			r.Get("/api/admin/users", h.ListUsers)
		})
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.LocalAuth {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "local auth is disabled"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var user model.User
	var passwordHash sql.NullString
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, username, display_name, role, password_hash, active
		FROM users WHERE username = $1 AND auth_provider = 'local'
	`, req.Username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &passwordHash, &user.Active)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !user.Active {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !passwordHash.Valid || !auth.CheckPassword(passwordHash.String, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg.JWTSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Update last login
	h.db.ExecContext(r.Context(), "UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60, // 8 hours
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

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var user model.User
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, username, COALESCE(email, ''), display_name, role, auth_provider, active, created_at, updated_at
		FROM users WHERE id = $1
	`, claims.UserID).Scan(&user.ID, &user.Username, new(string), &user.DisplayName, &user.Role, &user.AuthProvider, &user.Active, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, username, COALESCE(email, ''), display_name, role, auth_provider, active, last_login_at, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, new(string), &u.DisplayName, &u.Role, &u.AuthProvider, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
