package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
)

func setupProxyTest(t *testing.T, proxyCfg config.ProxyAuthConfig) (*httptest.Server, func()) {
	t.Helper()
	cfg := &config.Config{
		JWT:           config.JWTConfig{Secret: "test-jwt-secret-32chars-long!!!"},
		Auth:          config.AuthConfig{LocalEnabled: true, ProxyAuth: proxyCfg},
		Admin:         config.AdminConfig{Username: "admin", Password: "admin"},
		EncryptionKey: "01234567890123456789012345678901",
	}
	h := New(testDB, cfg, nil, "ssh-ed25519 AAAA-test-key phoebus-instance", nil)
	r := chi.NewRouter()
	h.RegisterRoutes(context.Background(), r)
	srv := httptest.NewServer(r)
	return srv, func() { srv.Close() }
}

func TestProxyAuth_Disabled(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Request with proxy header but proxy auth disabled → should be unauthorized
	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxyuser")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProxyAuth_NewUser(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:      true,
		HeaderUser:   "X-Remote-User",
		HeaderGroups: "X-Remote-Groups",
		DefaultRole:  "learner",
	})
	defer cleanup()

	// Request with proxy header → should auto-create user and return /api/me
	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-new-user-1")
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Should have set a session cookie
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "phoebus_session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected phoebus_session cookie to be set")
	}

	data := readJSON(t, resp)
	if data["username"] != "proxy-new-user-1" {
		t.Errorf("expected username proxy-new-user-1, got %v", data["username"])
	}
	if data["role"] != string(model.RoleLearner) {
		t.Errorf("expected role learner, got %v", data["role"])
	}
}

func TestProxyAuth_GroupToRole_Instructor(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:      true,
		HeaderUser:   "X-Remote-User",
		HeaderGroups: "X-Remote-Groups",
		DefaultRole:  "learner",
		GroupToRole: map[string]string{
			"trainers":   "instructor",
			"admin-team": "admin",
		},
	})
	defer cleanup()

	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-instructor-1")
	req.Header.Set("X-Remote-Groups", "developers, trainers")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data := readJSON(t, resp)
	if data["role"] != string(model.RoleInstructor) {
		t.Errorf("expected role instructor, got %v", data["role"])
	}
}

func TestProxyAuth_GroupToRole_Admin(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:      true,
		HeaderUser:   "X-Remote-User",
		HeaderGroups: "X-Remote-Groups",
		DefaultRole:  "learner",
		GroupToRole: map[string]string{
			"trainers":   "instructor",
			"admin-team": "admin",
		},
	})
	defer cleanup()

	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-admin-1")
	req.Header.Set("X-Remote-Groups", "trainers, admin-team")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data := readJSON(t, resp)
	if data["role"] != string(model.RoleAdmin) {
		t.Errorf("expected role admin, got %v", data["role"])
	}
}

func TestProxyAuth_CustomHeaders(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:           true,
		HeaderUser:        "X-Forwarded-User",
		HeaderGroups:      "X-Forwarded-Groups",
		HeaderEmail:       "X-Forwarded-Email",
		HeaderDisplayName: "X-Forwarded-Name",
		DefaultRole:       "learner",
	})
	defer cleanup()

	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Forwarded-User", "proxy-custom-hdr-1")
	req.Header.Set("X-Forwarded-Email", "custom@example.com")
	req.Header.Set("X-Forwarded-Name", "Custom User")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	data := readJSON(t, resp)
	if data["display_name"] != "Custom User" {
		t.Errorf("expected display_name 'Custom User', got %v", data["display_name"])
	}
	if data["email"] != "custom@example.com" {
		t.Errorf("expected email 'custom@example.com', got %v", data["email"])
	}
}

func TestProxyAuth_ExistingCookie(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:     true,
		HeaderUser:  "X-Remote-User",
		DefaultRole: "learner",
	})
	defer cleanup()

	// First: create user via proxy header
	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-cookie-test-1")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "phoebus_session" {
			sessionCookie = c
			break
		}
	}
	resp.Body.Close()

	if sessionCookie == nil {
		t.Fatal("expected phoebus_session cookie from proxy auth")
	}

	// Second: request with cookie (no proxy header) should still work
	req, _ = http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with cookie, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestProxyAuth_Providers(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:     true,
		HeaderUser:  "X-Remote-User",
		DefaultRole: "learner",
	})
	defer cleanup()

	resp := doRequest(t, srv, "GET", "/api/auth/providers", nil, nil)
	data := readJSON(t, resp)
	if data["proxy"] != true {
		t.Errorf("expected proxy=true, got %v", data["proxy"])
	}
}

func TestProxyAuth_PreservesAdminAssignedRole(t *testing.T) {
	srv, cleanup := setupProxyTest(t, config.ProxyAuthConfig{
		Enabled:      true,
		HeaderUser:   "X-Remote-User",
		HeaderGroups: "X-Remote-Groups",
		DefaultRole:  "learner",
		GroupToRole: map[string]string{
			"trainers": "instructor",
		},
	})
	defer cleanup()

	// Step 1: Create user via proxy (gets "learner" role)
	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-role-preserve-1")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data := readJSON(t, resp)
	if data["role"] != "learner" {
		t.Fatalf("expected initial role learner, got %v", data["role"])
	}

	// Step 2: Admin changes user role to "instructor" directly in DB
	_, err = testDB.Exec(`UPDATE users SET role = 'instructor' WHERE username = 'proxy-role-preserve-1'`)
	if err != nil {
		t.Fatal(err)
	}

	// Step 3: User logs in again via proxy (still no group header) — role should stay "instructor"
	req, _ = http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.Header.Set("X-Remote-User", "proxy-role-preserve-1")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data = readJSON(t, resp)
	if data["role"] != "instructor" {
		t.Errorf("expected preserved role instructor, got %v", data["role"])
	}
}
