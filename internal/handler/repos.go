package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Admin Repository CRUD ---

func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	var repos []model.GitRepository
	err := h.db.SelectContext(r.Context(), &repos, `
		SELECT id, clone_url, branch, auth_type, webhook_uuid, sync_status, sync_error, last_synced_at, created_at, updated_at
		FROM git_repositories ORDER BY created_at DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list repositories"})
		return
	}
	if repos == nil {
		repos = []model.GitRepository{}
	}
	writeJSON(w, http.StatusOK, repos)
}

func (h *Handler) GetRepo(w http.ResponseWriter, r *http.Request) {
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

type createRepoRequest struct {
	CloneURL    string `json:"clone_url"`
	Branch      string `json:"branch"`
	AuthType    string `json:"auth_type"`
	Credentials string `json:"credentials,omitempty"`
}

func (h *Handler) CreateRepo(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.CloneURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "clone_url is required"})
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.AuthType == "" {
		req.AuthType = "none"
	}

	webhookUUID := uuid.New()
	var repo model.GitRepository
	err := h.db.GetContext(r.Context(), &repo, `
		INSERT INTO git_repositories (clone_url, branch, auth_type, credentials, webhook_uuid)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, clone_url, branch, auth_type, webhook_uuid, sync_status, sync_error, last_synced_at, created_at, updated_at
	`, req.CloneURL, req.Branch, req.AuthType, []byte(req.Credentials), webhookUUID)
	if err != nil {
		slog.Error("failed to create repository", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create repository"})
		return
	}

	// Enqueue initial sync
	h.enqueueSync(r.Context(), repo.ID)

	writeJSON(w, http.StatusCreated, repo)
}

func (h *Handler) UpdateRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")

	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var repo model.GitRepository
	err := h.db.GetContext(r.Context(), &repo, `
		UPDATE git_repositories
		SET clone_url = COALESCE(NULLIF($1, ''), clone_url),
		    branch = COALESCE(NULLIF($2, ''), branch),
		    auth_type = COALESCE(NULLIF($3, ''), auth_type),
		    credentials = CASE WHEN $4 = '' THEN credentials ELSE $4::bytea END,
		    updated_at = now()
		WHERE id = $5
		RETURNING id, clone_url, branch, auth_type, webhook_uuid, sync_status, sync_error, last_synced_at, created_at, updated_at
	`, req.CloneURL, req.Branch, req.AuthType, req.Credentials, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}

	writeJSON(w, http.StatusOK, repo)
}

func (h *Handler) DeleteRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")
	result, err := h.db.ExecContext(r.Context(), "DELETE FROM git_repositories WHERE id = $1", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete repository"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) SyncRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")
	repoUUID, err := uuid.Parse(id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}

	h.enqueueSync(r.Context(), repoUUID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync enqueued"})
}

// --- Webhook ---

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "uuid")

	var repo model.GitRepository
	err := h.db.GetContext(r.Context(), &repo, `
		SELECT id, sync_status FROM git_repositories WHERE webhook_uuid = $1
	`, webhookID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	// Debounce: ignore if already syncing
	if repo.SyncStatus == "syncing" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already syncing"})
		return
	}

	h.enqueueSync(r.Context(), repo.ID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync enqueued"})
}
