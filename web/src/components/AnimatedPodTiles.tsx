import type { ResourceNode } from "../api/types";
import { useAnimatedList } from "../hooks/useAnimatedList";
import { podTileChar, podTileClass, podTileTitle } from "../k8s/podTile";
import {
  useGroupedPodAnimation,
  type AnimatedListItem,
  type AnimStatus,
} from "../state/groupedPodAnimations";

const sizeClass = {
  xs: "size-5 text-[9px] rounded-[3px]",
  sm: "size-6 text-[9px] rounded-[3px]",
  md: "size-7 text-[10px] rounded-[4px]",
} as const;

function animClass(status: AnimStatus): string {
  if (status === "entering") return "pod-tile-enter";
  if (status === "exiting") return "pod-tile-exit pod-tile-terminating";
  return "";
}

function tileChar(p: ResourceNode, status: AnimStatus): string {
  if (status === "exiting") return "×";
  if (status === "entering") return "…";
  const ph = (p.podPhase ?? "").trim();
  if (ph === "Terminating") return "×";
  return podTileChar(p);
}

function tileClass(p: ResourceNode, status: AnimStatus): string {
  if (status === "exiting" || p.podPhase === "Terminating") {
    return "bg-orange-500/90 text-white border-orange-300/50";
  }
  return podTileClass(p);
}

function tileTitle(p: ResourceNode, status: AnimStatus): string {
  if (status === "exiting") return `${p.name} · terminating`;
  return podTileTitle(p);
}

function renderTiles(
  animated: AnimatedListItem<ResourceNode>[],
  pods: ResourceNode[],
  opts: {
    size: keyof typeof sizeClass;
    maxShown?: number;
    onPodClick?: (pod: ResourceNode) => void;
    className?: string;
  },
) {
  const { size, maxShown, onPodClick, className = "" } = opts;
  const headKeys = new Set(
    maxShown != null ? pods.slice(0, maxShown).map((p) => p.uid) : pods.map((p) => p.uid),
  );
  const visible = animated.filter(
    ({ item, status }) => status === "exiting" || headKeys.has(item.uid),
  );
  const hiddenCount = maxShown != null && pods.length > maxShown ? pods.length - maxShown : 0;

  if (visible.length === 0) return null;

  return (
    <div className={`flex flex-wrap gap-1 ${className}`}>
      {visible.map(({ item: p, key, status }) => {
        const tile = (
          <span
            className={`inline-flex shrink-0 items-center justify-center border font-bold leading-none ${sizeClass[size]} ${tileClass(p, status)} ${animClass(status)}`}
            title={tileTitle(p, status)}
          >
            {tileChar(p, status)}
          </span>
        );
        if (onPodClick && status === "active") {
          return (
            <button
              key={key}
              type="button"
              className="p-0 border-0 bg-transparent cursor-pointer hover:brightness-110"
              onClick={() => onPodClick(p)}
            >
              {tile}
            </button>
          );
        }
        return (
          <span key={key} className="inline-flex">
            {tile}
          </span>
        );
      })}
      {hiddenCount > 0 && (
        <span className="text-[9px] text-[var(--color-text-muted)] self-center pl-0.5">
          +{hiddenCount}
        </span>
      )}
    </div>
  );
}

function AnimatedPodTilesLocal({
  pods,
  size,
  maxShown,
  onPodClick,
  className,
}: {
  pods: ResourceNode[];
  size: keyof typeof sizeClass;
  maxShown?: number;
  onPodClick?: (pod: ResourceNode) => void;
  className?: string;
}) {
  const animated = useAnimatedList(pods, (p) => p.uid);
  return renderTiles(animated, pods, { size, maxShown, onPodClick, className });
}

function AnimatedPodTilesPersisted({
  parentKey,
  pods,
  size,
  maxShown,
  onPodClick,
  className,
}: {
  parentKey: string;
  pods: ResourceNode[];
  size: keyof typeof sizeClass;
  maxShown?: number;
  onPodClick?: (pod: ResourceNode) => void;
  className?: string;
}) {
  const animated = useGroupedPodAnimation(parentKey, pods);
  return renderTiles(animated, pods, { size, maxShown, onPodClick, className });
}

