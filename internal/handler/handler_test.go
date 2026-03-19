package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/database"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	testDB  *sqlx.DB
	testCfg *config.Config
)

func TestMain(m *testing.M) {
	var err error
	var containerID string

	dsn := os.Getenv("PHOEBUS_TEST_DB")
	if dsn == "" {
		// No external DB provided — start an ephemeral PostgreSQL container.
		containerName := fmt.Sprintf("phoebus-test-pg-%d", os.Getpid())
		port := 15432 + (os.Getpid() % 1000)

		cmd := exec.Command("docker", "run", "-d",
			"--name", containerName,
			"-e", "POSTGRES_USER=test",
			"-e", "POSTGRES_PASSWORD=test",
			"-e", "POSTGRES_DB=phoebus_test",
			"-p", fmt.Sprintf("%d:5432", port),
			"postgres:16-alpine",
		)
		out, cErr := cmd.CombinedOutput()
		if cErr != nil {
			fmt.Fprintf(os.Stderr, "failed to start postgres: %v\n%s\n", cErr, out)
			os.Exit(1)
		}
		containerID = strings.TrimSpace(string(out))
		dsn = fmt.Sprintf("postgres://test:test@localhost:%d/phoebus_test?sslmode=disable", port)
	}
	if containerID != "" {
		defer exec.Command("docker", "rm", "-f", containerID).Run()
	}

	// Wait for PostgreSQL
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		testDB, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			if err = testDB.Ping(); err == nil {
				break
			}
			testDB.Close()
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres not ready: %v\n", err)
		os.Exit(1)
	}

	if err := database.Migrate(testDB); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	testCfg = &config.Config{
		JWT:           config.JWTConfig{Secret: "test-jwt-secret-32chars-long!!!"},
		Auth:          config.AuthConfig{LocalEnabled: true},
		Admin:         config.AdminConfig{Username: "admin", Password: "admin"},
		EncryptionKey: "01234567890123456789012345678901",
	}

	os.Exit(m.Run())
}

func setupTest(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	h := New(testDB, testCfg, nil, "ssh-ed25519 AAAA-test-key phoebus-instance", nil)
	r := chi.NewRouter()
	h.RegisterRoutes(context.Background(), r)
	srv := httptest.NewServer(r)

	return srv, func() { srv.Close() }
}

// loginAs creates a user with the given role and returns a session cookie.
func loginAs(t *testing.T, role model.Role) *http.Cookie {
	t.Helper()
	id := uuid.New()
	username := fmt.Sprintf("user-%s-%s", role, id.String()[:8])
	hash, _ := auth.HashPassword("password")

	_, err := testDB.Exec(`
		INSERT INTO users (id, username, display_name, password_hash, role, auth_provider, active)
		VALUES ($1, $2, $3, $4, $5, 'local', true)
		ON CONFLICT (username) DO NOTHING
	`, id, username, username, hash, role)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	user := &model.User{ID: id, Username: username, Role: role}
	token, _ := auth.GenerateToken(user, testCfg.JWT.Secret)
	return &http.Cookie{Name: "phoebus_session", Value: token}
}

func doRequest(t *testing.T, srv *httptest.Server, method, path string, body any, cookie *http.Cookie) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, srv.URL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	return data
}

// --- Health ---

func TestHealthEndpoint(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	resp := doRequest(t, srv, "GET", "/api/health", nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readJSON(t, resp)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want ok", data["status"])
	}
}

// --- Login ---

func TestLoginValid(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Create a user
	hash, _ := auth.HashPassword("testpass")
	testDB.Exec(`INSERT INTO users (id, username, display_name, password_hash, role, auth_provider, active)
		VALUES ($1, 'loginuser', 'Login User', $2, 'learner', 'local', true)
		ON CONFLICT (username) DO NOTHING`, uuid.New(), hash)

	resp := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
		"username": "loginuser",
		"password": "testpass",
	}, nil)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	// Check cookie is set
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "phoebus_session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected phoebus_session cookie")
	}
}

