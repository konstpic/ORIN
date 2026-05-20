package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orin/orin/internal/domain"
)

// Repositories is the Repository repo (sorry).
type Repositories struct{ pool *pgxpool.Pool }

// Create inserts a repository.
func (r *Repositories) Create(ctx context.Context, repo *domain.Repository) error {
	if repo.ID == "" {
		repo.ID = uuid.NewString()
	}
	if repo.CreatedAt.IsZero() {
		repo.CreatedAt = time.Now().UTC()
	}
	if repo.Type == "" {
		repo.Type = domain.RepoTypeGit
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO repositories (id, url, type, credentials_encrypted, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		repo.ID, repo.URL, repo.Type, repo.CredentialsEncrypted, repo.CreatedAt)
	return err
}

// GetByID fetches a repo by id.
func (r *Repositories) GetByID(ctx context.Context, id string) (*domain.Repository, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, url, type, credentials_encrypted, created_at FROM repositories WHERE id=$1`, id)
	return scanRepo(row)
}

// GetByURL looks up by URL.
func (r *Repositories) GetByURL(ctx context.Context, url string) (*domain.Repository, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, url, type, credentials_encrypted, created_at FROM repositories WHERE url=$1`, url)
	return scanRepo(row)
}

// List returns all repositories.
func (r *Repositories) List(ctx context.Context) ([]*domain.Repository, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, url, type, credentials_encrypted, created_at FROM repositories ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Repository
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, repo)
	}
	return out, rows.Err()
}

// Delete removes a repository by id.
func (r *Repositories) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM repositories WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRepo(s scanner) (*domain.Repository, error) {
	var repo domain.Repository
	if err := s.Scan(&repo.ID, &repo.URL, &repo.Type, &repo.CredentialsEncrypted, &repo.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &repo, nil
}

// Clusters is the Cluster repository.
type Clusters struct{ pool *pgxpool.Pool }

// Upsert idempotently registers a cluster by name.
func (c *Clusters) Upsert(ctx context.Context, cl *domain.Cluster) error {
	if cl.ID == "" {
		cl.ID = uuid.NewString()
	}
	if cl.CreatedAt.IsZero() {
		cl.CreatedAt = time.Now().UTC()
	}
	_, err := c.pool.Exec(ctx, `
		INSERT INTO clusters (id, name, server_url, ca_cert, auth_config_encrypted, in_cluster, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (name) DO UPDATE SET
		  server_url=EXCLUDED.server_url,
		  ca_cert=EXCLUDED.ca_cert,
		  auth_config_encrypted=EXCLUDED.auth_config_encrypted,
		  in_cluster=EXCLUDED.in_cluster`,
		cl.ID, cl.Name, cl.ServerURL, cl.CACert, cl.AuthConfigEncrypted, cl.InCluster, cl.CreatedAt)
	return err
}

// GetByID fetches a cluster by id.
func (c *Clusters) GetByID(ctx context.Context, id string) (*domain.Cluster, error) {
	row := c.pool.QueryRow(ctx, `SELECT id, name, server_url, ca_cert, auth_config_encrypted, in_cluster, created_at FROM clusters WHERE id=$1`, id)
	return scanCluster(row)
}

// GetByName fetches a cluster by its unique name.
func (c *Clusters) GetByName(ctx context.Context, name string) (*domain.Cluster, error) {
	row := c.pool.QueryRow(ctx, `SELECT id, name, server_url, ca_cert, auth_config_encrypted, in_cluster, created_at FROM clusters WHERE name=$1`, name)
	return scanCluster(row)
}

// List returns all clusters.
func (c *Clusters) List(ctx context.Context) ([]*domain.Cluster, error) {
	rows, err := c.pool.Query(ctx, `SELECT id, name, server_url, ca_cert, auth_config_encrypted, in_cluster, created_at FROM clusters ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Cluster
	for rows.Next() {
		cl, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cl)
	}
	return out, rows.Err()
}

func scanCluster(s scanner) (*domain.Cluster, error) {
	var cl domain.Cluster
	if err := s.Scan(&cl.ID, &cl.Name, &cl.ServerURL, &cl.CACert, &cl.AuthConfigEncrypted, &cl.InCluster, &cl.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &cl, nil
}
