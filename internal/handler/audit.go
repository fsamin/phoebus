package handler

import (
	"context"
	"encoding/json"

	"github.com/fsamin/phoebus/internal/auth"
	"github.com/fsamin/phoebus/internal/logging"
)

// auditLog records an action to the audit_log table.
//
// resource_id is a UUID column: callers acting on something that has no UUID
// (an instance-wide setting, for instance) pass an empty resourceID, which is
// stored as NULL rather than making PostgreSQL reject the whole row.
func (h *Handler) auditLog(ctx context.Context, claims *auth.Claims, action, resourceType, resourceID string, metadata map[string]any) {
	var actorID interface{}
	if claims != nil {
		actorID = claims.UserID
	}
	var resID interface{}
	if resourceID != "" {
		resID = resourceID
	}
	metaJSON := []byte("{}")
	if metadata != nil {
		metaJSON, _ = json.Marshal(metadata)
	}
	if _, err := h.db.ExecContext(ctx, `
		INSERT INTO audit_log (actor_id, action, resource_type, resource_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, actorID, action, resourceType, resID, metaJSON); err != nil {
		logging.FromContext(ctx).Error("failed to record audit entry",
			"action", action, "resource_type", resourceType, "error", err.Error())
	}
}
