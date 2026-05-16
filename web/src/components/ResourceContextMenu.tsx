import { useEffect, useRef, type ReactNode } from "react";
import { Trash2, RefreshCw, RotateCcw } from "lucide-react";

export type ResourceAction = "delete" | "restart" | "sync";

export interface ContextMenuState {
  x: number;
  y: number;
  /** The node descriptor used to identify which resource to act on. */
  node: {
    kind: string;
    group: string;
    version: string;
    namespace?: string;
    name: string;
    uid: string;
  };
}

const ACTION_LABEL: Record<ResourceAction, string> = {
  delete: "Delete",
  restart: "Restart",
  sync: "Sync",
};

const ACTION_ICON: Record<ResourceAction, ReactNode> = {
  delete: <Trash2 className="size-3.5" strokeWidth={2} />,
  restart: <RotateCcw className="size-3.5" strokeWidth={2} />,
  sync: <RefreshCw className="size-3.5" strokeWidth={2} />,
};

const ACTION_DANGER: Record<ResourceAction, boolean> = {
  delete: true,
  restart: true,
  sync: false,
};

/** Returns which actions are available for a given Kubernetes resource kind. */
export function actionsForKind(kind: string): ResourceAction[] {
  if (kind === "Pod") {
    return ["restart", "delete", "sync"];
  }
  if (kind === "Deployment" || kind === "ReplicaSet") {
    return ["restart", "sync", "delete"];
  }
  // Synthetic / UI-only nodes have no live cluster operations
  if (kind === "Application" || kind === "PodGroup" || kind === "KindGroup") {
    return [];
  }
  return ["sync", "delete"];
}

export function ResourceContextMenu({
  state,
  onAction,
  onClose,
}: {
  state: ContextMenuState;
  onAction: (action: ResourceAction, node: ContextMenuState["node"]) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const actions = actionsForKind(state.node.kind);

  // Dismiss on outside click or Escape
  useEffect(() => {
    const down = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("mousedown", down, true);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("mousedown", down, true);
      document.removeEventListener("keydown", key, true);
    };
  }, [onClose]);

  if (actions.length === 0) return null;

  // Keep menu inside the viewport
  const menuW = 160;
  const menuH = actions.length * 36 + 8;
  const left = Math.min(state.x, window.innerWidth - menuW - 8);
  const top = Math.min(state.y, window.innerHeight - menuH - 8);

  return (
    <div
      ref={ref}
      role="menu"
      className="fixed z-[9999] min-w-[160px] rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-elevated)] shadow-2xl py-1 text-sm"
      style={{ left, top }}
      onContextMenu={(e) => e.preventDefault()}
    >
      <div className="px-3 py-1.5 border-b border-[var(--color-border)] mb-1">
        <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">
          {state.node.kind}
        </span>
        <div className="text-xs font-medium text-[var(--color-text)] truncate max-w-[140px]">
          {state.node.name}
        </div>
      </div>
      {actions.map((action) => (
        <button
          key={action}
          role="menuitem"
          type="button"
          className={`w-full flex items-center gap-2.5 px-3 py-2 text-left transition-all duration-150 hover:bg-[var(--color-surface-muted)] active:scale-[0.98] ${
            ACTION_DANGER[action]
              ? "text-red-400 hover:text-red-300"
              : "text-[var(--color-text)] hover:text-[var(--color-accent)]"
          }`}
          onClick={() => {
            onAction(action, state.node);
            onClose();
          }}
        >
          {ACTION_ICON[action]}
          {ACTION_LABEL[action]}
        </button>
      ))}
    </div>
  );
}
