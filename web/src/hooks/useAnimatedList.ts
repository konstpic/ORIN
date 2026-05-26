import { useEffect, useRef, useState } from "react";

export type AnimStatus = "active" | "entering" | "exiting";

export type AnimatedListItem<T> = {
  item: T;
  key: string;
  status: AnimStatus;
};

const EXIT_MS = 520;

/**
 * Keeps removed items visible briefly with status "exiting", animates new items as "entering".
 */
export function useAnimatedList<T>(items: T[], getKey: (item: T) => string): AnimatedListItem<T>[] {
  const [display, setDisplay] = useState<AnimatedListItem<T>[]>(() =>
    items.map((item) => ({ item, key: getKey(item), status: "active" as const })),
  );
  const prevKeysRef = useRef<Set<string>>(new Set(items.map(getKey)));

  useEffect(() => {
    const itemByKey = new Map(items.map((i) => [getKey(i), i]));
    const prevKeys = prevKeysRef.current;

    setDisplay((prev) => {
      const prevByKey = new Map(prev.map((r) => [r.key, r]));
      const result: AnimatedListItem<T>[] = [];

      for (const item of items) {
        const key = getKey(item);
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
        } else {
          result.push({ ...row, status: "exiting" });
        }
      }

      return result;
    });

    prevKeysRef.current = new Set(items.map(getKey));
  }, [items, getKey]);

  useEffect(() => {
    if (!display.some((d) => d.status === "entering")) return;
    const id = requestAnimationFrame(() => {
      setDisplay((prev) =>
        prev.map((d) => (d.status === "entering" ? { ...d, status: "active" as const } : d)),
      );
    });
    return () => cancelAnimationFrame(id);
  }, [display]);

  useEffect(() => {
    if (!display.some((d) => d.status === "exiting")) return;
    const t = setTimeout(() => {
      setDisplay((prev) => prev.filter((d) => d.status !== "exiting"));
    }, EXIT_MS);
    return () => clearTimeout(t);
  }, [display]);

  return display;
}
