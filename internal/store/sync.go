package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orin/orin/internal/domain"
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

// CreateIfNotBusy atomically creates a pending sync operation only if no
// pending/running operation exists for the app. Returns true if the row was
// created, false if the app is already busy.
func (s *SyncOperations) CreateIfNotBusy(ctx context.Context, op *domain.SyncOperation) (bool, error) {
	if op.ID == "" {
		op.ID = uuid.NewString()
	}
	if op.StartedAt.IsZero() {
		op.StartedAt = time.Now().UTC()
	}
	resources, err := json.Marshal(op.Resources)
	if err != nil {
		return false, err
	}
	reqJSON, err := json.Marshal(op.Request)
	if err != nil {
		return false, err
	}
	var created bool
	err = s.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO sync_operations
			  (id, app_id, started_at, finished_at, revision, initiated_by, status, message, resources_json, request_json)
			SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
			WHERE NOT EXISTS (
				SELECT 1 FROM sync_operations
				WHERE app_id = $2 AND status IN ('Pending','Running')
			)
			RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM inserted)`,
		op.ID, op.AppID, op.StartedAt, op.FinishedAt, op.Revision,
		op.InitiatedBy, op.Status, op.Message, resources, reqJSON).Scan(&created)
	return created, err
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

// ClaimNextPending atomically claims the oldest pending sync operation for an
// application by setting its status to 'Running'. Returns the claimed op, or
// nil if no pending op exists. Uses SELECT FOR UPDATE SKIP LOCKED so that
// multiple controller pods cannot claim the same operation.
func (s *SyncOperations) ClaimNextPending(ctx context.Context, appID string) (*domain.SyncOperation, error) {
	var claimed bool
	var id string
	var startedAt time.Time
	var finishedAt *time.Time
	var revision string
	var initiatedBy string
	var status string
	var msg string
	var resources []byte
	var reqJSON []byte

	err := s.pool.QueryRow(ctx, `
		UPDATE sync_operations SET status = 'Running'
		WHERE id = (
			SELECT id FROM sync_operations
			WHERE app_id = $1 AND status = 'Pending'
			ORDER BY started_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, started_at, finished_at, revision, initiated_by, status, message, resources_json, request_json`,
		appID).Scan(&id, &startedAt, &finishedAt, &revision, &initiatedBy, &status, &msg, &resources, &reqJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		claimed = false
	} else if err != nil {
		return nil, err
	} else {
		claimed = true
	}

	if !claimed {
		return nil, nil
	}

	op := &domain.SyncOperation{
		ID:          id,
		AppID:       appID,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		Revision:    revision,
		InitiatedBy: initiatedBy,
		Status:      domain.SyncOpStatus(status),
		Message:     msg,
	}
	if len(resources) > 0 {
		if err := json.Unmarshal(resources, &op.Resources); err != nil {
			return nil, err
		}
	}
	if len(reqJSON) > 0 {
		_ = json.Unmarshal(reqJSON, &op.Request)
	}
	return op, nil
}

// NextPending returns the oldest pending op for an application, or nil.
// DEPRECATED: use ClaimNextPending instead for atomic claim.
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

// Cancel marks a pending or running sync operation as failed/cancelled.
func (s *SyncOperations) Cancel(ctx context.Context, syncID string) error {
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		UPDATE sync_operations
		SET status=$1, message=$2, finished_at=$3
		WHERE id=$4 AND status IN ($5, $6)`,
		domain.SyncOpFailed, "Cancelled by user", now, syncID,
		domain.SyncOpPending, domain.SyncOpRunning)
	return err
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
		SELECT app_id, sync_status, health_status, observed_revision, last_synced_at, last_manual_apply_at, message, updated_at
		FROM application_status WHERE app_id=$1`, appID)
	var s domain.ApplicationStatus
	var lastSyncedAt, lastManualApplyAt *time.Time
	if err := row.Scan(&s.AppID, &s.SyncStatus, &s.HealthStatus, &s.ObservedRevision,
		&lastSyncedAt, &lastManualApplyAt, &s.Message, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.LastSyncedAt = lastSyncedAt
	s.LastManualApplyAt = lastManualApplyAt
	return &s, nil
}

// Upsert writes the latest reconcile result.
func (st *Statuses) Upsert(ctx context.Context, s *domain.ApplicationStatus) error {
	s.UpdatedAt = time.Now().UTC()
	_, err := st.pool.Exec(ctx, `
		INSERT INTO application_status (app_id, sync_status, health_status, observed_revision, last_synced_at, last_manual_apply_at, message, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (app_id) DO UPDATE SET
		  sync_status=EXCLUDED.sync_status,
		  health_status=EXCLUDED.health_status,
		  observed_revision=EXCLUDED.observed_revision,
		  last_synced_at=EXCLUDED.last_synced_at,
		  last_manual_apply_at=COALESCE(EXCLUDED.last_manual_apply_at, application_status.last_manual_apply_at),
		  message=EXCLUDED.message,
		  updated_at=EXCLUDED.updated_at`,
		s.AppID, s.SyncStatus, s.HealthStatus, s.ObservedRevision,
		s.LastSyncedAt, s.LastManualApplyAt, s.Message, s.UpdatedAt)
	return err
}

// UpsertLastManualApply records the last manual live-apply timestamp for an
// application. This is used to suppress auto-sync during the grace period
// and is persisted to DB so it survives pod restarts and leader failovers.
func (st *Statuses) UpsertLastManualApply(ctx context.Context, appID string, ts time.Time) error {
	ts = ts.UTC()
	_, err := st.pool.Exec(ctx, `
		UPDATE application_status SET
		  last_manual_apply_at = $2,
		  updated_at = NOW()
		WHERE app_id = $1`, appID, ts)
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
