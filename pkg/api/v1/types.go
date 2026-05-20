// Package v1 defines the public DTOs exchanged between the React frontend
// and the Go API server. The Go types are the source of truth; an
// OpenAPI 3 schema is hand-maintained at internal/api/openapi.yaml.
package v1

import (
	"encoding/json"
	"time"
)

// Application is the public representation of an application.
type Application struct {
	Name        string         `json:"name"`
	Project     string         `json:"project"`
	Source      AppSource      `json:"source"`
	Destination AppDestination `json:"destination"`
	SyncPolicy  SyncPolicy     `json:"syncPolicy"`
	Status      AppStatus      `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// AppSource references a Git source.
type AppSource struct {
	RepoURL        string `json:"repoUrl"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
	// HelmValues is optional JSON merged into helm template (-f) when Path points at a Helm chart.
	HelmValues json.RawMessage `json:"helmValues,omitempty"`
	// HelmValueFiles are paths relative to the chart directory passed as extra -f layers.
	// Equivalent to Argo CD spec.source.helm.valueFiles.
	HelmValueFiles []string `json:"helmValueFiles,omitempty"`
}

// AppDestination references a destination cluster + namespace.
type AppDestination struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
}

// SyncPolicy mirrors domain.SyncPolicy.
type SyncPolicy struct {
	Automated *AutomatedSync `json:"automated,omitempty"`
	// SyncOptions are Argo-style strings, e.g. CreateNamespace=true (subset honored by orin).
	SyncOptions []string `json:"syncOptions,omitempty"`
	// ManagedNamespaceMetadata is merged into the Namespace when create namespace runs.
	ManagedNamespaceMetadata *ManagedNamespaceMetadata `json:"managedNamespaceMetadata,omitempty"`
	// CreateNamespace applies the destination namespace before other manifests (Argo sync-option).
	CreateNamespace bool `json:"createNamespace,omitempty"`
	// MaterializeChildApps is deprecated and ignored. Child applications are now always
	// materialized from orin.io/Application and argoproj.io/Application objects rendered
	// by the parent chart.
	MaterializeChildApps *bool `json:"materializeChildApps,omitempty"`
	// IgnoreDifferences suppresses OutOfSync for specific resource fields (Argo-compatible).
	IgnoreDifferences []IgnoreDifferenceRule `json:"ignoreDifferences,omitempty"`
}

