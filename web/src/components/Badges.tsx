import clsx from "clsx";
import {
  AlertOctagon,
  ArrowLeftRight,
  CheckCheck,
  CircleDashed,
  CirclePause,
  CircleX,
  CloudOff,
  HeartPulse,
  HelpCircle,
  Loader2,
  XCircle,
} from "lucide-react";
import type { HealthStatus, SyncStatus } from "../api/types";

const syncColor: Record<SyncStatus, string> = {
  Synced: "bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-500/35",
  OutOfSync: "bg-amber-500/20 text-amber-300 ring-1 ring-amber-500/35",
  Unknown: "bg-zinc-500/20 text-zinc-300 ring-1 ring-zinc-500/35",
};

const healthColor: Record<HealthStatus, string> = {
  Healthy: "bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-500/35",
  Progressing: "bg-sky-500/20 text-sky-300 ring-1 ring-sky-500/35",
  Degraded: "bg-red-500/20 text-red-300 ring-1 ring-red-500/35",
  Suspended: "bg-violet-500/20 text-violet-300 ring-1 ring-violet-500/35",
  Missing: "bg-orange-500/20 text-orange-300 ring-1 ring-orange-500/35",
  Unknown: "bg-zinc-500/20 text-zinc-300 ring-1 ring-zinc-500/35",
};

const syncLabel: Record<SyncStatus, string> = {
  Synced: "Synced",
  OutOfSync: "Out of sync",
  Unknown: "Sync unknown",
};

const healthLabel: Record<HealthStatus, string> = {
  Healthy: "Healthy",
  Progressing: "Progressing",
  Degraded: "Degraded",
  Suspended: "Suspended",
  Missing: "Missing",
  Unknown: "Health unknown",
};

type BadgeSize = "sm" | "md";

const shellSize: Record<BadgeSize, string> = {
  sm: "size-6 rounded-md",
  md: "size-7 rounded-lg",
};

const iconSize: Record<BadgeSize, string> = {
  sm: "size-3.5",
  md: "size-4",
};

function SyncIcon({ status, size }: { status: SyncStatus; size: BadgeSize }) {
  const cls = clsx(iconSize[size], "shrink-0");
  const stroke = 2.25;
  if (status === "Synced") return <CheckCheck className={cls} strokeWidth={stroke} aria-hidden />;
  if (status === "OutOfSync") return <ArrowLeftRight className={cls} strokeWidth={stroke} aria-hidden />;
  return <HelpCircle className={cls} strokeWidth={stroke} aria-hidden />;
}

function HealthIcon({ status, size }: { status: HealthStatus; size: BadgeSize }) {
  const cls = clsx(iconSize[size], "shrink-0");
  const stroke = 2.25;
  switch (status) {
    case "Healthy":
      return <HeartPulse className={cls} strokeWidth={stroke} aria-hidden />;
    case "Progressing":
      return <Loader2 className={clsx(cls, "animate-spin")} strokeWidth={stroke} aria-hidden />;
    case "Degraded":
      return <AlertOctagon className={cls} strokeWidth={stroke} aria-hidden />;
    case "Suspended":
      return <CirclePause className={cls} strokeWidth={stroke} aria-hidden />;
    case "Missing":
      return <CircleX className={cls} strokeWidth={stroke} aria-hidden />;
    default:
      return <CircleDashed className={cls} strokeWidth={stroke} aria-hidden />;
  }
}

type StatusBadgeProps = {
  size?: BadgeSize;
  /** Show text label next to the icon (default: icon only with tooltip). */
  showLabel?: boolean;
  className?: string;
};

export function SyncBadge({ status, size = "md", showLabel = false, className }: StatusBadgeProps & { status: SyncStatus }) {
  const label = syncLabel[status] ?? syncLabel.Unknown;
  return (
    <span
      className={clsx(
        "inline-flex items-center justify-center gap-1.5 font-medium transition-colors",
        showLabel ? "rounded-lg px-2 py-0.5 text-xs" : shellSize[size],
        syncColor[status] ?? syncColor.Unknown,
        className,
      )}
      title={label}
      aria-label={`Sync: ${label}`}
      role="status"
    >
      <SyncIcon status={status} size={size} />
      {showLabel ? <span>{status === "OutOfSync" ? "Out of sync" : status}</span> : null}
    </span>
  );
}

export function HealthBadge({
  status,
  size = "md",
  showLabel = false,
  className,
}: StatusBadgeProps & { status: HealthStatus }) {
  const label = healthLabel[status] ?? healthLabel.Unknown;
  return (
    <span
      className={clsx(
        "inline-flex items-center justify-center gap-1.5 font-medium transition-colors",
        showLabel ? "rounded-lg px-2 py-0.5 text-xs" : shellSize[size],
        healthColor[status] ?? healthColor.Unknown,
        className,
      )}
      title={label}
      aria-label={`Health: ${label}`}
      role="status"
    >
      <HealthIcon status={status} size={size} />
      {showLabel ? <span>{status}</span> : null}
    </span>
  );
}

/** Inline indicator for in-flight sync (Pending / Running). */
export function SyncOperationBadge({ status, message }: { status: string; message?: string }) {
  const label = status === "Pending" ? "Sync queued" : "Syncing…";
  return (
    <span
      className="inline-flex size-7 items-center justify-center rounded-lg bg-cyan-500/15 text-cyan-200 ring-1 ring-cyan-500/35"
      title={message ? `${label}: ${message}` : label}
      aria-label={label}
      role="status"
    >
      <Loader2 className="size-4 shrink-0 animate-spin" strokeWidth={2.25} aria-hidden />
    </span>
  );
}

export function LastSyncFailedBadge({ message }: { message?: string }) {
  const label = "Last sync failed";
  return (
    <span
      className="inline-flex size-7 items-center justify-center rounded-lg bg-red-500/15 text-red-300 ring-1 ring-red-500/40"
      title={message || "One or more resources failed to apply"}
      aria-label={label}
      role="status"
    >
      <XCircle className="size-4 shrink-0" strokeWidth={2.25} aria-hidden />
    </span>
  );
}

/** Optional compact sync-off indicator for empty / unreachable states. */
export function SyncUnknownBadge({ title = "Sync unknown" }: { title?: string }) {
  return (
    <span
      className="inline-flex size-7 items-center justify-center rounded-lg bg-zinc-500/20 text-zinc-300 ring-1 ring-zinc-500/35"
      title={title}
      aria-label={title}
      role="status"
    >
      <CloudOff className="size-4 shrink-0" strokeWidth={2.25} aria-hidden />
    </span>
  );
}
