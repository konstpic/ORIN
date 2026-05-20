package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orin/orin/internal/domain"
)

// Applications is the Application repository.
type Applications struct{ pool *pgxpool.Pool }

// Create inserts a new application and an empty status row.
func (a *Applications) Create(ctx context.Context, app *domain.Application) error {
	if app.ID == "" {
		app.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	app.CreatedAt, app.UpdatedAt = now, now
	policy, err := json.Marshal(app.SyncPolicy)
	if err != nil {
		return err
	}
	hvFiles, err := nullableStringSliceJSON(app.HelmValueFiles)
	if err != nil {
		return err
	}
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	_, err = tx.Exec(ctx, `
		INSERT INTO applications (id, name, project, repo_id, path, target_revision,
		                          dest_cluster_id, dest_namespace, sync_policy_json,
		                          helm_values_json, helm_value_files_json, parent_app, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		app.ID, app.Name, app.Project, app.RepoID, app.Path, app.TargetRevision,
		app.DestClusterID, app.DestNamespace, policy,
		nullableHelmValuesJSON(app.HelmValuesJSON), hvFiles,
		app.ParentApp,
		app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO application_status (app_id) VALUES ($1)`, app.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Update mutates the application source/destination/policy.
func (a *Applications) Update(ctx context.Context, app *domain.Application) error {
	app.UpdatedAt = time.Now().UTC()
	policy, err := json.Marshal(app.SyncPolicy)
	if err != nil {
		return err
	}
	hvFiles, err := nullableStringSliceJSON(app.HelmValueFiles)
	if err != nil {
		return err
	}
	tag, err := a.pool.Exec(ctx, `
		UPDATE applications SET
		  repo_id=$2, path=$3, target_revision=$4,
		  dest_cluster_id=$5, dest_namespace=$6, sync_policy_json=$7,
		  helm_values_json=$8, helm_value_files_json=$9,
		  parent_app=$10, updated_at=$11
		WHERE id=$1`,
		app.ID, app.RepoID, app.Path, app.TargetRevision,
		app.DestClusterID, app.DestNamespace, policy,
		nullableHelmValuesJSON(app.HelmValuesJSON), hvFiles,
		app.ParentApp, app.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByParent returns all applications that have the given parent app name.
func (a *Applications) ListByParent(ctx context.Context, parentName string) ([]*domain.Application, error) {
	rows, err := a.pool.Query(ctx, fmt.Sprintf(`%s WHERE parent_app=$1 ORDER BY name`, baseAppSelect), parentName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Application
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, rows.Err()
}

// Delete removes the application; status and sync history cascade.
func (a *Applications) Delete(ctx context.Context, id string) error {
	tag, err := a.pool.Exec(ctx, `DELETE FROM applications WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByName fetches a single application by its unique name.
func (a *Applications) GetByName(ctx context.Context, name string) (*domain.Application, error) {
	return a.queryOne(ctx, `WHERE name=$1`, name)
}

// GetByID fetches a single application by id.
func (a *Applications) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	return a.queryOne(ctx, `WHERE id=$1`, id)
}

// List returns all applications, oldest first.
func (a *Applications) List(ctx context.Context) ([]*domain.Application, error) {
	rows, err := a.pool.Query(ctx, fmt.Sprintf(`%s ORDER BY created_at`, baseAppSelect))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Application
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, rows.Err()
}

// ListByRepoRevision returns apps subscribed to a repo and ref pair.
func (a *Applications) ListByRepoRevision(ctx context.Context, repoID, revision string) ([]*domain.Application, error) {
	rows, err := a.pool.Query(ctx, fmt.Sprintf(`%s WHERE repo_id=$1 AND target_revision=$2`, baseAppSelect), repoID, revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Application
	for rows.Next() {
		app, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, rows.Err()
}

const baseAppSelect = `SELECT id, name, project, repo_id, path, target_revision,
		dest_cluster_id, dest_namespace, sync_policy_json, helm_values_json, helm_value_files_json, parent_app, created_at, updated_at
	FROM applications`

func (a *Applications) queryOne(ctx context.Context, where string, args ...any) (*domain.Application, error) {
	row := a.pool.QueryRow(ctx, fmt.Sprintf(`%s %s`, baseAppSelect, where), args...)
	app, err := scanApp(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return app, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanApp(s scanner) (*domain.Application, error) {
	var app domain.Application
	var policy []byte
	var helm []byte
	var helmFiles []byte
	if err := s.Scan(
		&app.ID, &app.Name, &app.Project, &app.RepoID, &app.Path,
		&app.TargetRevision, &app.DestClusterID, &app.DestNamespace,
		&policy, &helm, &helmFiles, &app.ParentApp, &app.CreatedAt, &app.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(policy, &app.SyncPolicy); err != nil {
		return nil, err
	}
	if len(helm) > 0 {
		app.HelmValuesJSON = append([]byte(nil), helm...)
	}
	if len(helmFiles) > 0 {
		if err := json.Unmarshal(helmFiles, &app.HelmValueFiles); err != nil {
			return nil, fmt.Errorf("unmarshal helm_value_files_json: %w", err)
		}
	}
	return &app, nil
}

func nullableStringSliceJSON(ss []string) (any, error) {
	if len(ss) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(ss)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func nullableHelmValuesJSON(b []byte) any {
	if b == nil {
		return nil
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}
