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

// Plugins is the Plugin repository.
type Plugins struct{ pool *pgxpool.Pool }

// NewPlugins constructs a Plugins store.
func NewPlugins(pool *pgxpool.Pool) *Plugins { return &Plugins{pool: pool} }

// Create inserts a new plugin.
func (p *Plugins) Create(ctx context.Context, plugin *domain.Plugin) error {
	if plugin.ID == "" {
		plugin.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	plugin.CreatedAt, plugin.UpdatedAt = now, now

	genJSON, err := json.Marshal(plugin.Generate)
	if err != nil {
		return err
	}
	envJSON, err := nullableEnvJSON(plugin.Env)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO plugins (id, name, generate_json, env_json, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		plugin.ID, plugin.Name, genJSON, envJSON, plugin.CreatedAt, plugin.UpdatedAt)
	return err
}

// Update replaces a plugin's generate spec and env.
func (p *Plugins) Update(ctx context.Context, plugin *domain.Plugin) error {
	plugin.UpdatedAt = time.Now().UTC()
	genJSON, err := json.Marshal(plugin.Generate)
	if err != nil {
		return err
	}
	envJSON, err := nullableEnvJSON(plugin.Env)
	if err != nil {
		return err
	}
	tag, err := p.pool.Exec(ctx, `
		UPDATE plugins SET generate_json=$2, env_json=$3, updated_at=$4
		WHERE id=$1`,
		plugin.ID, genJSON, envJSON, plugin.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a plugin by id.
func (p *Plugins) Delete(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM plugins WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID fetches a plugin by id.
func (p *Plugins) GetByID(ctx context.Context, id string) (*domain.Plugin, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, name, generate_json, env_json, created_at, updated_at FROM plugins WHERE id=$1`, id)
	return scanPlugin(row)
}

// GetByName fetches a plugin by its unique name.
func (p *Plugins) GetByName(ctx context.Context, name string) (*domain.Plugin, error) {
	row := p.pool.QueryRow(ctx,
		`SELECT id, name, generate_json, env_json, created_at, updated_at FROM plugins WHERE name=$1`, name)
	return scanPlugin(row)
}

// List returns all plugins, oldest first.
func (p *Plugins) List(ctx context.Context) ([]*domain.Plugin, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, name, generate_json, env_json, created_at, updated_at FROM plugins ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Plugin
	for rows.Next() {
		pl, err := scanPlugin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pl)
	}
	return out, rows.Err()
}

func scanPlugin(s scanner) (*domain.Plugin, error) {
	var pl domain.Plugin
	var genJSON []byte
	var envJSON []byte
	if err := s.Scan(&pl.ID, &pl.Name, &genJSON, &envJSON, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(genJSON, &pl.Generate); err != nil {
		return nil, err
	}
	if len(envJSON) > 0 {
		if err := json.Unmarshal(envJSON, &pl.Env); err != nil {
			return nil, err
		}
	}
	return &pl, nil
}

func nullableEnvJSON(env []domain.EnvVar) (any, error) {
	if len(env) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return b, nil
}
