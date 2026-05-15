import type { LucideIcon } from "lucide-react";
import {
  Boxes,
  Briefcase,
  CalendarClock,
  Copy,
  Database,
  FileJson,
  Globe,
  HardDrive,
  KeyRound,
  Layers,
  Network,
  Package,
  Rocket,
  Server,
  Shield,
  UserCircle,
} from "lucide-react";

export type KindIconTone = "accent" | "info" | "success" | "warning" | "neutral";

const kindIcons: Record<string, LucideIcon> = {
  Application: Package,
  Deployment: Rocket,
  StatefulSet: Database,
  DaemonSet: Server,
  ReplicaSet: Copy,
  Pod: Boxes,
  Service: Network,
  Ingress: Globe,
  ConfigMap: FileJson,
  Secret: KeyRound,
  ServiceAccount: UserCircle,
  Job: Briefcase,
  CronJob: CalendarClock,
  PersistentVolumeClaim: HardDrive,
  PodGroup: Layers,
};

const kindTones: Record<string, KindIconTone> = {
  Application: "accent",
  Deployment: "accent",
  StatefulSet: "info",
  DaemonSet: "info",
  ReplicaSet: "neutral",
  Pod: "success",
  Service: "info",
  Ingress: "accent",
  ConfigMap: "neutral",
  Secret: "warning",
  ServiceAccount: "neutral",
  Job: "info",
  CronJob: "info",
  PersistentVolumeClaim: "neutral",
  PodGroup: "accent",
};

export function iconForKind(kind: string): LucideIcon {
  return kindIcons[kind] ?? Package;
}

export function toneForKind(kind: string): KindIconTone {
  return kindTones[kind] ?? "neutral";
}

const toneClass: Record<KindIconTone, string> = {
  accent: "bg-[var(--color-accent-muted)] text-[var(--color-accent)] ring-1 ring-[var(--color-border-strong)]",
  info: "bg-sky-500/10 text-sky-300 ring-1 ring-sky-500/20",
  success: "bg-emerald-500/10 text-emerald-300 ring-1 ring-emerald-500/20",
  warning: "bg-amber-500/10 text-amber-300 ring-1 ring-amber-500/25",
  neutral: "bg-white/5 text-[var(--color-text-muted)] ring-1 ring-[var(--color-border)]",
};

export function kindIconTileClass(kind: string): string {
  return toneClass[toneForKind(kind)];
}
