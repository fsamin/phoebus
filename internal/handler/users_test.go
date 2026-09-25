package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/google/uuid"
)

type listUsersResponse struct {
	Users []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"users"`
	Total int `json:"total"`
}

// insertUser creates a user directly in the database. The users table is shared
// by the whole package, so callers embed a unique tag in the searchable fields
// and always search on it.
func insertUser(t *testing.T, username, displayName, email string, createdAt time.Time) string {
	t.Helper()
	id := uuid.New().String()
	testDB.MustExec(`
		INSERT INTO users (id, username, display_name, email, role, auth_provider, active, created_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), 'learner', 'local', true, $5)
	`, id, username, displayName, email, createdAt)
	return id
}

func listUsers(t *testing.T, srv *httptest.Server, admin *http.Cookie, q string, page, perPage int) listUsersResponse {
	t.Helper()
	path := "/api/admin/users?q=" + url.QueryEscape(q) + "&page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(perPage)
	resp := doRequest(t, srv, "GET", path, nil, admin)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200", path, resp.StatusCode)
	}
	var out listUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	return out
}

// An admin looking for someone must find them wherever they sit in the list,
// not only among the users displayed on the current page, and the pagination
// must describe the matching set.
func TestListUsersSearchCoversAllPages(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]

	// Oldest user, so it lands at the very end of the created_at DESC ordering.
	needle := insertUser(t, "needle-"+tag, "Needle", "", time.Now().AddDate(-1, 0, 0))
	for i := range 25 {
		insertUser(t, "filler-"+tag+"-"+strconv.Itoa(i), "Filler", "", time.Now())
	}

	got := listUsers(t, srv, admin, "needle-"+tag, 1, 20)
	if got.Total != 1 {
		t.Errorf("total = %d, want 1 (total must count matching users only)", got.Total)
	}
	if len(got.Users) != 1 || got.Users[0].ID != needle {
		t.Errorf("users = %+v, want only %s", got.Users, needle)
	}
}

// Admins search by whatever they know about a user: login, name or email, in
// any case.
func TestListUsersSearchFields(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]

	byUsername := insertUser(t, "Zeta"+tag, "Someone", "", time.Now())
	byDisplayName := insertUser(t, "u2-"+tag, "Display "+tag+" Name", "", time.Now())
	byEmail := insertUser(t, "u3-"+tag, "Someone", "e-"+tag+"@mail.test", time.Now())

	cases := []struct {
		name, q, want string
	}{
		{"username, case-insensitive", strings.ToUpper("zeta" + tag), byUsername},
		{"display name", "display " + tag, byDisplayName},
		{"email", "e-" + tag + "@mail", byEmail},
	}
	for _, c := range cases {
		got := listUsers(t, srv, admin, c.q, 1, 20)
		if got.Total != 1 || len(got.Users) != 1 || got.Users[0].ID != c.want {
			t.Errorf("%s: q=%q returned total=%d users=%+v, want only %s", c.name, c.q, got.Total, got.Users, c.want)
		}
	}
}

// LIKE wildcards typed in the search box are literal characters: searching
// for "%" must not return every user.
func TestListUsersSearchEscapesWildcards(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]
	insertUser(t, "pct-"+tag, "50% "+tag, "", time.Now())

	var want int
	testDB.Get(&want, `
		SELECT COUNT(*) FROM users
		WHERE strpos(username, '%') > 0 OR strpos(display_name, '%') > 0 OR strpos(COALESCE(email, ''), '%') > 0
	`)
	if got := listUsers(t, srv, admin, "%", 1, 100); got.Total != want {
		t.Errorf(`q="%%": total = %d, want %d (users literally containing "%%")`, got.Total, want)
	}
	if got := listUsers(t, srv, admin, "p_t-"+tag, 1, 100); got.Total != 0 {
		t.Errorf(`q="p_t-%s": total = %d, want 0 ("_" must not match any character)`, tag, got.Total)
	}
}

// Users provisioned in bulk share the same created_at. Walking the pages must
// show each of them exactly once.
func TestListUsersPaginationIsStable(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]

	createdAt := time.Now()
	want := map[string]bool{}
	for i := range 30 {
		want[insertUser(t, "bulk-"+tag+"-"+strconv.Itoa(i), "Bulk", "", createdAt)] = true
	}

	seen := map[string]int{}
	for page := 1; page <= 5; page++ {
		got := listUsers(t, srv, admin, "bulk-"+tag, page, 7)
		if got.Total != 30 {
			t.Fatalf("page %d: total = %d, want 30", page, got.Total)
		}
		for _, u := range got.Users {
			seen[u.ID]++
		}
	}
	for id := range want {
		if seen[id] != 1 {
			t.Errorf("user %s seen %d times across pages, want 1", id, seen[id])
		}
	}
}

// A path counts as completed only when every one of its steps is: finishing
// one step out of two must not show up as a completed path.
func TestListUsersCompletedPaths(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]
	now := time.Now()
	steps := createKPIPath(t, true, "lesson", "lesson")
	disabled := createKPIPath(t, false, "lesson")

	partial := insertUser(t, "cp-"+tag+"-partial", "x", "", now)
	addProgress(t, partial, steps[0], "completed", now)
	full := insertUser(t, "cp-"+tag+"-full", "x", "", now)
	addProgress(t, full, steps[0], "completed", now)
	addProgress(t, full, steps[1], "completed", now)
	// A disabled path is not offered to learners and must not count.
	addProgress(t, full, disabled[0], "completed", now)

	resp := doRequest(t, srv, "GET", "/api/admin/users?q=cp-"+tag, nil, admin)
	var body struct {
		Users []struct {
			ID             string `json:"id"`
			CompletedPaths int    `json:"completed_paths"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()
	want := map[string]int{partial: 0, full: 1}
	if len(body.Users) != 2 {
		t.Fatalf("users = %+v, want 2", body.Users)
	}
	for _, u := range body.Users {
		if u.CompletedPaths != want[u.ID] {
			t.Errorf("user %s: completed_paths = %d, want %d", u.ID, u.CompletedPaths, want[u.ID])
		}
	}
}

