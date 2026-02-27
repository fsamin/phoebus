package handler

import (
	"bytes"
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
	h.RegisterRoutes(r)
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
