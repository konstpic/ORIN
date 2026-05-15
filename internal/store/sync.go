package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

// SyncOperations persists sync history.
type SyncOperations struct{ pool *pgxpool.Pool }

// Create inserts a pending sync operation.
func (s *SyncOperations) Create(ctx context.Context, op *domain.SyncOperation) error {
	if op.ID == "" {
		op.ID = uuid.NewString()
	}
	if op.StartedAt.IsZero() {
		op.StartedAt = time.Now().UTC()
	}
	resources, err := json.Marshal(op.Resources)
	if err != nil {
		return err
	}
	reqJSON, err := json.Marshal(op.Request)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sync_operations
		  (id, app_id, started_at, finished_at, revision, initiated_by, status, message, resources_json, request_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		op.ID, op.AppID, op.StartedAt, op.FinishedAt, op.Revision,
		op.InitiatedBy, op.Status, op.Message, resources, reqJSON)
	return err
}

// Update updates the running state of a sync operation.
func (s *SyncOperations) Update(ctx context.Context, op *domain.SyncOperation) error {
	resources, err := json.Marshal(op.Resources)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE sync_operations SET
		  finished_at=$2, status=$3, message=$4, resources_json=$5
		WHERE id=$1`,
		op.ID, op.FinishedAt, op.Status, op.Message, resources)
	return err
}

// HasPendingOrRunning reports whether this app already has a sync job queued or executing.
func (s *SyncOperations) HasPendingOrRunning(ctx context.Context, appID string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM sync_operations
		WHERE app_id=$1 AND status IN ('Pending','Running')`, appID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// NextPending returns the oldest pending op for an application, or nil.
func (s *SyncOperations) NextPending(ctx context.Context, appID string) (*domain.SyncOperation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, app_id, started_at, finished_at, revision, initiated_by, status, message, resources_json, request_json
		FROM sync_operations
		WHERE app_id=$1 AND status='Pending'
		ORDER BY started_at
		LIMIT 1`, appID)
	op, err := scanSyncOp(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil
	}
	return op, err
}

// ListByApp returns recent ops for an application.
func (s *SyncOperations) ListByApp(ctx context.Context, appID string, limit int) ([]*domain.SyncOperation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, app_id, started_at, finished_at, revision, initiated_by, status, message, resources_json, request_json
		FROM sync_operations
		WHERE app_id=$1
		ORDER BY started_at DESC
		LIMIT $2`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.SyncOperation
	for rows.Next() {
		op, err := scanSyncOp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, rows.Err()
}

func scanSyncOp(s scanner) (*domain.SyncOperation, error) {
	var op domain.SyncOperation
	var resources []byte
	var reqJSON []byte
	if err := s.Scan(&op.ID, &op.AppID, &op.StartedAt, &op.FinishedAt, &op.Revision,
		&op.InitiatedBy, &op.Status, &op.Message, &resources, &reqJSON); err != nil {
		return nil, err
	}
	if len(resources) > 0 {
		if err := json.Unmarshal(resources, &op.Resources); err != nil {
			return nil, err
		}
	}
	if len(reqJSON) > 0 {
		_ = json.Unmarshal(reqJSON, &op.Request)
	}
	return &op, nil
}

// Statuses persists application_status rows.
type Statuses struct{ pool *pgxpool.Pool }

// Get fetches the status row for an application.
func (st *Statuses) Get(ctx context.Context, appID string) (*domain.ApplicationStatus, error) {
	row := st.pool.QueryRow(ctx, `
		SELECT app_id, sync_status, health_status, observed_revision, last_synced_at, message, updated_at
		FROM application_status WHERE app_id=$1`, appID)
	var s domain.ApplicationStatus
	if err := row.Scan(&s.AppID, &s.SyncStatus, &s.HealthStatus, &s.ObservedRevision,
		&s.LastSyncedAt, &s.Message, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// Upsert writes the latest reconcile result.
func (st *Statuses) Upsert(ctx context.Context, s *domain.ApplicationStatus) error {
	s.UpdatedAt = time.Now().UTC()
	_, err := st.pool.Exec(ctx, `
		INSERT INTO application_status (app_id, sync_status, health_status, observed_revision, last_synced_at, message, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (app_id) DO UPDATE SET
		  sync_status=EXCLUDED.sync_status,
		  health_status=EXCLUDED.health_status,
		  observed_revision=EXCLUDED.observed_revision,
		  last_synced_at=EXCLUDED.last_synced_at,
		  message=EXCLUDED.message,
		  updated_at=EXCLUDED.updated_at`,
		s.AppID, s.SyncStatus, s.HealthStatus, s.ObservedRevision,
		s.LastSyncedAt, s.Message, s.UpdatedAt)
	return err
}

// Auditor persists audit entries.
type Auditor struct{ pool *pgxpool.Pool }

// Append writes an audit row.
func (a *Auditor) Append(ctx context.Context, e *domain.AuditEntry) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	_, err := a.pool.Exec(ctx, `
		INSERT INTO audit_log (id, ts, actor, action, resource, payload_json)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		e.ID, e.TS, e.Actor, e.Action, e.Resource, []byte(e.Payload))
	return err
}

var _ = pgxpool.Pool{} // keep import in case future helpers move out