func TestLoginInvalid(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	resp := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
		"username": "nonexistent",
		"password": "wrong",
	}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// --- Register ---

func TestRegisterSuccess(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	username := fmt.Sprintf("newuser-%s", uuid.New().String()[:8])
	resp := doRequest(t, srv, "POST", "/api/auth/register", map[string]string{
		"username":     username,
		"display_name": "New User",
		"password":     "securepass123",
	}, nil)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Register: status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	username := fmt.Sprintf("dupuser-%s", uuid.New().String()[:8])
	// First registration
	doRequest(t, srv, "POST", "/api/auth/register", map[string]string{
		"username": username, "display_name": "Dup", "password": "pass123",
	}, nil)
	// Second registration — should fail
	resp := doRequest(t, srv, "POST", "/api/auth/register", map[string]string{
		"username": username, "display_name": "Dup", "password": "pass123",
	}, nil)
	if resp.StatusCode == 200 {
		t.Fatal("expected error for duplicate username")
	}
}

// --- Auth Middleware ---

func TestAuthMiddlewareNoToken(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	resp := doRequest(t, srv, "GET", "/api/me", nil, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/me", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// --- RBAC ---

func TestRBACLearnerAccessLearningPaths(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/learning-paths", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("learner → /api/learning-paths: status = %d, want 200", resp.StatusCode)
	}
}

func TestRBACLearnerDeniedAnalytics(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/analytics/overview", nil, cookie)
	if resp.StatusCode != 403 {
		t.Fatalf("learner → /api/analytics/overview: status = %d, want 403", resp.StatusCode)
	}
}

func TestRBACLearnerDeniedAdmin(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/admin/repos", nil, cookie)
	if resp.StatusCode != 403 {
		t.Fatalf("learner → /api/admin/repos: status = %d, want 403", resp.StatusCode)
	}
}

func TestRBACInstructorAccessAnalytics(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleInstructor)
	resp := doRequest(t, srv, "GET", "/api/analytics/overview", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("instructor → /api/analytics/overview: status = %d, want 200", resp.StatusCode)
	}
}

func TestRBACInstructorDeniedAdmin(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleInstructor)
	resp := doRequest(t, srv, "GET", "/api/admin/repos", nil, cookie)
	if resp.StatusCode != 403 {
		t.Fatalf("instructor → /api/admin/repos: status = %d, want 403", resp.StatusCode)
	}
}

func TestRBACAdminAccessAnalytics(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleAdmin)
	resp := doRequest(t, srv, "GET", "/api/analytics/overview", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("admin → /api/analytics/overview: status = %d, want 200", resp.StatusCode)
	}
}

func TestRBACAdminAccessRepos(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleAdmin)
	resp := doRequest(t, srv, "GET", "/api/admin/repos", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("admin → /api/admin/repos: status = %d, want 200", resp.StatusCode)
	}
}

// --- CRUD Repos ---

func TestCRUDRepos(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleAdmin)

	// Create
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "https://github.com/test/repo.git",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Create: status = %d, body = %s", resp.StatusCode, body)
	}
	created := readJSON(t, resp)
	repoID := created["id"].(string)

	// List
	resp = doRequest(t, srv, "GET", "/api/admin/repos", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("List: status = %d", resp.StatusCode)
	}

	// Get
	resp = doRequest(t, srv, "GET", "/api/admin/repos/"+repoID, nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("Get: status = %d", resp.StatusCode)
	}

	// Update
	resp = doRequest(t, srv, "PUT", "/api/admin/repos/"+repoID, map[string]string{
		"clone_url": "https://github.com/test/updated.git",
		"branch":    "develop",
		"auth_type": "none",
	}, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Update: status = %d, body = %s", resp.StatusCode, body)
	}

	// Delete
	resp = doRequest(t, srv, "DELETE", "/api/admin/repos/"+repoID, nil, cookie)
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		t.Fatalf("Delete: status = %d", resp.StatusCode)
	}
}

