import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Navigate, Route, Routes, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useCallback, useEffect, useState } from "react";
import {
  ChevronLeft,
  FileDiff,
  GitCommitHorizontal,
  History,
  MoreHorizontal,
  RefreshCcw,
  Settings,
  Trash2,
  PlayCircle,
  User,
  Clock,
  AlertTriangle,
  Loader2,
  XCircle,
} from "lucide-react";
import { api } from "../api/client";
import { HealthBadge, LastSyncFailedBadge, SyncBadge, SyncOperationBadge } from "../components/Badges";
import { SyncPreviewDrawer, type SyncSubmitPayload } from "../components/SyncPreviewDrawer";
import { ApplicationDetailsDrawer } from "../components/ApplicationDetailsDrawer";
import { ResourceTreePanel } from "../components/ResourceTreePanel";
import { DiffDrawer } from "../components/DiffDrawer";
import { HistoryDrawer } from "../components/HistoryDrawer";
import { OutOfSyncHelper } from "../components/OutOfSyncHelper";
import { OutOfSyncDrawer } from "../components/OutOfSyncDrawer";
import { useAuth } from "../state/auth";
import { openAppEvents } from "../api/ws";

function fmtShort(ts?: string | null) {
  if (!ts) return "—";
  try {
    return new Date(ts).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
  } catch {
    return ts;
  }
}