export function AnimatedPodTiles({
  pods,
  parentKey,
  size = "sm",
  maxShown,
  onPodClick,
  className = "",
}: {
  pods: ResourceNode[];
  parentKey?: string;
  size?: keyof typeof sizeClass;
  maxShown?: number;
  onPodClick?: (pod: ResourceNode) => void;
  className?: string;
}) {
  if (parentKey) {
    return (
      <AnimatedPodTilesPersisted
        parentKey={parentKey}
        pods={pods}
        size={size}
        maxShown={maxShown}
        onPodClick={onPodClick}
        className={className}
      />
    );
  }
  return (
    <AnimatedPodTilesLocal
      pods={pods}
      size={size}
      maxShown={maxShown}
      onPodClick={onPodClick}
      className={className}
    />
  );
}

function GroupedPodsBlockBody({
  animated,
  pods,
  size,
  maxShown,
  onPodClick,
  className,
  variant,
}: {
  animated: AnimatedListItem<ResourceNode>[];
  pods: ResourceNode[];
  size: keyof typeof sizeClass;
  maxShown?: number;
  onPodClick?: (pod: ResourceNode) => void;
  className?: string;
  variant: "topology" | "card";
}) {
  const tiles = renderTiles(animated, pods, { size, maxShown, onPodClick, className });
  if (!tiles) return null;

  const active = animated.filter((d) => d.status !== "exiting").length;
  const terminating = animated.some((d) => d.status === "exiting");

  const label = (
    <div
      className={
        variant === "card"
          ? "text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide flex items-center gap-2"
          : "text-[9px] uppercase tracking-wide text-[var(--color-text-muted)] mb-1 flex items-center gap-1.5"
      }
    >
      <span>Pods ({active})</span>
      {terminating ? (
        <span className="normal-case text-orange-400/90 animate-pulse font-medium">terminating…</span>
      ) : null}
    </div>
  );

  if (variant === "card") {
    return (
      <div className="space-y-3">
        {label}
        {tiles}
      </div>
    );
  }

  return (
    <div className="mt-1.5 pt-1.5 border-t border-[var(--color-border)]/70">
      {label}
      {tiles}
    </div>
  );
}

/** Local animation only — safe inside React Flow custom nodes. */
function GroupedPodsBlockTopology(
  props: Omit<Parameters<typeof GroupedPodsBlock>[0], "variant" | "parentKey"> & { pods: ResourceNode[] },
) {
  const animated = useAnimatedList(props.pods, (p) => p.uid);
  return (
    <GroupedPodsBlockBody
      animated={animated}
      pods={props.pods}
      size={props.size ?? "xs"}
      maxShown={props.maxShown}
      onPodClick={props.onPodClick}
      className={props.className}
      variant="topology"
    />
  );
}

/** Persisted animation (survives remount) — detail sidebar only. */
function GroupedPodsBlockSidebar(
  props: Omit<Parameters<typeof GroupedPodsBlock>[0], "variant"> & { parentKey: string; pods: ResourceNode[] },
) {
  const animated = useGroupedPodAnimation(props.parentKey, props.pods);
  return (
    <GroupedPodsBlockBody
      animated={animated}
      pods={props.pods}
      size={props.size ?? "md"}
      maxShown={props.maxShown}
      onPodClick={props.onPodClick}
      className={props.className}
      variant="card"
    />
  );
}

/** Pod tiles under ReplicaSet / Deployment (topology map or detail sidebar). */
export function GroupedPodsBlock({
  parentKey,
  pods,
  size = "xs",
  maxShown,
  onPodClick,
  className = "",
  variant = "topology",
}: {
  parentKey: string;
  pods: ResourceNode[];
  size?: keyof typeof sizeClass;
  maxShown?: number;
  onPodClick?: (pod: ResourceNode) => void;
  className?: string;
  variant?: "topology" | "card";
}) {
  if (variant === "topology") {
    return (
      <GroupedPodsBlockTopology
        pods={pods}
        size={size}
        maxShown={maxShown}
        onPodClick={onPodClick}
        className={className}
      />
    );
  }
  return (
    <GroupedPodsBlockSidebar
      parentKey={parentKey}
      pods={pods}
      size={size}
      maxShown={maxShown}
      onPodClick={onPodClick}
      className={className}
    />
  );
}

