package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

func (h *Handler) GetOnboarding(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var raw json.RawMessage
	err := h.db.GetContext(r.Context(), &raw,
		"SELECT onboarding_tours_seen FROM users WHERE id = $1", claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch onboarding status"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func (h *Handler) MarkOnboardingSeen(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Tour string `json:"tour"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Tour == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'tour' field"})
		return
	}

	// Merge the tour key into the existing JSONB
	_, err := h.db.ExecContext(r.Context(), `
		UPDATE users
		SET onboarding_tours_seen = onboarding_tours_seen || jsonb_build_object($2::text, true),
		    updated_at = now()
		WHERE id = $1
	`, claims.UserID, req.Tour)
	if err != nil {
		log.Printf("MarkOnboardingSeen error: %v (userID=%s, tour=%s)", err, claims.UserID, req.Tour)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update onboarding status"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ResetOnboarding(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE users SET onboarding_tours_seen = '{}', updated_at = now() WHERE id = $1
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reset onboarding"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
