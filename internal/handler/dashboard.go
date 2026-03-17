package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/fsamin/phoebus/internal/model"
)

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var serverStartTime = time.Now()

// Dashboard returns aggregated data for the learner dashboard.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())

	// Continue learning: most recent in_progress step
	var continueStep *struct {
		StepID    string `json:"step_id" db:"step_id"`
		StepSlug  string `json:"step_slug" db:"step_slug"`
		StepTitle string `json:"step_title" db:"step_title"`
		PathID    string `json:"path_id" db:"path_id"`
		PathSlug  string `json:"path_slug" db:"path_slug"`
		PathTitle string `json:"path_title" db:"path_title"`
	}
	row := struct {
		StepID    string `json:"step_id" db:"step_id"`
		StepSlug  string `json:"step_slug" db:"step_slug"`
		StepTitle string `json:"step_title" db:"step_title"`
		PathID    string `json:"path_id" db:"path_id"`
		PathSlug  string `json:"path_slug" db:"path_slug"`
		PathTitle string `json:"path_title" db:"path_title"`
	}{}
	err := h.db.GetContext(r.Context(), &row, `
		SELECT s.id AS step_id, s.slug AS step_slug, s.title AS step_title,
		       lp.id AS path_id, lp.slug AS path_slug, lp.title AS path_title
		FROM progress p
		JOIN steps s ON s.id = p.step_id
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE p.user_id = $1 AND p.status = 'in_progress' AND s.deleted_at IS NULL
		ORDER BY p.updated_at DESC LIMIT 1
	`, claims.UserID)
	if err == nil {
		continueStep = &row
	}

	// Enrolled paths with progress
	type pathProgress struct {
		PathID    string `json:"path_id" db:"path_id"`
		PathSlug  string `json:"path_slug" db:"path_slug"`
		PathTitle string `json:"path_title" db:"path_title"`
		PathIcon  string `json:"path_icon" db:"path_icon"`
		Total     int    `json:"total" db:"total"`
		Completed int    `json:"completed" db:"completed"`
	}
	var enrolledPaths []pathProgress
	h.db.SelectContext(r.Context(), &enrolledPaths, `
		SELECT lp.id AS path_id, lp.slug AS path_slug, lp.title AS path_title, COALESCE(lp.icon, '') AS path_icon,
		       COUNT(DISTINCT s.id) AS total,
		       COUNT(DISTINCT CASE WHEN p.status = 'completed' THEN s.id END) AS completed
		FROM progress p
		JOIN steps s ON s.id = p.step_id
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE p.user_id = $1 AND s.deleted_at IS NULL
		GROUP BY lp.id, lp.title, lp.slug, lp.icon
	`, claims.UserID)
	if enrolledPaths == nil {
		enrolledPaths = []pathProgress{}
	}

	// Competencies: acquired (all steps in module completed) + pending (enrolled but not all done)
	type competency struct {
		Name      string `json:"name"`
		Acquired  bool   `json:"acquired"`
		PathTitle string `json:"path_title" db:"path_title"`
	}
	var competencies []competency
	// Acquired: modules where all steps are completed
	h.db.SelectContext(r.Context(), &competencies, `
		SELECT UNNEST(m.competencies) AS name, true AS acquired, lp.title AS path_title
		FROM modules m
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE m.deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM steps s
			WHERE s.module_id = m.id AND s.deleted_at IS NULL
			AND NOT EXISTS (
				SELECT 1 FROM progress p
				WHERE p.step_id = s.id AND p.user_id = $1 AND p.status = 'completed'
			)
		)
		AND EXISTS (SELECT 1 FROM steps s WHERE s.module_id = m.id AND s.deleted_at IS NULL)
	`, claims.UserID)
	// Pending: modules where user has some progress but not all steps completed
	var pendingCompetencies []competency
	h.db.SelectContext(r.Context(), &pendingCompetencies, `
		SELECT UNNEST(m.competencies) AS name, false AS acquired, lp.title AS path_title
		FROM modules m
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE m.deleted_at IS NULL AND EXISTS (
			SELECT 1 FROM progress p
			JOIN steps s ON s.id = p.step_id
			WHERE s.module_id = m.id AND p.user_id = $1 AND s.deleted_at IS NULL
		)
		AND EXISTS (
			SELECT 1 FROM steps s
			WHERE s.module_id = m.id AND s.deleted_at IS NULL
			AND NOT EXISTS (
				SELECT 1 FROM progress p
				WHERE p.step_id = s.id AND p.user_id = $1 AND p.status = 'completed'
			)
		)
		AND array_length(m.competencies, 1) > 0
	`, claims.UserID)
	competencies = append(competencies, pendingCompetencies...)
	if competencies == nil {
		competencies = []competency{}
	}

	// Stats
	var stats struct {
		StepsCompleted int `json:"steps_completed" db:"steps_completed"`
		TotalExercises int `json:"total_exercises" db:"total_exercises"`
		StepsInProgress int `json:"steps_in_progress" db:"steps_in_progress"`
	}
	h.db.GetContext(r.Context(), &stats, `
		SELECT
			COUNT(CASE WHEN p.status = 'completed' THEN 1 END) AS steps_completed,
			COUNT(CASE WHEN p.status = 'in_progress' THEN 1 END) AS steps_in_progress,
			(SELECT COUNT(*) FROM exercise_attempts WHERE user_id = $1) AS total_exercises
		FROM progress p WHERE p.user_id = $1
	`, claims.UserID)

	// Recent activity
	type activity struct {
		StepTitle string `json:"step_title" db:"step_title"`
		PathTitle string `json:"path_title" db:"path_title"`
		PathID    string `json:"path_id" db:"path_id"`
		PathSlug  string `json:"path_slug" db:"path_slug"`
		StepID    string `json:"step_id" db:"step_id"`
		StepSlug  string `json:"step_slug" db:"step_slug"`
		Event     string `json:"event" db:"event"`
		Timestamp string `json:"timestamp" db:"timestamp"`
	}
	var recentActivity []activity
	h.db.SelectContext(r.Context(), &recentActivity, `
		SELECT s.title AS step_title, lp.title AS path_title,
		       lp.id AS path_id, lp.slug AS path_slug,
		       s.id AS step_id, s.slug AS step_slug,
		       p.status AS event, p.updated_at::text AS timestamp
		FROM progress p
		JOIN steps s ON s.id = p.step_id
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE p.user_id = $1 AND s.deleted_at IS NULL
		ORDER BY p.updated_at DESC LIMIT 10
	`, claims.UserID)
	if recentActivity == nil {
		recentActivity = []activity{}
	}

	// Instructor repos (if user is instructor or admin)
	type instructorRepo struct {
		ID           string  `json:"id" db:"id"`
		CloneURL     string  `json:"clone_url" db:"clone_url"`
		Branch       string  `json:"branch" db:"branch"`
		SyncStatus   string  `json:"sync_status" db:"sync_status"`
		SyncError    *string `json:"sync_error,omitempty" db:"sync_error"`
		LastSyncedAt *string `json:"last_synced_at,omitempty" db:"last_synced_at"`
		PathTitles   string  `json:"path_titles_raw" db:"path_titles"`
	}
	type instructorRepoOut struct {
		ID           string   `json:"id"`
		CloneURL     string   `json:"clone_url"`
		Branch       string   `json:"branch"`
		SyncStatus   string   `json:"sync_status"`
		SyncError    *string  `json:"sync_error,omitempty"`
		LastSyncedAt *string  `json:"last_synced_at,omitempty"`
		PathTitles   []string `json:"path_titles"`
	}
	var instructorRepos []instructorRepoOut
	if claims.Role == model.RoleInstructor || claims.Role == model.RoleAdmin {
		var rows []instructorRepo
		h.db.SelectContext(r.Context(), &rows, `
			SELECT gr.id::text AS id, gr.clone_url, gr.branch, gr.sync_status, gr.sync_error,
			       to_char(gr.last_synced_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS last_synced_at,
			       COALESCE(string_agg(lp.title, '||'), '') AS path_titles
			FROM git_repositories gr
			JOIN repository_owners ro ON ro.repo_id = gr.id
			LEFT JOIN learning_paths lp ON lp.repo_id = gr.id AND lp.deleted_at IS NULL
			WHERE ro.user_id = $1
			GROUP BY gr.id
			ORDER BY gr.created_at DESC
		`, claims.UserID)
		for _, row := range rows {
			titles := []string{}
			if row.PathTitles != "" {
				for _, t := range splitNonEmpty(row.PathTitles, "||") {
					titles = append(titles, t)
				}
			}
			instructorRepos = append(instructorRepos, instructorRepoOut{
				ID: row.ID, CloneURL: row.CloneURL, Branch: row.Branch,
				SyncStatus: row.SyncStatus, SyncError: row.SyncError,
				LastSyncedAt: row.LastSyncedAt, PathTitles: titles,
			})
		}
	}
	if instructorRepos == nil {
		instructorRepos = []instructorRepoOut{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"continue_learning": continueStep,
		"enrolled_paths":    enrolledPaths,
		"competencies":      competencies,
		"stats":             stats,
		"recent_activity":   recentActivity,
		"instructor_repos":  instructorRepos,
	})
}

