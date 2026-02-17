package handler

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

func (h *Handler) enqueueSync(ctx context.Context, repoID uuid.UUID) {
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO sync_jobs (repo_id, status) VALUES ($1, 'pending')
	`, repoID)
	if err != nil {
		slog.Error("failed to enqueue sync job", "repo_id", repoID, "error", err)
		return
	}

	// Update repo status
	h.db.ExecContext(ctx, `
		UPDATE git_repositories SET sync_status = 'syncing', updated_at = now() WHERE id = $1
	`, repoID)

	// Trigger sync in background
	go h.syncer.ProcessRepo(context.Background(), repoID)
}
