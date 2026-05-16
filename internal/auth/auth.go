// Package auth implements token-based authentication with RBAC.
// It validates bearer tokens, loads user roles/permissions from the database,
// and provides permission-checking helpers.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/k8s-ui/k8s-ui/internal/rbac"
)

type ctxKey string

const userCtxKey ctxKey = "k8sui.user"

// User is the authenticated principal with resolved permissions.
type User struct {
	ID              string
	Subject         string // alias for ID, kept for backward compat
	Email           string
	DisplayName     string
	Role            string                     // primary role name
	Permissions     map[rbac.Permission]bool   // flattened from all bindings
	Projects        []string                   // "*" means all; union of all binding scopes
	BindingProjects map[string][]string        // roleID -> projects for that binding
}

// WithUser returns a new context carrying u.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// FromContext returns the authenticated user, if any.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}

// HasPermission returns true if the user has the given permission.
func (u *User) HasPermission(p rbac.Permission) bool {
	if u == nil {
		return false
	}
	return u.Permissions[p]
}

// CanAccessProject reports whether u may see or mutate resources in project.
func (u *User) CanAccessProject(project string) bool {
	if u == nil {
		return false
	}
	if slices.Contains(u.Projects, "*") {
		return true
	}
	return slices.Contains(u.Projects, project)
}

