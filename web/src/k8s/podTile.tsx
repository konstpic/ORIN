import type { ResourceNode } from "../api/types";

/** One-letter / symbol hint for compact pod tiles on topology. */
export function podTileChar(pod: ResourceNode): string {
  const ph = (pod.podPhase ?? "").trim();
  if (ph === "Running") return "R";
  if (ph === "Pending") return "P";
  if (ph === "Succeeded") return "S";
  if (ph === "Failed") return "F";
  if (ph) return ph.slice(0, 1).toUpperCase();
  switch (pod.health) {
    case "Healthy":
      return "✓";
    case "Progressing":
      return "…";
    case "Degraded":
      return "!";
    case "Missing":
      return "−";
    default:
      return "?";
  }
}

export function podTileClass(pod: ResourceNode): string {
  if (pod.health === "Degraded" || pod.podPhase === "Failed") {
    return "bg-red-500/90 text-white border-red-300/40";
  }
  if (pod.podPhase === "Pending" || pod.health === "Progressing") {
    return "bg-amber-500/90 text-[#0a0e14] border-amber-200/50";
  }
  if (pod.podPhase === "Succeeded") {
    return "bg-sky-600/90 text-white border-sky-300/40";
  }
  if (pod.podPhase === "Running" || pod.health === "Healthy") {
    return "bg-emerald-600/90 text-white border-emerald-300/40";
  }
  return "bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] border-[var(--color-border)]";
}

export function podTileTitle(pod: ResourceNode): string {
  const bits = [pod.name, pod.podPhase, pod.health].filter(Boolean);
  return bits.join(" · ");
}