// AdminHealth returns platform health metrics.
func (h *Handler) AdminHealth(w http.ResponseWriter, r *http.Request) {
	// Database status
	dbOK := h.db.Ping() == nil

	// Repo sync status
	type repoStatus struct {
		ID         string  `json:"id" db:"id"`
		CloneURL   string  `json:"clone_url" db:"clone_url"`
		SyncStatus string  `json:"sync_status" db:"sync_status"`
		SyncError  *string `json:"sync_error,omitempty" db:"sync_error"`
		LastSynced *string `json:"last_synced_at,omitempty" db:"last_synced_at"`
	}
	var repos []repoStatus
	h.db.SelectContext(r.Context(), &repos, `
		SELECT id, clone_url, sync_status, sync_error, last_synced_at::text AS last_synced_at
		FROM git_repositories ORDER BY created_at
	`)
	if repos == nil {
		repos = []repoStatus{}
	}

	syncedCount := 0
	for _, r := range repos {
		if r.SyncStatus == "synced" {
			syncedCount++
		}
	}

	// Active users (24h)
	var activeUsers24h int
	h.db.GetContext(r.Context(), &activeUsers24h, `
		SELECT COUNT(*) FROM users WHERE last_login_at > now() - interval '24 hours' AND active = true
	`)

	// Total users
	var totalUsers int
	h.db.GetContext(r.Context(), &totalUsers, `SELECT COUNT(*) FROM users WHERE active = true`)

	// Uptime
	uptime := time.Since(serverStartTime).Round(time.Second).String()

	// Latency percentiles
	p50, p95, p99 := latencyTracker.Percentiles()

	writeJSON(w, http.StatusOK, map[string]any{
		"api":            map[string]any{"status": "ok", "uptime": uptime},
		"database":       map[string]any{"connected": dbOK},
		"repositories":   map[string]any{"total": len(repos), "synced": syncedCount, "details": repos},
		"active_users_24h": activeUsers24h,
		"total_users":    totalUsers,
		"latency": map[string]any{
			"p50_ms": p50,
			"p95_ms": p95,
			"p99_ms": p99,
		},
	})
}
