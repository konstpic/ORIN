import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import { diffResourceKey, manifestResourceKey } from "../utils/manifestKey";

type ManifestRow = { key: string; kind: string; name: string; namespace: string; raw: Record<string, unknown> };

function manifestRows(manifests: unknown[]): ManifestRow[] {
  return manifests.map((raw, i) => {
    const m = raw as Record<string, unknown>;
    const meta = (m.metadata as Record<string, unknown>) ?? {};
    const kind = String(m.kind ?? "?");
    const name = String(meta.name ?? `obj-${i}`);
    const namespace = String(meta.namespace ?? "");
    return {
      key: manifestResourceKey(m),
      kind,
      name,
      namespace,
      raw: m,
    };
  });
}

type DryRunResult = {
  syncId: string;
  status: string;
  /** Populated after dry-run completes */
  resources?: Array<{ key: string; kind: string; name: string; namespace: string; status: string; message: string }>;
};

export type SyncSubmitPayload = {
  prune: boolean;
  dryRun: boolean;
  /** Omit or empty = sync all rendered manifests */
  resources?: string[];
};

export function SyncPreviewDrawer({
  appName,
  open,
  onClose,
  onConfirm,
  isSubmitting,
}: {
  appName: string;
  open: boolean;
  onClose: () => void;
  onConfirm: (payload: SyncSubmitPayload) => void;
  isSubmitting: boolean;
}) {
  const [prune, setPrune] = useState(false);
  const [dryRun, setDryRun] = useState(false);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);
  const [dryRunInProgress, setDryRunInProgress] = useState(false);
  const [dryRunError, setDryRunError] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["app-manifests", appName],
    queryFn: () => api.appManifests(appName),
    enabled: open && !!appName,
  });

  const { data: diffData } = useQuery({
    queryKey: ["app-diff", appName],
    queryFn: () => api.appDiff(appName),
    enabled: open && !!appName,
    staleTime: 5000,
  });

  const rows = useMemo(() => (data?.manifests ? manifestRows(data.manifests) : []), [data?.manifests]);

  const outOfSyncKeys = useMemo(() => {
    const s = new Set<string>();
    for (const r of diffData?.resources ?? []) {
      if (r.sync === "OutOfSync") {
        s.add(diffResourceKey(r));
      }
    }
    return s;
  }, [diffData?.resources]);

  const allKeys = useMemo(() => rows.map((r) => r.key), [rows]);
  const selectedKeys = useMemo(() => allKeys.filter((k) => selected[k]), [allKeys, selected]);
  const partial = selectedKeys.length > 0 && selectedKeys.length < allKeys.length;

  useEffect(() => {
    if (!open || !allKeys.length) return;
    setPrune(false);
    setDryRun(false);
    setDryRunResult(null);
    setDryRunInProgress(false);
    setDryRunError(null);
    setSelected(Object.fromEntries(allKeys.map((k) => [k, true])));
  }, [open, appName, data?.revision, allKeys.join("|")]);

  if (!open) return null;

  const setAll = (on: boolean) => {
    const next: Record<string, boolean> = {};
    for (const k of allKeys) next[k] = on;
    setSelected(next);
  };

  const selectOutOfSync = () => {
    const next: Record<string, boolean> = {};
    for (const r of rows) {
      next[r.key] = outOfSyncKeys.has(r.key);
    }
    setSelected(next);
  };

  const toggleRow = (key: string) => {
    setSelected((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleDryRun = async () => {
    setDryRunResult(null);
    setDryRunError(null);
    setDryRunInProgress(true);
    try {
      const res = await api.syncAppDryRun(appName, data?.revision);
      // The backend returns syncId + status. If resources come back, use them;
      // otherwise we build a minimal result from what we know.
      setDryRunResult({
        syncId: res.syncId,
        status: res.status,
        resources: (res as Record<string, unknown>).resources
          ? ((res as Record<string, unknown>).resources as Array<{
              group: string;
              version: string;
              kind: string;
              namespace?: string;
              name: string;
              status: string;
              message: string;
            }>).map((r) => ({
              key: `${r.group}/${r.version}/${r.kind}/${r.namespace ?? ""}/${r.name}`,
              kind: r.kind,
              name: r.name,
              namespace: r.namespace ?? "",
              status: r.status,
              message: r.message,
            }))
          : undefined,
      });
    } catch (e) {
      setDryRunError(e instanceof Error ? e.message : "Dry run failed");
    } finally {
      setDryRunInProgress(false);
    }
  };

  const handleConfirm = () => {
    if (dryRun && !dryRunResult) {
      // First click in dry-run mode: run the dry-run preview
      handleDryRun();
      return;
    }
    // Either not dry-run mode, or dry-run already completed → proceed with real sync
    const resources =
      selectedKeys.length > 0 && selectedKeys.length < allKeys.length ? selectedKeys : undefined;
    onConfirm({
      prune,
      dryRun: false,
      resources: resources?.length ? resources : undefined,
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end pointer-events-none">
      <button type="button" className="absolute inset-0 bg-black/50 pointer-events-auto" aria-label="Close" onClick={onClose} />
      <aside className="relative pointer-events-auto h-full w-full max-w-2xl border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col mt-0">
        <div className="px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] flex justify-between items-center gap-2">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text)]">Synchronize</h2>
            <p className="text-xs text-[var(--color-text-muted)] mt-0.5 font-mono">{appName}</p>
          </div>
          <button type="button" className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline" onClick={onClose}>
            Cancel
          </button>
        </div>

        <div className="px-4 py-2 text-xs text-[var(--color-text-muted)] border-b border-[var(--color-border)] space-y-1">
          {data?.revision && (
            <div>
              Revision: <code className="text-[var(--color-accent)]">{data.revision.slice(0, 12)}</code>
            </div>
          )}
          <div className="text-[var(--color-text-muted)]/90">
            Prune removes tracked objects missing from Git (full sync only). Partial sync never prunes.
          </div>
        </div>

        <div className="px-4 py-3 border-b border-[var(--color-border)] space-y-2">
          <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">Options</div>
          <label className="flex items-center gap-2 text-sm text-[var(--color-text)] cursor-pointer">
            <input type="checkbox" checked={prune} onChange={(e) => setPrune(e.target.checked)} disabled={dryRun} />
            Prune (respects policy; one-time when checked)
          </label>
          <label className="flex items-center gap-2 text-sm text-[var(--color-text)] cursor-pointer">
            <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} disabled={dryRunInProgress} />
            Dry run (server-side apply dry-run; no cluster changes)
          </label>
        </div>

        <div className="px-4 py-2 border-b border-[var(--color-border)] flex flex-wrap gap-2">
          <button type="button" className="text-xs rounded border border-[var(--color-border)] px-2 py-1 hover:border-[var(--color-border-strong)]" onClick={() => setAll(true)}>
            Select all
          </button>
          <button type="button" className="text-xs rounded border border-[var(--color-border)] px-2 py-1 hover:border-[var(--color-border-strong)]" onClick={() => setAll(false)}>
            Select none
          </button>
          <button
            type="button"
            className="text-xs rounded border border-[var(--color-border)] px-2 py-1 hover:border-[var(--color-border-strong)] disabled:opacity-40"
            onClick={selectOutOfSync}
            disabled={!outOfSyncKeys.size}
          >
            Out of sync only
          </button>
        </div>

        {/* Dry-run results panel */}
        {dryRun && (
          <div className="px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
            {dryRunInProgress && (
              <div className="text-sm text-[var(--color-text-muted)] flex items-center gap-2">
                <span className="inline-block size-3 rounded-full border-2 border-[var(--color-accent)] border-t-transparent animate-spin" />
                Running dry-run…
              </div>
            )}
            {dryRunError && !dryRunInProgress && (
              <div>
                <div className="text-sm text-red-400 font-medium">Dry-run failed</div>
                <div className="text-xs text-red-400/80 mt-1 font-mono">{dryRunError}</div>
                <button
                  type="button"
                  className="mt-2 text-xs text-[var(--color-accent)] underline hover:no-underline"
                  onClick={() => { setDryRunError(null); handleDryRun(); }}
                >
                  Retry
                </button>
              </div>
            )}
            {dryRunResult && !dryRunInProgress && (
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-xs font-semibold uppercase tracking-wide text-emerald-400">Dry-run complete</span>
                  <span className="text-[10px] text-[var(--color-text-muted)] font-mono">sync: {dryRunResult.syncId.slice(0, 12)}</span>
                </div>
                {dryRunResult.resources && dryRunResult.resources.length > 0 ? (
                  <>
                    {(() => {
                      const counts = { Applied: 0, Pruned: 0, Failed: 0, DryRun: 0, Other: 0 };
                      for (const r of dryRunResult.resources) {
                        if (r.status === "Applied") counts.Applied++;
                        else if (r.status === "Pruned") counts.Pruned++;
                        else if (r.status === "Failed") counts.Failed++;
                        else if (r.status === "DryRun") counts.DryRun++;
                        else counts.Other++;
                      }
                      return (
                        <div className="flex flex-wrap gap-2 mb-2 text-xs">
                          <span className="text-emerald-400">{counts.Applied} would be Applied</span>
                          {counts.Pruned > 0 && <span className="text-amber-400">{counts.Pruned} would be Pruned</span>}
                          {counts.Failed > 0 && <span className="text-red-400">{counts.Failed} would Fail</span>}
                        </div>
                      );
                    })()}
                    <div className="max-h-48 overflow-y-auto space-y-1 border border-[var(--color-border)] rounded-md p-2">
                      {dryRunResult.resources.map((r) => {
                        const statusColor =
                          r.status === "Applied" ? "text-emerald-400" :
                          r.status === "Pruned" ? "text-amber-400" :
                          r.status === "Failed" ? "text-red-400" :
                          "text-[var(--color-text-muted)]";
                        return (
                          <div key={r.key} className="flex items-center gap-2 text-xs font-mono">
                            <span className={statusColor}>{r.status}</span>
                            <span className="text-[var(--color-text)]">{r.kind}</span>
                            <span className="text-[var(--color-text-muted)]">{r.name}</span>
                            {r.namespace && <span className="text-[var(--color-text-muted)]">· {r.namespace}</span>}
                          </div>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div className="text-sm text-[var(--color-text-muted)]">No resources selected for dry-run.</div>
                )}
              </div>
            )}
          </div>
        )}

        <div className="flex-1 min-h-0 overflow-y-auto px-4 py-3">
          {isLoading && <div className="text-sm text-[var(--color-text-muted)]">Loading manifests…</div>}
          {error && <div className="text-sm text-red-400">{(error as Error).message}</div>}
          {!isLoading && !error && rows.length === 0 && (
            <div className="text-sm text-[var(--color-text-muted)]">No rendered objects — check repo path and revision.</div>
          )}
          {rows.length > 0 && (
            <ul className="space-y-1">
              {rows.map((r) => {
                const oos = outOfSyncKeys.has(r.key);
                const checked = !!selected[r.key];
                return (
                  <li key={r.key}>
                    <label className="flex items-start gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-3 py-2 text-sm cursor-pointer hover:border-[var(--color-border-strong)]">
                      <input type="checkbox" className="mt-1 shrink-0" checked={checked} onChange={() => toggleRow(r.key)} />
                      <span className="min-w-0 flex-1">
                        <span className="font-mono text-xs text-[var(--color-accent)]">{r.kind}</span>
                        <span className="text-[var(--color-text)] font-medium ml-2">{r.name}</span>
                        {r.namespace && <span className="text-xs text-[var(--color-text-muted)] ml-2">· {r.namespace}</span>}
                        {oos && (
                          <span className="ml-2 text-[10px] uppercase font-semibold text-amber-400 border border-amber-500/40 rounded px-1 py-0.5">OutOfSync</span>
                        )}
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <div className="p-4 border-t border-[var(--color-border)] flex flex-wrap items-center justify-between gap-2 bg-[var(--color-surface-muted)]">
          <span className="text-xs text-[var(--color-text-muted)]">
            {selectedKeys.length}/{allKeys.length} selected
            {partial ? " · partial sync" : ""}
            {dryRun && dryRunResult && !dryRunInProgress ? " · dry-run done" : ""}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text)]"
              onClick={onClose}
            >
              Cancel
            </button>
            {dryRun && dryRunResult && !dryRunInProgress ? (
              <>
                <button
                  type="button"
                  className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                  onClick={() => { setDryRunResult(null); setDryRunError(null); }}
                >
                  Re-run dry-run
                </button>
                <button
                  type="button"
                  className="rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                  disabled={isSubmitting || isLoading || !!error || rows.length === 0 || selectedKeys.length === 0}
                  onClick={handleConfirm}
                >
                  {isSubmitting ? "Synchronizing…" : "Proceed with real sync"}
                </button>
              </>
            ) : (
              <button
                type="button"
                className="rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                disabled={isSubmitting || isLoading || !!error || rows.length === 0 || selectedKeys.length === 0 || dryRunInProgress}
                onClick={handleConfirm}
              >
                {isSubmitting ? "Synchronizing…" : dryRunInProgress ? "Running dry-run…" : dryRun ? "Run Dry Run" : "Synchronize"}
              </button>
            )}
          </div>
        </div>
      </aside>
    </div>
  );
}
