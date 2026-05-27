// Thin typed fetch wrapper. All requests go through this single helper so
// we can attach the bearer cookie and surface server errors uniformly.

import type {
  Application,
  Cluster,
  ClusterHealth,
  CreateApplicationRequest,
  DiffResponse,
  GitCommit,
  NodeInfo,
  PodEvent,
  PodSummary,
  Repository,
  ResourceTree,
  SyncOperation,
  UpdateApplicationRequest,
  Role,
  RoleBinding,
  PermissionInfo,
  UserInfo,
  CreateRoleRequest,
  UpdateRoleRequest,
  CreateRoleBindingRequest,
  UpdateRoleBindingRequest,
  CreateUserRequest,
  UpdateUserRequest,
  SystemConfig,
} from "./types";
import type { NetworkMapResponse } from "../components/NetworkMapView";

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  // Fetch has no default timeout; if the backend/ingress hangs the connection,
  // react-query will keep showing the initial loading state indefinitely.
  const timeoutMs = 30_000;
  const ctrl = new AbortController();
  const timeout = window.setTimeout(() => ctrl.abort(), timeoutMs);
  const extraHeaders = init.headers as Record<string, string> | undefined;
  const contentType = extraHeaders?.["Content-Type"] ?? "application/json";
  // We keep an auth cookie for server-side convenience, but for local/dev and
  // for cases where the UI is opened via a different host/IP, also attach the
  // token from localStorage as a Bearer header (backend accepts it).
  const storedToken =
    typeof window !== "undefined" ? window.localStorage.getItem("k8sui-token") : null;
  let res: Response;
  try {
    res = await fetch(path, {
      ...init,
      signal: init.signal ?? ctrl.signal,
      credentials: "include",
      headers: {
        "Content-Type": contentType,
        ...(storedToken ? { Authorization: `Bearer ${storedToken}` } : {}),
        ...(init.headers ?? {}),
      },
    });
  } catch (e) {
    // Normalize timeouts / aborts into ApiError so UI can render a message.
    if (e instanceof DOMException && e.name === "AbortError") {
      throw new ApiError(0, "timeout", `Request timed out after ${Math.round(timeoutMs / 1000)}s`);
    }
    throw e;
  } finally {
    window.clearTimeout(timeout);
  }
  if (!res.ok) {
    let code = res.statusText;
    let msg = "";
    try {
      const body = await res.json();
      code = body.error ?? code;
      msg = body.message ?? "";
    } catch {
      // fallthrough
    }
    throw new ApiError(res.status, code, msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  login: (token: string) =>
    request<{ token: string; role: string }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ token }),
    }),
  me: () => request<{ subject: string; role: string }>("/api/v1/auth/userinfo"),

  listApps: () => request<Application[]>("/api/v1/applications"),
  getApp: (name: string) => request<Application>(`/api/v1/applications/${name}`),
  createApp: (req: CreateApplicationRequest) =>
    request<Application>("/api/v1/applications", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  updateApp: (name: string, req: UpdateApplicationRequest) =>
    request<Application>(`/api/v1/applications/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),
  deleteApp: (name: string) =>
    request<void>(`/api/v1/applications/${name}`, { method: "DELETE" }),
  syncApp: (
    name: string,
    opts?: { revision?: string; prune?: boolean; dryRun?: boolean; resources?: string[] },
  ) => {
    const body: Record<string, unknown> = {};
    if (opts?.revision) body.revision = opts.revision;
    if (opts?.prune) body.prune = true;
    if (opts?.dryRun) body.dryRun = true;
    if (opts?.resources?.length) body.resources = opts.resources;
    return request<{ syncId: string; status: string }>(`/api/v1/applications/${name}/sync`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },
  cancelSync: (name: string, syncId: string) =>
    request<void>(`/api/v1/applications/${encodeURIComponent(name)}/sync/${encodeURIComponent(syncId)}`, {
      method: "DELETE",
    }),
  refreshApp: (name: string) =>
    request<void>(`/api/v1/applications/${name}/refresh`, { method: "POST" }),
  appDiff: (name: string) =>
    request<DiffResponse>(`/api/v1/applications/${name}/diff`),
  appTree: (name: string) =>
    request<ResourceTree>(`/api/v1/applications/${name}/resource-tree`),
  appManifests: (name: string) =>
    request<{ revision: string; manifests: unknown[] }>(`/api/v1/applications/${name}/manifests`),
  appHistory: (name: string) =>
    request<SyncOperation[]>(`/api/v1/applications/${name}/history`),

  appRevisions: (name: string, limit = 50) =>
    request<{ commits: GitCommit[] }>(
      `/api/v1/applications/${encodeURIComponent(name)}/revisions?limit=${limit}`,
    ),

  appRevisionDiff: (name: string, from: string, to: string) =>
    request<{ diff: string }>(
      `/api/v1/applications/${encodeURIComponent(name)}/revision-diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
    ),

  appRollback: (name: string, revision: string) =>
    request<{ targetRevision: string }>(
      `/api/v1/applications/${encodeURIComponent(name)}/rollback`,
      {
        method: "POST",
        body: JSON.stringify({ revision }),
      },
    ),

  syncAppDryRun: (name: string, revision?: string) =>
    request<{ syncId: string; status: string }>(
      `/api/v1/applications/${encodeURIComponent(name)}/sync`,
      {
        method: "POST",
        body: JSON.stringify({ dryRun: true, revision }),
      },
    ),

  applyLiveResource: (appName: string, yaml: string) =>
    request<Record<string, unknown>>(`/api/v1/applications/${encodeURIComponent(appName)}/live-resource`, {
      method: "PUT",
      headers: { "Content-Type": "application/yaml" },
      body: yaml,
    }),

  deleteLiveResource: (
    appName: string,
    res: { group: string; version: string; kind: string; namespace?: string; name: string },
  ) => {
    const q = new URLSearchParams({
      group: res.group,
      version: res.version,
      kind: res.kind,
      name: res.name,
    });
    if (res.namespace) q.set("namespace", res.namespace);
    return request<void>(
      `/api/v1/applications/${encodeURIComponent(appName)}/live-resource?${q}`,
      { method: "DELETE" },
    );
  },

  syncLiveResource: (
    appName: string,
    res: { group: string; version: string; kind: string; namespace?: string; name: string },
  ) => {
    const q = new URLSearchParams({
      group: res.group,
      version: res.version,
      kind: res.kind,
      name: res.name,
    });
    if (res.namespace) q.set("namespace", res.namespace);
    return request<{ syncId: string; status: string }>(
      `/api/v1/applications/${encodeURIComponent(appName)}/live-resource/sync?${q}`,
      { method: "POST" },
    );
  },

  restartLiveResource: (
    appName: string,
    res: { group: string; version: string; kind: string; namespace?: string; name: string },
  ) => {
    const q = new URLSearchParams({
      group: res.group,
      version: res.version,
      kind: res.kind,
      name: res.name,
    });
    if (res.namespace) q.set("namespace", res.namespace);
    return request<{ message?: string; action?: string }>(
      `/api/v1/applications/${encodeURIComponent(appName)}/live-resource/restart?${q}`,
      { method: "POST" },
    );
  },

  deletePod: (app: string, pod: string) =>
    request<void>(
      `/api/v1/applications/${encodeURIComponent(app)}/pods/${encodeURIComponent(pod)}`,
      { method: "DELETE" },
    ),

  getPod: (app: string, pod: string) =>
    request<PodSummary>(
      `/api/v1/applications/${encodeURIComponent(app)}/pods/${encodeURIComponent(pod)}`,
    ),

  getPodEvents: (app: string, pod: string) =>
    request<PodEvent[]>(
      `/api/v1/applications/${encodeURIComponent(app)}/pods/${encodeURIComponent(pod)}/events`,
    ),

  getPodShell: (app: string, pod: string, container: string) =>
    request<{ shell: string }>(
      `/api/v1/applications/${encodeURIComponent(app)}/pods/${encodeURIComponent(pod)}/shell?container=${encodeURIComponent(container)}`,
    ),

  getResourceEvents: (app: string, kind: string, name: string, namespace?: string) => {
    const q = new URLSearchParams({ kind, name });
    if (namespace) q.set("namespace", namespace);
    return request<PodEvent[]>(
      `/api/v1/applications/${encodeURIComponent(app)}/resource-events?${q}`,
    );
  },

  async getPodLog(
    app: string,
    pod: string,
    opts: { container?: string; tailLines?: number; follow?: boolean } = {},
  ): Promise<string> {
    const q = new URLSearchParams();
    if (opts.container) q.set("container", opts.container);
    if (opts.tailLines != null) q.set("tailLines", String(opts.tailLines));
    if (opts.follow) q.set("follow", "true");
    const res = await fetch(
      `/api/v1/applications/${encodeURIComponent(app)}/pods/${encodeURIComponent(pod)}/log?${q}`,
      { credentials: "include" },
    );
    if (!res.ok) {
      let code = res.statusText;
      let msg = "";
      try {
        const body = await res.json();
        code = body.error ?? code;
        msg = body.message ?? "";
      } catch {
        // ignore
      }
      throw new ApiError(res.status, code, msg);
    }
    return res.text();
  },

  listRepos: () => request<Repository[]>("/api/v1/repositories"),
  createRepo: (url: string, username?: string, password?: string) =>
    request<Repository>("/api/v1/repositories", {
      method: "POST",
      body: JSON.stringify({ url, username, password }),
    }),
  deleteRepo: (id: string) =>
    request<void>(`/api/v1/repositories/${id}`, { method: "DELETE" }),

  listClusters: () => request<Cluster[]>("/api/v1/clusters"),

  // Generic HTTP helpers for extension endpoints
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body: unknown) =>
    request<T>(path, { method: "POST", body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: "PUT", body: JSON.stringify(body) }),
  del: <T>(path: string) => request<T>(path, { method: "DELETE" }),

  // RBAC
  listRoles: () => request<Role[]>("/api/v1/rbac/roles"),
  getRole: (id: string) => request<Role>(`/api/v1/rbac/roles/${id}`),
  createRole: (req: CreateRoleRequest) =>
    request<Role>("/api/v1/rbac/roles", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  updateRole: (id: string, req: UpdateRoleRequest) =>
    request<Role>(`/api/v1/rbac/roles/${id}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),
  deleteRole: (id: string) =>
    request<void>(`/api/v1/rbac/roles/${id}`, { method: "DELETE" }),

  listRoleBindings: () => request<RoleBinding[]>("/api/v1/rbac/bindings"),
  createRoleBinding: (req: CreateRoleBindingRequest) =>
    request<RoleBinding>("/api/v1/rbac/bindings", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  updateRoleBinding: (id: string, req: UpdateRoleBindingRequest) =>
    request<RoleBinding>(`/api/v1/rbac/bindings/${id}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),
  deleteRoleBinding: (id: string) =>
    request<void>(`/api/v1/rbac/bindings/${id}`, { method: "DELETE" }),

  listPermissions: () => request<PermissionInfo[]>("/api/v1/rbac/permissions"),

  listUsers: () => request<UserInfo[]>("/api/v1/users"),
  getUser: (id: string) => request<UserInfo>(`/api/v1/users/${id}`),
  createUser: (req: CreateUserRequest) =>
    request<UserInfo>("/api/v1/users", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  updateUser: (id: string, req: UpdateUserRequest) =>
    request<UserInfo>(`/api/v1/users/${id}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),
  deleteUser: (id: string) =>
    request<void>(`/api/v1/users/${id}`, { method: "DELETE" }),

  // System configuration
  getSystemConfig: () => request<SystemConfig>("/api/v1/system/config"),
  updateSystemConfig: (body: Record<string, unknown>) =>
    request<SystemConfig>("/api/v1/system/config", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  // Cluster health & nodes
  listClusterHealth: () => request<ClusterHealth[]>("/api/v1/clusters/health"),
  listClusterNodes: (clusterId: string) =>
    request<NodeInfo[]>(`/api/v1/clusters/${clusterId}/nodes`),

  // Network map
  appNetworkMap: (appName: string) =>
    request<NetworkMapResponse>(`/api/v1/applications/${appName}/network-map`),
};
