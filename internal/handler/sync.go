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

	// Notify the sync worker that a new job is available
	if h.syncer != nil {
		h.syncer.Notify()
	}
}
