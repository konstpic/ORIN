// Package domain holds the core entity types used by the orin control
// plane. These mirror rows in the Postgres schema and are the canonical
// in-memory representation used by handlers, reconcilers, and the API DTOs.
package domain

import (
	"encoding/json"
	"time"
)

// SyncStatus is the high-level Git-vs-cluster comparison result for an Application.
type SyncStatus string

const (
	SyncStatusUnknown   SyncStatus = "Unknown"
	SyncStatusSynced    SyncStatus = "Synced"
	SyncStatusOutOfSync SyncStatus = "OutOfSync"
)

// HealthStatus is the aggregated workload health of an Application.
type HealthStatus string

const (
	HealthUnknown     HealthStatus = "Unknown"
	HealthHealthy     HealthStatus = "Healthy"
	HealthProgressing HealthStatus = "Progressing"
	HealthDegraded    HealthStatus = "Degraded"
	HealthSuspended   HealthStatus = "Suspended"
	HealthMissing     HealthStatus = "Missing"
)

// EnvVar is a name/value pair injected into a plugin's execution environment.
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PluginGenerateSpec describes the shell command that generates manifests.
// The command is executed inside the checked-out repository path and must
// write valid YAML/JSON Kubernetes manifests to stdout.
type PluginGenerateSpec struct {
	// Command is the executable (e.g. "sh", "vault", "helmfile").
	Command string `json:"command"`
	// Args are the command-line arguments (e.g. ["-c", "helm template ."]).
	Args []string `json:"args,omitempty"`
}

