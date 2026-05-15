import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { DiffEditor } from "@monaco-editor/react";
import { api } from "../api/client";
import { SyncBadge } from "./Badges";
import { stripManagedFieldsYaml } from "../utils/yamlManagedFields";
import type { ResourceDiff } from "../api/types";

export function DiffView({ name, variant = "page" }: { name: string; variant?: "page" | "drawer" }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const isDrawer = variant === "drawer";
  const stackedFromUrl = searchParams.get("view") === "stacked";
  const [stackedLocal, setStackedLocal] = useState(true);
  const stacked = isDrawer ? stackedLocal : stackedFromUrl;

  const { data, isLoading, error } = useQuery({
    queryKey: ["app-diff", name],
    queryFn: () => api.appDiff(name),
    refetchInterval: 10_000,
  });
  const [selected, setSelected] = useState(0);
  const [hideManaged, setHideManaged] = useState(true);
  const [inlineDiff, setInlineDiff] = useState(false);

  const items = useMemo(() => data?.resources ?? [], [data?.resources]);

  const setStacked = (v: boolean) => {
    if (isDrawer) {
      setStackedLocal(v);
      return;
    }
    const next = new URLSearchParams(searchParams);
    if (v) next.set("view", "stacked");
    else next.delete("view");
    setSearchParams(next, { replace: true });
  };

  if (isLoading) return <div className="text-sm text-[var(--color-text-muted)]">Computing diff…</div>;
  if (error) return <div className="text-sm text-red-400">{(error as Error).message}</div>;
  if (!data) return null;
  if (!items.length) return <div className="text-sm text-[var(--color-text-muted)]">Nothing to diff.</div>;

  const scrollClass = isDrawer ? "flex-1 min-h-0 overflow-y-auto pr-1" : "space-y-6 max-h-[calc(100vh-220px)] overflow-y-auto pr-1";

  if (stacked) {
    return (
      <div className={isDrawer ? "flex flex-col h-full min-h-0 gap-3" : "space-y-4"}>
        <div className="flex flex-wrap items-center justify-between gap-3 shrink-0">
          <div className="text-sm text-[var(--color-text)]">
            <span className="font-medium">All resources</span>
            <span className="text-[var(--color-text-muted)] ml-2">
              {data.outOfSync} out of sync · {data.synced} synced
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-4 text-xs">
            <label className="inline-flex items-center gap-1.5 text-[var(--color-text)] cursor-pointer">
              <input type="checkbox" checked={hideManaged} onChange={(e) => setHideManaged(e.target.checked)} />
              Hide managed fields
            </label>
            <label className="inline-flex items-center gap-1.5 text-[var(--color-text)] cursor-pointer">
              <input type="checkbox" checked={inlineDiff} onChange={(e) => setInlineDiff(e.target.checked)} />
              Inline diff
            </label>
            <button type="button" className="text-[var(--color-accent)] underline" onClick={() => setStacked(false)}>
              Per-resource view
            </button>
          </div>
        </div>
        <div className={scrollClass}>
          {items.map((r, i) => (
            <StackedDiffBlock key={`${r.kind}-${r.name}-${i}`} r={r} hideManaged={hideManaged} inlineDiff={inlineDiff} />
          ))}
        </div>
      </div>
    );
  }

  const cur = items[selected];

  return (
    <div className={isDrawer ? "flex flex-col h-full min-h-0 gap-3" : "flex flex-col gap-3 h-[70vh]"}>
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs shrink-0">
        <button type="button" className="text-[var(--color-accent)] underline" onClick={() => setStacked(true)}>
          Stacked view (all parts)
        </button>
        <div className="flex flex-wrap gap-4">
          <label className="inline-flex items-center gap-1.5 text-[var(--color-text)] cursor-pointer">
            <input type="checkbox" checked={hideManaged} onChange={(e) => setHideManaged(e.target.checked)} />
            Hide managed fields
          </label>
          <label className="inline-flex items-center gap-1.5 text-[var(--color-text)] cursor-pointer">
            <input type="checkbox" checked={inlineDiff} onChange={(e) => setInlineDiff(e.target.checked)} />
            Inline diff
          </label>
        </div>
      </div>
      <div className="flex flex-1 min-h-0 gap-4">
        <aside className="w-72 shrink-0 border border-[var(--color-border)] rounded-lg bg-[var(--color-surface)] overflow-auto">
          <div className="px-3 py-2 text-xs text-[var(--color-text-muted)] border-b border-[var(--color-border)]">
            {data.outOfSync} out of sync · {data.synced} synced
          </div>
          {items.map((r, i) => (
            <button
              key={`${r.kind}-${r.name}`}
              type="button"
              onClick={() => setSelected(i)}
              className={`block w-full text-left px-3 py-2 text-sm text-[var(--color-text)] hover:bg-[var(--color-accent-muted)]/50 ${
                i === selected ? "bg-[var(--color-accent-muted)]/35" : ""
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium truncate">
                  {r.kind}/{r.name}
                </span>
                <SyncBadge status={r.sync} />
              </div>
              <div className="text-xs text-[var(--color-text-muted)]">{r.namespace}</div>
            </button>
          ))}
        </aside>
        <section className="flex-1 border border-[var(--color-border)] rounded-lg bg-[var(--color-surface-muted)] overflow-hidden min-h-0">
          <SingleResourceDiff key={`${selected}-${inlineDiff}-${hideManaged}`} r={cur} hideManaged={hideManaged} inlineDiff={inlineDiff} />
        </section>
      </div>
    </div>
  );
}

function StackedDiffBlock({ r, hideManaged, inlineDiff }: { r: ResourceDiff; hideManaged: boolean; inlineDiff: boolean }) {
  const live = hideManaged ? stripManagedFieldsYaml(r.liveYaml || "# (no live object)") : r.liveYaml || "# (no live object)";
  const desired = hideManaged ? stripManagedFieldsYaml(r.desiredYaml) : r.desiredYaml;
  return (
    <div className="rounded-lg border border-[var(--color-border)] overflow-hidden bg-[var(--color-surface-muted)] mb-6 last:mb-0">
      <div className="px-3 py-2 text-xs font-medium text-[var(--color-text)] border-b border-[var(--color-border)] flex items-center justify-between gap-2 bg-[var(--color-surface)]">
        <span>
          {r.kind} / {r.name}
          {r.namespace && <span className="text-[var(--color-text-muted)] ml-2">{r.namespace}</span>}
        </span>
        <SyncBadge status={r.sync} />
      </div>
      <div className="h-[min(360px,40vh)] min-h-[200px]">
        <DiffEditor
          height="100%"
          theme="vs-dark"
          language="yaml"
          original={live}
          modified={desired}
          options={{ readOnly: true, renderSideBySide: !inlineDiff, minimap: { enabled: false } }}
        />
      </div>
    </div>
  );
}

function SingleResourceDiff({
  r,
  hideManaged,
  inlineDiff,
}: {
  r: ResourceDiff;
  hideManaged: boolean;
  inlineDiff: boolean;
}) {
  const live = hideManaged ? stripManagedFieldsYaml(r.liveYaml || "# (no live object)") : r.liveYaml || "# (no live object)";
  const desired = hideManaged ? stripManagedFieldsYaml(r.desiredYaml) : r.desiredYaml;
  return (
    <DiffEditor
      height="100%"
      theme="vs-dark"
      language="yaml"
      original={live}
      modified={desired}
      options={{ readOnly: true, renderSideBySide: !inlineDiff, minimap: { enabled: false } }}
    />
  );
}
