package handler

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// resolvePathSlug resolves a pathId parameter (UUID or slug) to the path's UUID.
func (h *Handler) resolvePathSlug(ctx context.Context, param string) (uuid.UUID, error) {
	if id, err := uuid.Parse(param); err == nil {
		return id, nil
	}
	var id uuid.UUID
	err := h.db.GetContext(ctx, &id, `
		SELECT id FROM learning_paths WHERE slug = $1 AND deleted_at IS NULL
	`, param)
	if err != nil {
		return uuid.Nil, fmt.Errorf("learning path not found: %w", err)
	}
	return id, nil
}

// resolveStepSlug resolves a stepId parameter (UUID or slug) to the step's UUID.
// If lpSlug is provided, it scopes the lookup to that learning path.
func (h *Handler) resolveStepSlug(ctx context.Context, param string, lpSlug string) (uuid.UUID, error) {
	if id, err := uuid.Parse(param); err == nil {
		return id, nil
	}
	var id uuid.UUID
	var err error
	if lpSlug != "" {
		err = h.db.GetContext(ctx, &id, `
			SELECT s.id FROM steps s
			JOIN modules m ON m.id = s.module_id AND m.deleted_at IS NULL
			JOIN learning_paths lp ON lp.id = m.learning_path_id AND lp.deleted_at IS NULL
			WHERE s.slug = $1 AND lp.slug = $2 AND s.deleted_at IS NULL
		`, param, lpSlug)
	} else {
		// Without path context, just find by slug (may be ambiguous)
		err = h.db.GetContext(ctx, &id, `
			SELECT id FROM steps WHERE slug = $1 AND deleted_at IS NULL LIMIT 1
		`, param)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("step not found")
		}
		return uuid.Nil, fmt.Errorf("step not found: %w", err)
	}
	return id, nil
}
