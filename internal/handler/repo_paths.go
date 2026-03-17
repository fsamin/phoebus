package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ListRepoPaths returns learning paths for a specific repository with their enabled status.
func (h *Handler) ListRepoPaths(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	if _, err := uuid.Parse(repoID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}

	type repoPath struct {
		ID                string `json:"id" db:"id"`
		Slug              string `json:"slug" db:"slug"`
		Title             string `json:"title" db:"title"`
		Description       string `json:"description" db:"description"`
		Enabled           bool   `json:"enabled" db:"enabled"`
		ModuleCount       int    `json:"module_count" db:"module_count"`
		StepCount         int    `json:"step_count" db:"step_count"`
	}

	var paths []repoPath
	err := h.db.SelectContext(r.Context(), &paths, `
		SELECT lp.id::text, lp.slug, lp.title, lp.description, lp.enabled,
		       COUNT(DISTINCT m.id) AS module_count,
		       COUNT(DISTINCT s.id) AS step_count
		FROM learning_paths lp
		LEFT JOIN modules m ON m.learning_path_id = lp.id AND m.deleted_at IS NULL
		LEFT JOIN steps s ON s.module_id = m.id AND s.deleted_at IS NULL
		WHERE lp.repo_id = $1 AND lp.deleted_at IS NULL
		GROUP BY lp.id
		ORDER BY lp.title
	`, repoID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list paths"})
		return
	}
	if paths == nil {
		paths = []repoPath{}
	}

	writeJSON(w, http.StatusOK, paths)
}

// ToggleRepoPath enables or disables a learning path.
func (h *Handler) ToggleRepoPath(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	pathParam := chi.URLParam(r, "pathId")
	if _, err := uuid.Parse(repoID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}
	pathUUID, err := h.resolvePathSlug(r.Context(), pathParam)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE learning_paths SET enabled = $1, updated_at = now()
		WHERE id = $2 AND repo_id = $3 AND deleted_at IS NULL
	`, req.Enabled, pathUUID, repoID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update path"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "learning path not found"})
		return
	}

	action := "disable"
	if req.Enabled {
		action = "enable"
	}
	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), action, "learning_path", pathUUID.String(), map[string]any{"repo_id": repoID, "enabled": req.Enabled})

	writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "enabled": req.Enabled})
}
