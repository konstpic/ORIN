import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, type NavigateFunction } from "react-router-dom";
import { useState, useMemo } from "react";
import { LayoutGrid, List, Table2, type LucideIcon } from "lucide-react";
import { api } from "../api/client";
import { HealthBadge, SyncBadge } from "../components/Badges";
import { CreateAppDialog } from "../components/CreateAppDialog";
import { SyncPreviewDrawer, type SyncSubmitPayload } from "../components/SyncPreviewDrawer";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";
import type { Application } from "../api/types";
import { relativeTime } from "../utils/relativeTime";

type AppsView = "cards" | "table" | "list";

export function AppsPage() {
  const navigate = useNavigate();
  const { data: appsRaw, isLoading, error } = useQuery({
    queryKey: ["apps"],
    queryFn: api.listApps,
    refetchInterval: 5000,
  });

  const apps = useMemo(() => {
    if (!appsRaw) return appsRaw;
    return [...appsRaw].sort((a, b) => {
      const aOos = a.status.sync === "OutOfSync" ? 0 : 1;
      const bOos = b.status.sync === "OutOfSync" ? 0 : 1;
      if (aOos !== bOos) return aOos - bOos;
      return a.name.localeCompare(b.name);
    });
  }, [appsRaw]);
  const [showCreate, setShowCreate] = useState(false);
  const [view, setView] = useState<AppsView>("cards");
  const [syncPreviewApp, setSyncPreviewApp] = useState<string | null>(null);
  const qc = useQueryClient();
  const sync = useMutation({
    mutationFn: (args: { name: string } & SyncSubmitPayload) =>
      api.syncApp(args.name, {
        prune: args.prune,
        dryRun: args.dryRun,
        resources: args.resources,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["apps"] });
      setSyncPreviewApp(null);
    },
  });

  const IconApp = iconForKind("Application");

  return (
    <div className="p-6 max-w-[1600px] mx-auto flex-1 min-h-0 overflow-y-auto w-full">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-[var(--color-text)] tracking-tight">Applications</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-0.5">GitOps applications and sync status</p>
        </div>
        <div className="flex flex-wrap items-center gap-2 justify-end">
          <div className="inline-flex rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-0.5">
            <button
              type="button"
              title="List"
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium ${
                view === "list"
                  ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                  : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              }`}
              onClick={() => setView("list")}
            >
              <List className="size-4 shrink-0" />
              List
            </button>
            <button
              type="button"
              title="Cards"
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium ${
                view === "cards"
                  ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                  : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              }`}
              onClick={() => setView("cards")}
            >
              <LayoutGrid className="size-4 shrink-0" />
              Cards
            </button>
            <button
              type="button"
              title="Table"
              className={`inline-flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium ${
                view === "table"
                  ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                  : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              }`}
              onClick={() => setView("table")}
            >
              <Table2 className="size-4 shrink-0" />
              Table
            </button>
          </div>
          <button
            className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[#0a0e14] shadow hover:brightness-110 active:brightness-95"
            onClick={() => setShowCreate(true)}
          >
            New application
          </button>
        </div>
      </div>
      {isLoading && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
      {error && <div className="text-sm text-red-400">{(error as Error).message}</div>}
      {apps && apps.length === 0 && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-8 text-sm text-[var(--color-text-muted)]">
          No applications yet. Register a repository in{" "}
          <Link to="/settings/repositories" className="text-[var(--color-accent)] font-medium hover:underline">
            Settings → Repositories
          </Link>{" "}
          and then create one.
        </div>
      )}
      {apps && apps.length > 0 && view === "list" && (
        <AppsList apps={apps} IconApp={IconApp} navigate={navigate} onSyncClick={(n) => setSyncPreviewApp(n)} />
      )}
      {apps && apps.length > 0 && view === "table" && (
        <AppsTable apps={apps} navigate={navigate} onSyncClick={(n) => setSyncPreviewApp(n)} />
      )}
      {apps && apps.length > 0 && view === "cards" && (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {apps.map((app) => (
            <AppCard key={app.name} app={app} IconApp={IconApp} navigate={navigate} onSyncClick={() => setSyncPreviewApp(app.name)} />
          ))}
        </div>
      )}
      {showCreate && <CreateAppDialog onClose={() => setShowCreate(false)} />}
      <SyncPreviewDrawer
        appName={syncPreviewApp ?? ""}
        open={!!syncPreviewApp}
        onClose={() => setSyncPreviewApp(null)}
        onConfirm={(p) => syncPreviewApp && sync.mutate({ name: syncPreviewApp, ...p })}
        isSubmitting={sync.isPending}
      />
    </div>
  );
}