// ManagedNamespaceMetadata mirrors Argo spec.syncPolicy.managedNamespaceMetadata.
type ManagedNamespaceMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IgnoreDifferenceRule mirrors Argo CD spec.ignoreDifferences — suppresses
// OutOfSync signals for the listed JSON pointer paths on matching resources.
type IgnoreDifferenceRule struct {
	Group        string   `json:"group"`
	Kind         string   `json:"kind"`
	Name         string   `json:"name,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	JSONPointers []string `json:"jsonPointers,omitempty"`
}

// AutomatedSync mirrors domain.AutomatedSync.
type AutomatedSync struct {
	Prune    bool `json:"prune"`
	SelfHeal bool `json:"selfHeal"`
}

// AppStatus is the live status of an Application.
type AppStatus struct {
	Sync             string     `json:"sync"`   // Synced / OutOfSync / Unknown
	Health           string     `json:"health"` // Healthy / Degraded / ...
	ObservedRevision string     `json:"observedRevision"`
	LastSyncedAt     *time.Time `json:"lastSyncedAt,omitempty"`
	Message          string     `json:"message"`
	// ObservedCommit is the git commit metadata for the current observedRevision (omitted when unknown).
	ObservedCommit *GitCommit `json:"observedCommit,omitempty"`
	// SyncOperation is set while a sync apply job is queued or running (sync_operations row).
	SyncOperation *SyncOperationProgress `json:"syncOperation,omitempty"`
	// LastCompletedSync is the most recently finished sync job (Succeeded or Failed).
	LastCompletedSync *CompletedSyncSummary `json:"lastCompletedSync,omitempty"`
}

// SyncOperationProgress describes the in-flight sync apply job.
type SyncOperationProgress struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // Pending | Running
	Message string `json:"message,omitempty"`
}

// CompletedSyncSummary describes the last finished sync job.
type CompletedSyncSummary struct {
	Status  string `json:"status"` // Succeeded | Failed
	Message string `json:"message,omitempty"`
}

// CreateApplicationRequest is accepted by POST /api/v1/applications.
type CreateApplicationRequest struct {
	Name        string         `json:"name"`
	Project     string         `json:"project,omitempty"`
	Source      AppSource      `json:"source"`
	Destination AppDestination `json:"destination"`
	SyncPolicy  SyncPolicy     `json:"syncPolicy"`
}

// UpdateApplicationRequest is accepted by PUT /api/v1/applications/{name}.
type UpdateApplicationRequest struct {
	Source      AppSource      `json:"source"`
	Destination AppDestination `json:"destination"`
	SyncPolicy  SyncPolicy     `json:"syncPolicy"`
}

// SyncRequest triggers a sync.
type SyncRequest struct {
	Revision  string   `json:"revision,omitempty"` // override target revision
	Prune     bool     `json:"prune,omitempty"`
	DryRun    bool     `json:"dryRun,omitempty"`
	Resources []string `json:"resources,omitempty"` // keys "group/Kind/namespace/name"; empty = all
}

// ResourceNode is one node in an application's resource tree.
type ResourceNode struct {
	Group       string `json:"group"`
	Version     string `json:"version"`
	Kind        string `json:"kind"`
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name"`
	UID         string `json:"uid"`
	Health      string `json:"health"`
	Sync        string `json:"sync"`
	SyncMessage string `json:"syncMessage,omitempty"`
	// PodPhase is set for Kind=Pod (status.phase).
	PodPhase  string `json:"podPhase,omitempty"`
	ParentUID string `json:"parentUid,omitempty"`
	// CreationTimestamp is the RFC3339 creation time of the live object.
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
	// ResourceVersion carries the k8s resourceVersion (incremented on each change).
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// Labels are the object's labels (used for revision display, e.g. pod-template-hash).
	Labels   map[string]string `json:"labels,omitempty"`
	Children []ResourceNode    `json:"children,omitempty"`
}

// ActiveSyncInfo describes an in-flight sync operation (for UI progress).
type ActiveSyncInfo struct {
	ID        string               `json:"id"`
	Status    string               `json:"status"`
	Message   string               `json:"message,omitempty"`
	Resources []SyncResourceResult `json:"resources,omitempty"`
}

// ResourceTree wraps the rooted forest.
type ResourceTree struct {
	Nodes      []ResourceNode  `json:"nodes"`
	ActiveSync *ActiveSyncInfo `json:"activeSync,omitempty"`
}

// PodSummary is returned by GET /api/v1/applications/{name}/pods/{pod}.
type PodSummary struct {
	Name           string         `json:"name"`
	Namespace      string         `json:"namespace"`
	Phase          string         `json:"phase"`
	Containers     []PodContainer `json:"containers"`
	InitContainers []PodContainer `json:"initContainers,omitempty"`
}

// PodContainer names one container in a pod spec.
type PodContainer struct {
	Name string `json:"name"`
}

// PodEvent is a Kubernetes event involving a pod (subset for the UI).
type PodEvent struct {
	Type      string     `json:"type"`
	Reason    string     `json:"reason"`
	Message   string     `json:"message"`
	Count     int32      `json:"count"`
	FirstTime *time.Time `json:"firstTime,omitempty"`
	LastTime  *time.Time `json:"lastTime,omitempty"`
	// Category classifies the event: PodStart, ImagePull, LivenessProbe, ReadinessProbe,
	// StartupProbe, ContainerCrash, ContainerStart, ContainerStop, PodStop, etc.
	Category string `json:"category,omitempty"`
	// ResourceKind is the kind of Kubernetes resource (Pod, Deployment, ReplicaSet, StatefulSet, etc.)
	ResourceKind string `json:"resourceKind,omitempty"`
	// ResourceName is the name of the resource that generated this event
	ResourceName string `json:"resourceName,omitempty"`
	// Namespace is the namespace of the resource
	Namespace string `json:"namespace,omitempty"`
}

