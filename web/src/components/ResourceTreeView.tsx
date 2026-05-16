import type { MouseEvent } from "react";
import { HealthBadge, SyncBadge } from "./Badges";
import type { ResourceNode } from "../api/types";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";

export function ResourceTreeView({
  roots,
  onNodeSelect,
  onNodeContextMenu,
}: {
  roots: ResourceNode[];
  onNodeSelect?: (n: ResourceNode) => void;
  onNodeContextMenu?: (e: MouseEvent, n: ResourceNode) => void;
}) {
  return (
    <div className="space-y-2 rounded-lg p-4">
      {roots.map((n) => (
        <NodeRow key={n.uid} node={n} depth={0} onNodeSelect={onNodeSelect} onNodeContextMenu={onNodeContextMenu} />
      ))}
    </div>
  );
}

function NodeRow({
  node,
  depth,
  onNodeSelect,
  onNodeContextMenu,
}: {
  node: ResourceNode;
  depth: number;
  onNodeSelect?: (n: ResourceNode) => void;
  onNodeContextMenu?: (e: MouseEvent, n: ResourceNode) => void;
}) {
  const clickable = !!onNodeSelect;
  const Icon = iconForKind(node.kind);
  return (
    <div>
      <div
        role={clickable ? "button" : undefined}
        tabIndex={clickable ? 0 : undefined}
        onClick={() => clickable && onNodeSelect?.(node)}
        onContextMenu={(e) => onNodeContextMenu?.(e, node)}
        onKeyDown={(e) => {
          if (clickable && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            onNodeSelect?.(node);
          }
        }}
        className={`flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] px-3 py-2.5 transition-all duration-150 ${
          clickable
            ? "cursor-pointer hover:border-[var(--color-border-strong)] hover:shadow-md hover:scale-[1.005] active:scale-[0.995]"
            : ""
        }`}
        style={{ marginLeft: depth * 16 }}
        title={node.syncMessage || undefined}
      >
        <span
          className={`inline-flex shrink-0 items-center justify-center rounded-lg size-8 [&_svg]:size-3.5 ${kindIconTileClass(node.kind)}`}
          aria-hidden
        >
          <Icon strokeWidth={1.65} />
        </span>
        <span className="text-xs font-mono text-[var(--color-text-muted)] w-28 shrink-0 truncate">{node.kind}</span>
        <span className="text-sm font-medium text-[var(--color-text)] truncate flex-1">{node.name}</span>
        <span className="text-xs text-[var(--color-text-muted)] shrink-0">{node.namespace}</span>
        <span className="shrink-0 flex flex-wrap gap-1 items-center">
          <HealthBadge status={node.health} />
          <SyncBadge status={node.sync} />
        </span>
      </div>
      {node.children?.map((c) => (
        <NodeRow key={c.uid} node={c} depth={depth + 1} onNodeSelect={onNodeSelect} onNodeContextMenu={onNodeContextMenu} />
      ))}
    </div>
  );
}
