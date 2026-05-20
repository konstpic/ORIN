package store

import (
	"context"
	"encoding/json"

	"github.com/orin/orin/internal/domain"
)

// ListRecent returns newest audit rows up to limit (capped at 5000).
func (a *Auditor) ListRecent(ctx context.Context, limit int) ([]domain.AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := a.pool.Query(ctx, `
		SELECT id, ts, actor, action, resource, payload_json
		FROM audit_log ORDER BY ts DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var payload []byte
		if err := rows.Scan(&e.ID, &e.TS, &e.Actor, &e.Action, &e.Resource, &payload); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			e.Payload = json.RawMessage(payload)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