// Sorting by completed paths must rank across every page: the user with the
// most completed paths comes first even when created long before the others.
func TestListUsersSortByCompletedPaths(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)
	tag := uuid.New().String()[:8]
	now := time.Now()
	pathA := createKPIPath(t, true, "lesson")
	pathB := createKPIPath(t, true, "lesson")

	two := insertUser(t, "sortcp-"+tag+"-two", "x", "", now.AddDate(-1, 0, 0))
	addProgress(t, two, pathA[0], "completed", now)
	addProgress(t, two, pathB[0], "completed", now)
	one := insertUser(t, "sortcp-"+tag+"-one", "x", "", now.AddDate(0, -1, 0))
	addProgress(t, one, pathA[0], "completed", now)
	for i := range 3 {
		insertUser(t, "sortcp-"+tag+"-zero-"+strconv.Itoa(i), "x", "", now)
	}

	sorted := func(order string, page int) listUsersResponse {
		t.Helper()
		resp := doRequest(t, srv, "GET", "/api/admin/users?q=sortcp-"+tag+"&sort=completed_paths&order="+order+
			"&per_page=2&page="+strconv.Itoa(page), nil, admin)
		defer resp.Body.Close()
		var out listUsersResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	desc := sorted("desc", 1)
	if desc.Total != 5 || len(desc.Users) != 2 || desc.Users[0].ID != two || desc.Users[1].ID != one {
		t.Errorf("desc, first page of 2: total=%d users=%+v, want [two one]", desc.Total, desc.Users)
	}
	asc := sorted("asc", 3)
	if len(asc.Users) != 1 || asc.Users[0].ID != two {
		t.Errorf("asc, last page: users=%+v, want [two]", asc.Users)
	}
}