// Collapsible sync failure banner — shows a summary line, expands to full message
function SyncFailureBanner({ message, onHistory }: { message: string; onHistory: () => void }) {
  const [expanded, setExpanded] = useState(false);

  // Parse "N resources failed. First 3: ..." or "Some resources failed: ..." into individual items
  const items = message
    .replace(/^(\d+ resources failed\. First \d+: |Some resources failed: )/, "")
    .split(/;\s*/)
    .map((s) => s.trim())
    .filter(Boolean);

  const headerMatch = message.match(/^(\d+) resources failed/);
  const totalCount = headerMatch ? parseInt(headerMatch[1]) : items.length;
  const summary = totalCount > 1
    ? `${totalCount} resources failed to sync`
    : items[0]?.split(":")[0] ?? "Sync failed";

  return (
    <div className="rounded-lg border border-red-500/50 bg-red-500/10 px-4 py-3">
      {/* Header row */}
      <div className="flex items-start gap-3">
        <AlertTriangle className="size-4 shrink-0 text-red-400 mt-0.5" strokeWidth={2} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-red-200">{summary}</span>
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="text-xs text-red-300/70 hover:text-red-200 underline"
            >
              {expanded ? "collapse" : "details"}
            </button>
          </div>
        </div>
        <button
          type="button"
          className="text-xs text-red-200 hover:text-red-100 underline inline-flex items-center gap-1 shrink-0"
          onClick={onHistory}
        >
          <History className="size-3" />
          History
        </button>
      </div>

      {/* Expanded list */}
      {expanded && items.length > 0 && (
        <div className="mt-2 pt-2 border-t border-red-500/30 space-y-1 max-h-48 overflow-y-auto">
          {items.map((item, i) => {
            const colonIdx = item.indexOf(":");
            const resource = colonIdx > -1 ? item.slice(0, colonIdx) : item;
            const detail = colonIdx > -1 ? item.slice(colonIdx + 1).trim() : "";
            return (
              <div key={i} className="text-xs text-red-300/90 leading-snug">
                <span className="font-mono font-medium text-red-200">{resource}</span>
                {detail && <span className="text-red-300/70">: {detail}</span>}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

export function AppDetailPage() {
  const { name = "" } = useParams();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [syncPreviewOpen, setSyncPreviewOpen] = useState(false);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [overflowOpen, setOverflowOpen] = useState(false);
  const token = useAuth((s) => s.token) ?? undefined;

  const diffOpen = searchParams.get("diff") === "1";
  const historyOpen = searchParams.get("history") === "1";
  const outOfSyncOpen = searchParams.get("outOfSync") === "1";

  const setDiffOpen = useCallback(
    (open: boolean) => {
      const next = new URLSearchParams(searchParams);
      if (open) next.set("diff", "1");
      else next.delete("diff");
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const setHistoryOpen = useCallback(
    (open: boolean) => {
      const next = new URLSearchParams(searchParams);
      if (open) next.set("history", "1");
      else next.delete("history");
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const setOutOfSyncOpen = useCallback(
    (open: boolean) => {
      const next = new URLSearchParams(searchParams);
      if (open) next.set("outOfSync", "1");
      else next.delete("outOfSync");
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const { data: app, isLoading, error } = useQuery({
    queryKey: ["app", name],
    queryFn: () => api.getApp(name),
    refetchInterval: 10_000,
  });

  // Fetch diff data to show OutOfSync count
  const { data: diffData } = useQuery({
    queryKey: ["app-diff", name],
    queryFn: () => api.appDiff(name),
    enabled: !!app && app.status.sync === "OutOfSync",
    refetchInterval: 15_000,
  });

  useEffect(() => {
    if (!name) return;
    const conn = openAppEvents(name, token, () => {
      qc.invalidateQueries({ queryKey: ["app", name] });
      qc.invalidateQueries({ queryKey: ["app-tree", name] });
      qc.invalidateQueries({ queryKey: ["app-diff", name] });
      qc.invalidateQueries({ queryKey: ["app-history", name] });
    });
    return () => conn.close();
  }, [name, token, qc]);

  const sync = useMutation({
    mutationFn: (payload: SyncSubmitPayload) =>
      api.syncApp(name, {
        prune: payload.prune,
        dryRun: payload.dryRun,
        resources: payload.resources,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["app", name] });
      qc.invalidateQueries({ queryKey: ["app-tree", name] });
      setSyncPreviewOpen(false);
    },
  });
  const refresh = useMutation({
    mutationFn: () => api.refreshApp(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["app", name] });
      qc.invalidateQueries({ queryKey: ["app-tree", name] });
      qc.invalidateQueries({ queryKey: ["app-diff", name] });
    },
  });
  const cancelSync = useMutation({
    mutationFn: (syncId: string) => api.cancelSync(name, syncId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["app", name] });
      qc.invalidateQueries({ queryKey: ["app-tree", name] });
      qc.invalidateQueries({ queryKey: ["app-history", name] });
    },
  });
  const del = useMutation({
    mutationFn: () => api.deleteApp(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["apps"] });
      navigate("/applications");
    },
  });

  if (isLoading) return <div className="p-8 text-sm text-[var(--color-text-muted)]">Loading…</div>;
  if (error || !app) return <div className="p-8 text-sm text-red-400">{(error as Error)?.message}</div>;

  const appBase = `/applications/${encodeURIComponent(name)}`;

  return (
    <div className="flex-1 min-h-0 h-full flex flex-col">
      <header className="border-b border-[var(--color-border)] bg-[var(--color-surface)] shadow-lg shrink-0">
        <div className="max-w-[1800px] mx-auto px-6 py-3 flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:bg-[var(--color-surface-muted)]"
            onClick={() => navigate("/applications")}
            title="Back to applications"
          >
            <ChevronLeft className="size-3.5" />
            Applications
          </button>
          <div className="flex-1" />
          <div className="flex flex-wrap items-center gap-2 justify-end">
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
              onClick={() => refresh.mutate()}
              disabled={refresh.isPending}
              title="Re-render manifests & refresh tree"
            >
              <RefreshCcw className={`size-3.5 ${refresh.isPending ? "animate-spin" : ""}`} />
              Refresh
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
              onClick={() => setDiffOpen(true)}
            >
              <FileDiff className="size-3.5" />
              Diff
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
              onClick={() => setHistoryOpen(true)}
            >
              <History className="size-3.5" />
              History
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
              onClick={() => setDetailsOpen(true)}
            >
              <Settings className="size-3.5" />
              Details
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-semibold text-[#0a0e14] hover:brightness-110 disabled:opacity-50 shadow"
              onClick={() => setSyncPreviewOpen(true)}
              disabled={sync.isPending || !!app.status.syncOperation}
            >
              <PlayCircle className="size-3.5" />
              {app.status.syncOperation ? "Sync…" : "Sync"}
            </button>
            <div className="relative">
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                onClick={() => setOverflowOpen((v) => !v)}
                aria-haspopup="menu"
                aria-expanded={overflowOpen}
                title="More actions"
              >
                <MoreHorizontal className="size-3.5" />
              </button>
              {overflowOpen && (
                <>
                  <button
                    type="button"
                    aria-label="Close menu"
                    className="fixed inset-0 z-40 cursor-default"
                    onClick={() => setOverflowOpen(false)}
                  />
                  <div
                    role="menu"
                    className="absolute right-0 z-50 mt-1 w-48 rounded-md border border-[var(--color-border)] bg-[var(--color-elevated)] shadow-xl py-1 text-xs"
                  >
                    <button
                      type="button"
                      role="menuitem"
                      className="w-full text-left px-3 py-2 hover:bg-[var(--color-surface-muted)] text-red-300 inline-flex items-center gap-2"
                      onClick={() => {
                        setOverflowOpen(false);
                        if (window.confirm(`Delete application "${app.name}"?`)) del.mutate();
                      }}
                    >
                      <Trash2 className="size-3.5" />
                      Delete application
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
        <div className="max-w-[1800px] mx-auto px-6 pb-2">
          <div className="rounded border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-3 py-2">
            {/* Application name and project - compact */}
            <div className="flex items-center gap-4 mb-2 pb-2 border-b border-[var(--color-border)]">
              <div className="flex-1 min-w-0">
                <div className="text-[9px] font-medium uppercase tracking-wider text-[var(--color-text-muted)]">
                  {app.project}
                </div>
                <h2 className="text-sm font-semibold text-[var(--color-text)] tracking-tight truncate" title={app.name}>
                  {app.name}
                </h2>
              </div>
              <div className="flex items-center gap-2">
                <HealthBadge status={app.status.health} />
                <div className="flex items-center gap-1">
                  {app.status.sync === "OutOfSync" ? (
                    <button
                      type="button"
                      title="Click to see what's out of sync"
                      onClick={() => setOutOfSyncOpen(true)}
                      className="hover:opacity-80 transition-opacity"
                    >
                      <SyncBadge status={app.status.sync} />
                    </button>
                  ) : (
                    <SyncBadge status={app.status.sync} />
                  )}
                  {app.status.syncOperation ? (
                    <>
                      <SyncOperationBadge
                        status={app.status.syncOperation.status}
                        message={app.status.syncOperation.message}
                      />
                      <button
                        type="button"
                        className="inline-flex items-center gap-1 rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-xs font-medium text-red-200 hover:bg-red-500/20 transition-colors"
                        onClick={() => {
                          if (app.status.syncOperation && window.confirm("Cancel this sync operation?")) {
                            cancelSync.mutate(app.status.syncOperation.id);
                          }
                        }}
                        title="Cancel sync"
                      >
                        <XCircle className="size-3" strokeWidth={2} />
                        Cancel
                      </button>
                    </>
                  ) : null}
                  {!app.status.syncOperation && app.status.lastCompletedSync?.status === "Failed" ? (
                    <LastSyncFailedBadge message={app.status.lastCompletedSync.message} />
                  ) : null}
                </div>
              </div>
            </div>

            {/* Compact info grid */}
            <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
              {/* Revision */}
              <div className="flex items-center gap-1.5 text-[var(--color-text-muted)]">
                <GitCommitHorizontal className="size-3 shrink-0" />
                <div className="group relative">
                  <span className="font-mono text-[var(--color-text)] cursor-help" title={app.status.observedRevision}>
                    {app.status.observedRevision ? app.status.observedRevision.slice(0, 7) : "—"}
                  </span>
                  {app.status.observedCommit && (
                    <div className="invisible group-hover:visible absolute left-0 top-full mt-1 z-50 w-80 rounded-lg border border-[var(--color-border)] bg-[var(--color-elevated)] shadow-2xl p-3">
                      <div className="text-xs text-[var(--color-text)] mb-2 leading-relaxed break-words">
                        {app.status.observedCommit.message}
                      </div>
                      <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
                        <User className="size-3" />
                        <span>{app.status.observedCommit.author || "unknown"}</span>
                        <span>•</span>
                        <span>{new Date(app.status.observedCommit.authorDate).toLocaleString()}</span>
                      </div>
                    </div>
                  )}
                </div>
                <span className="text-[9px]">@ {app.source.targetRevision}</span>
              </div>

              {/* Last sync */}
              <div className="flex items-center gap-1.5 text-[var(--color-text-muted)]">
                <Clock className="size-3 shrink-0" />
                <span className="text-[var(--color-text)]">{fmtShort(app.status.lastSyncedAt)}</span>
              </div>

              {/* Source */}
              <div className="flex items-center gap-1.5 text-[var(--color-text-muted)] col-span-2 truncate">
                <span className="shrink-0">Source:</span>
                <span className="font-mono text-[var(--color-text)] truncate text-[11px]" title={`${app.source.repoUrl} · ${app.source.path || "."}`}>
                  {app.source.repoUrl.replace(/^https?:\/\//, '')}
                  {app.source.path ? <span className="text-[var(--color-text-muted)]"> · {app.source.path}</span> : null}
                </span>
              </div>

              {/* Destination */}
              <div className="flex items-center gap-1.5 text-[var(--color-text-muted)] col-span-2">
                <span className="shrink-0">Dest:</span>
                <span className="font-mono text-[var(--color-text)] text-[11px]">
                  {app.destination.cluster}/{app.destination.namespace}
                </span>
              </div>
            </div>

            {/* Status message */}
            {app.status.message?.trim() ? (
              <div className="mt-2 pt-2 border-t border-[var(--color-border)]">
                <p className="text-xs leading-snug text-amber-300/90 break-words line-clamp-1" title={app.status.message}>
                  {app.status.message}
                </p>
              </div>
            ) : null}
          </div>
        </div>
        {/* Prominent error banner for sync failures */}
        {!app.status.syncOperation && app.status.lastCompletedSync?.status === "Failed" && app.status.lastCompletedSync.message && (
          <div className="max-w-[1800px] mx-auto px-6 pb-3">
            <SyncFailureBanner message={app.status.lastCompletedSync.message} onHistory={() => setHistoryOpen(true)} />
          </div>
        )}
        {app.status.syncOperation?.message && (
          <div className="max-w-[1800px] mx-auto px-6 pb-3">
            <div className="rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-4 py-2.5 flex items-start gap-3">
              <Loader2 className="size-4 shrink-0 text-cyan-400 mt-0.5 animate-spin" strokeWidth={2} />
              <div className="flex-1 min-w-0">
                <div className="text-xs text-cyan-200/90 leading-relaxed break-words">
                  {app.status.syncOperation.message}
                </div>
              </div>
            </div>
          </div>
        )}
        {/* OutOfSync helper with troubleshooting tips */}
        {app.status.sync === "OutOfSync" && !app.status.syncOperation && (
          <div className="max-w-[1800px] mx-auto px-6 pb-3">
            <OutOfSyncHelper appName={name} outOfSyncCount={diffData?.outOfSync ?? 0} />
          </div>
        )}
      </header>

      <div className="flex-1 flex flex-col min-h-0 w-full px-4 sm:px-6 py-3">
        <Routes>
          <Route index element={<ResourceTreePanel name={name} app={app} />} />
          <Route path="diff" element={<Navigate to={`${appBase}?diff=1`} replace />} />
          <Route path="manifests" element={<Navigate to={appBase} replace />} />
          <Route path="history" element={<Navigate to={`${appBase}?history=1`} replace />} />
          <Route path="*" element={<Navigate to={appBase} replace />} />
        </Routes>
      </div>
      <SyncPreviewDrawer
        appName={name}
        open={syncPreviewOpen}
        onClose={() => setSyncPreviewOpen(false)}
        onConfirm={(p) => sync.mutate(p)}
        isSubmitting={sync.isPending}
      />
      <ApplicationDetailsDrawer app={app} open={detailsOpen} onClose={() => setDetailsOpen(false)} />
      <DiffDrawer appName={name} open={diffOpen} onClose={() => setDiffOpen(false)} />
      <HistoryDrawer appName={name} open={historyOpen} onClose={() => setHistoryOpen(false)} />
      <OutOfSyncDrawer appName={name} open={outOfSyncOpen} onClose={() => setOutOfSyncOpen(false)} />
    </div>
  );
}

function StatTile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-3 py-2">
      <div className="text-[10px] uppercase font-semibold tracking-wider text-[var(--color-text-muted)]">{label}</div>
      <div className="mt-1">{children}</div>
    </div>
  );
}