// ResourceDiff is one element of a /diff response.
type ResourceDiff struct {
	Group          string `json:"group"`
	Version        string `json:"version"`
	Kind           string `json:"kind"`
	Namespace      string `json:"namespace,omitempty"`
	Name           string `json:"name"`
	Sync           string `json:"sync"` // Synced / OutOfSync
	DesiredYAML    string `json:"desiredYaml"`
	LiveYAML       string `json:"liveYaml"`
	NormalizedDiff string `json:"normalizedDiff"`
}

// DiffResponse aggregates per-resource diffs and a summary.
type DiffResponse struct {
	Resources []ResourceDiff `json:"resources"`
	OutOfSync int            `json:"outOfSync"`
	Synced    int            `json:"synced"`
}

// GitCommit is one commit affecting an application's source path.
type GitCommit struct {
	SHA        string    `json:"sha"`
	ShortSHA   string    `json:"shortSha"`
	Message    string    `json:"message"`
	Author     string    `json:"author"`
	AuthorDate time.Time `json:"authorDate"`
}

// RevisionListResponse lists commits for the app source path.
type RevisionListResponse struct {
	Commits []GitCommit `json:"commits"`
}

// RevisionDiffResponse is a raw unified diff for the path between two SHAs.
type RevisionDiffResponse struct {
	Diff string `json:"diff"`
}

// RollbackRequest pins the app to a Git revision (branch, tag, or SHA).
type RollbackRequest struct {
	Revision string `json:"revision"`
}

// ApplicationBatchItem is one row in a batch create.
type ApplicationBatchItem struct {
	Name           string          `json:"name"`
	DestNamespace  string          `json:"destNamespace,omitempty"`
	RepoURL        string          `json:"repoUrl,omitempty"`
	Path           string          `json:"path,omitempty"`
	TargetRevision string          `json:"targetRevision,omitempty"`
	Cluster        string          `json:"cluster,omitempty"`
	Project        string          `json:"project,omitempty"`
	HelmValues     json.RawMessage `json:"helmValues,omitempty"`
	// CreateNamespace overrides template syncPolicy when present.
	CreateNamespace *bool `json:"createNamespace,omitempty"`
	// MaterializeChildApps is deprecated and ignored.
	MaterializeChildApps *bool `json:"materializeChildApps,omitempty"`
}

// ApplicationBatchCreateRequest creates multiple apps from one template.
type ApplicationBatchCreateRequest struct {
	Template CreateApplicationRequest `json:"template"`
	Items    []ApplicationBatchItem   `json:"items"`
}

// CreateClusterRequest registers an out-of-cluster API target (kubeconfig YAML).
type CreateClusterRequest struct {
	Name           string `json:"name"`
	KubeconfigYAML string `json:"kubeconfigYaml"`
}

