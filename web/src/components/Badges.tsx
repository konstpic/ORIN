import clsx from "clsx";
import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  CircleSlash,
  Loader2,
  PauseCircle,
  XCircle,
} from "lucide-react";
import type { HealthStatus, SyncStatus } from "../api/types";

const syncColor: Record<SyncStatus, string> = {
  Synced: "bg-sync-synced text-white",
  OutOfSync: "bg-sync-outOfSync text-white",
  Unknown: "bg-sync-unknown text-white",
};

const healthColor: Record<HealthStatus, string> = {
  Healthy: "bg-health-healthy text-white",
  Progressing: "bg-health-progressing text-white",
  Degraded: "bg-health-degraded text-white",
  Suspended: "bg-health-suspended text-white",
  Missing: "bg-health-missing text-white",
  Unknown: "bg-health-missing text-white",
};

function SyncIcon({ status }: { status: SyncStatus }) {
  const cls = "size-3.5 shrink-0 opacity-95";
  if (status === "Synced") return <CheckCircle2 className={cls} strokeWidth={2.25} aria-hidden />;
  if (status === "OutOfSync") return <AlertTriangle className={cls} strokeWidth={2.25} aria-hidden />;
  return <CircleHelp className={cls} strokeWidth={2.25} aria-hidden />;
}

function HealthIcon({ status }: { status: HealthStatus }) {
  const cls = "size-3.5 shrink-0 opacity-95";
  switch (status) {
    case "Healthy":
      return <CheckCircle2 className={cls} strokeWidth={2.25} aria-hidden />;
    case "Progressing":
      return <Loader2 className={`${cls} animate-spin`} strokeWidth={2.25} aria-hidden />;
    case "Degraded":
      return <AlertTriangle className={cls} strokeWidth={2.25} aria-hidden />;
    case "Suspended":
      return <PauseCircle className={cls} strokeWidth={2.25} aria-hidden />;
    case "Missing":
      return <XCircle className={cls} strokeWidth={2.25} aria-hidden />;
    default:
      return <CircleSlash className={cls} strokeWidth={2.25} aria-hidden />;
  }
}

export function SyncBadge({ status }: { status: SyncStatus }) {
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium",
        syncColor[status] ?? syncColor.Unknown,
      )}
    >
      <SyncIcon status={status} />
      {status}
    </span>
  );
}

export function HealthBadge({ status }: { status: HealthStatus }) {
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium",
        healthColor[status] ?? healthColor.Unknown,
      )}
    >
      <HealthIcon status={status} />
      {status}
    </span>
  );
}

/** Inline label for in-flight sync (Pending / Running). */
export function SyncOperationBadge({ status, message }: { status: string; message?: string }) {
  const label = status === "Pending" ? "Sync queued" : "Syncing…";
  return (
    <span
      className="inline-flex items-center gap-1 rounded border border-cyan-500/35 bg-cyan-500/15 px-2 py-0.5 text-xs font-medium text-cyan-100"
      title={message || undefined}
    >
      <Loader2 className="size-3.5 shrink-0 animate-spin opacity-90" strokeWidth={2.25} aria-hidden />
      {label}
    </span>
  );
}

export function LastSyncFailedBadge({ message }: { message?: string }) {
  return (
    <span
      className="inline-flex items-center gap-1 rounded border border-red-500/40 bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-200"
      title={message || "One or more resources failed to apply"}
    >
      <XCircle className="size-3.5 shrink-0 opacity-95" strokeWidth={2.25} aria-hidden />
      Last sync failed
    </span>
  );
}
