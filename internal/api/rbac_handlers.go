package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/k8s-ui/k8s-ui/internal/rbac"
	"github.com/k8s-ui/k8s-ui/internal/rbacenforce"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

// --- Roles ---

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	roles, err := s.opts.Store.Roles.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]apiv1.Role, len(roles))
	for i, role := range roles {
		out[i] = toAPIRole(role)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getRole(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	role, err := s.opts.Store.Roles.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPIRole(role))
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	var req apiv1.CreateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	perms := make([]rbac.Permission, len(req.Permissions))
	for i, p := range req.Permissions {
		perms[i] = rbac.Permission(p)
	}

	role := &rbac.Role{
		ID:          generateID(),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Permissions: perms,
		BuiltIn:     false,
	}
	if err := s.opts.Store.Roles.Create(r.Context(), role); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIRole(role))
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	role, err := s.opts.Store.Roles.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.BuiltIn {
		writeError(w, http.StatusForbidden, "forbidden", "built-in roles cannot be modified")
		return
	}

	var req apiv1.UpdateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	role.DisplayName = req.DisplayName
	role.Description = req.Description
	perms := make([]rbac.Permission, len(req.Permissions))
	for i, p := range req.Permissions {
		perms[i] = rbac.Permission(p)
	}
	role.Permissions = perms

	if err := s.opts.Store.Roles.Update(r.Context(), role); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIRole(role))
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	role, err := s.opts.Store.Roles.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if role.BuiltIn {
		writeError(w, http.StatusForbidden, "forbidden", "built-in roles cannot be deleted")
		return
	}
	if err := s.opts.Store.Roles.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Role Bindings ---

func (s *Server) listRoleBindings(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	bindings, err := s.opts.Store.RoleBindings.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]apiv1.RoleBinding, len(bindings))
	for i, b := range bindings {
		out[i] = toAPIRoleBinding(r, s, b)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createRoleBinding(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	var req apiv1.CreateRoleBindingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	binding := &rbac.RoleBinding{
		ID:       generateID(),
		UserID:   req.UserID,
		RoleID:   req.RoleID,
		Projects: req.Projects,
	}
	if err := s.opts.Store.RoleBindings.Create(r.Context(), binding); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toAPIRoleBinding(r, s, binding))
}

func (s *Server) updateRoleBinding(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	binding, err := s.opts.Store.RoleBindings.Get(r.Context(), id)
	if err != nil {
		notFoundOr500(w, err)
		return
	}

	var req apiv1.UpdateRoleBindingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	binding.RoleID = req.RoleID
	binding.Projects = req.Projects

	if err := s.opts.Store.RoleBindings.Update(r.Context(), binding); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toAPIRoleBinding(r, s, binding))
}

