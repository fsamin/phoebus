package handler

import (
	"context"
	"encoding/json"

	"github.com/fsamin/phoebus/internal/auth"
)

// auditLog records an action to the audit_log table.
func (h *Handler) auditLog(ctx context.Context, claims *auth.Claims, action, resourceType, resourceID string, metadata map[string]any) {
	var metaJSON []byte
	if metadata != nil {
		metaJSON, _ = json.Marshal(metadata)
	}
	h.db.ExecContext(ctx, `
		INSERT INTO audit_log (actor_id, action, resource_type, resource_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, claims.UserID, action, resourceType, resourceID, metaJSON)
}
