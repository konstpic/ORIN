package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k8s-ui/k8s-ui/internal/rbac"
)

// RoleStore handles persistence of roles and their permissions.
type RoleStore struct {
	pool *pgxpool.Pool
}

// NewRoleStore creates a new RoleStore.
func NewRoleStore(pool *pgxpool.Pool) *RoleStore {
	return &RoleStore{pool: pool}
}

// Create inserts a new role and its permissions in a transaction.
func (s *RoleStore) Create(ctx context.Context, r *rbac.Role) error {
	now := time.Now()
	r.CreatedAt = now
	r.UpdatedAt = now

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO roles (id, name, display_name, description, built_in, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, r.ID, r.Name, r.DisplayName, r.Description, r.BuiltIn, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return err
	}

	if err := s.setPermissionsInTx(ctx, tx, r.ID, r.Permissions); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Get returns a role by ID, including its permissions.
func (s *RoleStore) Get(ctx context.Context, id string) (*rbac.Role, error) {
	r := &rbac.Role{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, display_name, description, built_in, created_at, updated_at
		FROM roles WHERE id = $1
	`, id).Scan(&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.BuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	perms, err := s.getPermissions(ctx, id)
	if err != nil {
		return nil, err
	}
	r.Permissions = perms
	return r, nil
}

// GetByName returns a role by name, including its permissions.
func (s *RoleStore) GetByName(ctx context.Context, name string) (*rbac.Role, error) {
	r := &rbac.Role{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, display_name, description, built_in, created_at, updated_at
		FROM roles WHERE name = $1
	`, name).Scan(&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.BuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	perms, err := s.getPermissions(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	r.Permissions = perms
	return r, nil
}

// List returns all roles with their permissions.
func (s *RoleStore) List(ctx context.Context) ([]*rbac.Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, display_name, description, built_in, created_at, updated_at
		FROM roles ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]*rbac.Role, 0)
	for rows.Next() {
		r := &rbac.Role{}
		if err := rows.Scan(&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.BuiltIn, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		perms, err := s.getPermissions(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		r.Permissions = perms
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// Update modifies a role and its permissions.
func (s *RoleStore) Update(ctx context.Context, r *rbac.Role) error {
	r.UpdatedAt = time.Now()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE roles SET display_name = $1, description = $2, updated_at = $3
		WHERE id = $4
	`, r.DisplayName, r.Description, r.UpdatedAt, r.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if err := s.setPermissionsInTx(ctx, tx, r.ID, r.Permissions); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Delete removes a role and its permissions.
func (s *RoleStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *RoleStore) getPermissions(ctx context.Context, roleID string) ([]rbac.Permission, error) {
	rows, err := s.pool.Query(ctx, `SELECT permission FROM role_permissions WHERE role_id = $1 ORDER BY permission`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []rbac.Permission
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, rbac.Permission(p))
	}
	return perms, rows.Err()
}

func (s *RoleStore) setPermissionsInTx(ctx context.Context, tx pgx.Tx, roleID string, perms []rbac.Permission) error {
	_, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return err
	}

	for _, p := range perms {
		_, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
		if err != nil {
			return err
		}
	}
	return nil
}

// RoleBindingStore handles persistence of role bindings.
type RoleBindingStore struct {
	pool *pgxpool.Pool
}

// NewRoleBindingStore creates a new RoleBindingStore.
func NewRoleBindingStore(pool *pgxpool.Pool) *RoleBindingStore {
	return &RoleBindingStore{pool: pool}
}

// Create inserts a new role binding.
func (s *RoleBindingStore) Create(ctx context.Context, b *rbac.RoleBinding) error {
	b.CreatedAt = time.Now()
	projectsJSON, err := json.Marshal(b.Projects)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO role_bindings (id, user_id, role_id, projects, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, b.ID, b.UserID, b.RoleID, projectsJSON, b.CreatedAt)
	return err
}

// Get returns a role binding by ID.
func (s *RoleBindingStore) Get(ctx context.Context, id string) (*rbac.RoleBinding, error) {
	b := &rbac.RoleBinding{}
	var projectsJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, role_id, projects, created_at
		FROM role_bindings WHERE id = $1
	`, id).Scan(&b.ID, &b.UserID, &b.RoleID, &projectsJSON, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(projectsJSON, &b.Projects); err != nil {
		return nil, err
	}
	return b, nil
}

// ListByUser returns all role bindings for a user.
func (s *RoleBindingStore) ListByUser(ctx context.Context, userID string) ([]*rbac.RoleBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, role_id, projects, created_at
		FROM role_bindings WHERE user_id = $1 ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]*rbac.RoleBinding, 0)
	for rows.Next() {
		b := &rbac.RoleBinding{}
		var projectsJSON []byte
		if err := rows.Scan(&b.ID, &b.UserID, &b.RoleID, &projectsJSON, &b.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(projectsJSON, &b.Projects); err != nil {
			return nil, err
		}
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

// List returns all role bindings.
func (s *RoleBindingStore) List(ctx context.Context) ([]*rbac.RoleBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, role_id, projects, created_at
		FROM role_bindings ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]*rbac.RoleBinding, 0)
	for rows.Next() {
		b := &rbac.RoleBinding{}
		var projectsJSON []byte
		if err := rows.Scan(&b.ID, &b.UserID, &b.RoleID, &projectsJSON, &b.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(projectsJSON, &b.Projects); err != nil {
			return nil, err
		}
		bindings = append(bindings, b)
	}
	return bindings, rows.Err()
}

// Update modifies a role binding.
func (s *RoleBindingStore) Update(ctx context.Context, b *rbac.RoleBinding) error {
	projectsJSON, err := json.Marshal(b.Projects)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE role_bindings SET role_id = $1, projects = $2 WHERE id = $3
	`, b.RoleID, projectsJSON, b.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Delete removes a role binding.
func (s *RoleBindingStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM role_bindings WHERE id = $1`, id)
	return err
}

// DeleteByUser removes all role bindings for a user.
func (s *RoleBindingStore) DeleteByUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM role_bindings WHERE user_id = $1`, userID)
	return err
}