// Repository is the public view of a Git repo registration.
type Repository struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Type      string    `json:"type"`
	HasCreds  bool      `json:"hasCreds"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateRepositoryRequest is accepted by POST /api/v1/repositories.
type CreateRepositoryRequest struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Cluster is the public view of a managed cluster.
type Cluster struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ServerURL string    `json:"serverUrl"`
	InCluster bool      `json:"inCluster"`
	CreatedAt time.Time `json:"createdAt"`
}

// ClusterHealth is the real-time health probe result for a cluster.
type ClusterHealth struct {
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Status      string `json:"status"`     // "Ready", "Unreachable", "Degraded"
	K8sVersion  string `json:"k8sVersion"` // e.g. "v1.29.0"
	NodeCount   int    `json:"nodeCount"`
	AppCount    int    `json:"appCount"` // apps targeting this cluster
	Error       string `json:"error,omitempty"`
}

// NodeInfo describes a Kubernetes node with resource usage.
type NodeInfo struct {
	Name           string    `json:"name"`
	Roles          []string  `json:"roles"` // e.g. ["control-plane", "master"]
	KubeletVersion string    `json:"kubeletVersion"`
	OS             string    `json:"os"`             // e.g. "linux"
	Arch           string    `json:"arch"`           // e.g. "arm64"
	Status         string    `json:"status"`         // "Ready", "NotReady"
	CPUCapacity    string    `json:"cpuCapacity"`    // e.g. "8"
	CPUAllocatable string    `json:"cpuAllocatable"` // e.g. "7800m"
	CPUUsed        string    `json:"cpuUsed"`        // e.g. "3200m"
	CPUUsedPercent float64   `json:"cpuUsedPercent"`
	MemCapacity    string    `json:"memCapacity"`    // e.g. "16Gi"
	MemAllocatable string    `json:"memAllocatable"` // e.g. "15Gi"
	MemUsed        string    `json:"memUsed"`        // e.g. "8Gi"
	MemUsedPercent float64   `json:"memUsedPercent"`
	PodCount       int       `json:"podCount"`
	Pods           []PodRef  `json:"pods"` // pods scheduled on this node
	CreatedAt      time.Time `json:"createdAt"`
}

// PodRef is a lightweight pod reference for node view.
type PodRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`   // owner kind (Deployment, StatefulSet, etc.)
	Owner     string `json:"owner"`  // owner name
	CPUReq    string `json:"cpuReq"` // requested CPU
	MemReq    string `json:"memReq"` // requested memory
	Status    string `json:"status"` // Running, Pending, etc.
	Health    string `json:"health"` // Healthy, Progressing, Degraded
}

// ProjectResourceRule is one entry in a cluster/namespace resource list.
type ProjectResourceRule struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

// ProjectDestination is one permitted cluster+namespace pair.
type ProjectDestination struct {
	Server    string `json:"server,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace"`
}

// ProjectPolicies mirrors Argo CD AppProject policy fields.
type ProjectPolicies struct {
	SourceRepos                []string              `json:"sourceRepos,omitempty"`
	Destinations               []ProjectDestination  `json:"destinations,omitempty"`
	ClusterResourceWhitelist   []ProjectResourceRule `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceBlacklist []ProjectResourceRule `json:"namespaceResourceBlacklist,omitempty"`
}

// Project is an Argo-style grouping scope with optional policy constraints.
type Project struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Policies    ProjectPolicies `json:"policies"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// CreateProjectRequest is accepted by POST /api/v1/projects.
type CreateProjectRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Policies    ProjectPolicies `json:"policies"`
}

// UpdateProjectRequest is accepted by PUT /api/v1/projects/{name}.
type UpdateProjectRequest struct {
	Description string          `json:"description,omitempty"`
	Policies    ProjectPolicies `json:"policies"`
}

// SyncOperation is the public view of a sync history row.
type SyncOperation struct {
	ID          string               `json:"id"`
	AppName     string               `json:"appName"`
	Revision    string               `json:"revision"`
	StartedAt   time.Time            `json:"startedAt"`
	FinishedAt  *time.Time           `json:"finishedAt,omitempty"`
	Status      string               `json:"status"`
	InitiatedBy string               `json:"initiatedBy"`
	Message     string               `json:"message"`
	Resources   []SyncResourceResult `json:"resources"`
}

// SyncResourceResult mirrors domain.SyncResourceResult for the wire.
type SyncResourceResult struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// WSMessage is the framing for messages over the multiplexed WebSocket.
type WSMessage struct {
	Topic   string          `json:"topic"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// NotificationConfig represents a webhook/Slack delivery config.
type NotificationConfig struct {
	ID        string    `json:"id"`
	AppID     string    `json:"appId"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateNotificationConfigRequest is the body for creating a notification config.
type CreateNotificationConfigRequest struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// UpdateNotificationConfigRequest is the body for updating a notification config.
type UpdateNotificationConfigRequest struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// SyncHook represents a pre-sync/post-sync/sync-fail hook Job.
type SyncHook struct {
	ID        string    `json:"id"`
	AppID     string    `json:"appId"`
	Name      string    `json:"name"`
	Phase     string    `json:"phase"`
	YAML      string    `json:"yaml"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateSyncHookRequest is the body for creating a sync hook.
type CreateSyncHookRequest struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"`
	YAML    string `json:"yaml"`
	Enabled bool   `json:"enabled"`
}

// UpdateSyncHookRequest is the body for updating a sync hook.
type UpdateSyncHookRequest struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"`
	YAML    string `json:"yaml"`
	Enabled bool   `json:"enabled"`
}