// Plugin is a globally-registered manifest generator.  It mirrors the concept
// of an Argo CD Config Management Plugin: a named shell command that runs in
// the repo checkout directory and writes rendered manifests to stdout.
//
// Per-application env overrides are stored on Application.PluginEnv and are
// merged (right-wins) with the plugin's base Env at render time.
type Plugin struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Generate PluginGenerateSpec `json:"generate"`
	// Env is the base set of env vars always passed to the generator.
	Env       []EnvVar  `json:"env,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RepositoryType narrows manifest renderer selection (MVP supports plain only).
type RepositoryType string

const (
	RepoTypeGit RepositoryType = "git"
)

// Repository describes a Git source. Credentials are stored encrypted at
// rest; the in-memory `Credentials` field is only populated after decryption.
type Repository struct {
	ID                   string         `json:"id"`
	URL                  string         `json:"url"`
	Type                 RepositoryType `json:"type"`
	CredentialsEncrypted []byte         `json:"-"`
	Credentials          *RepoCreds     `json:"-"`
	CreatedAt            time.Time      `json:"createdAt"`
}

// RepoCreds carries decrypted authentication material.
type RepoCreds struct {
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	SSHPrivKey string `json:"sshPrivKey,omitempty"`
}

// Cluster is a managed Kubernetes target. For the MVP we always have a
// single "in-cluster" row.
type Cluster struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	ServerURL           string    `json:"serverUrl"`
	CACert              []byte    `json:"-"`
	AuthConfigEncrypted []byte    `json:"-"`
	InCluster           bool      `json:"inCluster"`
	CreatedAt           time.Time `json:"createdAt"`
}

// IgnoreDifferenceRule suppresses OutOfSync signals for specific fields on
// matching resources (mirrors Argo CD spec.ignoreDifferences).
type IgnoreDifferenceRule struct {
	// Group is the API group, e.g. "apps". Empty string matches core resources.
	Group string `json:"group"`
	// Kind is the resource kind, e.g. "Deployment".
	Kind string `json:"kind"`
	// Name restricts the rule to a single resource name (optional).
	Name string `json:"name,omitempty"`
	// Namespace restricts the rule to a single namespace (optional).
	Namespace string `json:"namespace,omitempty"`
	// JSONPointers are RFC 6901 paths removed from both desired and live objects
	// before comparison, e.g. "/spec/replicas".
	JSONPointers []string `json:"jsonPointers,omitempty"`
}

// SyncPolicy controls automation knobs on an Application.
type SyncPolicy struct {
	Automated *AutomatedSync `json:"automated,omitempty"`
	// SyncOptions lists Argo-style options, e.g. CreateNamespace=true (subset is interpreted by orin).
	SyncOptions []string `json:"syncOptions,omitempty"`
	// ManagedNamespaceMetadata is applied when EffectiveCreateNamespace() is true.
	ManagedNamespaceMetadata *ManagedNamespaceMetadata `json:"managedNamespaceMetadata,omitempty"`
	// CreateNamespace applies the destination Namespace before other resources (Argo: CreateNamespace=true).
	CreateNamespace bool `json:"createNamespace,omitempty"`
	// IgnoreDifferences suppresses OutOfSync for specific fields (Argo-compatible).
	IgnoreDifferences []IgnoreDifferenceRule `json:"ignoreDifferences,omitempty"`
}

// AutomatedSync mirrors ArgoCD's automated sync policy shape.
type AutomatedSync struct {
	Prune    bool `json:"prune"`
	SelfHeal bool `json:"selfHeal"`
}

// ProjectResourceRule is an allow/deny entry for a resource group+kind.
type ProjectResourceRule struct {
	Group string `json:"group"` // "" = core
	Kind  string `json:"kind"`  // "*" = any
}

// ProjectDestination is an allowed destination for an AppProject.
type ProjectDestination struct {
	// Server is the cluster server URL; "*" = any cluster.
	Server string `json:"server,omitempty"`
	// Name is the orin cluster name; "*" = any cluster.
	Name string `json:"name,omitempty"`
	// Namespace pattern; "*" = any namespace.
	Namespace string `json:"namespace"`
}

// ProjectPolicies mirrors Argo CD AppProject policy fields.
type ProjectPolicies struct {
	// SourceRepos is the list of allowed Git repository URL patterns. "*" = any.
	SourceRepos []string `json:"sourceRepos,omitempty"`
	// Destinations lists permitted cluster+namespace combinations.
	Destinations []ProjectDestination `json:"destinations,omitempty"`
	// ClusterResourceWhitelist is the set of cluster-scoped resource group/kind
	// allowed for this project. Empty = none allowed (unless admin).
	ClusterResourceWhitelist []ProjectResourceRule `json:"clusterResourceWhitelist,omitempty"`
	// NamespaceResourceBlacklist lists namespace-scoped resource group/kind
	// that are denied for this project.
	NamespaceResourceBlacklist []ProjectResourceRule `json:"namespaceResourceBlacklist,omitempty"`
}

// Project is a tenancy / RBAC scope (Argo CD project).
type Project struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Policies    ProjectPolicies `json:"policies"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// Application is the user-defined desired-state declaration.
type Application struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Project        string `json:"project"`
	RepoID         string `json:"repoId"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
	// HelmValuesJSON is optional JSON merged into `helm template` (-f). Nil/empty = chart defaults only.
	HelmValuesJSON []byte `json:"-"`
	// HelmValueFiles lists paths relative to the chart directory that are passed
	// as additional -f layers to helm template (Argo: spec.source.helm.valueFiles).
	HelmValueFiles []string   `json:"helmValueFiles,omitempty"`
	DestClusterID  string     `json:"destClusterId"`
	DestNamespace  string     `json:"destNamespace"`
	SyncPolicy     SyncPolicy `json:"syncPolicy"`
	// ParentApp is the name of the parent Application that declared this app
	// as a child (App of Apps pattern). Empty string means a top-level app.
	ParentApp string `json:"parentApp,omitempty"`
	// PluginName references a globally-registered Plugin by name.  When set,
	// the named plugin's generate command is used instead of auto-detecting
	// Helm / Kustomize / plain-YAML.
	PluginName string `json:"pluginName,omitempty"`
	// PluginEnv carries per-application env var overrides that are merged
	// (right-wins) with the plugin's base Env at render time.
	PluginEnv []EnvVar  `json:"pluginEnv,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ApplicationStatus is the hot-path status row, separated from Application
// to limit row churn during high-frequency reconciles.
type ApplicationStatus struct {
	AppID             string       `json:"appId"`
	SyncStatus        SyncStatus   `json:"syncStatus"`
	HealthStatus      HealthStatus `json:"healthStatus"`
	ObservedRevision  string       `json:"observedRevision"`
	LastSyncedAt      *time.Time   `json:"lastSyncedAt,omitempty"`
	LastManualApplyAt *time.Time   `json:"lastManualApplyAt,omitempty"`
	Message           string       `json:"message"`
	UpdatedAt         time.Time    `json:"updatedAt"`
}

