// Package rbacenforce provides HTTP middleware and helpers for enforcing
// RBAC permissions on API requests.
package rbacenforce

import (
	"encoding/json"
	"net/http"

	"github.com/k8s-ui/k8s-ui/internal/auth"
	"github.com/k8s-ui/k8s-ui/internal/rbac"
)

// RequirePermission returns a chi middleware that checks the user has the
// given permission. Returns 403 Forbidden if not.
func RequirePermission(perm rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := auth.FromContext(r.Context())
			if !ok {
				writeForbidden(w, "unauthorized", "authentication required")
				return
			}
			if !u.HasPermission(perm) {
				writeForbidden(w, "forbidden", "missing permission: "+string(perm))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermissionAndProject returns a handler wrapper that checks both a
// permission and project access. The project name is extracted via the
// projectFn callback.
func RequirePermissionAndProject(perm rbac.Permission, projectFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := auth.FromContext(r.Context())
			if !ok {
				writeForbidden(w, "unauthorized", "authentication required")
				return
			}
			if !u.HasPermission(perm) {
				writeForbidden(w, "forbidden", "missing permission: "+string(perm))
				return
			}
			project := projectFn(r)
			if project != "" && !u.CanAccessProject(project) {
				writeForbidden(w, "project_forbidden", "no access to project: "+project)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission checks the user has perm, writing 403 if not.
// Use this inline in handlers for per-action checks.
func CheckPermission(w http.ResponseWriter, r *http.Request, perm rbac.Permission) bool {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeForbidden(w, "unauthorized", "authentication required")
		return false
	}
	if !u.HasPermission(perm) {
		writeForbidden(w, "forbidden", "missing permission: "+string(perm))
		return false
	}
	return true
}

// RequireProjectAccess checks the user can access the given project.
func RequireProjectAccess(w http.ResponseWriter, r *http.Request, project string) bool {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeForbidden(w, "unauthorized", "authentication required")
		return false
	}
	if !u.CanAccessProject(project) {
		writeForbidden(w, "project_forbidden", "no access to project: "+project)
		return false
	}
	return true
}

func writeForbidden(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": msg,
	})
}