// --- SyncLogs ---

func TestSyncLogs(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleAdmin)

	// Create a repo first
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "https://github.com/test/sync-logs.git",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	created := readJSON(t, resp)
	repoID := created["id"].(string)

	// Insert a sync job manually
	testDB.Exec(`INSERT INTO sync_jobs (id, repo_id, status, created_at, updated_at)
		VALUES ($1, $2, 'done', now(), now())`, uuid.New(), repoID)

	resp = doRequest(t, srv, "GET", fmt.Sprintf("/api/admin/repos/%s/sync-logs", repoID), nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("SyncLogs: status = %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var logs []any
	json.NewDecoder(resp.Body).Decode(&logs)
	if len(logs) < 1 {
		t.Error("expected at least 1 sync log")
	}
}

// --- Webhook ---

func TestWebhookValid(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Create a repo and get its webhook UUID
	cookie := loginAs(t, model.RoleAdmin)
	resp := doRequest(t, srv, "POST", "/api/admin/repos", map[string]string{
		"clone_url": "https://github.com/test/webhook.git",
		"branch":    "main",
		"auth_type": "none",
	}, cookie)
	created := readJSON(t, resp)
	webhookUUID := created["webhook_uuid"].(string)

	// POST to webhook (no auth needed)
	resp = doRequest(t, srv, "POST", "/api/webhooks/"+webhookUUID, nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("Webhook: status = %d", resp.StatusCode)
	}
}

func TestWebhookUnknownUUID(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	resp := doRequest(t, srv, "POST", "/api/webhooks/"+uuid.New().String(), nil, nil)
	if resp.StatusCode != 404 {
		t.Fatalf("Webhook unknown: status = %d, want 404", resp.StatusCode)
	}
}

// --- Dashboard ---

func TestDashboard(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/me/dashboard", nil, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Dashboard: status = %d, body = %s", resp.StatusCode, body)
	}
}

// --- SSH Public Key ---

func TestSSHPublicKeyEndpoint(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Unauthenticated → 401
	resp := doRequest(t, srv, "GET", "/api/admin/ssh-public-key", nil, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauthenticated: status = %d, want 401", resp.StatusCode)
	}

	// Learner → 403
	learnerCookie := loginAs(t, model.RoleLearner)
	resp = doRequest(t, srv, "GET", "/api/admin/ssh-public-key", nil, learnerCookie)
	if resp.StatusCode != 403 {
		t.Fatalf("learner: status = %d, want 403", resp.StatusCode)
	}

	// Admin → 200 with key
	adminCookie := loginAs(t, model.RoleAdmin)
	resp = doRequest(t, srv, "GET", "/api/admin/ssh-public-key", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("admin: status = %d, want 200", resp.StatusCode)
	}
	data := readJSON(t, resp)
	key, ok := data["public_key"].(string)
	if !ok || key == "" {
		t.Error("expected non-empty public_key in response")
	}
	if !strings.HasPrefix(key, "ssh-ed25519 ") {
		t.Errorf("public_key should start with ssh-ed25519, got: %s", key[:20])
	}
}

// --- Competencies & Prerequisites ---