function AppCard({
  app,
  IconApp,
  navigate,
  onSyncClick,
}: {
  app: Application;
  IconApp: LucideIcon;
  navigate: NavigateFunction;
  onSyncClick: () => void;
}) {
  return (
    <div className="relative rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-md hover:border-[var(--color-border-strong)] transition-colors flex flex-col gap-3">
      <button
        type="button"
        className="absolute inset-0 z-0 rounded-xl cursor-pointer"
        aria-label={`Open application ${app.name}`}
        onClick={() => navigate(`/applications/${app.name}`)}
      />
      <div className="relative z-10 flex flex-col gap-3 pointer-events-none">
        <div className="flex items-start gap-3">
          <span
            className={`inline-flex shrink-0 items-center justify-center rounded-xl size-12 [&_svg]:size-6 ${kindIconTileClass("Application")}`}
          >
            <IconApp strokeWidth={1.65} />
          </span>
          <div className="min-w-0 flex-1">
            <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">Application</div>
            <div className="text-lg font-semibold text-[var(--color-accent)] truncate">{app.name}</div>
            <div className="text-xs text-[var(--color-text-muted)] mt-0.5">{app.project}</div>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 pointer-events-auto">
          {app.status.sync === "OutOfSync" ? (
            <Link to={`/applications/${app.name}?diff=1`} onClick={(e) => e.stopPropagation()}>
              <SyncBadge status={app.status.sync} />
            </Link>
          ) : (
            <SyncBadge status={app.status.sync} />
          )}
          <HealthBadge status={app.status.health} />
        </div>
        <div className="text-xs text-[var(--color-text-muted)] space-y-1 border-t border-[var(--color-border)] pt-3">
          <div className="truncate" title={app.source.repoUrl}>
            {app.source.repoUrl}
          </div>
          <div className="truncate">
            {app.source.path || "."} @ {app.source.targetRevision}
          </div>
          <div className="font-mono text-[var(--color-text)]/90">
            {app.destination.cluster}/{app.destination.namespace}
          </div>
          <div className="flex items-center justify-between gap-2 pt-0.5">
            {app.status.observedRevision ? (
              <span className="font-mono text-[var(--color-text-muted)] truncate" title={app.status.observedRevision}>
                {app.status.observedRevision.slice(0, 7)}
              </span>
            ) : <span />}
            {app.status.lastSyncedAt ? (
              <span className="shrink-0 tabular-nums" title={app.status.lastSyncedAt}>
                {relativeTime(app.status.lastSyncedAt)}
              </span>
            ) : (
              <span className="shrink-0 tabular-nums" title={app.createdAt}>
                {relativeTime(app.createdAt)}
              </span>
            )}
          </div>
        </div>
        <div className="flex gap-2 pt-1 pointer-events-auto">
          <Link
            to={`/applications/${app.name}`}
            className="flex-1 text-center rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)] relative z-20"
            onClick={(e) => e.stopPropagation()}
          >
            Open
          </Link>
          <button
            type="button"
            className="flex-1 rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110 relative z-20"
            onClick={(e) => {
              e.stopPropagation();
              onSyncClick();
            }}
          >
            Sync
          </button>
        </div>
      </div>
    </div>
  );
}

