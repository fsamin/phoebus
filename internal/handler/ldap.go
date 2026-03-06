package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/config"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

// LDAPLogin handles authentication against an LDAP server.
func (h *Handler) LDAPLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.LDAP.Enabled {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "LDAP authentication is not enabled"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	cfg := h.cfg.Auth.LDAP

	// Connect to LDAP server
	conn, err := ldap.DialURL(cfg.ServerURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to connect to LDAP server"})
		return
	}
	defer conn.Close()

	// Bind with service account to search for user (if configured)
	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "LDAP service bind failed"})
			return
		}
	}

	// Search for user
	filter := strings.ReplaceAll(cfg.UserSearchFilter, "{username}", ldap.EscapeFilter(req.Username))
	attrs := []string{"dn"}
	if cfg.AttributeMapping.DisplayName != "" {
		attrs = append(attrs, cfg.AttributeMapping.DisplayName)
	}
	if cfg.AttributeMapping.Email != "" {
		attrs = append(attrs, cfg.AttributeMapping.Email)
	}

	searchReq := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // size limit
		10, // time limit
		false,
		filter,
		attrs,
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil || len(sr.Entries) == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	userEntry := sr.Entries[0]
	userDN := userEntry.DN

	// Bind as the user to verify password
	if err := conn.Bind(userDN, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Extract attributes
	displayName := req.Username
	var email string
	if cfg.AttributeMapping.DisplayName != "" {
		if v := userEntry.GetAttributeValue(cfg.AttributeMapping.DisplayName); v != "" {
			displayName = v
		}
	}
	if cfg.AttributeMapping.Email != "" {
		email = userEntry.GetAttributeValue(cfg.AttributeMapping.Email)
	}

	// Resolve role from LDAP groups
	role := model.RoleLearner
	if len(cfg.GroupToRole) > 0 && cfg.GroupSearchBase != "" {
		role = h.resolveLDAPRole(conn, userDN, cfg)
	}

	// Override role for forced admins
	if h.cfg.IsForcedAdmin(req.Username) {
		role = model.RoleAdmin
	}

	// Upsert user
	user, err := h.upsertLDAPUser(r, req.Username, email, displayName, userDN, role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create/update user"})
		return
	}

	if !user.Active {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "user account is inactive"})
		return
	}

	token, err := auth.GenerateToken(user, h.cfg.JWT.Secret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	h.db.ExecContext(r.Context(), "UPDATE users SET last_login_at = now() WHERE id = $1", user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     "phoebus_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
		},
	})
}

func (h *Handler) resolveLDAPRole(conn *ldap.Conn, userDN string, cfg config.LDAPConfig) model.Role {
	filter := strings.ReplaceAll(cfg.GroupSearchFilter, "{userDN}", ldap.EscapeFilter(userDN))
	if filter == "" {
		filter = fmt.Sprintf("(member=%s)", ldap.EscapeFilter(userDN))
	}

	searchReq := ldap.NewSearchRequest(
		cfg.GroupSearchBase,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 10, false,
		filter,
		[]string{"cn"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return model.RoleLearner
	}

	// Find highest-privilege matching role
	bestRole := model.RoleLearner
	for _, entry := range sr.Entries {
		cn := entry.GetAttributeValue("cn")
		if mappedRole, ok := cfg.GroupToRole[cn]; ok {
			r := model.Role(mappedRole)
			if r == model.RoleAdmin {
				return model.RoleAdmin
			}
			if r == model.RoleInstructor {
				bestRole = model.RoleInstructor
			}
		}
	}
	return bestRole
}

func (h *Handler) upsertLDAPUser(r *http.Request, username, email, displayName, externalID string, role model.Role) (*model.User, error) {
	var user model.User

	err := h.db.GetContext(r.Context(), &user, `
		SELECT id, username, email, display_name, role, password_hash, external_id, auth_provider, active, last_login_at, created_at, updated_at
		FROM users WHERE external_id = $1 AND auth_provider = 'ldap'
	`, externalID)
	if err == nil {
		// Update on every login (sync attributes + role from groups)
		h.db.ExecContext(r.Context(), `
			UPDATE users SET display_name = $1, email = $2, role = $3, updated_at = now() WHERE id = $4
		`, displayName, email, role, user.ID)
		user.DisplayName = displayName
		user.Email = &email
		user.Role = role
		return &user, nil
	}

	// Create new user
	newUser := &model.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        &email,
		DisplayName:  displayName,
		Role:         role,
		ExternalID:   &externalID,
		AuthProvider: "ldap",
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO users (id, username, email, display_name, role, external_id, auth_provider, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, newUser.ID, newUser.Username, newUser.Email, newUser.DisplayName, newUser.Role, newUser.ExternalID, newUser.AuthProvider, newUser.Active, newUser.CreatedAt, newUser.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert ldap user: %w", err)
	}

	return newUser, nil
}