// seedContentForCompetencyTests creates two learning paths with modules and competencies.
// Path A ("Linux Fundamentals") provides competencies: linux-cli, linux-fs
// Path B ("Docker Fundamentals") provides competencies: docker-basics, and has prerequisite: linux-cli
// Returns (pathA_id, pathB_id, repoID)
func seedContentForCompetencyTests(t *testing.T) (string, string, string) {
	t.Helper()
	repoID := uuid.New()
	pathAID := uuid.New()
	pathBID := uuid.New()
	modAID := uuid.New()
	modBID := uuid.New()
	stepA1 := uuid.New()
	stepA2 := uuid.New()
	stepB1 := uuid.New()
	suffix := pathAID.String()[:8]

	testDB.MustExec(`INSERT INTO git_repositories (id, clone_url, branch, auth_type, webhook_uuid, sync_status, created_at, updated_at)
		VALUES ($1, 'https://github.com/test/comp.git', 'main', 'none', $2, 'synced', now(), now())`, repoID, uuid.New())

	testDB.MustExec(`INSERT INTO learning_paths (id, repo_id, title, description, tags, prerequisites, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Linux Fundamentals', 'Learn Linux', '{linux,cli}', '{}', 'linux/', 'linux-fundamentals-' || $3, now(), now())`, pathAID, repoID, suffix)
	testDB.MustExec(`INSERT INTO learning_paths (id, repo_id, title, description, tags, prerequisites, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Docker Fundamentals', 'Learn Docker', '{docker,containers}', '{linux-cli}', 'docker/', 'docker-fundamentals-' || $3, now(), now())`, pathBID, repoID, suffix)

	testDB.MustExec(`INSERT INTO modules (id, learning_path_id, title, description, competencies, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'CLI Basics', 'Learn the CLI', '{linux-cli,linux-fs}', 0, 'cli/', 'cli-basics-' || $3, now(), now())`, modAID, pathAID, suffix)
	testDB.MustExec(`INSERT INTO modules (id, learning_path_id, title, description, competencies, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Docker Basics', 'Learn Docker', '{docker-basics}', 0, 'docker/', 'docker-basics-' || $3, now(), now())`, modBID, pathBID, suffix)

	testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, content_md, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Intro to CLI', 'lesson', '# CLI', 0, 'cli/01.md', 'intro-to-cli-' || $3, now(), now())`, stepA1, modAID, suffix)
	testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, content_md, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'File System', 'lesson', '# FS', 1, 'cli/02.md', 'file-system-' || $3, now(), now())`, stepA2, modAID, suffix)
	testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, content_md, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Docker Intro', 'lesson', '# Docker', 0, 'docker/01.md', 'docker-intro-' || $3, now(), now())`, stepB1, modBID, suffix)

	return pathAID.String(), pathBID.String(), repoID.String()
}

// --- Slug Resolution ---

// seedContentForSlugTests creates a learning path with a module and step, all with slugs.
// Returns (pathID, pathSlug, moduleID, moduleSlug, stepID, stepSlug, repoID).
func seedContentForSlugTests(t *testing.T) (string, string, string, string, string, string, string) {
	t.Helper()
	repoID := uuid.New()
	pathID := uuid.New()
	modID := uuid.New()
	stepID := uuid.New()
	suffix := pathID.String()[:8]

	pathSlug := "slug-test-path-" + suffix
	modSlug := "slug-test-module-" + suffix
	stepSlug := "slug-test-step-" + suffix

	testDB.MustExec(`INSERT INTO git_repositories (id, clone_url, branch, auth_type, webhook_uuid, sync_status, created_at, updated_at)
		VALUES ($1, 'https://github.com/test/slug-test.git', 'main', 'none', $2, 'synced', now(), now())`, repoID, uuid.New())

	testDB.MustExec(`INSERT INTO learning_paths (id, repo_id, title, description, tags, prerequisites, file_path, slug, enabled, created_at, updated_at)
		VALUES ($1, $2, 'Slug Test Path', 'Testing slug resolution', '{}', '{}', 'slug-test/', $3, true, now(), now())`, pathID, repoID, pathSlug)

	testDB.MustExec(`INSERT INTO modules (id, learning_path_id, title, description, competencies, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Slug Test Module', 'Module for slug tests', '{}', 0, 'slug-test/mod/', $3, now(), now())`, modID, pathID, modSlug)

	testDB.MustExec(`INSERT INTO steps (id, module_id, title, type, content_md, position, file_path, slug, created_at, updated_at)
		VALUES ($1, $2, 'Slug Test Step', 'lesson', '# Slug Test', 0, 'slug-test/mod/01.md', $3, now(), now())`, stepID, modID, stepSlug)

	return pathID.String(), pathSlug, modID.String(), modSlug, stepID.String(), stepSlug, repoID.String()
}

