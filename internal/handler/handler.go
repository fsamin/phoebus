package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fsamin/phoebus/internal/assets"
	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/logging"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/fsamin/phoebus/internal/syncer"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	db           *sqlx.DB
	cfg          *config.Config
	syncer       *syncer.Syncer
	sshPublicKey string
	assetStore   assets.Store
}

func New(db *sqlx.DB, cfg *config.Config, s *syncer.Syncer, sshPublicKey string, assetStore assets.Store) *Handler {
	return &Handler{
		db:           db,
		cfg:          cfg,
		syncer:       s,
		sshPublicKey: sshPublicKey,
		assetStore:   assetStore,
	}
}

func (h *Handler) RegisterRoutes(ctx context.Context, r chi.Router) {
	// Latency & metrics tracking middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w, statusCode: 200}
			next.ServeHTTP(sr, req)
			duration := time.Since(start)
			latencyTracker.Record(duration)

			path := normalizePath(req.URL.Path)
			httpRequestsTotal.WithLabelValues(req.Method, path, fmt.Sprintf("%d", sr.statusCode)).Inc()
			httpRequestDuration.WithLabelValues(req.Method, path).Observe(duration.Seconds())
		})
	})

	// Security headers
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "0") // Modern browsers use CSP instead
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' blob:; "+ // blob: needed for Monaco Editor web workers
					"worker-src 'self' blob:; "+
					"style-src 'self' 'unsafe-inline'; "+ // Ant Design uses inline styles
					"img-src 'self' data: blob:; "+
					"media-src 'self'; "+
					"font-src 'self' data:; "+
					"connect-src 'self'; "+
					"frame-ancestors 'none'")
			next.ServeHTTP(w, req)
		})
	})

	// Public
	r.Get("/api/health", h.Health)
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/auth/ldap/login", h.LDAPLogin)
	r.Get("/api/auth/oidc/redirect", h.OIDCRedirect)
	r.Get("/api/auth/oidc/callback", h.OIDCCallback)
	r.Get("/api/auth/providers", h.AuthProviders)
	r.Post("/api/webhooks/{uuid}", h.Webhook)
	r.Get("/api/assets/{hash}", h.ServeAsset)
	r.Handle("/metrics", h.Metrics())

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(h.ProxyAuthMiddleware)
		r.Use(h.AuthMiddleware)
		r.Get("/api/me", h.Me)
		r.Get("/api/me/dashboard", h.Dashboard)
		r.Get("/api/me/onboarding", h.GetOnboarding)
		r.Patch("/api/me/onboarding", h.MarkOnboardingSeen)
		r.Delete("/api/me/onboarding", h.ResetOnboarding)
		r.Post("/api/auth/logout", h.Logout)
		r.Post("/api/auth/refresh", h.RefreshToken)

		// Learning paths (all authenticated users)
		r.Get("/api/learning-paths", h.ListLearningPaths)
		r.Get("/api/learning-paths/dependencies", h.ListPathDependencies)
		r.Get("/api/learning-paths/{pathId}", h.GetLearningPath)
		r.Get("/api/learning-paths/{pathId}/steps/{stepId}", h.GetStep)
		r.Get("/api/competencies", h.ListCompetencies)

		// Progress & exercises (all authenticated users)
		r.Get("/api/progress", h.GetProgress)
		r.Post("/api/progress", h.UpdateProgress)
		r.Post("/api/exercises/{stepId}/attempt", h.SubmitAttempt)
		r.Post("/api/exercises/{stepId}/reset", h.ResetExercise)
		r.Get("/api/exercises/{stepId}/attempts", h.GetStepAttempts)

		// Analytics (instructor/admin)
		r.Group(func(r chi.Router) {
			r.Use(h.RequireRole(model.RoleInstructor))
			r.Get("/api/analytics/overview", h.AnalyticsOverview)
			r.Get("/api/analytics/activity", h.AnalyticsActivity)
			r.Get("/api/analytics/paths/{pathId}", h.AnalyticsPath)
			r.Get("/api/analytics/paths/{pathId}/steps/{stepId}", h.AnalyticsStep)
			r.Get("/api/analytics/learners", h.AnalyticsLearners)
			r.Get("/api/analytics/learners/{learnerId}", h.AnalyticsLearner)

			// Instructor repos (ownership-verified)
			r.Get("/api/instructor/repos", h.InstructorListRepos)
			r.Route("/api/instructor/repos/{repoId}", func(r chi.Router) {
				r.Use(h.instructorOwnerMiddleware)
				r.Get("/", h.InstructorGetRepo)
				r.Post("/sync", h.InstructorSyncRepo)
				r.Get("/sync-logs", h.InstructorSyncLogs)
				r.Get("/sync-logs/{jobId}", h.InstructorSyncJobLogs)
			})
		})

		// Admin only
		r.Group(func(r chi.Router) {
			r.Use(h.RequireRole(model.RoleAdmin))
			r.Get("/api/admin/users", h.ListUsers)
			r.Post("/api/admin/users", h.CreateUser)
			r.Patch("/api/admin/users/{userId}", h.UpdateUser)
			r.Get("/api/admin/instructor-users", h.ListInstructorUsers)
			r.Get("/api/admin/repos", h.ListRepos)
			r.Get("/api/admin/repos/{repoId}", h.GetRepo)
			r.Post("/api/admin/repos", h.CreateRepo)
			r.Put("/api/admin/repos/{repoId}", h.UpdateRepo)
			r.Delete("/api/admin/repos/{repoId}", h.DeleteRepo)
			r.Post("/api/admin/repos/{repoId}/sync", h.SyncRepo)
			r.Get("/api/admin/repos/{repoId}/sync-logs", h.SyncLogs)
			r.Get("/api/admin/repos/{repoId}/sync-logs/{jobId}", h.SyncJobLogs)
			r.Get("/api/admin/repos/{repoId}/paths", h.ListRepoPaths)
			r.Patch("/api/admin/repos/{repoId}/paths/{pathId}", h.ToggleRepoPath)
			r.Get("/api/admin/health", h.AdminHealth)
			r.Get("/api/admin/ssh-public-key", h.SSHPublicKey)
			r.Get("/api/admin/banner", h.GetBanner)
			r.Put("/api/admin/banner", h.UpdateBanner)
			r.Get("/api/admin/dependencies", h.ListManualDependencies)
			r.Post("/api/admin/dependencies", h.CreatePathDependency)
			r.Delete("/api/admin/dependencies/{depId}", h.DeletePathDependency)
		})
	})

	// Start background gauge updater
	go h.updateGauges(ctx)
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
	h.execBestEffort(r.Context(), "failed to update last login", "UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)

	// Enforce forced admin role on every login
	if h.cfg.IsForcedAdmin(user.Username) && user.Role != model.RoleAdmin {
		h.execBestEffort(r.Context(), "failed to enforce forced admin role", "UPDATE users SET role = 'admin', updated_at = now() WHERE id = $1", user.ID)
		user.Role = model.RoleAdmin
	}

	setSessionCookie(w, r, token)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
		},
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.LocalEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "local auth is disabled"})
		return
	}

	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate
	if len(req.Username) < 4 || len(req.Username) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 4-32 characters"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
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
	role := "learner"
	if h.cfg.IsForcedAdmin(req.Username) {
		role = "admin"
	}
	err = h.db.QueryRowxContext(r.Context(), `
		INSERT INTO users (id, username, display_name, email, password_hash, role, auth_provider, active)
		VALUES (gen_random_uuid(), $1, $2, NULLIF($3, ''), $4, $5, 'local', true)
		RETURNING id, username, display_name, role, auth_provider, active, created_at, updated_at
	`, req.Username, req.DisplayName, req.Email, hash, role).StructScan(&user)
	if err != nil {
		if isDuplicateKey(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already taken"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg.JWT.Secret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	setSessionCookie(w, r, token)

	h.auditLog(r.Context(), &auth.Claims{UserID: user.ID.String(), Username: user.Username, Role: user.Role}, "register", "user", user.ID.String(), map[string]any{"username": user.Username})

	writeJSON(w, http.StatusCreated, map[string]any{
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
		SELECT id, username, email, display_name, role, auth_provider, active, onboarding_tours_seen, created_at, updated_at
		FROM users WHERE id = $1
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := 1
	perPage := 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}
	offset := (page - 1) * perPage

	// Search is done server-side so it covers every user, not only the current
	// page, and so that total matches the filtered set. LIKE wildcards typed by
	// the admin are matched literally.
	pattern := likePattern(r.URL.Query().Get("q"))
	const searchFilter = `$1 = '' OR username ILIKE $1 OR display_name ILIKE $1 OR COALESCE(email, '') ILIKE $1`

	var total int
	if err := h.db.GetContext(r.Context(), &total, "SELECT COUNT(*) FROM users WHERE "+searchFilter, pattern); err != nil {
		writeDBError(w, r, "failed to count users", err)
		return
	}

	// Completed paths come from the learner KPIs so that both pages agree, and
	// sorting on them is done in SQL to span every page, not only the current one.
	orderBy := "u.created_at DESC"
	if r.URL.Query().Get("sort") == "completed_paths" {
		orderBy = "ls.completed_paths DESC"
		if r.URL.Query().Get("order") == "asc" {
			orderBy = "ls.completed_paths ASC"
		}
		orderBy += ", u.created_at DESC"
	}

	type userResponse struct {
		model.User
		CompletedPaths int  `json:"completed_paths" db:"completed_paths"`
		RoleLocked     bool `json:"role_locked" db:"-"`
	}
	// id breaks ties between users created at the same instant, otherwise rows
	// can be repeated or skipped across pages.
	var out []userResponse
	err := h.db.SelectContext(r.Context(), &out, learnerStatsCTE+`
		SELECT u.id, u.username, u.email, u.display_name, u.role, u.auth_provider, u.active,
		       u.last_login_at, u.created_at, u.updated_at, ls.completed_paths
		FROM users u
		JOIN learner_stats ls ON ls.id = u.id
		WHERE $2 = '' OR u.username ILIKE $2 OR u.display_name ILIKE $2 OR COALESCE(u.email, '') ILIKE $2
		ORDER BY `+orderBy+`, u.id
		LIMIT $3 OFFSET $4
	`, learnerStuckDays, pattern, perPage, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}
	if out == nil {
		out = []userResponse{}
	}
	for i := range out {
		out[i].RoleLocked = h.cfg.IsForcedAdmin(out[i].Username)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":    out,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// likePattern turns a user-typed search into an ILIKE "contains" pattern where
// LIKE wildcards are literal. An empty search yields an empty pattern.
func likePattern(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q) + "%"
}

// execBestEffort runs a non-critical write: a failure is logged but does not
// fail the request (e.g. refreshing last_login_at after a successful login).
func (h *Handler) execBestEffort(ctx context.Context, what, query string, args ...any) {
	if _, err := h.db.ExecContext(ctx, query, args...); err != nil {
		logging.FromContext(ctx).Warn(what, "error", err.Error())
	}
}

// writeDBError logs a failed query and reports it to the client as a 500.
// Callers must return immediately after calling it.
func writeDBError(w http.ResponseWriter, r *http.Request, what string, err error) {
	logging.FromContext(r.Context()).Error(what, "error", err.Error())
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": what})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
