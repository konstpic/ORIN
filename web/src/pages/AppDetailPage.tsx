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
} from "lucide-react";
import { api } from "../api/client";
import { HealthBadge, LastSyncFailedBadge, SyncBadge, SyncOperationBadge } from "../components/Badges";
import { SyncPreviewDrawer, type SyncSubmitPayload } from "../components/SyncPreviewDrawer";
import { ApplicationDetailsDrawer } from "../components/ApplicationDetailsDrawer";
import { ResourceTreePanel } from "../components/ResourceTreePanel";
import { DiffDrawer } from "../components/DiffDrawer";
import { HistoryDrawer } from "../components/HistoryDrawer";
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

  const { data: app, isLoading, error } = useQuery({
    queryKey: ["app", name],
    queryFn: () => api.getApp(name),
    refetchInterval: 10_000,
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
          <div className="min-w-0 flex-1">
            <div className="text-[10px] font-medium uppercase tracking-wider text-[var(--color-text-muted)]">
              {app.project}
            </div>
            <h1 className="text-lg font-semibold text-[var(--color-text)] tracking-tight truncate" title={app.name}>
              {app.name}
            </h1>
          </div>
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
        <div className="max-w-[1800px] mx-auto px-6 pb-3 grid gap-2 sm:grid-cols-2">
          <StatTile label="Health">
            <HealthBadge status={app.status.health} />
          </StatTile>
          <StatTile label="Sync · Revision · Last sync">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
              {/* Sync status */}
              <div className="flex flex-wrap items-center gap-1.5">
                {app.status.sync === "OutOfSync" ? (
                  <button type="button" title="Open diff" onClick={() => setDiffOpen(true)}>
                    <SyncBadge status={app.status.sync} />
                  </button>
                ) : (
                  <SyncBadge status={app.status.sync} />
                )}
                {app.status.syncOperation ? (
                  <SyncOperationBadge status={app.status.syncOperation.status} message={app.status.syncOperation.message} />
                ) : null}
                {!app.status.syncOperation && app.status.lastCompletedSync?.status === "Failed" ? (
                  <LastSyncFailedBadge message={app.status.lastCompletedSync.message} />
                ) : null}
              </div>
              {/* Revision */}
              <div className="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <GitCommitHorizontal className="size-3.5 shrink-0" />
                <span className="font-mono text-[var(--color-text)]" title={app.status.observedRevision}>
                  {app.status.observedRevision ? app.status.observedRevision.slice(0, 7) : "—"}
                </span>
                <span className="text-[10px]">@ {app.source.targetRevision}</span>
              </div>
              {/* Last sync */}
              <div className="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                <Clock className="size-3.5 shrink-0" />
                <span className="text-[var(--color-text)]">{fmtShort(app.status.lastSyncedAt)}</span>
              </div>
            </div>
            {/* Commit info */}
            {app.status.observedCommit && (
              <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px]">
                <span className="text-[var(--color-text)] truncate max-w-[280px]" title={app.status.observedCommit.message}>
                  {app.status.observedCommit.message}
                </span>
                <span className="flex items-center gap-1 text-[var(--color-text-muted)] shrink-0">
                  <User className="size-3" />
                  {app.status.observedCommit.author || "unknown"}
                </span>
              </div>
            )}
            {app.status.message?.trim() ? (
              <p className="mt-1 text-[10px] leading-snug text-amber-300/90 break-words line-clamp-2" title={app.status.message}>
                {app.status.message}
              </p>
            ) : null}
          </StatTile>
        </div>
        <div className="max-w-[1800px] mx-auto px-6 pb-3 text-[11px] text-[var(--color-text-muted)] flex flex-wrap gap-x-4 gap-y-1">
          <span className="break-all"><span className="text-[var(--color-text-muted)]/80">Source:</span> {app.source.repoUrl} · {app.source.path || "."}</span>
          <span className="font-mono"><span className="text-[var(--color-text-muted)]/80">Destination:</span> {app.destination.cluster}/{app.destination.namespace}</span>
        </div>
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
