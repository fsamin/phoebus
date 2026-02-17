package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/fsamin/phoebus/internal/syncer"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	db     *sqlx.DB
	cfg    *config.Config
	syncer *syncer.Syncer
}

func New(db *sqlx.DB, cfg *config.Config, s *syncer.Syncer) *Handler {
	return &Handler{
		db:     db,
		cfg:    cfg,
		syncer: s,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public
	r.Get("/api/health", h.Health)
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/webhooks/{uuid}", h.Webhook)

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(h.AuthMiddleware)
		r.Get("/api/me", h.Me)
		r.Post("/api/auth/logout", h.Logout)

		// Learning paths (all authenticated users)
		r.Get("/api/learning-paths", h.ListLearningPaths)
		r.Get("/api/learning-paths/{pathId}", h.GetLearningPath)
		r.Get("/api/learning-paths/{pathId}/steps/{stepId}", h.GetStep)

		// Progress & exercises (all authenticated users)
		r.Get("/api/progress", h.GetProgress)
		r.Post("/api/progress", h.UpdateProgress)
		r.Post("/api/exercises/{stepId}/attempt", h.SubmitAttempt)
		r.Post("/api/exercises/{stepId}/reset", h.ResetExercise)
		r.Get("/api/exercises/{stepId}/attempts", h.GetStepAttempts)

		// Admin only
		r.Group(func(r chi.Router) {
			r.Use(h.RequireRole(model.RoleAdmin))
			r.Get("/api/admin/users", h.ListUsers)
			r.Get("/api/admin/repos", h.ListRepos)
			r.Get("/api/admin/repos/{repoId}", h.GetRepo)
			r.Post("/api/admin/repos", h.CreateRepo)
			r.Put("/api/admin/repos/{repoId}", h.UpdateRepo)
			r.Delete("/api/admin/repos/{repoId}", h.DeleteRepo)
			r.Post("/api/admin/repos/{repoId}/sync", h.SyncRepo)
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
	if !h.cfg.Auth.LocalEnabled {
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
	err := h.db.QueryRowxContext(r.Context(), `
		SELECT id, username, display_name, role, password_hash, active
		FROM users WHERE username = $1 AND auth_provider = 'local'
	`, req.Username).StructScan(&user)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !user.Active {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if user.PasswordHash == nil || !auth.CheckPassword(*user.PasswordHash, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg.JWT.Secret)
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
	err := h.db.GetContext(r.Context(), &user, `
		SELECT id, username, email, display_name, role, auth_provider, active, created_at, updated_at
		FROM users WHERE id = $1
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []model.User
	err := h.db.SelectContext(r.Context(), &users, `
		SELECT id, username, email, display_name, role, auth_provider, active, last_login_at, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
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
