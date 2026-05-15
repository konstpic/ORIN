import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Application, SyncPolicy, UpdateApplicationRequest } from "../api/types";
import { HealthBadge, SyncBadge } from "./Badges";

function fmt(ts: string) {
  try {
    const d = new Date(ts);
    return d.toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
  } catch {
    return ts;
  }
}

function effectiveCreateNamespace(p: SyncPolicy): boolean {
  if (p.createNamespace) return true;
  const opts = p.syncOptions ?? [];
  return opts.some((raw) => {
    const s = raw.trim();
    const i = s.indexOf("=");
    if (i < 0) return false;
    const k = s.slice(0, i).trim().toLowerCase();
    const v = s.slice(i + 1).trim().toLowerCase();
    return k === "createnamespace" && v === "true";
  });
}

type Mode = "view" | "edit";

type EditState = {
  repoUrl: string;
  path: string;
  targetRevision: string;
  helmValueFiles: string;
  cluster: string;
  namespace: string;
  automated: boolean;
  prune: boolean;
  selfHeal: boolean;
  createNamespace: boolean;
  syncOptions: string;
};

function buildState(app: Application): EditState {
  return {
    repoUrl: app.source.repoUrl,
    path: app.source.path ?? "",
    targetRevision: app.source.targetRevision ?? "HEAD",
    helmValueFiles: (app.source.helmValueFiles ?? []).join("\n"),
    cluster: app.destination.cluster,
    namespace: app.destination.namespace,
    automated: !!app.syncPolicy.automated,
    prune: !!app.syncPolicy.automated?.prune,
    selfHeal: !!app.syncPolicy.automated?.selfHeal,
    createNamespace: !!app.syncPolicy.createNamespace,
    syncOptions: (app.syncPolicy.syncOptions ?? []).join("\n"),
  };
}

function stateToPayload(s: EditState, base: Application): UpdateApplicationRequest {
  const helmValueFiles = s.helmValueFiles
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  const syncOptions = s.syncOptions
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  return {
    source: {
      repoUrl: s.repoUrl.trim(),
      path: s.path.trim(),
      targetRevision: s.targetRevision.trim() || "HEAD",
      helmValueFiles,
      helmValues: base.source.helmValues,
    },
    destination: {
      cluster: s.cluster.trim(),
      namespace: s.namespace.trim(),
    },
    syncPolicy: {
      automated: s.automated ? { prune: s.prune, selfHeal: s.selfHeal } : null,
      syncOptions,
      managedNamespaceMetadata: base.syncPolicy.managedNamespaceMetadata,
      createNamespace: s.createNamespace,
      ignoreDifferences: base.syncPolicy.ignoreDifferences,
    },
  };
}

