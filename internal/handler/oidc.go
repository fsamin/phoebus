package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func (h *Handler) oidcProvider(ctx context.Context) (*oidc.Provider, *oauth2.Config, error) {
	provider, err := oidc.NewProvider(ctx, h.cfg.Auth.OIDC.IssuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc provider: %w", err)
	}

	scopes := h.cfg.Auth.OIDC.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     h.cfg.Auth.OIDC.ClientID,
		ClientSecret: h.cfg.Auth.OIDC.ClientSecret,
		RedirectURL:  h.cfg.Auth.OIDC.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return provider, oauth2Cfg, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// OIDCRedirect initiates the OIDC authorization code flow.
func (h *Handler) OIDCRedirect(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.OIDC.Enabled {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "OIDC is not enabled"})
		return
	}

	_, oauth2Cfg, err := h.oidcProvider(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to initialize OIDC provider"})
		return
	}

	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_oidc_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})

	http.Redirect(w, r, oauth2Cfg.AuthCodeURL(state), http.StatusFound)
}

// OIDCCallback handles the OIDC callback with the authorization code.
func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.OIDC.Enabled {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "OIDC is not enabled"})
		return
	}

	// Validate state
	stateCookie, err := r.Cookie("phoebus_oidc_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid state parameter"})
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_oidc_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Check for error from provider
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("OIDC error: %s — %s", errParam, desc)})
		return
	}

	provider, oauth2Cfg, err := h.oidcProvider(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to initialize OIDC provider"})
		return
	}

	// Exchange code for tokens
	code := r.URL.Query().Get("code")
	token, err := oauth2Cfg.Exchange(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "failed to exchange authorization code"})
		return
	}

	// Extract and verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no id_token in response"})
		return
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: h.cfg.Auth.OIDC.ClientID})
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "failed to verify ID token"})
		return
	}

	// Extract claims
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse claims"})
		return
	}

	mapping := h.cfg.Auth.OIDC.ClaimMapping
	externalID := claimString(claims, mapping.ExternalID)
	email := claimString(claims, mapping.Email)
	displayName := claimString(claims, mapping.DisplayName)

	if externalID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing external_id claim"})
		return
	}

	// Upsert user: match by external_id+oidc, then by email
	user, err := h.upsertOIDCUser(r.Context(), externalID, email, displayName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create/update user"})
		return
	}

	if !user.Active {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "user account is inactive"})
		return
	}

	// Generate session token
	sessionToken, err := auth.GenerateToken(user, h.cfg.JWT.Secret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate session"})
		return
	}

	h.db.ExecContext(r.Context(), "UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})

	// Redirect to SPA
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) upsertOIDCUser(ctx context.Context, externalID, email, displayName string) (*model.User, error) {
	var user model.User

	// Try to find by external_id + oidc
	err := h.db.GetContext(ctx, &user, `
		SELECT id, username, email, display_name, role, password_hash, external_id, auth_provider, active, last_login_at, created_at, updated_at
		FROM users WHERE external_id = $1 AND auth_provider = 'oidc'
	`, externalID)
	if err == nil {
		// Update display name and email
		h.db.ExecContext(ctx, `
			UPDATE users SET display_name = $1, email = $2, updated_at = now() WHERE id = $3
		`, displayName, email, user.ID)
		user.DisplayName = displayName
		user.Email = &email
		return &user, nil
	}

	// Try to find by email (link existing account)
	if email != "" {
		err = h.db.GetContext(ctx, &user, `
			SELECT id, username, email, display_name, role, password_hash, external_id, auth_provider, active, last_login_at, created_at, updated_at
			FROM users WHERE email = $1 AND auth_provider = 'oidc'
		`, email)
		if err == nil {
			h.db.ExecContext(ctx, `
				UPDATE users SET external_id = $1, display_name = $2, updated_at = now() WHERE id = $3
			`, externalID, displayName, user.ID)
			return &user, nil
		}
	}

	// Create new user
	username := externalID
	if email != "" {
		username = strings.Split(email, "@")[0]
	}

	newUser := &model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        &email,
		DisplayName:  displayName,
		Role:         model.RoleLearner,
		ExternalID:   &externalID,
		AuthProvider: "oidc",
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = h.db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, display_name, role, external_id, auth_provider, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, newUser.ID, newUser.Username, newUser.Email, newUser.DisplayName, newUser.Role, newUser.ExternalID, newUser.AuthProvider, newUser.Active, newUser.CreatedAt, newUser.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert oidc user: %w", err)
	}

	return newUser, nil
}

func claimString(claims map[string]interface{}, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
