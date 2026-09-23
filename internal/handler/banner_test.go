package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fsamin/phoebus/internal/model"
)

// clearBanner puts the instance back in its never-configured state.
// instance_settings is instance-wide and the package shares one database, so
// every test here sets up the state it expects rather than assuming a clean row.
func clearBanner(t *testing.T) {
	t.Helper()
	testDB.MustExec(`DELETE FROM instance_settings WHERE key = 'dashboard_banner'`)
}

func setBanner(t *testing.T, enabled bool, level, message string) {
	t.Helper()
	value, err := json.Marshal(map[string]any{"enabled": enabled, "level": level, "message_md": message})
	if err != nil {
		t.Fatalf("marshal banner: %v", err)
	}
	testDB.MustExec(`
		INSERT INTO instance_settings (key, value, encrypted) VALUES ('dashboard_banner', $1, false)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, string(value))
}

// The banner is instance-wide configuration displayed to everyone: a learner
// able to write it could deface every dashboard on the platform.
func TestBannerRequiresAdmin(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	body := map[string]any{"enabled": false, "level": "info", "message_md": ""}

	if resp := doRequest(t, srv, "PUT", "/api/admin/banner", body, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous PUT: status = %d, want 401", resp.StatusCode)
	}

	learner := loginAs(t, model.RoleLearner)
	if resp := doRequest(t, srv, "GET", "/api/admin/banner", nil, learner); resp.StatusCode != http.StatusForbidden {
		t.Errorf("learner GET: status = %d, want 403", resp.StatusCode)
	}
	if resp := doRequest(t, srv, "PUT", "/api/admin/banner", body, learner); resp.StatusCode != http.StatusForbidden {
		t.Errorf("learner PUT: status = %d, want 403", resp.StatusCode)
	}

	admin := loginAs(t, model.RoleAdmin)
	if resp := doRequest(t, srv, "GET", "/api/admin/banner", nil, admin); resp.StatusCode != http.StatusOK {
		t.Errorf("admin GET: status = %d, want 200", resp.StatusCode)
	}
}

// A fresh instance has no row at all. That is the normal initial state, so the
// admin form must load, not fail.
func TestBannerDefaultsWhenUnset(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	clearBanner(t)

	resp := doRequest(t, srv, "GET", "/api/admin/banner", nil, loginAs(t, model.RoleAdmin))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readJSON(t, resp)
	if data["enabled"] != false {
		t.Errorf("enabled = %v, want false", data["enabled"])
	}
	if data["level"] != "info" {
		t.Errorf("level = %v, want info (the form's Select must never be empty)", data["level"])
	}
	if data["message_md"] != "" {
		t.Errorf("message_md = %q, want empty", data["message_md"])
	}
}

// What an admin saves is what they read back, Markdown characters included —
// and saving twice replaces the banner instead of stacking rows.
func TestBannerRoundTrip(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	clearBanner(t)
	admin := loginAs(t, model.RoleAdmin)

	const msg = "**Maintenance** on [Sunday](https://example.com)"
	resp := doRequest(t, srv, "PUT", "/api/admin/banner",
		map[string]any{"enabled": true, "level": "warning", "message_md": msg}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: status = %d, want 200", resp.StatusCode)
	}

	data := readJSON(t, doRequest(t, srv, "GET", "/api/admin/banner", nil, admin))
	if data["enabled"] != true || data["level"] != "warning" || data["message_md"] != msg {
		t.Errorf("read back %v, want enabled=true level=warning message=%q", data, msg)
	}

	resp = doRequest(t, srv, "PUT", "/api/admin/banner",
		map[string]any{"enabled": true, "level": "danger", "message_md": "second"}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second PUT: status = %d, want 200", resp.StatusCode)
	}
	data = readJSON(t, doRequest(t, srv, "GET", "/api/admin/banner", nil, admin))
	if data["level"] != "danger" || data["message_md"] != "second" {
		t.Errorf("after second save: %v, want level=danger message=second", data)
	}

	var rows int
	if err := testDB.Get(&rows, `SELECT COUNT(*) FROM instance_settings WHERE key = 'dashboard_banner'`); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 1 {
		t.Errorf("instance_settings holds %d banner rows, want 1 — saving must upsert, not insert", rows)
	}

	// audit_log.resource_id is a UUID column and auditLog swallows insert
	// failures, so a non-UUID resource id would drop the entry unnoticed and the
	// change would look audited without being so.
	var audited int
	if err := testDB.Get(&audited, `
		SELECT COUNT(*) FROM audit_log
		WHERE resource_type = 'instance_setting' AND action = 'update' AND metadata->>'key' = 'dashboard_banner'
	`); err != nil {
		t.Fatalf("count audit entries: %v", err)
	}
	if audited < 2 {
		t.Errorf("audit_log holds %d banner entries, want one per save (2)", audited)
	}
}

// Bad input must come back as an actionable 400, never as a 500 from the
// database or as a silently ignored setting.
func TestBannerValidation(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	admin := loginAs(t, model.RoleAdmin)

	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown level", map[string]any{"enabled": true, "level": "critical", "message_md": "x"}, http.StatusBadRequest},
		{"enabled with blank message", map[string]any{"enabled": true, "level": "info", "message_md": "   \n  "}, http.StatusBadRequest},
		{"message too long", map[string]any{"enabled": true, "level": "info", "message_md": strings.Repeat("a", maxBannerLength+1)}, http.StatusBadRequest},
		{"disabling with empty message clears the banner", map[string]any{"enabled": false, "level": "info", "message_md": ""}, http.StatusOK},
		{"missing level defaults to info", map[string]any{"enabled": true, "message_md": "hello"}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearBanner(t)
			resp := doRequest(t, srv, "PUT", "/api/admin/banner", tt.body, admin)
			if resp.StatusCode != tt.want {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d (body: %s)", resp.StatusCode, tt.want, body)
			}
		})
	}
}

// The banner must reach learners when it is on, and its text must never leave
// the server when it is off — an unpublished banner is a draft.
func TestDashboardExposesBanner(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()
	learner := loginAs(t, model.RoleLearner)

	const secret = "Scheduled outage nobody should read yet"

	t.Run("enabled banner reaches the learner", func(t *testing.T) {
		setBanner(t, true, "warning", secret)
		data := readJSON(t, doRequest(t, srv, "GET", "/api/me/dashboard", nil, learner))
		banner, ok := data["banner"].(map[string]any)
		if !ok {
			t.Fatalf("banner = %v, want an object", data["banner"])
		}
		if banner["level"] != "warning" || banner["message_md"] != secret {
			t.Errorf("banner = %v, want level=warning and the message", banner)
		}
		if _, leaked := banner["enabled"]; leaked {
			t.Error("dashboard payload should not carry the enabled flag, it is always true there")
		}
	})

	t.Run("disabled banner leaks nothing", func(t *testing.T) {
		setBanner(t, false, "warning", secret)
		resp := doRequest(t, srv, "GET", "/api/me/dashboard", nil, learner)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		resp.Body.Close()
		if strings.Contains(string(body), secret) {
			t.Errorf("a disabled banner's draft text reached the learner: %s", body)
		}
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data["banner"] != nil {
			t.Errorf("banner = %v, want null", data["banner"])
		}
	})

	// Written straight to the table, bypassing the handler's validation: the
	// dashboard defends itself instead of trusting whoever wrote the row.
	t.Run("enabled but empty renders nothing", func(t *testing.T) {
		setBanner(t, true, "info", "")
		data := readJSON(t, doRequest(t, srv, "GET", "/api/me/dashboard", nil, learner))
		if data["banner"] != nil {
			t.Errorf("banner = %v, want null", data["banner"])
		}
	})

	t.Run("never configured renders nothing", func(t *testing.T) {
		clearBanner(t)
		data := readJSON(t, doRequest(t, srv, "GET", "/api/me/dashboard", nil, learner))
		if data["banner"] != nil {
			t.Errorf("banner = %v, want null", data["banner"])
		}
	})
}
