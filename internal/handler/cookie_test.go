package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The session cookie carries the JWT. Without Secure it is also sent over plain
// HTTP, so any HTTPS deployment must get the flag — including behind a
// TLS-terminating proxy, which is how this is normally run.
func TestSessionCookieSecureFlag(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*http.Request)
		wantSecure bool
	}{
		{"plain http", func(*http.Request) {}, false},
		{"direct TLS", func(r *http.Request) { r.TLS = &tls.ConnectionState{} }, true},
		{"behind a TLS proxy", func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") }, true},
		{"proxy reports http", func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http") }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/auth/login", nil)
			tt.setup(r)
			w := httptest.NewRecorder()

			setSessionCookie(w, r, "a-token")

			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("got %d cookies, want 1", len(cookies))
			}
			c := cookies[0]
			if c.Secure != tt.wantSecure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
			if !c.HttpOnly {
				t.Error("HttpOnly must stay set: the JWT is never read from JS")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", c.SameSite)
			}
		})
	}
}

// Logging out must clear the cookie under the same attributes, otherwise the
// browser keeps the original one alongside the deletion.
func TestClearSessionCookieMatchesAttributes(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/auth/logout", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	clearSessionCookie(w, r)

	c := w.Result().Cookies()[0]
	if c.Value != "" || c.MaxAge != -1 {
		t.Errorf("cookie not cleared: value=%q maxAge=%d", c.Value, c.MaxAge)
	}
	if !c.Secure || !c.HttpOnly {
		t.Errorf("cleared cookie must keep Secure/HttpOnly, got secure=%v httpOnly=%v", c.Secure, c.HttpOnly)
	}
}
