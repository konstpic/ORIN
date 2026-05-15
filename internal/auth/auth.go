// Package auth implements the (MVP) static-token authentication middleware.
// Replace with OIDC by swapping the middleware constructor.
package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

type ctxKey string

const userCtxKey ctxKey = "k8sui.user"

// User is the authenticated principal.
type User struct {
	Subject  string
	Role     string
	Projects []string // "*" means all; populated by OIDC in a future release.
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

// CanAccessProject reports whether u may see or mutate resources in project.
func CanAccessProject(u User, project string) bool {
	if u.Role == "admin" {
		return true
	}
	if slices.Contains(u.Projects, "*") {
		return true
	}
	return slices.Contains(u.Projects, project)
}

// StaticToken returns middleware that accepts a fixed bearer token.
// It also accepts the same token as a `?token=` query parameter for the
// WebSocket upgrade (browsers cannot set custom headers there).
func StaticToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if presented := extract(r); presented != "" && presented == token {
				ctx := WithUser(r.Context(), User{Subject: "admin", Role: "admin", Projects: []string{"*"}})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

func extract(r *http.Request) string {
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
