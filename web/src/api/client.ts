// Thin typed fetch wrapper. All requests go through this single helper so
// we can attach the bearer cookie and surface server errors uniformly.

import type {
  Application,
  Cluster,
  CreateApplicationRequest,
  DiffResponse,
  GitCommit,
  PodEvent,
  PodSummary,
  Repository,
  ResourceTree,
  SyncOperation,
  UpdateApplicationRequest,
} from "./types";

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const extraHeaders = init.headers as Record<string, string> | undefined;
  const contentType = extraHeaders?.["Content-Type"] ?? "application/json";
  const res = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": contentType,
      ...(init.headers ?? {}),
    },
  });
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
};
