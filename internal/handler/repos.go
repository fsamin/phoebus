package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fsamin/phoebus/internal/crypto"
	"github.com/fsamin/phoebus/internal/logging"
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

	// Fetch owners for all repos
	type ownerRow struct {
		RepoID      string `db:"repo_id" json:"-"`
		ID          string `db:"id" json:"id"`
		Username    string `db:"username" json:"username"`
		DisplayName string `db:"display_name" json:"display_name"`
	}
	var ownerRows []ownerRow
	h.db.SelectContext(r.Context(), &ownerRows, `
		SELECT ro.repo_id::text AS repo_id, u.id::text AS id, u.username, u.display_name
		FROM repository_owners ro
		JOIN users u ON u.id = ro.user_id
		ORDER BY u.display_name
	`)
	ownerMap := map[string][]ownerRow{}
	for _, o := range ownerRows {
		ownerMap[o.RepoID] = append(ownerMap[o.RepoID], o)
	}

	type repoResponse struct {
		model.GitRepository
		PathTitles []string   `json:"path_titles"`
		Owners     []ownerRow `json:"owners"`
	}
	out := make([]repoResponse, len(repos))
	for i, repo := range repos {
		titles := titleMap[repo.ID.String()]
		if titles == nil {
			titles = []string{}
		}
		owners := ownerMap[repo.ID.String()]
		if owners == nil {
			owners = []ownerRow{}
		}
		out[i] = repoResponse{GitRepository: repo, PathTitles: titles, Owners: owners}
	}
	writeJSON(w, http.StatusOK, out)
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

	type ownerInfo struct {
		ID          string `db:"id" json:"id"`
		Username    string `db:"username" json:"username"`
		DisplayName string `db:"display_name" json:"display_name"`
	}
	var owners []ownerInfo
	h.db.SelectContext(r.Context(), &owners, `
		SELECT u.id::text AS id, u.username, u.display_name
		FROM repository_owners ro
		JOIN users u ON u.id = ro.user_id
		WHERE ro.repo_id = $1
		ORDER BY u.display_name
	`, id)
	if owners == nil {
		owners = []ownerInfo{}
	}

	writeJSON(w, http.StatusOK, struct {
		model.GitRepository
		Owners []ownerInfo `json:"owners"`
	}{repo, owners})
}

type createRepoRequest struct {
	CloneURL    string   `json:"clone_url"`
	Branch      string   `json:"branch"`
	AuthType    string   `json:"auth_type"`
	Credentials string   `json:"credentials,omitempty"`
	OwnerIDs    []string `json:"owner_ids,omitempty"`
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

	// Encrypt credentials if provided and encryption key is configured
	var credBytes []byte
	if req.Credentials != "" {
		if h.cfg.EncryptionKey != "" {
			encrypted, err := crypto.Encrypt([]byte(req.Credentials), []byte(h.cfg.EncryptionKey))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt credentials"})
				return
			}
			credBytes = encrypted
		} else {
			credBytes = []byte(req.Credentials)
		}
	}

	var repo model.GitRepository
	err := h.db.GetContext(r.Context(), &repo, `
		INSERT INTO git_repositories (clone_url, branch, auth_type, credentials, webhook_uuid)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, clone_url, branch, auth_type, webhook_uuid, sync_status, sync_error, last_synced_at, created_at, updated_at
	`, req.CloneURL, req.Branch, req.AuthType, credBytes, webhookUUID)
	if err != nil {
		logging.FromContext(r.Context()).Error("failed to create repository", "error", err.Error())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create repository"})
		return
	}

	// Enqueue initial sync
	h.enqueueSync(r.Context(), repo.ID)

	// Save owners
	h.setRepoOwners(r.Context(), repo.ID.String(), req.OwnerIDs)

	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "create", "git_repository", repo.ID.String(), map[string]any{"clone_url": req.CloneURL})

	writeJSON(w, http.StatusCreated, repo)
}

func (h *Handler) UpdateRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "repoId")

	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Encrypt credentials if provided
	credValue := req.Credentials
	if credValue != "" && h.cfg.EncryptionKey != "" {
		encrypted, err := crypto.Encrypt([]byte(credValue), []byte(h.cfg.EncryptionKey))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt credentials"})
			return
		}
		credValue = string(encrypted)
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
	`, req.CloneURL, req.Branch, req.AuthType, credValue, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}

	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "update", "git_repository", id, nil)

	// Update owners
	h.setRepoOwners(r.Context(), id, req.OwnerIDs)

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
	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "delete", "git_repository", id, nil)
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
	h.auditLog(r.Context(), ClaimsFromContext(r.Context()), "sync", "git_repository", id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync enqueued"})
}

func (h *Handler) SyncLogs(w http.ResponseWriter, r *http.Request) {
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

// SyncJobLogs returns the detailed logs for a specific sync job.
func (h *Handler) SyncJobLogs(w http.ResponseWriter, r *http.Request) {
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

// setRepoOwners replaces all owners for a given repo.
func (h *Handler) setRepoOwners(ctx context.Context, repoID string, ownerIDs []string) {
	h.db.ExecContext(ctx, "DELETE FROM repository_owners WHERE repo_id = $1", repoID)
	for _, uid := range ownerIDs {
		h.db.ExecContext(ctx, "INSERT INTO repository_owners (repo_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", repoID, uid)
	}
}

// ListInstructorUsers returns users with instructor or admin role (for owner selector).
func (h *Handler) ListInstructorUsers(w http.ResponseWriter, r *http.Request) {
	type userInfo struct {
		ID          string `db:"id" json:"id"`
		Username    string `db:"username" json:"username"`
		DisplayName string `db:"display_name" json:"display_name"`
	}
	var users []userInfo
	err := h.db.SelectContext(r.Context(), &users, `
		SELECT id::text AS id, username, display_name
		FROM users
		WHERE role IN ('instructor', 'admin') AND active = true
		ORDER BY display_name
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}
	if users == nil {
		users = []userInfo{}
	}
	writeJSON(w, http.StatusOK, users)
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
