package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Analytics endpoints (instructor/admin only) ---

type analyticsOverview struct {
	TotalPaths     int                    `json:"total_paths"`
	TotalLearners  int                    `json:"total_learners"`
	CompletionRate float64                `json:"completion_rate"`
	TotalAttempts  int                    `json:"total_attempts"`
	PathsAnalytics []pathAnalyticsSummary `json:"paths"`
}

type pathAnalyticsSummary struct {
	ID             string  `json:"id" db:"id"`
	Slug           string  `json:"slug" db:"slug"`
	Title          string  `json:"title" db:"title"`
	EnrolledCount  int     `json:"enrolled_count" db:"enrolled_count"`
	CompletionRate float64 `json:"completion_rate" db:"completion_rate"`
	TotalSteps     int     `json:"total_steps" db:"total_steps"`
}

func (h *Handler) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var overview analyticsOverview

	// Platform-wide counters and the overall completion rate, in one round-trip
	var counters struct {
		TotalPaths     int `db:"total_paths"`
		TotalLearners  int `db:"total_learners"`
		TotalAttempts  int `db:"total_attempts"`
		TotalSteps     int `db:"total_steps"`
		CompletedSteps int `db:"completed_steps"`
	}
	if err := h.db.GetContext(ctx, &counters, `
		SELECT (SELECT COUNT(*) FROM learning_paths WHERE deleted_at IS NULL) AS total_paths,
		       (SELECT COUNT(DISTINCT user_id) FROM progress) AS total_learners,
		       (SELECT COUNT(*) FROM exercise_attempts) AS total_attempts,
		       (SELECT COUNT(*) FROM progress) AS total_steps,
		       (SELECT COUNT(*) FROM progress WHERE status = 'completed') AS completed_steps
	`); err != nil {
		writeDBError(w, r, "failed to load analytics overview", err)
		return
	}
	overview.TotalPaths = counters.TotalPaths
	overview.TotalLearners = counters.TotalLearners
	overview.TotalAttempts = counters.TotalAttempts
	totalSteps, completedSteps := counters.TotalSteps, counters.CompletedSteps
	if totalSteps > 0 {
		overview.CompletionRate = float64(completedSteps) / float64(totalSteps) * 100
	}

	// Per-path analytics
	if err := h.db.SelectContext(ctx, &overview.PathsAnalytics, `
		WITH path_steps AS (
			SELECT lp.id AS path_id, lp.slug, lp.title, COUNT(s.id) AS total_steps
			FROM learning_paths lp
			LEFT JOIN modules m ON m.learning_path_id = lp.id AND m.deleted_at IS NULL
			LEFT JOIN steps s ON s.module_id = m.id AND s.deleted_at IS NULL
			WHERE lp.deleted_at IS NULL
			GROUP BY lp.id, lp.title, lp.slug
		),
		path_enrollment AS (
			SELECT ps.path_id, COUNT(DISTINCT p.user_id) AS enrolled_count
			FROM path_steps ps
			LEFT JOIN modules m ON m.learning_path_id = ps.path_id AND m.deleted_at IS NULL
			LEFT JOIN steps s ON s.module_id = m.id AND s.deleted_at IS NULL
			LEFT JOIN progress p ON p.step_id = s.id
			GROUP BY ps.path_id
		),
		path_completion AS (
			SELECT ps.path_id,
			       CASE WHEN ps.total_steps = 0 THEN 0
			       ELSE COALESCE(
			           (SELECT COUNT(*)::float FROM progress pr
			            JOIN steps s2 ON s2.id = pr.step_id AND s2.deleted_at IS NULL
			            JOIN modules m2 ON m2.id = s2.module_id AND m2.learning_path_id = ps.path_id AND m2.deleted_at IS NULL
			            WHERE pr.status = 'completed')
			           / NULLIF(ps.total_steps * NULLIF(pe.enrolled_count, 0), 0) * 100, 0)
			       END AS completion_rate
			FROM path_steps ps
			JOIN path_enrollment pe ON pe.path_id = ps.path_id
		)
		SELECT ps.path_id AS id, ps.slug, ps.title, ps.total_steps,
		       COALESCE(pe.enrolled_count, 0) AS enrolled_count,
		       COALESCE(pc.completion_rate, 0) AS completion_rate
		FROM path_steps ps
		LEFT JOIN path_enrollment pe ON pe.path_id = ps.path_id
		LEFT JOIN path_completion pc ON pc.path_id = ps.path_id
		ORDER BY ps.title
	`); err != nil {
		writeDBError(w, r, "failed to load path analytics", err)
		return
	}
	if overview.PathsAnalytics == nil {
		overview.PathsAnalytics = []pathAnalyticsSummary{}
	}

	writeJSON(w, http.StatusOK, overview)
}

