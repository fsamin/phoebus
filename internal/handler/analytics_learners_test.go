package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/google/uuid"
)

// createKPIPath creates a path with one module holding one step per given type
// and returns the step IDs in order.
func createKPIPath(t *testing.T, enabled bool, types ...string) []string {
	t.Helper()
	suffix := uuid.New().String()[:8]
	repoID, pathID, modID := uuid.New(), uuid.New(), uuid.New()
	testDB.MustExec(`INSERT INTO git_repositories (id, clone_url, branch, auth_type, webhook_uuid, sync_status)
		VALUES ($1, 'https://github.com/test/kpi-'||$3||'.git', 'main', 'none', $2, 'synced')`, repoID, uuid.New(), suffix)
	testDB.MustExec(`INSERT INTO learning_paths (id, repo_id, title, file_path, slug, enabled)
		VALUES ($1, $2, 'KPI Path', 'kpi/', 'kpi-'||$3, $4)`, pathID, repoID, suffix, enabled)
	testDB.MustExec(`INSERT INTO modules (id, learning_path_id, title, position, file_path, slug)
		VALUES ($1, $2, 'KPI Module', 0, 'kpi/mod/', 'kpi-mod-'||$3)`, modID, pathID, suffix)
	steps := make([]string, len(types))
	for i, typ := range types {
		steps[i] = uuid.New().String()
		testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, position, file_path, slug)
			VALUES ($1, $2, 'KPI Step', $3, $4, 'kpi/mod/'||$5||'.md', 'kpi-step-'||$5)`,
			steps[i], modID, typ, i, suffix+"-"+strconv.Itoa(i))
	}
	return steps
}

func addProgress(t *testing.T, userID, stepID, status string, at time.Time) {
	t.Helper()
	testDB.MustExec(`INSERT INTO progress (user_id, step_id, status, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, CASE WHEN $3 = 'completed' THEN $4::timestamptz END, $4, $4)`, userID, stepID, status, at)
}

func addAttempt(t *testing.T, userID, stepID string, correct bool, at time.Time) {
	t.Helper()
	testDB.MustExec(`INSERT INTO exercise_attempts (user_id, step_id, is_correct, created_at) VALUES ($1, $2, $3, $4)`,
		userID, stepID, correct, at)
}

type learnersResponse struct {
	Learners []struct {
		UserID        string   `json:"user_id"`
		Username      string   `json:"username"`
		EnrolledPaths int      `json:"enrolled_paths"`
		ProgressRate  *float64 `json:"progress_rate"`
		Inactive      bool     `json:"inactive"`
		StuckSteps    int      `json:"stuck_steps"`
	} `json:"learners"`
	Total int `json:"total"`
}

func listLearners(t *testing.T, srv *httptest.Server, cookie *http.Cookie, params url.Values) learnersResponse {
	t.Helper()
	resp := doRequest(t, srv, "GET", "/api/analytics/learners?"+params.Encode(), nil, cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET learners %v: status = %d, want 200", params, resp.StatusCode)
	}
	var out learnersResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode learners: %v", err)
	}
	return out
}

func usernames(r learnersResponse) []string {
	out := make([]string, len(r.Learners))
	for i, l := range r.Learners {
		out[i] = l.Username
	}
	return out
}

// The learner KPIs summarize how far someone got and how easily: they are
// computed over enabled paths only, a path counts as completed only when every
// step is, and the first-try rate reflects the first attempt of each exercise.
func TestLearnerDetailKPIs(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	instructor := loginAs(t, model.RoleInstructor)
	tag := uuid.New().String()[:8]
	now := time.Now()

	pathA := createKPIPath(t, true, "lesson", "quiz")
	pathB := createKPIPath(t, true, "lesson", "quiz")
	disabled := createKPIPath(t, false, "lesson")

	learner := insertUser(t, "kpi-"+tag, "KPI", "", now)
	// Path A fully completed; its quiz passed at the first attempt.
	addProgress(t, learner, pathA[0], "completed", now)
	addProgress(t, learner, pathA[1], "completed", now)
	addAttempt(t, learner, pathA[1], true, now)
	// Path B half done; its quiz started 20 days ago and failed twice.
	addProgress(t, learner, pathB[0], "completed", now)
	addProgress(t, learner, pathB[1], "in_progress", now.AddDate(0, 0, -20))
	addAttempt(t, learner, pathB[1], false, now.AddDate(0, 0, -20))
	addAttempt(t, learner, pathB[1], false, now.AddDate(0, 0, -20).Add(time.Minute))
	// A disabled path is not offered to learners and must not weigh on KPIs.
	addProgress(t, learner, disabled[0], "completed", now)

	resp := doRequest(t, srv, "GET", "/api/analytics/learners/"+learner, nil, instructor)
	var body struct {
		KPIs struct {
			EnrolledPaths  int      `json:"enrolled_paths"`
			CompletedPaths int      `json:"completed_paths"`
			CompletedSteps int      `json:"completed_steps"`
			EnrolledSteps  int      `json:"enrolled_steps"`
			ProgressRate   *float64 `json:"progress_rate"`
			FirstTryRate   *float64 `json:"first_try_rate"`
			AvgAttempts    *float64 `json:"avg_attempts"`
			StuckSteps     int      `json:"stuck_steps"`
			Inactive       bool     `json:"inactive"`
			ActiveDays30d  int      `json:"active_days_30d"`
			CurrentStreak  int      `json:"current_streak"`
		} `json:"kpis"`
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	k := body.KPIs

	if k.EnrolledPaths != 2 || k.CompletedPaths != 1 {
		t.Errorf("paths = %d completed / %d enrolled, want 1 / 2", k.CompletedPaths, k.EnrolledPaths)
	}
	if k.CompletedSteps != 3 || k.EnrolledSteps != 4 || k.ProgressRate == nil || *k.ProgressRate != 75 {
		t.Errorf("steps = %d/%d rate=%v, want 3/4 = 75%%", k.CompletedSteps, k.EnrolledSteps, k.ProgressRate)
	}
	if k.FirstTryRate == nil || *k.FirstTryRate != 50 {
		t.Errorf("first_try_rate = %v, want 50", k.FirstTryRate)
	}
	if k.AvgAttempts == nil || *k.AvgAttempts != 1.5 {
		t.Errorf("avg_attempts = %v, want 1.5", k.AvgAttempts)
	}
	if k.StuckSteps != 1 {
		t.Errorf("stuck_steps = %d, want 1 (quiz started 20 days ago, threshold %d)", k.StuckSteps, learnerStuckDays)
	}
	if k.Inactive {
		t.Error("inactive = true, want false: the learner was active today")
	}
	if k.ActiveDays30d != 2 || k.CurrentStreak != 1 {
		t.Errorf("active_days_30d=%d current_streak=%d, want 2 and 1", k.ActiveDays30d, k.CurrentStreak)
	}
}

// A learner who never started anything has no rate to show: nil, not 0%,
// which would read as a failure.
func TestLearnerDetailKPIsWithoutActivity(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	instructor := loginAs(t, model.RoleInstructor)
	learner := insertUser(t, "kpi-empty-"+uuid.New().String()[:8], "Empty", "", time.Now())

	resp := doRequest(t, srv, "GET", "/api/analytics/learners/"+learner, nil, instructor)
	body := readJSON(t, resp)
	kpis, _ := body["kpis"].(map[string]any)
	if kpis == nil {
		t.Fatalf("no kpis in response: %v", body)
	}
	for _, key := range []string{"progress_rate", "first_try_rate", "avg_attempts", "last_activity"} {
		if kpis[key] != nil {
			t.Errorf("%s = %v, want null", key, kpis[key])
		}
	}
}

// The list must surface learners who never started (to chase them), include
// instructors only when they actually follow a path, and hide deactivated accounts.
func TestAnalyticsLearnersPopulation(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	instructor := loginAs(t, model.RoleInstructor)
	tag := uuid.New().String()[:8]
	now := time.Now()
	steps := createKPIPath(t, true, "lesson")

	insertUser(t, "pop-"+tag+"-learner-idle", "x", "", now)
	insertUser(t, "pop-"+tag+"-instructor-idle", "x", "", now)
	learningInstructor := insertUser(t, "pop-"+tag+"-instructor-learning", "x", "", now)
	deactivated := insertUser(t, "pop-"+tag+"-learner-deactivated", "x", "", now)
	testDB.MustExec(`UPDATE users SET role = 'instructor' WHERE username LIKE 'pop-'||$1||'-instructor-%'`, tag)
	testDB.MustExec(`UPDATE users SET active = false WHERE id = $1`, deactivated)
	addProgress(t, learningInstructor, steps[0], "completed", now)
	addProgress(t, deactivated, steps[0], "completed", now)

	got := listLearners(t, srv, instructor, url.Values{"q": {"pop-" + tag}})
	names := map[string]bool{}
	for _, n := range usernames(got) {
		names[n] = true
	}
	if len(names) != 2 || !names["pop-"+tag+"-instructor-learning"] || !names["pop-"+tag+"-learner-idle"] {
		t.Errorf("learners = %v, want the idle learner and the learning instructor only", usernames(got))
	}
	if got.Total != 2 {
		t.Errorf("total = %d, want 2", got.Total)
	}
}

// Status filters are how an instructor finds who drops out: each one must
// select exactly the learners carrying that signal.
func TestAnalyticsLearnersStatusFilter(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	instructor := loginAs(t, model.RoleInstructor)
	tag := uuid.New().String()[:8]
	now := time.Now()
	steps := createKPIPath(t, true, "lesson", "quiz")

	active := insertUser(t, "st-"+tag+"-active", "x", "", now)
	addProgress(t, active, steps[0], "completed", now)
	inactive := insertUser(t, "st-"+tag+"-inactive", "x", "", now)
	addProgress(t, inactive, steps[0], "completed", now.AddDate(0, 0, -(learnerInactiveDays+1)))
	stuck := insertUser(t, "st-"+tag+"-stuck", "x", "", now)
	addProgress(t, stuck, steps[1], "in_progress", now.AddDate(0, 0, -(learnerStuckDays+1)))
	addAttempt(t, stuck, steps[1], false, now)
	insertUser(t, "st-"+tag+"-not-started", "x", "", now)

	for status, want := range map[string]string{
		"inactive":    "st-" + tag + "-inactive",
		"stuck":       "st-" + tag + "-stuck",
		"not_started": "st-" + tag + "-not-started",
	} {
		got := listLearners(t, srv, instructor, url.Values{"q": {"st-" + tag}, "status": {status}})
		if names := usernames(got); len(names) != 1 || names[0] != want {
			t.Errorf("status=%s: learners = %v, want [%s]", status, names, want)
		}
	}

	all := listLearners(t, srv, instructor, url.Values{"q": {"st-" + tag}})
	for _, l := range all.Learners {
		if wantInactive := l.UserID == inactive; l.Inactive != wantInactive {
			t.Errorf("%s: inactive = %v, want %v", l.Username, l.Inactive, wantInactive)
		}
	}

	if resp := doRequest(t, srv, "GET", "/api/analytics/learners?status=bogus", nil, instructor); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=bogus: status = %d, want 400", resp.StatusCode)
	}
}

// Sorting by progress must rank learners by how far they got, in both
// directions, and keep those with no progress at all at the end rather than
// mixing them with 0%.
func TestAnalyticsLearnersSortByProgress(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	instructor := loginAs(t, model.RoleInstructor)
	tag := uuid.New().String()[:8]
	now := time.Now()
	steps := createKPIPath(t, true, "lesson", "lesson", "lesson", "lesson")

	for i, done := range []int{1, 4, 2} {
		u := insertUser(t, "sort-"+tag+"-"+strconv.Itoa(i), "x", "", now)
		for s := range done {
			addProgress(t, u, steps[s], "completed", now)
		}
	}
	insertUser(t, "sort-"+tag+"-none", "x", "", now)

	cases := map[string][]string{
		"desc": {"sort-" + tag + "-1", "sort-" + tag + "-2", "sort-" + tag + "-0", "sort-" + tag + "-none"},
		"asc":  {"sort-" + tag + "-0", "sort-" + tag + "-2", "sort-" + tag + "-1", "sort-" + tag + "-none"},
	}
	for order, want := range cases {
		got := usernames(listLearners(t, srv, instructor, url.Values{"q": {"sort-" + tag}, "sort": {"progress"}, "order": {order}}))
		if len(got) != len(want) {
			t.Fatalf("order=%s: learners = %v, want %v", order, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("order=%s: learners = %v, want %v", order, got, want)
				break
			}
		}
	}
}

// Learner analytics expose other people's activity: learners must not read them.
func TestAnalyticsLearnersRequiresInstructor(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	learner := loginAs(t, model.RoleLearner)
	if resp := doRequest(t, srv, "GET", "/api/analytics/learners", nil, learner); resp.StatusCode != http.StatusForbidden {
		t.Errorf("learner: status = %d, want 403", resp.StatusCode)
	}
}

func TestActivityDays(t *testing.T) {
	now := time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)
	day := func(offset int) time.Time { return time.Date(2026, 3, 10+offset, 0, 0, 0, 0, time.UTC) }

	cases := []struct {
		name           string
		days           []time.Time
		last30, streak int
	}{
		{"no activity", nil, 0, 0},
		{"streak ending today", []time.Time{day(0), day(-1), day(-2), day(-4)}, 4, 3},
		// Not having come back yet today must not reset the streak.
		{"streak ending yesterday", []time.Time{day(-1), day(-2)}, 2, 2},
		{"streak broken", []time.Time{day(-2), day(-3)}, 2, 0},
		// Day -29 is the 30th day of the window, day -30 is outside it.
		{"30-day window bounds", []time.Time{day(-29), day(-30)}, 1, 0},
	}
	for _, c := range cases {
		last30, streak := activityDays(c.days, now)
		if last30 != c.last30 || streak != c.streak {
			t.Errorf("%s: got last30=%d streak=%d, want %d and %d", c.name, last30, streak, c.last30, c.streak)
		}
	}
}
