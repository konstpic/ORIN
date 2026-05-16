// Package leaderelection provides leader election via PostgreSQL advisory locks.
// Only one process can hold the lock at a time. When the holder's connection
// drops (crash, restart), the lock is released automatically by Postgres.
package leaderelection

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Leader manages a PostgreSQL advisory lock that acts as a leader election
// mechanism. The lock is connection-scoped: if the process crashes or the
// connection is closed, Postgres automatically releases the lock.
type Leader struct {
	pool    *pgxpool.Pool
	lockID  int64
	holding bool
}

// New creates a Leader for the given lock name. The name is hashed to a
// 64-bit integer for the advisory lock ID.
func New(pool *pgxpool.Pool, name string) *Leader {
	return &Leader{
		pool:   pool,
		lockID: hashName(name),
	}
}

// TryAcquire attempts to acquire the advisory lock. Returns true if
// successful, false if another instance holds it.
func (l *Leader) TryAcquire(ctx context.Context) (bool, error) {
	var acquired bool
	err := l.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", l.lockID).Scan(&acquired)
	if err != nil {
		return false, err
	}
	if acquired {
		l.holding = true
		slog.Info("leader lock acquired", "lock", l.lockID)
	}
	return acquired, nil
}

// Release releases the advisory lock.
func (l *Leader) Release(ctx context.Context) error {
	if !l.holding {
		return nil
	}
	_, err := l.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", l.lockID)
	if err != nil {
		return err
	}
	l.holding = false
	slog.Info("leader lock released", "lock", l.lockID)
	return nil
}

// IsHolding returns true if this instance currently holds the lock.
func (l *Leader) IsHolding() bool { return l.holding }

// hashName produces a deterministic int64 from a string for use as an
// advisory lock ID. Uses a simple FNV-1a variant.
func hashName(s string) int64 {
	var h uint64 = 0x811c9dc5
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 0x01000193
	}
	return int64(h)
}

// WaitAndRun repeatedly tries to acquire the lock. When acquired, it runs
// fn and releases the lock when fn returns or ctx is cancelled.
func (l *Leader) WaitAndRun(ctx context.Context, interval time.Duration, fn func(context.Context) error) error {
	for {
		acquired, err := l.TryAcquire(ctx)
		if err != nil {
			return err
		}
		if acquired {
			defer l.Release(context.Background())
			return fn(ctx)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
			// retry
		}
	}
}
