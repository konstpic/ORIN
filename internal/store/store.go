// Package store is the persistence layer. It is intentionally hand-written
// pgx + plain SQL (no sqlc/orm) to keep MVP dependencies small and queries
// readable.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // register pgx as database/sql driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a row is not found.
var ErrNotFound = errors.New("not found")

// Store wraps a pgx pool plus the typed repositories.
type Store struct {
	Pool           *pgxpool.Pool
	Applications   *Applications
	Repositories   *Repositories
	Clusters       *Clusters
	Projects       *Projects
	Sync           *SyncOperations
	Status         *Statuses
	Audit          *Auditor
	Notifications  *NotificationStore
	SyncHooks      *SyncHookStore
	Roles          *RoleStore
	RoleBindings   *RoleBindingStore
}

// Connect opens a pool and constructs the typed repositories.
func Connect(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{
		Pool:          pool,
		Applications:  &Applications{pool: pool},
		Repositories:  &Repositories{pool: pool},
		Clusters:      &Clusters{pool: pool},
		Projects:      &Projects{pool: pool},
		Sync:          &SyncOperations{pool: pool},
		Status:        &Statuses{pool: pool},
		Audit:         &Auditor{pool: pool},
		Notifications: NewNotificationStore(pool),
		SyncHooks:     NewSyncHookStore(pool),
		Roles:         NewRoleStore(pool),
		RoleBindings:  NewRoleBindingStore(pool),
	}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.Pool.Close() }

// MigrationsFS exposes the embedded SQL files for golang-migrate.
func MigrationsFS() embed.FS { return migrationsFS }

// Migrate runs all up-migrations.
func Migrate(url string) error {
	m, err := newMigrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateDown rolls back one migration.
func MigrateDown(url string) error {
	m, err := newMigrator(url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// MigrateStatus returns the current version and dirty state.
func MigrateStatus(url string) (uint, bool, error) {
	m, err := newMigrator(url)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, d, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, err
	}
	return v, d, nil
}

func newMigrator(url string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open db for migrations: %w", err)
	}
	drv, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return nil, fmt.Errorf("migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", drv)
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return m, nil
}
