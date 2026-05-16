// Hand-mirrored from pkg/api/v1/types.go. Replace with OpenAPI codegen
// (openapi-typescript) when the spec stabilises.

export type SyncStatus = "Synced" | "OutOfSync" | "Unknown";
export type HealthStatus =
  | "Healthy"
  | "Progressing"
  | "Degraded"
  | "Suspended"
  | "Missing"
  | "Unknown";

export interface AppSource {
  repoUrl: string;
  path: string;
  targetRevision: string;
  /** Optional inline Helm values JSON. */
  helmValues?: unknown;
  /** Paths inside the chart for `helm template -f`. */
  helmValueFiles?: string[];
}

export interface IgnoreDifferenceRule {
  group: string;
  kind: string;
  name?: string;
  namespace?: string;
  jsonPointers?: string[];
}

export interface AppDestination {
  cluster: string;
  namespace: string;
}

export interface AutomatedSync {
  prune: boolean;
  selfHeal: boolean;
}

export interface ManagedNamespaceMetadata {
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export interface SyncPolicy {
  automated?: AutomatedSync | null;
  /** Argo-style sync options, e.g. CreateNamespace=true. */
  syncOptions?: string[];
  managedNamespaceMetadata?: ManagedNamespaceMetadata;
  /** Apply destination namespace before sync (Argo-style). */
  createNamespace?: boolean;
  /** @deprecated Ignored. Child apps are always materialized from k8s-ui.io/Application objects. */
  materializeChildApps?: boolean;
  /** Suppress OutOfSync for specific resource fields (Argo-compatible). */
  ignoreDifferences?: IgnoreDifferenceRule[];
}

export interface GitCommit {
  sha: string;
  shortSha: string;
  message: string;
  author: string;
  authorDate: string;
}

export interface AppStatus {
  sync: SyncStatus;
  health: HealthStatus;
  observedRevision: string;
  lastSyncedAt?: string | null;
  message: string;
  /** Git commit info for the current observedRevision (omitted when unknown). */
  observedCommit?: GitCommit | null;
  /** Queued or running apply job (from sync_operations). */
  syncOperation?: { id: string; status: string; message?: string } | null;
  /** Most recently finished sync job (omitted while syncOperation is set). */
  lastCompletedSync?: { status: string; message?: string } | null;
}

export interface Application {
  name: string;
  project: string;
  source: AppSource;
  destination: AppDestination;
  syncPolicy: SyncPolicy;
  status: AppStatus;
  createdAt: string;
  updatedAt: string;
}

export interface CreateApplicationRequest {
  name: string;
  project?: string;
  source: AppSource;
  destination: AppDestination;
  syncPolicy?: SyncPolicy;
}

export interface UpdateApplicationRequest {
  source: AppSource;
  destination: AppDestination;
  syncPolicy: SyncPolicy;
}

export interface ResourceNode {
  group: string;
  version: string;
  kind: string;
  namespace?: string;
  name: string;
  uid: string;
  health: HealthStatus;
  sync: SyncStatus;
  syncMessage?: string;
  /** Pod status.phase from API (Pods only). */
  podPhase?: string;
  parentUid?: string;
  /** RFC3339 creation timestamp of the live k8s object. */
  creationTimestamp?: string;
  /** k8s labels of the live object (e.g. pod-template-hash for RS). */
  labels?: Record<string, string>;
  children?: ResourceNode[];
  /** UI-only: pods collapsed under a ReplicaSet in compact topology */
  groupedPods?: ResourceNode[];
  /** UI-only: ReplicaSet uid when this node is a synthetic PodGroup */
  replicaSetUid?: string;
  /** UI-only: workload kind that owns this PodGroup (ReplicaSet or Deployment) */
  groupParentKind?: string;
  /** UI-only: this node is a synthetic group of same-kind siblings (ConfigMap, Secret, Service, etc.). */
  isKindGroup?: boolean;
  /** UI-only: members of a kind-group node. */
  groupedMembers?: ResourceNode[];
}

export interface ActiveSyncInfo {
  id: string;
  status: string;
  message?: string;
  resources: SyncResourceResult[];
}

export interface ResourceTree {
  nodes: ResourceNode[];
  activeSync?: ActiveSyncInfo | null;
}

export interface ResourceDiff {
  group: string;
  version: string;
  kind: string;
  namespace?: string;
  name: string;
  sync: SyncStatus;
  desiredYaml: string;
  liveYaml: string;
  normalizedDiff: string;
}

export interface DiffResponse {
  resources: ResourceDiff[];
  outOfSync: number;
  synced: number;
}

export interface Repository {
  id: string;
  url: string;
  type: string;
  hasCreds: boolean;
  createdAt: string;
}

export interface Cluster {
  id: string;
  name: string;
  serverUrl: string;
  inCluster: boolean;
  createdAt: string;
}

export interface ClusterHealth {
  clusterId: string;
  clusterName: string;
  status: "Ready" | "Unreachable" | "Degraded";
  k8sVersion: string;
  nodeCount: number;
  appCount: number;
  error?: string;
}

export interface NodeInfo {
  name: string;
  roles: string[];
  kubeletVersion: string;
  os: string;
  arch: string;
  status: "Ready" | "NotReady";
  cpuCapacity: string;
  cpuAllocatable: string;
  cpuUsed: string;
  cpuUsedPercent: number;
  memCapacity: string;
  memAllocatable: string;
  memUsed: string;
  memUsedPercent: number;
  podCount: number;
  pods: PodRef[];
  createdAt: string;
}

export interface PodRef {
  name: string;
  namespace: string;
  kind: string;
  owner: string;
  cpuReq: string;
  memReq: string;
  status: string;
  health: string;
}

export interface SyncResourceResult {
  group: string;
  version: string;
  kind: string;
  namespace?: string;
  name: string;
  status: string;
  message: string;
}

export interface SyncOperation {
  id: string;
  appName: string;
  revision: string;
  startedAt: string;
  finishedAt?: string | null;
  status: string;
  initiatedBy: string;
  message: string;
  resources: SyncResourceResult[];
}

export interface PodSummary {
  name: string;
  namespace: string;
  phase: string;
  containers: { name: string }[];
  initContainers?: { name: string }[];
}

export interface PodEvent {
  type: string;
  reason: string;
  message: string;
  count: number;
  firstTime?: string | null;
  lastTime?: string | null;
  category?: string;
  resourceKind?: string;
  resourceName?: string;
  namespace?: string;
}

export interface WSMessage<T = unknown> {
  topic: string;
  type: string;
  payload?: T;
}

// --- RBAC types ---

export interface Role {
  id: string;
  name: string;
  displayName: string;
  description?: string;
  permissions: string[];
  builtIn: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRoleRequest {
  name: string;
  displayName: string;
  description?: string;
  permissions: string[];
}

export interface UpdateRoleRequest {
  displayName: string;
  description?: string;
  permissions: string[];
}

export interface RoleBinding {
  id: string;
  userId: string;
  userEmail?: string;
  roleId: string;
  roleName?: string;
  projects: string[];
  createdAt: string;
}

export interface CreateRoleBindingRequest {
  userId: string;
  roleId: string;
  projects?: string[];
}

export interface UpdateRoleBindingRequest {
  roleId: string;
  projects?: string[];
}

export interface PermissionInfo {
  id: string;
  category: string;
  description: string;
}

export interface UserInfo {
  id: string;
  email: string;
  displayName?: string;
  role: string;
  active: boolean;
}

export interface CreateUserRequest {
  email: string;
  displayName?: string;
  role: string;
  token: string;
}

export interface UpdateUserRequest {
  displayName?: string;
  active?: boolean;
  token?: string;
}

// --- System config types ---

export interface SystemConfig {
  reconcileWorkers: number;
  reconcileResync: string;
  repoPollInterval: string;
  repoRenderTimeout: string;
  syncApplyRetries: number;
  autoSyncGracePeriod: string;
  syncDenyRangeUtc: string;
  appsCatalogRepoUrl: string;
  appsCatalogPath: string;
  appsCatalogInterval: string;
}
