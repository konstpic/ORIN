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

// Projects manages Argo-style project records.
type Projects struct{ pool *pgxpool.Pool }

// List returns all projects ordered by name.
func (p *Projects) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, name, description, COALESCE(policies_json, '{}'), created_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Project
	for rows.Next() {
		pr, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

// GetByName fetches one project by name.
func (p *Projects) GetByName(ctx context.Context, name string) (*domain.Project, error) {
	row := p.pool.QueryRow(ctx, `SELECT id, name, description, COALESCE(policies_json, '{}'), created_at FROM projects WHERE name=$1`, name)
	pr, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

// Create inserts a new project.
func (p *Projects) Create(ctx context.Context, pr *domain.Project) error {
	if pr.ID == "" {
		pr.ID = uuid.NewString()
	}
	if pr.CreatedAt.IsZero() {
		pr.CreatedAt = time.Now().UTC()
	}
	pol, err := json.Marshal(pr.Policies)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO projects (id, name, description, policies_json, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		pr.ID, pr.Name, pr.Description, pol, pr.CreatedAt)
	return err
}

// Update modifies description and policies for an existing project.
func (p *Projects) Update(ctx context.Context, pr *domain.Project) error {
	pol, err := json.Marshal(pr.Policies)
	if err != nil {
		return err
	}
	tag, err := p.pool.Exec(ctx, `
		UPDATE projects SET description=$2, policies_json=$3 WHERE id=$1`,
		pr.ID, pr.Description, pol)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type projectScanner interface {
	Scan(dest ...any) error
}

func scanProject(s projectScanner) (domain.Project, error) {
	var pr domain.Project
	var polJSON []byte
	if err := s.Scan(&pr.ID, &pr.Name, &pr.Description, &polJSON, &pr.CreatedAt); err != nil {
		return domain.Project{}, err
	}
	if len(polJSON) > 0 {
		if err := json.Unmarshal(polJSON, &pr.Policies); err != nil {
			return domain.Project{}, err
		}
	}
	return pr, nil
}