function AppsList({
  apps,
  IconApp,
  navigate,
  onSyncClick,
}: {
  apps: Application[];
  IconApp: LucideIcon;
  navigate: NavigateFunction;
  onSyncClick: (name: string) => void;
}) {
  return (
    <ul className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] divide-y divide-[var(--color-border)] shadow-sm overflow-hidden">
      {apps.map((app) => (
        <li key={app.name}>
          <div
            role="button"
            tabIndex={0}
            className="flex flex-col sm:flex-row sm:items-center gap-3 p-4 hover:bg-[var(--color-accent-muted)]/25 transition-colors cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
            onClick={() => navigate(`/applications/${app.name}`)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                navigate(`/applications/${app.name}`);
              }
            }}
          >
            <div className="flex items-start gap-3 min-w-0 flex-1">
              <span
                className={`inline-flex shrink-0 items-center justify-center rounded-xl size-11 [&_svg]:size-5 ${kindIconTileClass("Application")}`}
              >
                <IconApp strokeWidth={1.65} />
              </span>
              <div className="min-w-0">
                <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">Application</div>
                <div className="text-base font-semibold text-[var(--color-accent)] truncate">{app.name}</div>
                <div className="text-xs text-[var(--color-text-muted)] mt-0.5">{app.project}</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {app.status.sync === "OutOfSync" ? (
                    <Link to={`/applications/${app.name}?diff=1`} onClick={(e) => e.stopPropagation()}>
                      <SyncBadge status={app.status.sync} />
                    </Link>
                  ) : (
                    <SyncBadge status={app.status.sync} />
                  )}
                  <HealthBadge status={app.status.health} />
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0 sm:ml-auto" onClick={(e) => e.stopPropagation()}>
              <Link
                to={`/applications/${app.name}`}
                className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
              >
                Open
              </Link>
              <button
                type="button"
                className="rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110"
                onClick={() => onSyncClick(app.name)}
              >
                Sync
              </button>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}

function AppsTable({ apps, navigate, onSyncClick }: { apps: Application[]; navigate: NavigateFunction; onSyncClick: (name: string) => void }) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] shadow-sm overflow-hidden">
      <table className="w-full text-sm text-left">
        <thead>
          <tr className="border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] text-xs uppercase tracking-wide text-[var(--color-text-muted)]">
            <th className="px-4 py-3 font-medium w-12" aria-hidden />
            <th className="px-4 py-3 font-medium">Name</th>
            <th className="px-4 py-3 font-medium">Project</th>
            <th className="px-4 py-3 font-medium">Sync</th>
            <th className="px-4 py-3 font-medium">Health</th>
            <th className="px-4 py-3 font-medium">Repository</th>
            <th className="px-4 py-3 font-medium">Destination</th>
            <th className="px-4 py-3 font-medium w-28 text-right">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--color-border)]">
          {apps.map((app) => {
            const Icon = iconForKind("Application");
            return (
              <tr
                key={app.name}
                className="hover:bg-[var(--color-accent-muted)]/40 transition-colors cursor-pointer"
                onClick={(e) => {
                  const el = e.target as HTMLElement;
                  if (el.closest("a,button")) return;
                  navigate(`/applications/${app.name}`);
                }}
              >
                <td className="px-2 py-3">
                  <span className={`inline-flex items-center justify-center rounded-lg size-9 [&_svg]:size-4 ${kindIconTileClass("Application")}`}>
                    <Icon strokeWidth={1.65} />
                  </span>
                </td>
                <td className="px-4 py-3">
                  <span className="font-semibold text-[var(--color-accent)]">{app.name}</span>
                </td>
                <td className="px-4 py-3 text-[var(--color-text-muted)]">{app.project}</td>
                <td className="px-4 py-3">
                  {app.status.sync === "OutOfSync" ? (
                    <Link to={`/applications/${app.name}?diff=1`} onClick={(e) => e.stopPropagation()}>
                      <SyncBadge status={app.status.sync} />
                    </Link>
                  ) : (
                    <SyncBadge status={app.status.sync} />
                  )}
                </td>
                <td className="px-4 py-3">
                  <HealthBadge status={app.status.health} />
                </td>
                <td className="px-4 py-3 max-w-md">
                  <div className="truncate text-[var(--color-text-muted)]" title={app.source.repoUrl}>
                    {app.source.repoUrl}
                  </div>
                  <div className="text-xs text-[var(--color-text-muted)]/80 truncate">
                    {app.source.path || "."} @ {app.source.targetRevision}
                  </div>
                </td>
                <td className="px-4 py-3 text-[var(--color-text-muted)] whitespace-nowrap">
                  {app.destination.cluster}/{app.destination.namespace}
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    type="button"
                    className="rounded border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1 text-xs font-medium text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                    onClick={(e) => {
                      e.stopPropagation();
                      onSyncClick(app.name);
                    }}
                  >
                    Sync
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