export function ApplicationDetailsDrawer({
  app,
  open,
  onClose,
}: {
  app: Application;
  open: boolean;
  onClose: () => void;
}) {
  const [mode, setMode] = useState<Mode>("view");
  const [state, setState] = useState<EditState>(() => buildState(app));
  const [error, setError] = useState<string | null>(null);
  const qc = useQueryClient();

  useEffect(() => {
    if (open) {
      setMode("view");
      setState(buildState(app));
      setError(null);
    }
  }, [open, app]);

  const saveMut = useMutation({
    mutationFn: () => api.updateApp(app.name, stateToPayload(state, app)),
    onSuccess: (next) => {
      qc.setQueryData(["app", app.name], next);
      qc.invalidateQueries({ queryKey: ["apps"] });
      qc.invalidateQueries({ queryKey: ["app", app.name] });
      setMode("view");
      setError(null);
    },
    onError: (e) => setError((e as Error).message),
  });

  if (!open) return null;

  const auto = app.syncPolicy.automated;
  const editing = mode === "edit";

  return (
    <div className="fixed inset-0 z-50 flex justify-center items-stretch sm:items-center p-0 sm:p-6 pointer-events-none">
      <button type="button" className="absolute inset-0 bg-black/55 pointer-events-auto" aria-label="Close" onClick={onClose} />
      <div className="relative pointer-events-auto w-full max-w-3xl max-h-[min(92vh,900px)] mt-auto sm:mt-0 rounded-t-2xl sm:rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col overflow-hidden">
        <div className="shrink-0 flex items-start justify-between gap-3 px-5 py-4 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)]">
          <div className="min-w-0">
            <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">Application</div>
            <h2 className="text-lg font-semibold text-[var(--color-text)] truncate">{app.name}</h2>
            <div className="mt-2 flex flex-wrap gap-2">
              <SyncBadge status={app.status.sync} />
              <HealthBadge status={app.status.health} />
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {!editing ? (
              <button
                type="button"
                className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                onClick={() => setMode("edit")}
              >
                Edit
              </button>
            ) : (
              <>
                <button
                  type="button"
                  className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                  onClick={() => { setState(buildState(app)); setMode("view"); setError(null); }}
                  disabled={saveMut.isPending}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-semibold text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                  onClick={() => saveMut.mutate()}
                  disabled={saveMut.isPending}
                >
                  {saveMut.isPending ? "Saving…" : "Save"}
                </button>
              </>
            )}
            <button type="button" className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline shrink-0" onClick={onClose}>
              Close
            </button>
          </div>
        </div>

        {error && (
          <div className="shrink-0 px-5 py-2 bg-red-500/10 border-b border-red-500/30 text-xs text-red-300">{error}</div>
        )}

        <div className="flex-1 min-h-0 overflow-y-auto px-5 py-4 text-sm space-y-6">
          <Section title="Source">
            {editing ? (
              <div className="grid gap-3 sm:grid-cols-[140px_1fr]">
                <Label>Repository</Label>
                <input
                  type="text"
                  className={inputClass}
                  value={state.repoUrl}
                  onChange={(e) => setState((s) => ({ ...s, repoUrl: e.target.value }))}
                  placeholder="https://github.com/org/repo.git"
                />
                <Label>Path</Label>
                <input
                  type="text"
                  className={inputClass}
                  value={state.path}
                  onChange={(e) => setState((s) => ({ ...s, path: e.target.value }))}
                  placeholder="."
                />
                <Label>Target revision</Label>
                <input
                  type="text"
                  className={inputClass}
                  value={state.targetRevision}
                  onChange={(e) => setState((s) => ({ ...s, targetRevision: e.target.value }))}
                  placeholder="HEAD"
                />
                <Label>Helm value files</Label>
                <textarea
                  className={`${inputClass} min-h-[64px] font-mono`}
                  value={state.helmValueFiles}
                  onChange={(e) => setState((s) => ({ ...s, helmValueFiles: e.target.value }))}
                  placeholder={"values.yaml\nenv/prod.yaml"}
                />
              </div>
            ) : (
              <dl className="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-[var(--color-text)]">
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Repository</dt>
                <dd className="break-all text-xs">{app.source.repoUrl}</dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Path</dt>
                <dd className="font-mono text-xs">{app.source.path || "."}</dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Target revision</dt>
                <dd className="font-mono text-xs">{app.source.targetRevision}</dd>
                {app.source.helmValueFiles && app.source.helmValueFiles.length > 0 ? (
                  <>
                    <dt className="text-[var(--color-text-muted)] text-xs uppercase">Helm value files</dt>
                    <dd className="font-mono text-xs">
                      {app.source.helmValueFiles.join(", ")}
                    </dd>
                  </>
                ) : null}
              </dl>
            )}
          </Section>

          <Section title="Destination">
            {editing ? (
              <div className="grid gap-3 sm:grid-cols-[140px_1fr]">
                <Label>Cluster</Label>
                <input
                  type="text"
                  className={inputClass}
                  value={state.cluster}
                  onChange={(e) => setState((s) => ({ ...s, cluster: e.target.value }))}
                  placeholder="in-cluster"
                />
                <Label>Namespace</Label>
                <input
                  type="text"
                  className={inputClass}
                  value={state.namespace}
                  onChange={(e) => setState((s) => ({ ...s, namespace: e.target.value }))}
                  placeholder="default"
                />
              </div>
            ) : (
              <dl className="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-[var(--color-text)]">
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Cluster</dt>
                <dd className="font-mono text-xs">{app.destination.cluster}</dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Namespace</dt>
                <dd className="font-mono text-xs">{app.destination.namespace}</dd>
              </dl>
            )}
          </Section>

          <Section title="Sync policy">
            {editing ? (
              <div className="space-y-3 text-sm">
                <label className="inline-flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={state.automated}
                    onChange={(e) => setState((s) => ({ ...s, automated: e.target.checked }))}
                  />
                  <span>Automated sync</span>
                </label>
                {state.automated && (
                  <div className="ml-6 space-y-1">
                    <label className="inline-flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={state.prune}
                        onChange={(e) => setState((s) => ({ ...s, prune: e.target.checked }))}
                      />
                      Prune resources removed from desired state
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={state.selfHeal}
                        onChange={(e) => setState((s) => ({ ...s, selfHeal: e.target.checked }))}
                      />
                      Self-heal live drift
                    </label>
                  </div>
                )}
                <label className="inline-flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={state.createNamespace}
                    onChange={(e) => setState((s) => ({ ...s, createNamespace: e.target.checked }))}
                  />
                  Create destination namespace before sync
                </label>
                <div>
                  <Label>Sync options (one per line)</Label>
                  <textarea
                    className={`${inputClass} min-h-[80px] font-mono`}
                    value={state.syncOptions}
                    onChange={(e) => setState((s) => ({ ...s, syncOptions: e.target.value }))}
                    placeholder={"CreateNamespace=true\nServerSideApply=true"}
                  />
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                {auto ? (
                  <ul className="text-sm text-[var(--color-text)] space-y-1 list-disc pl-5">
                    <li>Automated sync enabled</li>
                    <li>Prune: {auto.prune ? "yes" : "no"}</li>
                    <li>Self-heal: {auto.selfHeal ? "yes" : "no"}</li>
                  </ul>
                ) : (
                  <p className="text-sm text-[var(--color-text-muted)]">No automated sync (manual sync only).</p>
                )}
                <ul className="text-sm text-[var(--color-text)] space-y-1 list-disc pl-5">
                  <li>Create namespace on sync: {app.syncPolicy.createNamespace ? "yes" : "no"}</li>
                  <li>Effective create namespace: {effectiveCreateNamespace(app.syncPolicy) ? "yes" : "no"}</li>
                </ul>
                {app.syncPolicy.syncOptions && app.syncPolicy.syncOptions.length > 0 ? (
                  <div>
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">Sync options</div>
                    <ul className="text-xs font-mono text-[var(--color-text)] space-y-0.5 list-disc pl-5">
                      {app.syncPolicy.syncOptions.map((o, i) => (
                        <li key={`${i}-${o}`}>{o}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
                {app.syncPolicy.managedNamespaceMetadata &&
                (Object.keys(app.syncPolicy.managedNamespaceMetadata.labels ?? {}).length > 0 ||
                  Object.keys(app.syncPolicy.managedNamespaceMetadata.annotations ?? {}).length > 0) ? (
                  <div>
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">Managed namespace metadata</div>
                    <pre className="text-xs font-mono bg-[var(--color-surface-muted)] rounded-md p-2 overflow-x-auto border border-[var(--color-border)]">
                      {JSON.stringify(app.syncPolicy.managedNamespaceMetadata, null, 2)}
                    </pre>
                  </div>
                ) : null}
              </div>
            )}
          </Section>

          <Section title="Summary">
            <dl className="grid grid-cols-[140px_1fr] gap-x-3 gap-y-2 text-[var(--color-text)]">
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Project</dt>
              <dd className="font-medium">{app.project}</dd>
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Observed revision</dt>
              <dd className="font-mono text-xs">{app.status.observedRevision || "—"}</dd>
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Created</dt>
              <dd className="text-xs">{fmt(app.createdAt)}</dd>
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Updated</dt>
              <dd className="text-xs">{fmt(app.updatedAt)}</dd>
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Last synced</dt>
              <dd className="text-xs">{app.status.lastSyncedAt ? fmt(app.status.lastSyncedAt) : "—"}</dd>
              <dt className="text-[var(--color-text-muted)] text-xs uppercase">Status message</dt>
              <dd className="text-xs text-[var(--color-text-muted)] break-words">{app.status.message || "—"}</dd>
            </dl>
          </Section>
        </div>
      </div>
    </div>
  );
}

const inputClass =
  "w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs text-[var(--color-text)] focus:border-[var(--color-border-strong)] focus:outline-none";

function Label({ children }: { children: React.ReactNode }) {
  return <span className="text-[var(--color-text-muted)] text-[10px] uppercase tracking-wide font-semibold self-center">{children}</span>;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h3 className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-2">{title}</h3>
      {children}
    </section>
  );
}
