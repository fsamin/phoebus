package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/fsamin/phoebus/internal/model"
)

// bannerSettingKey is the instance_settings row holding the dashboard banner.
const bannerSettingKey = "dashboard_banner"

// maxBannerLength bounds the message: it is a banner, not an article, and it
// travels in every dashboard response.
const maxBannerLength = 2000

// loadBanner reads the banner from instance_settings. An instance that never
// configured one has no row at all — that is the normal initial state, not an
// error, and it yields a disabled banner.
func (h *Handler) loadBanner(ctx context.Context) (model.Banner, error) {
	banner := model.Banner{Level: model.BannerInfo}

	var raw string
	err := h.db.GetContext(ctx, &raw, `SELECT value FROM instance_settings WHERE key = $1`, bannerSettingKey)
	if errors.Is(err, sql.ErrNoRows) {
		return banner, nil
	}
	if err != nil {
		return banner, err
	}
	if err := json.Unmarshal([]byte(raw), &banner); err != nil {
		return model.Banner{Level: model.BannerInfo}, err
	}
	if banner.Level == "" {
		banner.Level = model.BannerInfo
	}
	return banner, nil
}

// GetBanner returns the banner as stored, disabled or not, so the admin form
// can be populated with the current draft.
func (h *Handler) GetBanner(w http.ResponseWriter, r *http.Request) {
	banner, err := h.loadBanner(r.Context())
	if err != nil {
		writeDBError(w, r, "failed to load banner", err)
		return
	}
	writeJSON(w, http.StatusOK, banner)
}

// UpdateBanner replaces the banner.
func (h *Handler) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	var req model.Banner
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Level == "" {
		req.Level = model.BannerInfo
	}
	switch req.Level {
	case model.BannerInfo, model.BannerWarning, model.BannerDanger:
		// valid
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid level (must be info, warning, or danger)"})
		return
	}

	req.MessageMD = strings.TrimSpace(req.MessageMD)
	if len([]rune(req.MessageMD)) > maxBannerLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is too long (2000 characters max)"})
		return
	}
	// Enabling an empty banner shows nothing at all: report it instead of
	// leaving the admin wondering why their banner never appears.
	if req.Enabled && req.MessageMD == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required when the banner is enabled"})
		return
	}

	value, err := json.Marshal(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid banner"})
		return
	}

	// instance_settings has no updated_at trigger — no table in this schema does.
	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO instance_settings (key, value, encrypted) VALUES ($1, $2, false)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, bannerSettingKey, string(value)); err != nil {
		writeDBError(w, r, "failed to save banner", err)
		return
	}

	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "update", "instance_setting", "",
		map[string]any{"key": bannerSettingKey, "enabled": req.Enabled, "level": req.Level})

	writeJSON(w, http.StatusOK, req)
}

// dashboardBanner returns what the dashboard should display, or nil. A disabled
// or empty banner yields nil: the stored message is possibly a draft, and it
// must not travel to every learner before the admin turns it on.
func (h *Handler) dashboardBanner(ctx context.Context) (any, error) {
	banner, err := h.loadBanner(ctx)
	if err != nil {
		return nil, err
	}
	if !banner.Enabled || banner.MessageMD == "" {
		return nil, nil
	}
	return map[string]any{"level": banner.Level, "message_md": banner.MessageMD}, nil
}