type activityEvent struct {
	UserID      string    `json:"user_id" db:"user_id"`
	Username    string    `json:"username" db:"username"`
	DisplayName string    `json:"display_name" db:"display_name"`
	StepTitle   string    `json:"step_title" db:"step_title"`
	PathTitle   string    `json:"path_title" db:"path_title"`
	Event       string    `json:"event" db:"event"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

func (h *Handler) AnalyticsActivity(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}

	var events []activityEvent
	if err := h.db.SelectContext(r.Context(), &events, `
		SELECT p.user_id, u.username, u.display_name,
		       s.title AS step_title, lp.title AS path_title,
		       p.status AS event, p.updated_at AS created_at
		FROM progress p
		JOIN users u ON u.id = p.user_id
		JOIN steps s ON s.id = p.step_id
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL
		ORDER BY p.updated_at DESC
		LIMIT $1
	`, limit); err != nil {
		writeDBError(w, r, "failed to load activity", err)
		return
	}
	if events == nil {
		events = []activityEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

type pathAnalyticsDetail struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	EnrolledCount  int               `json:"enrolled_count"`
	CompletionRate float64           `json:"completion_rate"`
	Steps          []stepAnalytics   `json:"steps"`
	Learners       []learnerProgress `json:"learners"`
}

type stepAnalytics struct {
	ID             string  `json:"id" db:"id"`
	Title          string  `json:"title" db:"title"`
	Type           string  `json:"type" db:"type"`
	Position       int     `json:"position" db:"position"`
	CompletionRate float64 `json:"completion_rate"`
	AvgAttempts    float64 `json:"avg_attempts"`
}

type learnerProgress struct {
	UserID       string  `json:"user_id" db:"user_id"`
	Username     string  `json:"username" db:"username"`
	DisplayName  string  `json:"display_name" db:"display_name"`
	Completed    int     `json:"completed" db:"completed"`
	Total        int     `json:"total" db:"total"`
	Percentage   float64 `json:"percentage"`
	LastActivity *string `json:"last_activity" db:"last_activity"`
}

func (h *Handler) AnalyticsPath(w http.ResponseWriter, r *http.Request) {
	pathParam := chi.URLParam(r, "pathId")
	ctx := r.Context()
	pathUUID, err := h.resolvePathSlug(ctx, pathParam)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "path not found"})
		return
	}
	pathID := pathUUID.String()

	// Get path info
	var detail pathAnalyticsDetail
	var title string
	err = h.db.GetContext(ctx, &title, `SELECT title FROM learning_paths WHERE id = $1 AND deleted_at IS NULL`, pathID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "path not found"})
		return
	}
	detail.ID = pathID
	detail.Title = title

	// Enrolled count
	if err := h.db.GetContext(ctx, &detail.EnrolledCount, `
		SELECT COUNT(DISTINCT p.user_id)
		FROM progress p
		JOIN steps s ON s.id = p.step_id AND s.deleted_at IS NULL
		JOIN modules m ON m.id = s.module_id AND m.learning_path_id = $1 AND m.deleted_at IS NULL
	`, pathID); err != nil {
		writeDBError(w, r, "failed to load enrolled count", err)
		return
	}

	// Per-step analytics: completion and average attempts are aggregated in the
	// same query to avoid one round-trip per step.
	type stepRow struct {
		ID          string  `db:"id"`
		Title       string  `db:"title"`
		Type        string  `db:"type"`
		Position    int     `db:"position"`
		Total       int     `db:"total"`
		Completed   int     `db:"completed"`
		AvgAttempts float64 `db:"avg_attempts"`
	}
	var steps []stepRow
	if err := h.db.SelectContext(ctx, &steps, `
		SELECT s.id, s.title, s.type, s.position,
		       (SELECT COUNT(*) FROM progress p WHERE p.step_id = s.id) AS total,
		       (SELECT COUNT(*) FROM progress p WHERE p.step_id = s.id AND p.status = 'completed') AS completed,
		       CASE WHEN s.type = 'lesson' THEN 0 ELSE COALESCE((
		           SELECT AVG(attempt_count)::float FROM (
		               SELECT COUNT(*) AS attempt_count
		               FROM exercise_attempts ea
		               WHERE ea.step_id = s.id
		               GROUP BY ea.user_id
		           ) sub
		       ), 0) END AS avg_attempts
		FROM steps s
		JOIN modules m ON m.id = s.module_id AND m.learning_path_id = $1 AND m.deleted_at IS NULL
		WHERE s.deleted_at IS NULL
		ORDER BY m.position, s.position
	`, pathID); err != nil {
		writeDBError(w, r, "failed to load step analytics", err)
		return
	}

	for _, s := range steps {
		sa := stepAnalytics{ID: s.ID, Title: s.Title, Type: s.Type, Position: s.Position, AvgAttempts: s.AvgAttempts}
		if s.Total > 0 {
			sa.CompletionRate = float64(s.Completed) / float64(s.Total) * 100
		}
		detail.Steps = append(detail.Steps, sa)
	}
	if detail.Steps == nil {
		detail.Steps = []stepAnalytics{}
	}

	// Total steps for completion rate
	totalSteps := len(steps)
	if totalSteps > 0 && detail.EnrolledCount > 0 {
		var totalCompleted int
		if err := h.db.GetContext(ctx, &totalCompleted, `
			SELECT COUNT(*)
			FROM progress p
			JOIN steps s ON s.id = p.step_id AND s.deleted_at IS NULL
			JOIN modules m ON m.id = s.module_id AND m.learning_path_id = $1 AND m.deleted_at IS NULL
			WHERE p.status = 'completed'
		`, pathID); err != nil {
			writeDBError(w, r, "failed to load path completion", err)
			return
		}
		detail.CompletionRate = float64(totalCompleted) / float64(totalSteps*detail.EnrolledCount) * 100
	}

	// Per-learner progress
	if err := h.db.SelectContext(ctx, &detail.Learners, `
		SELECT p.user_id, u.username, u.display_name,
		       COUNT(CASE WHEN p.status = 'completed' THEN 1 END) AS completed,
		       COUNT(*) AS total,
		       MAX(p.updated_at)::text AS last_activity
		FROM progress p
		JOIN users u ON u.id = p.user_id
		JOIN steps s ON s.id = p.step_id AND s.deleted_at IS NULL
		JOIN modules m ON m.id = s.module_id AND m.learning_path_id = $1 AND m.deleted_at IS NULL
		GROUP BY p.user_id, u.username, u.display_name
		ORDER BY u.username
	`, pathID); err != nil {
		writeDBError(w, r, "failed to load learner progress", err)
		return
	}
	for i := range detail.Learners {
		if detail.Learners[i].Total > 0 {
			detail.Learners[i].Percentage = float64(detail.Learners[i].Completed) / float64(totalSteps) * 100
		}
	}
	if detail.Learners == nil {
		detail.Learners = []learnerProgress{}
	}

	writeJSON(w, http.StatusOK, detail)
}

type stepAnalyticsDetail struct {
	StepID       string                   `json:"step_id"`
	Title        string                   `json:"title"`
	Type         string                   `json:"type"`
	WrongAnswers []map[string]interface{} `json:"wrong_answers"`
}

func (h *Handler) AnalyticsStep(w http.ResponseWriter, r *http.Request) {
	stepParam := chi.URLParam(r, "stepId")
	ctx := r.Context()
	stepUUID, err := h.resolveStepSlug(ctx, stepParam, "")
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}
	stepID := stepUUID.String()

	var detail stepAnalyticsDetail
	err = h.db.QueryRowxContext(ctx, `SELECT id, title, type FROM steps WHERE id = $1 AND deleted_at IS NULL`, stepID).Scan(&detail.StepID, &detail.Title, &detail.Type)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "step not found"})
		return
	}

	// Get most common wrong answers
	type wrongAnswer struct {
		Answers string `db:"answers"`
		Count   int    `db:"count"`
	}
	var wrong []wrongAnswer
	if err := h.db.SelectContext(ctx, &wrong, `
		SELECT answers::text, COUNT(*) AS count
		FROM exercise_attempts
		WHERE step_id = $1 AND is_correct = false
		GROUP BY answers::text
		ORDER BY count DESC
		LIMIT 10
	`, stepID); err != nil {
		writeDBError(w, r, "failed to load wrong answers", err)
		return
	}

	detail.WrongAnswers = make([]map[string]interface{}, 0, len(wrong))
	for _, w := range wrong {
		detail.WrongAnswers = append(detail.WrongAnswers, map[string]interface{}{
			"answers": w.Answers,
			"count":   w.Count,
		})
	}

	writeJSON(w, http.StatusOK, detail)
}

type learnerDetail struct {
	UserID        uuid.UUID         `json:"user_id" db:"id"`
	Username      string            `json:"username" db:"username"`
	DisplayName   string            `json:"display_name" db:"display_name"`
	Email         *string           `json:"email,omitempty" db:"email"`
	Role          string            `json:"role" db:"role"`
	LastLoginAt   *time.Time        `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
	EnrolledPaths []enrolledPath    `json:"enrolled_paths"`
	Activity      []activityItem    `json:"activity"`
	Performance   []performanceItem `json:"performance"`
	KPIs          *learnerKPIs      `json:"kpis" db:"-"`
}

