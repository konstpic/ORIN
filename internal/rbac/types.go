// Package rbac implements role-based access control for k8s-ui.
// It defines roles, permissions, and role bindings that map users to roles
// with optional project scoping.
package rbac

import (
	"time"
)

// Permission is an atomic action that can be allowed or denied.
// Permissions follow the format "resource:verb", e.g. "applications:sync",
// "projects:create", "clusters:list".
type Permission string

const (
	// Application permissions
	PermAppList    Permission = "applications:list"
	PermAppGet     Permission = "applications:get"
	PermAppCreate  Permission = "applications:create"
	PermAppUpdate  Permission = "applications:update"
	PermAppDelete  Permission = "applications:delete"
	PermAppSync    Permission = "applications:sync"
	PermAppRollback Permission = "applications:rollback"
	PermAppRefresh Permission = "applications:refresh"

	// Pod / exec permissions
	PermPodLogs   Permission = "pods:logs"
	PermPodExec   Permission = "pods:exec"
	PermPodShell  Permission = "pods:shell"
	PermPodDelete Permission = "pods:delete"

	// Live resource permissions
	PermLiveGet    Permission = "live-resource:get"
	PermLiveEdit   Permission = "live-resource:edit"
	PermLiveApply  Permission = "live-resource:apply"
	PermLiveDelete Permission = "live-resource:delete"
	PermLiveRestart Permission = "live-resource:restart"

	// Repository permissions
	PermRepoList   Permission = "repositories:list"
	PermRepoGet    Permission = "repositories:get"
	PermRepoCreate Permission = "repositories:create"
	PermRepoDelete Permission = "repositories:delete"

	// Cluster permissions
	PermClusterList   Permission = "clusters:list"
	PermClusterGet    Permission = "clusters:get"
	PermClusterCreate Permission = "clusters:create"
	PermClusterDelete Permission = "clusters:delete"

	// Project permissions
	PermProjectList   Permission = "projects:list"
	PermProjectGet    Permission = "projects:get"
	PermProjectCreate Permission = "projects:create"
	PermProjectUpdate Permission = "projects:update"
	PermProjectDelete Permission = "projects:delete"

	// Notification permissions
	PermNotificationList   Permission = "notifications:list"
	PermNotificationCreate Permission = "notifications:create"
	PermNotificationUpdate Permission = "notifications:update"
	PermNotificationDelete Permission = "notifications:delete"
	PermNotificationTest   Permission = "notifications:test"

	// Sync hook permissions
	PermHookList   Permission = "hooks:list"
	PermHookCreate Permission = "hooks:create"
	PermHookUpdate Permission = "hooks:update"
	PermHookDelete Permission = "hooks:delete"

	// RBAC management permissions (only for admin-like roles)
	PermRBACManage Permission = "rbac:manage"

	// System-level permissions
	PermAuditView Permission = "audit:view"
)

// AllPermissions returns every defined permission.
func AllPermissions() []Permission {
	return []Permission{
		PermAppList, PermAppGet, PermAppCreate, PermAppUpdate, PermAppDelete,
		PermAppSync, PermAppRollback, PermAppRefresh,
		PermPodLogs, PermPodExec, PermPodShell, PermPodDelete,
		PermLiveGet, PermLiveEdit, PermLiveApply, PermLiveDelete, PermLiveRestart,
		PermRepoList, PermRepoGet, PermRepoCreate, PermRepoDelete,
		PermClusterList, PermClusterGet, PermClusterCreate, PermClusterDelete,
		PermProjectList, PermProjectGet, PermProjectCreate, PermProjectUpdate, PermProjectDelete,
		PermNotificationList, PermNotificationCreate, PermNotificationUpdate, PermNotificationDelete,
		PermNotificationTest,
		PermHookList, PermHookCreate, PermHookUpdate, PermHookDelete,
		PermRBACManage,
		PermAuditView,
	}
}

// DefaultRolePresets returns the built-in role definitions.
func DefaultRolePresets() []RolePreset {
	return []RolePreset{
		{
			Name:        "admin",
			DisplayName: "Administrator",
			Description: "Full access to all resources and RBAC management",
			Permissions: AllPermissions(),
			BuiltIn:     true,
		},
		{
			Name:        "editor",
			DisplayName: "Editor",
			Description: "Can create, update, and sync applications within assigned projects",
			Permissions: []Permission{
				PermAppList, PermAppGet, PermAppCreate, PermAppUpdate,
				PermAppSync, PermAppRollback, PermAppRefresh,
				PermPodLogs, PermPodExec, PermPodShell,
				PermLiveGet, PermLiveEdit, PermLiveApply, PermLiveRestart,
				PermRepoList, PermRepoGet,
				PermClusterList, PermClusterGet,
				PermProjectList, PermProjectGet,
				PermNotificationList, PermNotificationCreate, PermNotificationUpdate, PermNotificationDelete,
				PermHookList, PermHookCreate, PermHookUpdate, PermHookDelete,
			},
			BuiltIn: true,
		},
		{
			Name:        "viewer",
			DisplayName: "Viewer",
			Description: "Read-only access to applications, clusters, and repos within assigned projects",
			Permissions: []Permission{
				PermAppList, PermAppGet, PermAppRefresh,
				PermPodLogs,
				PermLiveGet,
				PermRepoList, PermRepoGet,
				PermClusterList, PermClusterGet,
				PermProjectList, PermProjectGet,
				PermNotificationList,
				PermHookList,
			},
			BuiltIn: true,
		},
	}
}

// RolePreset is a template for seeding built-in roles.
type RolePreset struct {
	Name        string
	DisplayName string
	Description string
	Permissions []Permission
	BuiltIn     bool
}

// Role is a named collection of permissions.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions"`
	BuiltIn     bool         `json:"builtIn"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// HasPermission returns true if the role grants the given permission.
func (r *Role) HasPermission(p Permission) bool {
	for _, perm := range r.Permissions {
		if perm == p {
			return true
		}
	}
	return false
}

// RoleBinding maps a user to a role, optionally scoped to specific projects.
// An empty Projects slice means the binding applies to all projects.
type RoleBinding struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	RoleID    string    `json:"roleId"`
	Projects  []string  `json:"projects"` // empty = all projects, "*" = all
	CreatedAt time.Time `json:"createdAt"`
}
