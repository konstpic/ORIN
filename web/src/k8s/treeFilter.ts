import type { ResourceNode } from "../api/types";

function matchesQuery(n: ResourceNode, q: string): boolean {
  if (!q) return true;
  const s = q.toLowerCase();
  return n.name.toLowerCase().includes(s) || n.kind.toLowerCase().includes(s) || (n.namespace ?? "").toLowerCase().includes(s);
}

/** Returns roots whose subtree contains a node matching the name/kind/namespace query (Argo-style name filter). */
export function filterResourceForest(nodes: ResourceNode[], query: string): ResourceNode[] {
  const q = query.trim().toLowerCase();
  if (!q) return nodes;

  const walk = (list: ResourceNode[]): ResourceNode[] => {
    const out: ResourceNode[] = [];
    for (const n of list) {
      const kids = walk(n.children ?? []);
      const groupedPods = n.groupedPods ?? [];
      const groupedMatch = groupedPods.some((p) => matchesQuery(p, q));
      if (matchesQuery(n, q) || kids.length > 0 || groupedMatch) {
        out.push({
          ...n,
          children: kids.length > 0 ? kids : undefined,
          groupedPods: groupedMatch ? groupedPods : n.groupedPods,
        });
      }
    }
    return out;
  };

  return walk(nodes);
}
