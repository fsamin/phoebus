package handler

import (
	"context"

	"github.com/fsamin/phoebus/internal/logging"
	"github.com/google/uuid"
)

func (h *Handler) enqueueSync(ctx context.Context, repoID uuid.UUID) {
	logger := logging.FromContext(ctx)
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO sync_jobs (repo_id, status) VALUES ($1, 'pending')
	`, repoID)
	if err != nil {
		logger.Error("failed to enqueue sync job", "repo_id", repoID, "error", err.Error())
		return
	}

	// Notify the sync worker that a new job is available
	if h.syncer != nil {
		h.syncer.Notify()
	}
}