func TestGetLearningPathBySlug(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	_, pathSlug, _, _, _, _, _ := seedContentForSlugTests(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "GET", "/api/learning-paths/"+pathSlug, nil, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/learning-paths/%s: status = %d, body = %s", pathSlug, resp.StatusCode, body)
	}
	data := readJSON(t, resp)
	if data["slug"] != pathSlug {
		t.Errorf("slug = %v, want %s", data["slug"], pathSlug)
	}
	if data["title"] != "Slug Test Path" {
		t.Errorf("title = %v, want 'Slug Test Path'", data["title"])
	}
	// Should include modules
	mods, ok := data["modules"].([]any)
	if !ok || len(mods) == 0 {
		t.Error("expected at least 1 module in response")
	}
}

func TestGetLearningPathByUUID_Redirects(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	pathID, pathSlug, _, _, _, _, _ := seedContentForSlugTests(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "GET", "/api/learning-paths/"+pathID, nil, cookie)
	if resp.StatusCode != 301 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/learning-paths/%s: status = %d, want 301, body = %s", pathID, resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	expected := "/api/learning-paths/" + pathSlug
	if loc != expected {
		t.Errorf("Location = %q, want %q", loc, expected)
	}
}

func TestGetStepBySlug(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	_, pathSlug, _, _, _, stepSlug, _ := seedContentForSlugTests(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "GET", fmt.Sprintf("/api/learning-paths/%s/steps/%s", pathSlug, stepSlug), nil, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET step by slug: status = %d, body = %s", resp.StatusCode, body)
	}
	data := readJSON(t, resp)
	if data["slug"] != stepSlug {
		t.Errorf("step slug = %v, want %s", data["slug"], stepSlug)
	}
	if data["title"] != "Slug Test Step" {
		t.Errorf("title = %v, want 'Slug Test Step'", data["title"])
	}
}

func TestGetStepByUUID_Redirects(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	_, pathSlug, _, _, stepID, stepSlug, _ := seedContentForSlugTests(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "GET", fmt.Sprintf("/api/learning-paths/%s/steps/%s", pathSlug, stepID), nil, cookie)
	if resp.StatusCode != 301 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET step by UUID: status = %d, want 301, body = %s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	expected := fmt.Sprintf("/api/learning-paths/%s/steps/%s", pathSlug, stepSlug)
	if loc != expected {
		t.Errorf("Location = %q, want %q", loc, expected)
	}
}

func TestUpdateProgressWithSlug(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	_, _, _, _, _, stepSlug, _ := seedContentForSlugTests(t)
	cookie := loginAs(t, model.RoleLearner)

	resp := doRequest(t, srv, "POST", "/api/progress", map[string]string{
		"step_id": stepSlug,
		"status":  "in_progress",
	}, cookie)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("UpdateProgress with slug: status = %d, body = %s", resp.StatusCode, body)
	}
}

func TestGetLearningPathBySlug_NotFound(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/learning-paths/nonexistent-slug-xyz", nil, cookie)
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- Competencies & Prerequisites ---

func TestListCompetencies(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	seedContentForCompetencyTests(t)

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/competencies", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("ListCompetencies: status = %d, want 200", resp.StatusCode)
	}
	defer resp.Body.Close()

	var comps []map[string]any
	json.NewDecoder(resp.Body).Decode(&comps)

	if len(comps) < 3 {
		t.Fatalf("expected at least 3 competencies (linux-cli, linux-fs, docker-basics), got %d", len(comps))
	}

	// Check that each competency has name and learning_path_ids
	compNames := map[string]bool{}
	for _, c := range comps {
		name, _ := c["name"].(string)
		compNames[name] = true
		ids, _ := c["learning_path_ids"].([]any)
		if len(ids) == 0 {
			t.Errorf("competency %q has no learning_path_ids", name)
		}
	}
	for _, expected := range []string{"linux-cli", "linux-fs", "docker-basics"} {
		if !compNames[expected] {
			t.Errorf("expected competency %q not found", expected)
		}
	}
}