// SyncRunRequest is stored with a pending sync and read by the controller.
type SyncRunRequest struct {
	DryRun    bool     `json:"dryRun"`
	Prune     bool     `json:"prune"`
	Resources []string `json:"resources,omitempty"` // resource keys "group/Kind/namespace/name"; empty = all
}

// SyncOperation records a user- or automation-initiated apply attempt.
type SyncOperation struct {
	ID          string               `json:"id"`
	AppID       string               `json:"appId"`
	StartedAt   time.Time            `json:"startedAt"`
	FinishedAt  *time.Time           `json:"finishedAt,omitempty"`
	Revision    string               `json:"revision"`
	InitiatedBy string               `json:"initiatedBy"`
	Status      SyncOpStatus         `json:"status"`
	Message     string               `json:"message"`
	Request     SyncRunRequest       `json:"request"`
	Resources   []SyncResourceResult `json:"resources"`
}

// SyncOpStatus is the high level result of a SyncOperation.
type SyncOpStatus string

const (
	SyncOpPending   SyncOpStatus = "Pending"
	SyncOpRunning   SyncOpStatus = "Running"
	SyncOpSucceeded SyncOpStatus = "Succeeded"
	SyncOpFailed    SyncOpStatus = "Failed"
)

// SyncResourceResult is the per-object result of a SyncOperation.
type SyncResourceResult struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// AuditEntry is a single row in the audit log.
type AuditEntry struct {
	ID       string          `json:"id"`
	TS       time.Time       `json:"ts"`
	Actor    string          `json:"actor"`
	Action   string          `json:"action"`
	Resource string          `json:"resource"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// NotificationEventType enumerates the events that can trigger a notification.
type NotificationEventType string

const (
	EventSyncSucceeded   NotificationEventType = "sync_succeeded"
	EventSyncFailed      NotificationEventType = "sync_failed"
	EventHealthDegraded  NotificationEventType = "health_degraded"
	EventHealthRecovered NotificationEventType = "health_recovered"
	EventAppOutOfSync    NotificationEventType = "app_out_of_sync"
	EventAppSynced       NotificationEventType = "app_synced"
)

// NotificationType is the delivery channel type.
type NotificationType string

const (
	NotificationTypeWebhook NotificationType = "webhook"
	NotificationTypeSlack   NotificationType = "slack"
)

// NotificationConfig stores webhook/Slack delivery config per application.
type NotificationConfig struct {
	ID        string                  `json:"id"`
	AppID     string                  `json:"appId"`
	Name      string                  `json:"name"`
	Type      NotificationType        `json:"type"`
	URL       string                  `json:"url"`
	Events    []NotificationEventType `json:"events"`
	Enabled   bool                    `json:"enabled"`
	CreatedAt time.Time               `json:"createdAt"`
}

// SyncHookPhase defines when a hook Job runs during the sync lifecycle.
type SyncHookPhase string

const (
	HookPreSync  SyncHookPhase = "PreSync"
	HookPostSync SyncHookPhase = "PostSync"
	HookSyncFail SyncHookPhase = "SyncFail"
)

// SyncHook stores a Kubernetes manifest (Job/Pod) to run at a specific sync phase.
type SyncHook struct {
	ID        string        `json:"id"`
	AppID     string        `json:"appId"`
	Name      string        `json:"name"`
	Phase     SyncHookPhase `json:"phase"`
	YAML      string        `json:"yaml"`
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// SystemConfig holds runtime-overridable system settings.
type SystemConfig struct {
	ReconcileWorkers    int           `json:"reconcileWorkers"`
	ReconcileResync     time.Duration `json:"reconcileResync"`
	RepoPollInterval    time.Duration `json:"repoPollInterval"`
	RepoRenderTimeout   time.Duration `json:"repoRenderTimeout"`
	SyncApplyRetries    int           `json:"syncApplyRetries"`
	AutoSyncGracePeriod time.Duration `json:"autoSyncGracePeriod"`
	SyncDenyRangeUTC    string        `json:"syncDenyRangeUtc"`
	AppsCatalogRepoURL  string        `json:"appsCatalogRepoUrl"`
	AppsCatalogPath     string        `json:"appsCatalogPath"`
	AppsCatalogInterval time.Duration `json:"appsCatalogInterval"`
}
