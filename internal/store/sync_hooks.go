package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orin/orin/internal/domain"
)

type SyncHookStore struct {
	pool *pgxpool.Pool
}

func NewSyncHookStore(pool *pgxpool.Pool) *SyncHookStore {
	return &SyncHookStore{pool: pool}
}

func (s *SyncHookStore) Create(ctx context.Context, h *domain.SyncHook) error {
	now := time.Now().UTC()
	h.CreatedAt = now
	h.UpdatedAt = now
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_hooks (id, app_id, name, phase, yaml, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, h.ID, h.AppID, h.Name, h.Phase, h.YAML, h.Enabled, h.CreatedAt, h.UpdatedAt)
	return err
}

func (s *SyncHookStore) Update(ctx context.Context, h *domain.SyncHook) error {
	h.UpdatedAt = time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		UPDATE sync_hooks SET name = $3, phase = $4, yaml = $5, enabled = $6, updated_at = $7
		WHERE id = $1 AND app_id = $2
	`, h.ID, h.AppID, h.Name, h.Phase, h.YAML, h.Enabled, h.UpdatedAt)
	return err
}

func (s *SyncHookStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sync_hooks WHERE id = $1`, id)
	return err
}

func (s *SyncHookStore) Get(ctx context.Context, id string) (*domain.SyncHook, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, app_id, name, phase, yaml, enabled, created_at, updated_at
		FROM sync_hooks WHERE id = $1
	`, id)
	return scanSyncHook(row)
}

func (s *SyncHookStore) ListByApp(ctx context.Context, appID string) ([]*domain.SyncHook, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, app_id, name, phase, yaml, enabled, created_at, updated_at
		FROM sync_hooks WHERE app_id = $1 ORDER BY phase, created_at
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []*domain.SyncHook
	for rows.Next() {
		h, err := scanSyncHook(rows)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

func (s *SyncHookStore) ListByPhase(ctx context.Context, appID string, phase domain.SyncHookPhase) ([]*domain.SyncHook, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, app_id, name, phase, yaml, enabled, created_at, updated_at
		FROM sync_hooks WHERE app_id = $1 AND phase = $2 AND enabled = true ORDER BY created_at
	`, appID, phase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hooks []*domain.SyncHook
	for rows.Next() {
		h, err := scanSyncHook(rows)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

type syncHookScanner interface {
	Scan(dest ...any) error
}

func scanSyncHook(row syncHookScanner) (*domain.SyncHook, error) {
	h := &domain.SyncHook{}
	err := row.Scan(&h.ID, &h.AppID, &h.Name, &h.Phase, &h.YAML, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return h, nil
}
