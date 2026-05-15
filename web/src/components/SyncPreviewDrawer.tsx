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

  const handleConfirm = () => {
    const resources =
      selectedKeys.length > 0 && selectedKeys.length < allKeys.length ? selectedKeys : undefined;
    onConfirm({
      prune,
      dryRun,
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
            <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
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
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text)]"
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              type="button"
              className="rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
              disabled={isSubmitting || isLoading || !!error || rows.length === 0 || selectedKeys.length === 0}
              onClick={handleConfirm}
            >
              {isSubmitting ? "Synchronizing…" : "Synchronize"}
            </button>
          </div>
        </div>
      </aside>
    </div>
  );
}
