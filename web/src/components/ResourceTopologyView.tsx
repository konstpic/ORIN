import { useCallback, useEffect, useMemo, useState, type MouseEvent } from "react";
import dagre from "dagre";
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  Handle,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ExternalLink } from "lucide-react";
import { HealthBadge, SyncBadge } from "./Badges";
import type { HealthStatus, ResourceNode, SyncStatus } from "../api/types";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";
import { podTileChar, podTileClass, podTileTitle } from "../k8s/podTile";
import { relativeTime } from "../utils/relativeTime";

const maxGroupedPodsShown = 20;

/** Returns true if a ResourceNode is a real (child) Argo/orin Application CRD, not the synthetic root. */
function isChildAppNode(n: ResourceNode): boolean {
  return (
    n.kind === "Application" &&
    (n.group === "argoproj.io" || n.group === "orin.io") &&
    !n.uid.startsWith("synthetic:app:")
  );
}

function nodeMeasuredSize(n: Node): { w: number; h: number } {
  if (n.type === "application") return { w: 280, h: 120 };
  if (n.type === "childApp") return { w: 280, h: 120 };
  const d = n.data as FlowNodeData | undefined;
  const extraMsg = d?.syncMessage ? 28 : 0;
  const g = d?.raw.groupedPods?.length ?? 0;
  const extraPods =
    g > 0 && d?.raw && (d.raw.kind === "ReplicaSet" || d.raw.kind === "Deployment") ? 40 : 0;
  const groupMembers = d?.raw.groupedMembers?.length ?? 0;
  const extraGroup = d?.raw.isKindGroup && groupMembers > 0 ? 28 : 0;
  if (n.type === "kind") {
    return { w: 228, h: 88 + extraMsg + extraPods + extraGroup };
  }
  return { w: 220, h: 88 + extraMsg };
}

type FlowNodeData = {
  kind: string;
  name: string;
  health: HealthStatus;
  sync: SyncStatus;
  syncMessage?: string;
  raw: ResourceNode;
};