// Role is the public representation of an RBAC role.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
	BuiltIn     bool     `json:"builtIn"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// CreateRoleRequest is the body for creating a role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest is the body for updating a role.
type UpdateRoleRequest struct {
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// RoleBinding is the public representation of a user-role mapping.
type RoleBinding struct {
	ID        string   `json:"id"`
	UserID    string   `json:"userId"`
	UserEmail string   `json:"userEmail,omitempty"`
	RoleID    string   `json:"roleId"`
	RoleName  string   `json:"roleName,omitempty"`
	Projects  []string `json:"projects"`
	CreatedAt string   `json:"createdAt"`
}

// CreateRoleBindingRequest is the body for creating a role binding.
type CreateRoleBindingRequest struct {
	UserID   string   `json:"userId"`
	RoleID   string   `json:"roleId"`
	Projects []string `json:"projects,omitempty"`
}

// UpdateRoleBindingRequest is the body for updating a role binding.
type UpdateRoleBindingRequest struct {
	RoleID   string   `json:"roleId"`
	Projects []string `json:"projects,omitempty"`
}

// PermissionInfo describes a single permission for the UI.
type PermissionInfo struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// UserInfo is the public view of a user with their roles.
type UserInfo struct {
	ID          string        `json:"id"`
	Email       string        `json:"email"`
	DisplayName string        `json:"displayName,omitempty"`
	Role        string        `json:"role"`
	Active      bool          `json:"active"`
	Bindings    []RoleBinding `json:"bindings,omitempty"`
}

// CreateUserRequest is the body for creating a user.
type CreateUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
	Role        string `json:"role"`
	Token       string `json:"token"` // plaintext, will be hashed
}

// UpdateUserRequest is the body for updating a user.
type UpdateUserRequest struct {
	DisplayName string `json:"displayName,omitempty"`
	Active      *bool  `json:"active,omitempty"`
	Token       string `json:"token,omitempty"` // set user token (plaintext, will be hashed)
}

// SystemConfigResponse is the system configuration returned by the API.
type SystemConfigResponse struct {
	ReconcileWorkers    int    `json:"reconcileWorkers"`
	ReconcileResync     string `json:"reconcileResync"`
	RepoPollInterval    string `json:"repoPollInterval"`
	RepoRenderTimeout   string `json:"repoRenderTimeout"`
	SyncApplyRetries    int    `json:"syncApplyRetries"`
	AutoSyncGracePeriod string `json:"autoSyncGracePeriod"`
	SyncDenyRangeUtc    string `json:"syncDenyRangeUtc"`
	AppsCatalogRepoUrl  string `json:"appsCatalogRepoUrl"`
	AppsCatalogPath     string `json:"appsCatalogPath"`
	AppsCatalogInterval string `json:"appsCatalogInterval"`
}

// UpdateSystemConfigRequest is the body for updating system config.
type UpdateSystemConfigRequest struct {
	ReconcileWorkers    *int    `json:"reconcileWorkers,omitempty"`
	ReconcileResync     *string `json:"reconcileResync,omitempty"`
	RepoPollInterval    *string `json:"repoPollInterval,omitempty"`
	RepoRenderTimeout   *string `json:"repoRenderTimeout,omitempty"`
	SyncApplyRetries    *int    `json:"syncApplyRetries,omitempty"`
	AutoSyncGracePeriod *string `json:"autoSyncGracePeriod,omitempty"`
	SyncDenyRangeUtc    *string `json:"syncDenyRangeUtc,omitempty"`
	AppsCatalogRepoUrl  *string `json:"appsCatalogRepoUrl,omitempty"`
	AppsCatalogPath     *string `json:"appsCatalogPath,omitempty"`
	AppsCatalogInterval *string `json:"appsCatalogInterval,omitempty"`
}