func (s *Server) deleteRoleBinding(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.opts.Store.RoleBindings.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Permissions catalog ---

func (s *Server) listPermissions(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	perms := rbac.AllPermissions()
	out := make([]apiv1.PermissionInfo, len(perms))
	for i, p := range perms {
		cat, desc := permissionMetadata(p)
		out[i] = apiv1.PermissionInfo{
			ID:          string(p),
			Category:    cat,
			Description: desc,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Users ---

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	rows, err := s.opts.Store.Pool.Query(r.Context(), `
		SELECT id, email, display_name, role, active FROM users ORDER BY email
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	defer rows.Close()

	users := make([]apiv1.UserInfo, 0)
	for rows.Next() {
		var u apiv1.UserInfo
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	var u apiv1.UserInfo
	err := s.opts.Store.Pool.QueryRow(r.Context(), `
		SELECT id, email, display_name, role, active FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	var req apiv1.CreateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	id := generateID()
	tokenHash, err := hashToken(req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to hash token")
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Email
	}

	_, err = s.opts.Store.Pool.Exec(r.Context(), `
		INSERT INTO users (id, email, display_name, role, token_hash, active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, id, req.Email, displayName, req.Role, tokenHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	var u apiv1.UserInfo
	err = s.opts.Store.Pool.QueryRow(r.Context(), `
		SELECT id, email, display_name, role, active FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	var req apiv1.UpdateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.DisplayName != "" {
		updates = append(updates, "display_name = $"+itoa(argIdx))
		args = append(args, req.DisplayName)
		argIdx++
	}
	if req.Active != nil {
		updates = append(updates, "active = $"+itoa(argIdx))
		args = append(args, *req.Active)
		argIdx++
	}
	if req.Token != "" {
		h, err := hashToken(req.Token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to hash token")
			return
		}
		updates = append(updates, "token_hash = $"+itoa(argIdx))
		args = append(args, h)
		argIdx++
	}

	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "no fields to update")
		return
	}

	args = append(args, id)
	query := "UPDATE users SET " + joinStrings(updates, ", ") + " WHERE id = $" + itoa(argIdx)
	_, err := s.opts.Store.Pool.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	var u apiv1.UserInfo
	err = s.opts.Store.Pool.QueryRow(r.Context(), `
		SELECT id, email, display_name, role, active FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRBACManage) {
		return
	}
	id := chi.URLParam(r, "id")
	_, err := s.opts.Store.Pool.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

func toAPIRole(r *rbac.Role) apiv1.Role {
	perms := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = string(p)
	}
	return apiv1.Role{
		ID:          r.ID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Description: r.Description,
		Permissions: perms,
		BuiltIn:     r.BuiltIn,
		CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toAPIRoleBinding(r *http.Request, s *Server, b *rbac.RoleBinding) apiv1.RoleBinding {
	out := apiv1.RoleBinding{
		ID:       b.ID,
		UserID:   b.UserID,
		RoleID:   b.RoleID,
		Projects: b.Projects,
		CreatedAt: b.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	// Look up user email
	var email string
	_ = s.opts.Store.Pool.QueryRow(r.Context(), `SELECT email FROM users WHERE id = $1`, b.UserID).Scan(&email)
	out.UserEmail = email

	// Look up role name
	var roleName string
	_ = s.opts.Store.Pool.QueryRow(r.Context(), `SELECT name FROM roles WHERE id = $1`, b.RoleID).Scan(&roleName)
	out.RoleName = roleName

	return out
}

func hashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func permissionMetadata(p rbac.Permission) (category, description string) {
	switch p {
	case rbac.PermAppList:
		return "applications", "List applications"
	case rbac.PermAppGet:
		return "applications", "Get application details"
	case rbac.PermAppCreate:
		return "applications", "Create applications"
	case rbac.PermAppUpdate:
		return "applications", "Update applications"
	case rbac.PermAppDelete:
		return "applications", "Delete applications"
	case rbac.PermAppSync:
		return "applications", "Sync applications"
	case rbac.PermAppRollback:
		return "applications", "Rollback applications"
	case rbac.PermAppRefresh:
		return "applications", "Refresh applications"
	case rbac.PermPodLogs:
		return "pods", "View pod logs"
	case rbac.PermPodExec:
		return "pods", "Exec into pods"
	case rbac.PermPodShell:
		return "pods", "Open pod shell"
	case rbac.PermPodDelete:
		return "pods", "Delete pods"
	case rbac.PermLiveGet:
		return "live-resource", "View live resources"
	case rbac.PermLiveEdit:
		return "live-resource", "Edit live resources"
	case rbac.PermLiveApply:
		return "live-resource", "Apply live resources"
	case rbac.PermLiveDelete:
		return "live-resource", "Delete live resources"
	case rbac.PermLiveRestart:
		return "live-resource", "Restart live resources"
	case rbac.PermRepoList:
		return "repositories", "List repositories"
	case rbac.PermRepoGet:
		return "repositories", "Get repository details"
	case rbac.PermRepoCreate:
		return "repositories", "Create repositories"
	case rbac.PermRepoDelete:
		return "repositories", "Delete repositories"
	case rbac.PermClusterList:
		return "clusters", "List clusters"
	case rbac.PermClusterGet:
		return "clusters", "Get cluster details"
	case rbac.PermClusterCreate:
		return "clusters", "Create clusters"
	case rbac.PermClusterDelete:
		return "clusters", "Delete clusters"
	case rbac.PermProjectList:
		return "projects", "List projects"
	case rbac.PermProjectGet:
		return "projects", "Get project details"
	case rbac.PermProjectCreate:
		return "projects", "Create projects"
	case rbac.PermProjectUpdate:
		return "projects", "Update projects"
	case rbac.PermProjectDelete:
		return "projects", "Delete projects"
	case rbac.PermNotificationList:
		return "notifications", "List notification configs"
	case rbac.PermNotificationCreate:
		return "notifications", "Create notification configs"
	case rbac.PermNotificationUpdate:
		return "notifications", "Update notification configs"
	case rbac.PermNotificationDelete:
		return "notifications", "Delete notification configs"
	case rbac.PermNotificationTest:
		return "notifications", "Test notification configs"
	case rbac.PermHookList:
		return "hooks", "List sync hooks"
	case rbac.PermHookCreate:
		return "hooks", "Create sync hooks"
	case rbac.PermHookUpdate:
		return "hooks", "Update sync hooks"
	case rbac.PermHookDelete:
		return "hooks", "Delete sync hooks"
	case rbac.PermRBACManage:
		return "rbac", "Manage roles and bindings"
	case rbac.PermAuditView:
		return "audit", "View audit log"
	default:
		return "other", string(p)
	}
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
