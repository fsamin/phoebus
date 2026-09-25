package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Thresholds of the learner warning signals shown in analytics.
const (
	// A learner is inactive when nothing happened on their side for this long.
	learnerInactiveDays = 30
	// A step is stuck when it was started this long ago and is still not completed.
	learnerStuckDays = 14
)

// learnerStatsCTE computes per-user KPIs over enabled, non-deleted paths.
// $1 is learnerStuckDays. The final learner_stats relation has one row per user.
const learnerStatsCTE = `
	WITH path_steps AS (
		SELECT m.learning_path_id AS path_id, s.id AS step_id
		FROM steps s
		JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
		JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL AND lp.enabled = true
		WHERE s.deleted_at IS NULL
	),
	path_size AS (
		SELECT path_id, COUNT(*) AS steps FROM path_steps GROUP BY path_id
	),
	user_paths AS (
		SELECT p.user_id, ps.path_id, COUNT(*) FILTER (WHERE p.status = 'completed') AS completed
		FROM progress p
		JOIN path_steps ps ON ps.step_id = p.step_id
		GROUP BY p.user_id, ps.path_id
	),
	path_stats AS (
		SELECT up.user_id,
		       COUNT(*) AS enrolled_paths,
		       COUNT(*) FILTER (WHERE up.completed = sz.steps) AS completed_paths,
		       SUM(up.completed)::int AS completed_steps,
		       SUM(sz.steps)::int AS enrolled_steps
		FROM user_paths up
		JOIN path_size sz ON sz.path_id = up.path_id
		GROUP BY up.user_id
	),
	exercise_stats AS (
		SELECT user_id,
		       SUM(attempts)::int AS attempts,
		       COUNT(*) AS exercises,
		       COUNT(*) FILTER (WHERE first_correct) AS first_try_correct
		FROM (
			SELECT user_id, step_id, COUNT(*) AS attempts,
			       (array_agg(is_correct ORDER BY created_at))[1] AS first_correct
			FROM exercise_attempts
			GROUP BY user_id, step_id
		) per_exercise
		GROUP BY user_id
	),
	last_activity AS (
		SELECT user_id, MAX(ts) AS last_activity
		FROM (
			SELECT user_id, updated_at AS ts FROM progress
			UNION ALL
			SELECT user_id, created_at FROM exercise_attempts
		) events
		GROUP BY user_id
	),
	stuck AS (
		SELECT p.user_id, COUNT(*) AS stuck_steps
		FROM progress p
		JOIN path_steps ps ON ps.step_id = p.step_id
		WHERE p.status = 'in_progress' AND p.created_at < now() - make_interval(days => $1)
		GROUP BY p.user_id
	),
	learner_stats AS (
		SELECT u.id, u.username, u.display_name, u.email, u.role, u.active,
		       COALESCE(ps.enrolled_paths, 0) AS enrolled_paths,
		       COALESCE(ps.completed_paths, 0) AS completed_paths,
		       COALESCE(ps.enrolled_steps, 0) AS enrolled_steps,
		       COALESCE(ps.completed_steps, 0) AS completed_steps,
		       COALESCE(es.attempts, 0) AS attempts,
		       COALESCE(es.exercises, 0) AS exercises,
		       COALESCE(es.first_try_correct, 0) AS first_try_correct,
		       COALESCE(st.stuck_steps, 0) AS stuck_steps,
		       la.last_activity
		FROM users u
		LEFT JOIN path_stats ps ON ps.user_id = u.id
		LEFT JOIN exercise_stats es ON es.user_id = u.id
		LEFT JOIN stuck st ON st.user_id = u.id
		LEFT JOIN last_activity la ON la.user_id = u.id
	)
`

type learnerStats struct {
	UserID          uuid.UUID  `json:"user_id" db:"id"`
	Username        string     `json:"username" db:"username"`
	DisplayName     string     `json:"display_name" db:"display_name"`
	Email           *string    `json:"email,omitempty" db:"email"`
	Role            string     `json:"role" db:"role"`
	Active          bool       `json:"-" db:"active"`
	EnrolledPaths   int        `json:"enrolled_paths" db:"enrolled_paths"`
	CompletedPaths  int        `json:"completed_paths" db:"completed_paths"`
	EnrolledSteps   int        `json:"enrolled_steps" db:"enrolled_steps"`
	CompletedSteps  int        `json:"completed_steps" db:"completed_steps"`
	Attempts        int        `json:"attempts" db:"attempts"`
	Exercises       int        `json:"exercises" db:"exercises"`
	FirstTryCorrect int        `json:"first_try_correct" db:"first_try_correct"`
	StuckSteps      int        `json:"stuck_steps" db:"stuck_steps"`
	LastActivity    *time.Time `json:"last_activity" db:"last_activity"`
	Total           int        `json:"-" db:"total"`

	// Derived; nil when there is nothing to compute the rate from.
	ProgressRate *float64 `json:"progress_rate"`
	FirstTryRate *float64 `json:"first_try_rate"`
	AvgAttempts  *float64 `json:"avg_attempts"`
	Inactive     bool     `json:"inactive"`
}

func (s *learnerStats) derive(now time.Time) {
	if s.EnrolledSteps > 0 {
		v := float64(s.CompletedSteps) / float64(s.EnrolledSteps) * 100
		s.ProgressRate = &v
	}
	if s.Exercises > 0 {
		rate := float64(s.FirstTryCorrect) / float64(s.Exercises) * 100
		avg := float64(s.Attempts) / float64(s.Exercises)
		s.FirstTryRate, s.AvgAttempts = &rate, &avg
	}
	s.Inactive = s.LastActivity != nil && s.LastActivity.Before(now.AddDate(0, 0, -learnerInactiveDays))
}