type enrolledPath struct {
	PathID     string  `json:"path_id" db:"path_id"`
	PathSlug   string  `json:"path_slug" db:"path_slug"`
	PathTitle  string  `json:"path_title" db:"path_title"`
	Completed  int     `json:"completed" db:"completed"`
	Total      int     `json:"total" db:"total"`
	PathSteps  int     `json:"path_steps" db:"path_steps"`
	Percentage float64 `json:"percentage"`
}

type activityItem struct {
	StepTitle string    `json:"step_title" db:"step_title"`
	PathTitle string    `json:"path_title" db:"path_title"`
	Event     string    `json:"event" db:"event"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}

type performanceItem struct {
	StepID    string `json:"step_id" db:"step_id"`
	StepTitle string `json:"step_title" db:"step_title"`
	StepType  string `json:"step_type" db:"step_type"`
	Attempts  int    `json:"attempts" db:"attempts"`
	Correct   int    `json:"correct" db:"correct"`
}

func (h *Handler) AnalyticsLearner(w http.ResponseWriter, r *http.Request) {
	learnerID := chi.URLParam(r, "learnerId")
	ctx := r.Context()

	var detail learnerDetail
	err := h.db.GetContext(ctx, &detail, `
		SELECT id, username, display_name, email, role, last_login_at, created_at
		FROM users WHERE id = $1
	`, learnerID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "learner not found"})
		return
	}

	// Enrolled paths with progress. path_steps is the number of steps in the whole
	// path (not just the ones the learner started), used for the percentage.
	if err := h.db.SelectContext(ctx, &detail.EnrolledPaths, `
		SELECT DISTINCT lp.id AS path_id, lp.slug AS path_slug, lp.title AS path_title,
		       COUNT(CASE WHEN p.status = 'completed' THEN 1 END) AS completed,
		       COUNT(*) AS total,
		       (SELECT COUNT(s2.id) FROM steps s2
		        JOIN modules m2 ON m2.id = s2.module_id AND m2.learning_path_id = lp.id AND m2.deleted_at IS NULL
		        WHERE s2.deleted_at IS NULL) AS path_steps
		FROM progress p
		JOIN steps s ON s.id = p.step_id AND s.deleted_at IS NULL
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL
		WHERE p.user_id = $1
		GROUP BY lp.id, lp.title, lp.slug
		ORDER BY lp.title
	`, learnerID); err != nil {
		writeDBError(w, r, "failed to load enrolled paths", err)
		return
	}
	for i := range detail.EnrolledPaths {
		ep := &detail.EnrolledPaths[i]
		if ep.PathSteps > 0 {
			ep.Percentage = float64(ep.Completed) / float64(ep.PathSteps) * 100
		}
	}
	if detail.EnrolledPaths == nil {
		detail.EnrolledPaths = []enrolledPath{}
	}

	// Activity timeline
	if err := h.db.SelectContext(ctx, &detail.Activity, `
		SELECT s.title AS step_title, lp.title AS path_title,
		       p.status AS event, p.updated_at AS timestamp
		FROM progress p
		JOIN steps s ON s.id = p.step_id
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL
		WHERE p.user_id = $1
		ORDER BY p.updated_at DESC
		LIMIT 50
	`, learnerID); err != nil {
		writeDBError(w, r, "failed to load learner activity", err)
		return
	}
	if detail.Activity == nil {
		detail.Activity = []activityItem{}
	}

	// Exercise performance
	if err := h.db.SelectContext(ctx, &detail.Performance, `
		SELECT ea.step_id, s.title AS step_title, s.type AS step_type,
		       COUNT(*) AS attempts,
		       COUNT(CASE WHEN ea.is_correct THEN 1 END) AS correct
		FROM exercise_attempts ea
		JOIN steps s ON s.id = ea.step_id
		WHERE ea.user_id = $1
		GROUP BY ea.step_id, s.title, s.type
		ORDER BY s.title
	`, learnerID); err != nil {
		writeDBError(w, r, "failed to load learner performance", err)
		return
	}
	if detail.Performance == nil {
		detail.Performance = []performanceItem{}
	}

	if detail.KPIs, err = h.loadLearnerKPIs(r, detail.UserID); err != nil {
		writeDBError(w, r, "failed to load learner KPIs", err)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}
