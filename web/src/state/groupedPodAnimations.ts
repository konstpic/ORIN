import { useEffect, useSyncExternalStore } from "react";
import type { ResourceNode } from "../api/types";
import type { AnimatedListItem, AnimStatus } from "../hooks/useAnimatedList";

export const POD_TILE_EXIT_MS = 520;

/** Stable empty list — do not use `?? []` inline (new array every render breaks effect deps). */
export const EMPTY_GROUPED_PODS: ResourceNode[] = [];

type RegistryEntry = {
  display: AnimatedListItem<ResourceNode>[];
  exitTimers: Map<string, ReturnType<typeof setTimeout>>;
  listeners: Set<() => void>;
};

const registry = new Map<string, RegistryEntry>();

function getEntry(key: string): RegistryEntry {
  let entry = registry.get(key);
  if (!entry) {
    entry = { display: [], exitTimers: new Map(), listeners: new Set() };
    registry.set(key, entry);
  }
  return entry;
}

function notify(entry: RegistryEntry) {
  for (const listener of entry.listeners) {
    listener();
  }
}

function podKey(p: ResourceNode): string {
  return p.uid;
}

function displaySnapshot(display: AnimatedListItem<ResourceNode>[]): string {
  return display.map((d) => `${d.key}:${d.status}`).join("|");
}

function syncPods(parentKey: string, pods: ResourceNode[]) {
  const entry = getEntry(parentKey);
  const itemByKey = new Map(pods.map((i) => [podKey(i), i]));
  const prev = entry.display;
  const prevByKey = new Map(prev.map((r) => [r.key, r]));
  const result: AnimatedListItem<ResourceNode>[] = [];

  for (const item of pods) {
    const key = podKey(item);
    const was = prevByKey.get(key);
    if (was && was.status !== "exiting") {
      result.push({ item, key, status: "active" });
    } else {
      result.push({ item, key, status: "entering" });
    }
  }

  for (const row of prev) {
    if (itemByKey.has(row.key)) continue;
    if (row.status === "exiting") {
      result.push(row);
      continue;
    }
    result.push({ ...row, status: "exiting" });
    if (!entry.exitTimers.has(row.key)) {
      const timer = setTimeout(() => {
        entry.exitTimers.delete(row.key);
        entry.display = entry.display.filter((d) => d.key !== row.key);
        notify(entry);
      }, POD_TILE_EXIT_MS);
      entry.exitTimers.set(row.key, timer);
    }
  }

  const prevSnap = displaySnapshot(entry.display);
  entry.display = result;
  const nextSnap = displaySnapshot(entry.display);
  if (prevSnap !== nextSnap) {
    notify(entry);
  }

  if (result.some((r) => r.status === "entering")) {
    requestAnimationFrame(() => {
      const before = displaySnapshot(entry.display);
      entry.display = entry.display.map((d) =>
        d.status === "entering" ? { ...d, status: "active" as const } : d,
      );
      if (displaySnapshot(entry.display) !== before) {
        notify(entry);
      }
    });
  }
}

function podsSignature(pods: ResourceNode[]): string {
  return pods.map((p) => `${p.uid}:${p.podPhase ?? ""}:${p.health}`).join("|");
}

export function subscribeGroupedPods(parentKey: string, listener: () => void): () => void {
  const entry = getEntry(parentKey);
  entry.listeners.add(listener);
  return () => entry.listeners.delete(listener);
}

export function getGroupedPodDisplay(parentKey: string): AnimatedListItem<ResourceNode>[] {
  return getEntry(parentKey).display;
}

/** Survives React Flow node remounts — keyed by ReplicaSet/Deployment uid. */
export function useGroupedPodAnimation(parentKey: string, pods: ResourceNode[]) {
  const signature = podsSignature(pods);

  useEffect(() => {
    syncPods(parentKey, pods);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- pods read when signature changes
  }, [parentKey, signature]);

  return useSyncExternalStore(
    (cb) => subscribeGroupedPods(parentKey, cb),
    () => getGroupedPodDisplay(parentKey),
    () => getGroupedPodDisplay(parentKey),
  );
}

export function activePodCount(display: AnimatedListItem<ResourceNode>[]): number {
  return display.filter((d) => d.status !== "exiting").length;
}

export function hasTerminatingPods(display: AnimatedListItem<ResourceNode>[]): boolean {
  return display.some((d) => d.status === "exiting");
}

export type { AnimStatus, AnimatedListItem };