function ApplicationNode(props: NodeProps) {
  const { selected } = props;
  const data = props.data as FlowNodeData;
  const Icon = iconForKind("Application");
  return (
    <div
      className={`rounded-xl border bg-[var(--color-elevated)] px-4 py-3 shadow-lg min-w-[240px] max-w-[300px] transition-all duration-150 cursor-pointer hover:shadow-xl hover:scale-[1.02] active:scale-[0.98] ${
        selected
          ? "border-[var(--color-accent)] ring-2 ring-[var(--color-accent)]/35"
          : "border-[var(--color-border-strong)] hover:border-[var(--color-border)]"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
      <div className="flex items-start gap-3">
        <span
          className={`inline-flex shrink-0 items-center justify-center rounded-xl size-11 [&_svg]:size-5 ${kindIconTileClass("Application")}`}
        >
          <Icon strokeWidth={1.65} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">
            {data.kind}
          </div>
          <div className="text-base font-semibold text-[var(--color-text)] truncate" title={data.name}>
            {data.name}
          </div>
          <div className="mt-1.5 flex flex-wrap gap-1">
            <HealthBadge status={data.health} />
            <SyncBadge status={data.sync} />
          </div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

function KindNode(props: NodeProps) {
  const { selected } = props;
  const data = props.data as FlowNodeData;
  const Icon = iconForKind(data.kind);
  // For ReplicaSet nodes, show the pod-template-hash as the revision hint.
  const revision = data.raw.kind === "ReplicaSet"
    ? (data.raw.labels?.["pod-template-hash"] ?? "")
    : "";
  const age = relativeTime(data.raw.creationTimestamp);
  return (
    <div
      className={`rounded-lg border bg-[var(--color-surface)] px-3 py-2.5 shadow-md min-w-[180px] max-w-[240px] transition-all duration-150 cursor-pointer hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] ${
        selected
          ? "border-[var(--color-accent)] ring-2 ring-[var(--color-accent)]/30"
          : "border-[var(--color-border)] hover:border-[var(--color-border-strong)]"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
      <div className="flex items-start gap-2.5">
        <span
          className={`inline-flex shrink-0 items-center justify-center rounded-lg size-9 [&_svg]:size-4 ${kindIconTileClass(data.kind)}`}
        >
          <Icon strokeWidth={1.65} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold flex items-center gap-1">
            <span>{data.kind}</span>
            {data.raw.isKindGroup ? (
              <span className="inline-flex items-center justify-center rounded-full bg-[var(--color-accent-muted)] text-[var(--color-accent)] px-1.5 py-px text-[9px] font-bold leading-none">
                {data.raw.groupedMembers?.length ?? 0}
              </span>
            ) : null}
          </div>
          <div className="text-sm font-semibold text-[var(--color-text)] truncate" title={data.name}>
            {data.raw.isKindGroup ? `${data.raw.groupedMembers?.length ?? 0} ${data.kind}s` : data.name}
          </div>
          <div className="mt-1 flex flex-wrap gap-1 items-center">
            <HealthBadge status={data.health} />
            <SyncBadge status={data.sync} />
          </div>
          {data.syncMessage ? (
            <div className="mt-1 text-[10px] text-amber-300/90 leading-snug line-clamp-2" title={data.syncMessage}>
              {data.syncMessage}
            </div>
          ) : null}
          {data.raw.groupedPods &&
            data.raw.groupedPods.length > 0 &&
            (data.raw.kind === "ReplicaSet" || data.raw.kind === "Deployment") && (
              <div className="mt-1.5 pt-1.5 border-t border-[var(--color-border)]/70">
                <div className="text-[9px] uppercase tracking-wide text-[var(--color-text-muted)] mb-1">
                  Pods ({data.raw.groupedPods.length})
                </div>
                <div className="flex flex-wrap gap-1 max-w-[210px]">
                  {data.raw.groupedPods.slice(0, maxGroupedPodsShown).map((p) => (
                    <span
                      key={p.uid}
                      className={`inline-flex size-5 shrink-0 items-center justify-center rounded-[3px] border text-[9px] font-bold leading-none ${podTileClass(p)}`}
                      title={podTileTitle(p)}
                    >
                      {podTileChar(p)}
                    </span>
                  ))}
                  {data.raw.groupedPods.length > maxGroupedPodsShown && (
                    <span className="text-[9px] text-[var(--color-text-muted)] self-center pl-0.5">
                      +{data.raw.groupedPods.length - maxGroupedPodsShown}
                    </span>
                  )}
                </div>
              </div>
            )}
          {(revision || age) && (
            <div className="mt-1.5 pt-1 border-t border-[var(--color-border)]/50 flex items-center justify-between gap-2">
              {revision ? (
                <span className="font-mono text-[9px] text-[var(--color-text-muted)] truncate" title={`pod-template-hash: ${revision}`}>
                  #{revision}
                </span>
              ) : <span />}
              {age ? (
                <span className="text-[9px] text-[var(--color-text-muted)] shrink-0" title={data.raw.creationTimestamp}>
                  {age}
                </span>
              ) : null}
            </div>
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

/** Node for a child Application CRD (App of Apps pattern). */
function ChildAppNode(props: NodeProps) {
  const { selected } = props;
  const data = props.data as FlowNodeData;
  const Icon = iconForKind("Application");
  const age = relativeTime(data.raw.creationTimestamp);
  return (
    <div
      className={`rounded-xl border bg-[var(--color-elevated)] px-4 py-3 shadow-lg min-w-[240px] max-w-[300px] transition-all duration-150 cursor-pointer hover:shadow-xl hover:scale-[1.02] active:scale-[0.98] ${
        selected
          ? "border-[var(--color-accent)] ring-2 ring-[var(--color-accent)]/35"
          : "border-cyan-500/40 hover:border-cyan-500/60"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
      <div className="flex items-start gap-3">
        <span
          className={`inline-flex shrink-0 items-center justify-center rounded-xl size-11 [&_svg]:size-5 ${kindIconTileClass("Application")}`}
        >
          <Icon strokeWidth={1.65} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[10px] uppercase tracking-wide text-cyan-400/80 font-semibold flex items-center gap-1">
            Child App
            <ExternalLink className="size-3 opacity-70" />
          </div>
          <div className="text-base font-semibold text-[var(--color-text)] truncate" title={data.name}>
            {data.name}
          </div>
          <div className="mt-1.5 flex flex-wrap gap-1">
            <HealthBadge status={data.health} />
            <SyncBadge status={data.sync} />
          </div>
          {age && (
            <div className="mt-1 text-[9px] text-[var(--color-text-muted)]" title={data.raw.creationTimestamp}>
              {age}
            </div>
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

const nodeTypes = {
  application: ApplicationNode,
  childApp: ChildAppNode,
  kind: KindNode,
};

function layoutDagre(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  // Increased nodesep from 40 to 60 and ranksep from 72 to 100 to prevent nodes from overlapping
  g.setGraph({ rankdir: "LR", nodesep: 60, ranksep: 100, marginx: 40, marginy: 36 });
  nodes.forEach((n) => {
    const { w, h } = nodeMeasuredSize(n);
    g.setNode(n.id, { width: w, height: h });
  });
  edges.forEach((e) => {
    g.setEdge(e.source, e.target);
  });
  dagre.layout(g);
  return nodes.map((n) => {
    const { w, h } = nodeMeasuredSize(n);
    const pos = g.node(n.id);
    return {
      ...n,
      targetPosition: Position.Left,
      sourcePosition: Position.Right,
      position: {
        x: pos.x - w / 2,
        y: pos.y - h / 2,
      },
    };
  });
}

function flowTypeForResource(n: ResourceNode): keyof typeof nodeTypes {
  if (n.kind === "Application" && n.uid.startsWith("synthetic:app:")) return "application";
  if (isChildAppNode(n)) return "childApp";
  return "kind";
}

function treeToFlow(roots: ResourceNode[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  const walk = (n: ResourceNode, parentId: string | null) => {
    const fType = flowTypeForResource(n);
    nodes.push({
      id: n.uid,
      type: fType,
      position: { x: 0, y: 0 },
      data: {
        kind: n.kind,
        name: n.name,
        health: n.health,
        sync: n.sync,
        syncMessage: n.syncMessage,
        raw: n,
      },
    });
    if (parentId) {
      const isChildApp = isChildAppNode(n);
      edges.push({
        id: `${parentId}-${n.uid}`,
        source: parentId,
        target: n.uid,
        type: "smoothstep",
        animated: true,
        label: isChildApp ? "spawns" : undefined,
        labelStyle: isChildApp ? { fill: "#67e8f9", fontSize: 10 } : undefined,
        labelBgStyle: isChildApp ? { fill: "transparent" } : undefined,
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: isChildApp ? "#67e8f9" : "#22d3ee",
          width: isChildApp ? 18 : 16,
          height: isChildApp ? 18 : 16,
        },
        style: {
          stroke: isChildApp ? "rgba(103, 232, 249, 0.65)" : "rgba(34, 211, 238, 0.35)",
          strokeWidth: isChildApp ? 2 : 1.5,
          strokeDasharray: isChildApp ? "5 3" : undefined,
        },
      });
    }
    for (const c of n.children ?? []) {
      walk(c, n.uid);
    }
  };
  for (const r of roots) {
    walk(r, null);
  }
  return { nodes, edges };
}

function FitViewHelper({ shouldFit }: { shouldFit: boolean }) {
  const { fitView } = useReactFlow();
  useEffect(() => {
    if (shouldFit) {
      const id = requestAnimationFrame(() => {
        fitView({ padding: 0.18, duration: 250, minZoom: 0.5, maxZoom: 1.1 });
      });
      return () => cancelAnimationFrame(id);
    }
  }, [shouldFit, fitView]);
  return null;
}

function TopologyFlowInner({
  roots,
  onNodeSelect,
  onNodeContextMenu,
  onNavigateToApp,
}: {
  roots: ResourceNode[];
  onNodeSelect: (n: ResourceNode) => void;
  onNodeContextMenu?: (e: MouseEvent, n: ResourceNode) => void;
  onNavigateToApp?: (appName: string) => void;
}) {
  const { initialNodes, initialEdges } = useMemo(() => {
    const { nodes, edges } = treeToFlow(roots);
    const laid = layoutDagre(nodes, edges);
    return { initialNodes: laid, initialEdges: edges };
  }, [roots]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);
  const [isFirstRender, setIsFirstRender] = useState(true);

  useEffect(() => {
    setNodes(initialNodes);
    setEdges(initialEdges);
    // Only fit view on first render, not on every update
    if (isFirstRender && initialNodes.length > 0) {
      setIsFirstRender(false);
    }
  }, [initialNodes, initialEdges, setNodes, setEdges, isFirstRender]);

  const onNodeClick = useCallback(
    (_evt: unknown, node: Node) => {
      const raw = (node.data as { raw?: ResourceNode }).raw;
      if (!raw) return;
      // Child App nodes navigate to the child application instead of opening a detail panel.
      if (isChildAppNode(raw) && onNavigateToApp) {
        onNavigateToApp(raw.name);
        return;
      }
      onNodeSelect(raw);
    },
    [onNodeSelect, onNavigateToApp],
  );

  const onNodeContextMenuCb = useCallback(
    (evt: MouseEvent, node: Node) => {
      const raw = (node.data as { raw?: ResourceNode }).raw;
      if (!raw || !onNodeContextMenu) return;
      onNodeContextMenu(evt, raw);
    },
    [onNodeContextMenu],
  );

  if (!roots.length) {
    return <div className="text-sm text-[var(--color-text-muted)]">No resources to display.</div>;
  }

  return (
    <div className="flex-1 min-h-0 w-full">
      <ReactFlow
        className="!bg-transparent"
        style={{ width: "100%", height: "100%" }}
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        onNodeClick={onNodeClick}
        onNodeContextMenu={onNodeContextMenuCb}
        nodesDraggable={false}
        nodesConnectable={false}
        edgesReconnectable={false}
        minZoom={0.25}
        maxZoom={2}
        zoomOnDoubleClick={false}
        defaultEdgeOptions={{ type: "smoothstep" }}
        proOptions={{ hideAttribution: true }}
        elevateEdgesOnSelect
      >
      <Background gap={22} size={1} color="rgba(139, 148, 158, 0.12)" />
      <Controls
        className="!bg-[var(--color-surface)] !border-[var(--color-border)] !shadow-lg [&_button]:!fill-[var(--color-text)] [&_button]:hover:!fill-[var(--color-accent)]"
        showInteractive={false}
      />
      <MiniMap
        className="!bg-[var(--color-surface-muted)] !border-[var(--color-border)] !rounded-lg"
        nodeStrokeWidth={2}
        zoomable
        pannable
        maskColor="rgb(0 0 0 / 45%)"
        nodeColor={() => "rgba(34, 211, 238, 0.45)"}
      />
      <FitViewHelper shouldFit={isFirstRender && nodes.length > 0} />
    </ReactFlow>
    </div>
  );
}

export function ResourceTopologyView({
  roots,
  onNodeSelect,
  onNodeContextMenu,
  onNavigateToApp,
}: {
  roots: ResourceNode[];
  onNodeSelect: (n: ResourceNode) => void;
  onNodeContextMenu?: (e: MouseEvent, n: ResourceNode) => void;
  onNavigateToApp?: (appName: string) => void;
}) {
  if (!roots.length) {
    return <div className="text-sm text-[var(--color-text-muted)]">No resources to display.</div>;
  }

  return (
    <div className="w-full flex-1 min-h-0 flex flex-col rounded-lg border border-[var(--color-border)] bg-[var(--color-topology-bg)] shadow-inner overflow-hidden">
      <ReactFlowProvider>
        <div className="flex-1 min-h-0 flex flex-col min-h-[280px]">
          <TopologyFlowInner roots={roots} onNodeSelect={onNodeSelect} onNodeContextMenu={onNodeContextMenu} onNavigateToApp={onNavigateToApp} />
        </div>
      </ReactFlowProvider>
    </div>
  );
}