// ExtractBearer extracts the bearer token from the request.
func ExtractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("k8sui-token"); err == nil {
		return c.Value
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// TokenAuth is middleware that validates bearer tokens against the database
// and loads the user's roles + permissions into context.
type TokenAuth struct {
	pool       *pgxpool.Pool
	adminToken string
	cache      map[string]*cacheEntry
	mu         sync.RWMutex
	cacheTTL   time.Duration
}

type cacheEntry struct {
	user      User
	expiresAt time.Time
}

// NewTokenAuth creates a new TokenAuth middleware.
func NewTokenAuth(pool *pgxpool.Pool, adminToken string, cacheTTL time.Duration) *TokenAuth {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &TokenAuth{
		pool:       pool,
		adminToken: adminToken,
		cache:      make(map[string]*cacheEntry),
		cacheTTL:   cacheTTL,
	}
}

// Middleware returns the chi middleware handler.
func (ta *TokenAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearer(r)
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Check fallback admin token (bootstrap / static token)
			if ta.adminToken != "" && token == ta.adminToken {
				user, err := ta.loadAdminUser(r.Context())
				if err != nil {
					// DB unavailable — fall back to hardcoded full admin
					user = fullAdminUser()
				}
				ctx := WithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Check cache
			if user, ok := ta.fromCache(token); ok {
				ctx := WithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Validate token against DB
			user, err := ta.validateToken(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ta.setCache(token, user)
			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (ta *TokenAuth) fromCache(token string) (User, bool) {
	ta.mu.RLock()
	defer ta.mu.RUnlock()
	entry, ok := ta.cache[token]
	if !ok || time.Now().After(entry.expiresAt) {
		return User{}, false
	}
	return entry.user, true
}

func (ta *TokenAuth) setCache(token string, user User) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	ta.cache[token] = &cacheEntry{user: user, expiresAt: time.Now().Add(ta.cacheTTL)}
}

func (ta *TokenAuth) validateToken(ctx context.Context, token string) (User, error) {
	var userID, email, displayName, role string
	var tokenHash string
	var active bool

	err := ta.pool.QueryRow(ctx, `
		SELECT id, email, display_name, role, token_hash, active
		FROM users WHERE token_hash IS NOT NULL
	`).Scan(&userID, &email, &displayName, &role, &tokenHash, &active)
	if err != nil {
		return User{}, err
	}
	if !active {
		return User{}, bcrypt.ErrMismatchedHashAndPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(token)); err != nil {
		return User{}, err
	}
	return ta.buildUser(ctx, userID, email, displayName, role)
}

func (ta *TokenAuth) loadAdminUser(ctx context.Context) (User, error) {
	var userID, email string
	err := ta.pool.QueryRow(ctx, `
		SELECT id, email FROM users WHERE role = 'admin' AND active = true LIMIT 1
	`).Scan(&userID, &email)
	if err != nil {
		return fullAdminUser(), nil
	}

	// Admin token = full access regardless of DB bindings
	user := fullAdminUser()
	user.ID = userID
	user.Subject = userID
	user.Email = email
	return user, nil
}

func fullAdminUser() User {
	perms := make(map[rbac.Permission]bool)
	for _, p := range rbac.AllPermissions() {
		perms[p] = true
	}
	return User{
		ID:          "admin",
		Subject:     "admin",
		Email:       "admin@k8s-ui.local",
		DisplayName: "Administrator",
		Role:        "admin",
		Permissions: perms,
		Projects:    []string{"*"},
	}
}

func (ta *TokenAuth) buildUser(ctx context.Context, userID, email, displayName, primaryRole string) (User, error) {
	rows, err := ta.pool.Query(ctx, `
		SELECT rb.role_id, rb.projects, r.name
		FROM role_bindings rb
		JOIN roles r ON r.id = rb.role_id
		WHERE rb.user_id = $1
	`, userID)
	if err != nil {
		return User{}, err
	}
	defer rows.Close()

	permissions := make(map[rbac.Permission]bool)
	var allProjects []string
	bindingProjects := make(map[string][]string)
	var roleNames []string

	for rows.Next() {
		var roleID string
		var projectsJSON []byte
		var roleName string
		if err := rows.Scan(&roleID, &projectsJSON, &roleName); err != nil {
			return User{}, err
		}
		roleNames = append(roleNames, roleName)

		permRows, err := ta.pool.Query(ctx, `SELECT permission FROM role_permissions WHERE role_id = $1`, roleID)
		if err != nil {
			return User{}, err
		}
		for permRows.Next() {
			var perm string
			if err := permRows.Scan(&perm); err != nil {
				permRows.Close()
				return User{}, err
			}
			permissions[rbac.Permission(perm)] = true
		}
		permRows.Close()

		var projs []string
		if len(projectsJSON) > 0 && string(projectsJSON) != "null" {
			_ = json.Unmarshal(projectsJSON, &projs)
		}
		bindingProjects[roleID] = projs
		allProjects = append(allProjects, projs...)
	}

	if len(roleNames) == 0 {
		return User{
			ID: userID, Subject: userID, Email: email, DisplayName: displayName,
			Role: primaryRole, Permissions: permissions, Projects: []string{"*"},
		}, nil
	}

	seen := make(map[string]bool)
	var uniqueProjects []string
	for _, p := range allProjects {
		if p == "*" {
			uniqueProjects = []string{"*"}
			break
		}
		if !seen[p] {
			seen[p] = true
			uniqueProjects = append(uniqueProjects, p)
		}
	}

	// Empty projects list means no project restrictions — access to all projects.
	if len(uniqueProjects) == 0 {
		uniqueProjects = []string{"*"}
	}

	return User{
		ID:              userID,
		Subject:         userID,
		Email:           email,
		DisplayName:     displayName,
		Role:            roleNames[0],
		Permissions:     permissions,
		Projects:        uniqueProjects,
		BindingProjects: bindingProjects,
	}, nil
}

// StaticToken returns middleware that accepts a fixed bearer token (MVP / bootstrap).
// Kept for backward compatibility.
func StaticToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if presented := ExtractBearer(r); presented != "" && presented == token {
				perms := make(map[rbac.Permission]bool)
				for _, p := range rbac.AllPermissions() {
					perms[p] = true
				}
				ctx := WithUser(r.Context(), User{
					ID: "admin", Subject: "admin", Email: "admin@k8s-ui.local", Role: "admin",
					Permissions: perms, Projects: []string{"*"},
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// CanAccessProject is the legacy helper kept for backward compatibility with
// existing handlers that call auth.CanAccessProject(u, project).
func CanAccessProject(u User, project string) bool {
	return u.CanAccessProject(project)
}
