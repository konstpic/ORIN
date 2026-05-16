package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

type NotificationStore struct {
	pool *pgxpool.Pool
}

func NewNotificationStore(pool *pgxpool.Pool) *NotificationStore {
	return &NotificationStore{pool: pool}
}

func (s *NotificationStore) Create(ctx context.Context, cfg *domain.NotificationConfig) error {
	eventsJSON, _ := json.Marshal(cfg.Events)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification_configs (id, app_id, name, type, url, events, enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, cfg.ID, cfg.AppID, cfg.Name, cfg.Type, cfg.URL, eventsJSON, cfg.Enabled, cfg.CreatedAt)
	return err
}

func (s *NotificationStore) Update(ctx context.Context, cfg *domain.NotificationConfig) error {
	eventsJSON, _ := json.Marshal(cfg.Events)
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_configs
		SET name = $3, type = $4, url = $5, events = $6, enabled = $7
		WHERE id = $1 AND app_id = $2
	`, cfg.ID, cfg.AppID, cfg.Name, cfg.Type, cfg.URL, eventsJSON, cfg.Enabled)
	return err
}

func (s *NotificationStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM notification_configs WHERE id = $1`, id)
	return err
}

func (s *NotificationStore) Get(ctx context.Context, id string) (*domain.NotificationConfig, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, app_id, name, type, url, events, enabled, created_at
		FROM notification_configs WHERE id = $1
	`, id)
	return scanNotificationConfig(row)
}

func (s *NotificationStore) ListByApp(ctx context.Context, appID string) ([]*domain.NotificationConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, app_id, name, type, url, events, enabled, created_at
		FROM notification_configs WHERE app_id = $1 ORDER BY created_at DESC
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []*domain.NotificationConfig
	for rows.Next() {
		cfg, err := scanNotificationConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (s *NotificationStore) ListEnabledForEvent(ctx context.Context, appID string, event domain.NotificationEventType) ([]*domain.NotificationConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, app_id, name, type, url, events, enabled, created_at
		FROM notification_configs WHERE app_id = $1 AND enabled = true AND events::jsonb @> $2::jsonb
	`, appID, fmt.Sprintf(`["%s"]`, event))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []*domain.NotificationConfig
	for rows.Next() {
		cfg, err := scanNotificationConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotificationConfig(row notificationScanner) (*domain.NotificationConfig, error) {
	var eventsJSON []byte
	cfg := &domain.NotificationConfig{}
	err := row.Scan(&cfg.ID, &cfg.AppID, &cfg.Name, &cfg.Type, &cfg.URL, &eventsJSON, &cfg.Enabled, &cfg.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(eventsJSON) > 0 {
		_ = json.Unmarshal(eventsJSON, &cfg.Events)
	}
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now().UTC()
	}
	return cfg, nil
}