// learnerSorts maps the accepted sort keys to their ORDER BY expression.
// Rows without data for the key always come last.
var learnerSorts = map[string]string{
	"name":            "COALESCE(NULLIF(display_name, ''), username)",
	"last_activity":   "last_activity",
	"progress":        "completed_steps::float / NULLIF(enrolled_steps, 0)",
	"completed_paths": "completed_paths",
	"first_try_rate":  "first_try_correct::float / NULLIF(exercises, 0)",
}

// AnalyticsLearners lists learners with their KPIs. Listed users are the active
// learners, plus any active user who has progress on a path, so that learners
// who never started are visible too.
func (h *Handler) AnalyticsLearners(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, perPage := 1, 20
	if n, err := strconv.Atoi(query.Get("page")); err == nil && n > 0 {
		page = n
	}
	if n, err := strconv.Atoi(query.Get("per_page")); err == nil && n > 0 && n <= 100 {
		perPage = n
	}

	sortExpr, ok := learnerSorts[query.Get("sort")]
	if !ok {
		sortExpr = learnerSorts["name"]
	}
	direction := "ASC"
	if query.Get("order") == "desc" {
		direction = "DESC"
	}

	status := query.Get("status")
	switch status {
	case "", "not_started", "inactive", "stuck":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be one of not_started, inactive, stuck"})
		return
	}

	var learners []learnerStats
	if err := h.db.SelectContext(r.Context(), &learners, learnerStatsCTE+`
		SELECT *, COUNT(*) OVER () AS total
		FROM learner_stats
		WHERE active AND (role = 'learner' OR enrolled_paths > 0)
		  AND ($3 = '' OR username ILIKE $3 OR display_name ILIKE $3 OR COALESCE(email, '') ILIKE $3)
		  AND CASE $4::text
		      WHEN 'not_started' THEN enrolled_paths = 0
		      WHEN 'inactive' THEN last_activity < now() - make_interval(days => $2)
		      WHEN 'stuck' THEN stuck_steps > 0
		      ELSE true END
		ORDER BY `+sortExpr+` `+direction+` NULLS LAST, id
		LIMIT $5 OFFSET $6
	`, learnerStuckDays, learnerInactiveDays, likePattern(query.Get("q")), status, perPage, (page-1)*perPage); err != nil {
		writeDBError(w, r, "failed to list learners", err)
		return
	}

	total := 0
	now := time.Now()
	for i := range learners {
		learners[i].derive(now)
		total = learners[i].Total
	}
	if learners == nil {
		learners = []learnerStats{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"learners":      learners,
		"total":         total,
		"page":          page,
		"per_page":      perPage,
		"inactive_days": learnerInactiveDays,
		"stuck_days":    learnerStuckDays,
	})
}

type learnerKPIs struct {
	learnerStats
	ActiveDays30d int `json:"active_days_30d"`
	CurrentStreak int `json:"current_streak"`
	InactiveDays  int `json:"inactive_days"`
	StuckDays     int `json:"stuck_days"`
}

// loadLearnerKPIs computes the KPIs of a single user, whatever their role or
// status, for the learner detail page.
func (h *Handler) loadLearnerKPIs(r *http.Request, userID uuid.UUID) (*learnerKPIs, error) {
	ctx := r.Context()
	kpis := &learnerKPIs{InactiveDays: learnerInactiveDays, StuckDays: learnerStuckDays}
	if err := h.db.GetContext(ctx, &kpis.learnerStats, learnerStatsCTE+`
		SELECT *, 1 AS total FROM learner_stats WHERE id = $2
	`, learnerStuckDays, userID); err != nil {
		return nil, err
	}
	now := time.Now()
	kpis.derive(now)

	// Days with at least one event, in UTC. progress only keeps the latest
	// update of a step, so revisits of the same step on older days are lost;
	// attempts keep their full history.
	var days []time.Time
	if err := h.db.SelectContext(ctx, &days, `
		SELECT DISTINCT (ts AT TIME ZONE 'UTC')::date AS day
		FROM (
			SELECT created_at AS ts FROM progress WHERE user_id = $1
			UNION ALL SELECT completed_at FROM progress WHERE user_id = $1 AND completed_at IS NOT NULL
			UNION ALL SELECT updated_at FROM progress WHERE user_id = $1
			UNION ALL SELECT created_at FROM exercise_attempts WHERE user_id = $1
		) events
		WHERE ts > now() - interval '400 days'
		ORDER BY day DESC
	`, userID); err != nil {
		return nil, err
	}
	kpis.ActiveDays30d, kpis.CurrentStreak = activityDays(days, now)
	return kpis, nil
}

// activityDays returns the number of active days over the last 30 days
// (today included) and the current streak of consecutive active days. The
// streak is still running when the last active day is yesterday, so a learner
// does not lose it before having had a chance to come back today.
// days must be distinct UTC dates sorted from the most recent.
func activityDays(days []time.Time, now time.Time) (last30, streak int) {
	today := now.UTC().Truncate(24 * time.Hour)
	for _, d := range days {
		if age := int(today.Sub(d.UTC().Truncate(24*time.Hour)).Hours() / 24); age >= 0 && age < 30 {
			last30++
		}
	}

	expected := today
	for i, d := range days {
		d = d.UTC().Truncate(24 * time.Hour)
		if i == 0 && d.Equal(today.AddDate(0, 0, -1)) {
			expected = d
		}
		if !d.Equal(expected) {
			break
		}
		streak++
		expected = expected.AddDate(0, 0, -1)
	}
	return last30, streak
}