func TestListLearningPathsCompetenciesProvided(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	pathAID, pathBID, _ := seedContentForCompetencyTests(t)

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/learning-paths", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("ListLearningPaths: status = %d, want 200", resp.StatusCode)
	}
	defer resp.Body.Close()

	var paths []map[string]any
	json.NewDecoder(resp.Body).Decode(&paths)

	// Find our paths
	var pathA, pathB map[string]any
	for _, p := range paths {
		id, _ := p["id"].(string)
		if id == pathAID {
			pathA = p
		} else if id == pathBID {
			pathB = p
		}
	}

	if pathA == nil {
		t.Fatal("Linux Fundamentals path not found in response")
	}
	if pathB == nil {
		t.Fatal("Docker Fundamentals path not found in response")
	}

	// Path A should provide linux-cli, linux-fs
	compsA, _ := pathA["competencies_provided"].([]any)
	compsANames := map[string]bool{}
	for _, c := range compsA {
		compsANames[c.(string)] = true
	}
	if !compsANames["linux-cli"] || !compsANames["linux-fs"] {
		t.Errorf("Path A competencies_provided = %v, want linux-cli + linux-fs", compsA)
	}

	// Path B should provide docker-basics
	compsB, _ := pathB["competencies_provided"].([]any)
	if len(compsB) != 1 || compsB[0].(string) != "docker-basics" {
		t.Errorf("Path B competencies_provided = %v, want [docker-basics]", compsB)
	}
}

func TestPrerequisitesMetFalseWhenNotCompleted(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	_, pathBID, _ := seedContentForCompetencyTests(t)

	cookie := loginAs(t, model.RoleLearner)
	resp := doRequest(t, srv, "GET", "/api/learning-paths", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("ListLearningPaths: status = %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var paths []map[string]any
	json.NewDecoder(resp.Body).Decode(&paths)

	for _, p := range paths {
		if p["id"].(string) == pathBID {
			met, _ := p["prerequisites_met"].(bool)
			if met {
				t.Error("Docker path prerequisites_met should be false (linux-cli not completed)")
			}
			return
		}
	}
	t.Fatal("Docker path not found")
}

func TestPrerequisitesMetTrueAfterCompletion(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	pathAID, pathBID, _ := seedContentForCompetencyTests(t)

	cookie := loginAs(t, model.RoleLearner)

	// Get user ID from cookie claims
	claims, _ := auth.ValidateToken(cookie.Value, testCfg.JWT.Secret)
	userID := claims.UserID

	// Complete all steps in Path A (Linux Fundamentals)
	var stepIDs []string
	testDB.Select(&stepIDs, `
		SELECT s.id::text FROM steps s
		JOIN modules m ON m.id = s.module_id
		WHERE m.learning_path_id = $1 AND s.deleted_at IS NULL
	`, pathAID)

	for _, sid := range stepIDs {
		testDB.Exec(`INSERT INTO progress (id, user_id, step_id, status, completed_at, created_at, updated_at)
			VALUES ($1, $2, $3, 'completed', now(), now(), now())
			ON CONFLICT (user_id, step_id) DO UPDATE SET status = 'completed', completed_at = now()`,
			uuid.New(), userID, sid)
	}

	// Now check prerequisites_met for Docker path
	resp := doRequest(t, srv, "GET", "/api/learning-paths", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("ListLearningPaths: status = %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var paths []map[string]any
	json.NewDecoder(resp.Body).Decode(&paths)

	for _, p := range paths {
		if p["id"].(string) == pathBID {
			met, _ := p["prerequisites_met"].(bool)
			if !met {
				t.Error("Docker path prerequisites_met should be true after completing Linux path")
			}
			return
		}
	}
	t.Fatal("Docker path not found")
}
