package handler

import (
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/logging"
	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Instructor Repository Routes (ownership-verified) ---

// instructorOwnerMiddleware checks that the current user owns the repo.
func (h *Handler) instructorOwnerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		// Admins bypass ownership check
		if claims.Role == model.RoleAdmin {
			next.ServeHTTP(w, r)
			return
		}
		repoID := chi.URLParam(r, "repoId")
		var exists bool
		err := h.db.GetContext(r.Context(), &exists, `
			SELECT EXISTS(SELECT 1 FROM repository_owners WHERE repo_id = $1 AND user_id = $2)
		`, repoID, claims.UserID)
		if err != nil || !exists {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "you are not an owner of this repository"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// InstructorListRepos returns repos owned by the current instructor.
func (h *Handler) InstructorListRepos(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())

	var repos []model.GitRepository
	err := h.db.SelectContext(r.Context(), &repos, `
		SELECT gr.id, gr.clone_url, gr.branch, gr.auth_type, gr.webhook_uuid,
		       gr.sync_status, gr.sync_error, gr.last_synced_at, gr.created_at, gr.updated_at
		FROM git_repositories gr
		JOIN repository_owners ro ON ro.repo_id = gr.id
		WHERE ro.user_id = $1
		ORDER BY gr.created_at DESC
	`, claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list repositories"})
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}

	// Enrich with learning path titles
	type result struct {
		RepoID string `db:"repo_id"`
		Title  string `db:"title"`
	}
	var pathTitles []result
	h.db.SelectContext(r.Context(), &pathTitles, `SELECT repo_id, title FROM learning_paths`)
	titleMap := map[string][]string{}
	for _, pt := range pathTitles {
		titleMap[pt.RepoID] = append(titleMap[pt.RepoID], pt.Title)
	}

	type repoResponse struct {
		model.GitRepository
		PathTitles []string `json:"path_titles"`
	}
	out := make([]repoResponse, len(repos))
	for i, repo := range repos {
		titles := titleMap[repo.ID.String()]
		if titles == nil {
			titles = []string{}
		}
		out[i] = repoResponse{GitRepository: repo, PathTitles: titles}
	}
	writeJSON(w, http.StatusOK, out)
}

// InstructorGetRepo returns details for a single owned repo.
func (h *Handler) InstructorGetRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")
	var repo model.GitRepository
	err := h.db.GetContext(r.Context(), &repo, `
		SELECT id, clone_url, branch, auth_type, webhook_uuid, sync_status, sync_error, last_synced_at, created_at, updated_at
		FROM git_repositories WHERE id = $1
	`, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	writeJSON(w, http.StatusOK, repo)
}

// InstructorSyncRepo triggers a sync for an owned repo.
func (h *Handler) InstructorSyncRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}
	h.enqueueSync(r.Context(), repoUUID)
	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "sync", "git_repository", id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync enqueued"})
}

// InstructorSyncLogs returns sync logs for an owned repo.
func (h *Handler) InstructorSyncLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")
	if _, err := uuid.Parse(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}

	var logs []struct {
		ID          string  `json:"id" db:"id"`
		RepoID      string  `json:"repo_id" db:"repo_id"`
		Status      string  `json:"status" db:"status"`
		Error       *string `json:"error" db:"error"`
		Attempts    int     `json:"attempts" db:"attempts"`
		StartedAt   *string `json:"started_at" db:"started_at"`
		CompletedAt *string `json:"completed_at" db:"completed_at"`
		CreatedAt   string  `json:"created_at" db:"created_at"`
	}
	if err := h.db.SelectContext(r.Context(), &logs, `
		SELECT id, repo_id, status, error, attempts,
			   to_char(started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS started_at,
			   to_char(completed_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS completed_at,
			   to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		FROM sync_jobs WHERE repo_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, id); err != nil {
		logging.FromContext(r.Context()).Error("failed to fetch sync logs", "error", err.Error())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if logs == nil {
		logs = []struct {
			ID          string  `json:"id" db:"id"`
			RepoID      string  `json:"repo_id" db:"repo_id"`
			Status      string  `json:"status" db:"status"`
			Error       *string `json:"error" db:"error"`
			Attempts    int     `json:"attempts" db:"attempts"`
			StartedAt   *string `json:"started_at" db:"started_at"`
			CompletedAt *string `json:"completed_at" db:"completed_at"`
			CreatedAt   string  `json:"created_at" db:"created_at"`
		}{}
	}
	writeJSON(w, http.StatusOK, logs)
}

// InstructorSyncJobLogs returns detailed logs for a specific sync job.
func (h *Handler) InstructorSyncJobLogs(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	jobID := chi.URLParam(r, "jobId")
	if _, err := uuid.Parse(repoID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}
	if _, err := uuid.Parse(jobID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid job id"})
		return
	}

	var logsJSON *json.RawMessage
	err := h.db.GetContext(r.Context(), &logsJSON, `
		SELECT logs FROM sync_jobs WHERE id = $1 AND repo_id = $2
	`, jobID, repoID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sync job not found"})
		return
	}
	if logsJSON == nil {
		empty := json.RawMessage("[]")
		logsJSON = &empty
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(*logsJSON)
}
