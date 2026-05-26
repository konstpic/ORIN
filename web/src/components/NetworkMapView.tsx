import { useMemo } from "react";
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  Handle,
  Position,
  MarkerType,
  type Node,
  type Edge,
  type NodeProps,
} from "@xyflow/react";
import dagre from "dagre";
import "@xyflow/react/dist/style.css";
import { HealthBadge, SyncBadge } from "./Badges";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";
import { relativeTime } from "../utils/relativeTime";
import type { HealthStatus, SyncStatus } from "../api/types";

// ── Custom node types ────────────────────────────────────────────────

function CloudNode({ data }: { data: { label: string } }) {
  return (
    <div className="flex flex-col items-center gap-1">
      <svg width="48" height="32" viewBox="0 0 48 32" fill="none">
        <path d="M12 26a6 6 0 01-.9-11.9A8 8 0 0127.5 10a6 6 0 012.5 11.6V26H12z"
              fill="#334155" stroke="#64748b" strokeWidth="1.5" strokeLinejoin="round" />
      </svg>
      <span className="text-[10px] font-mono text-[var(--color-text-muted)]">{data.label}</span>
    </div>
  );
}

function NodeIPNode({ data }: { data: { label: string; ip: string } }) {
  return (
    <div className="rounded-lg border bg-[var(--color-surface)] px-3 py-2.5 shadow-md min-w-[180px] max-w-[240px]">
      <Handle type="target" position={Position.Left} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
      <div className="flex items-start gap-2.5">
        <span className="inline-flex shrink-0 items-center justify-center rounded-lg size-9 bg-[var(--color-accent-muted)] text-[var(--color-accent)] [&_svg]:size-4">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65">
            <rect x="2" y="3" width="20" height="14" rx="2" />
            <path d="M8 21h8M12 17v4" />
          </svg>
        </span>
        <div className="min-w-0 flex-1">
          <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">
            Node
          </div>
          <div className="text-sm font-semibold text-[var(--color-text)] truncate" title={data.ip}>
            {data.ip}
          </div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

type NetworkFlowNodeData = {
  kind: string;
  name: string;
  health?: string;
  sync?: string;
  creationTimestamp?: string;
  raw: {
    uid: string;
    kind: string;
    name: string;
    labels?: Record<string, string>;
    isKindGroup?: boolean;
    groupedMembers?: unknown[];
    groupedPods?: unknown[];
    creationTimestamp?: string;
  };
};

function NetworkResourceNode(props: NodeProps) {
  const { selected } = props;
  const data = props.data as NetworkFlowNodeData;
  const Icon = iconForKind(data.kind);
  const age = data.raw.creationTimestamp ? relativeTime(data.raw.creationTimestamp) : "";
  const healthStatus: HealthStatus = (data.health as HealthStatus) ?? "Unknown";
  const syncStatus: SyncStatus = (data.sync as SyncStatus) ?? "Unknown";

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
            <HealthBadge status={healthStatus} size="sm" />
            <SyncBadge status={syncStatus} size="sm" />
          </div>
          {age && (
            <div className="mt-1.5 pt-1 border-t border-[var(--color-border)]/50 flex items-center justify-between gap-2">
              <span />
              <span className="text-[9px] text-[var(--color-text-muted)] shrink-0" title={data.raw.creationTimestamp}>
                {age}
              </span>
            </div>
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

function PolicyNode(props: NodeProps) {
  const { selected } = props;
  const data = props.data as NetworkFlowNodeData;
  const Icon = iconForKind(data.kind);

  return (
    <div
      className={`rounded-lg border bg-[var(--color-surface)] px-3 py-2.5 shadow-md min-w-[180px] max-w-[240px] transition-all duration-150 cursor-pointer hover:shadow-lg hover:scale-[1.02] active:scale-[0.98] border-dashed ${
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
          <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">
            {data.kind}
          </div>
          <div className="text-sm font-semibold text-[var(--color-text)] truncate" title={data.name}>
            {data.name}
          </div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!opacity-0 !pointer-events-none !w-0 !h-0 !min-w-0 !min-h-0 !border-0" />
    </div>
  );
}

const nodeTypes = {
  cloud: CloudNode,
  nodeIP: NodeIPNode,
  service: NetworkResourceNode,
  pod: NetworkResourceNode,
  policy: PolicyNode,
};

// ── API types ────────────────────────────────────────────────────────

export interface NetworkMapNode {
  uid: string;
  group: string;
  version: string;
  kind: string;
  namespace: string;
  name: string;
  health?: string;
  sync?: string;
  labels?: Record<string, string>;
  selector?: Record<string, string>;
  ingressBackends?: string[];
  netPolicyPodSelector?: Record<string, string>;
  netPolicyIngressFrom?: Array<{ podSelector?: Record<string, string>; namespaceSelector?: Record<string, string> }>;
  netPolicyEgressTo?: Array<{ podSelector?: Record<string, string> }>;
}

export interface NetworkMapEdge {
  sourceUid: string;
  targetUid: string;
  type: "routes" | "selects" | "ingress-allow" | "egress-allow";
  label: string;
}

export interface NetworkMapResponse {
  nodes: NetworkMapNode[];
  edges: NetworkMapEdge[];
}

// ── Main component ───────────────────────────────────────────────────

export function NetworkMapView({
  data,
  onNodeSelect,
}: {
  data: NetworkMapResponse;
  onNodeSelect: (node: NetworkMapNode) => void;
}) {
  const { flowNodes, flowEdges } = useMemo(() => buildNetworkGraph(data.nodes, data.edges), [data]);

  return (
    <div className="w-full flex-1 min-h-0 flex flex-col rounded-lg border border-[var(--color-border)] bg-[var(--color-topology-bg)] shadow-inner overflow-hidden relative">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.18, minZoom: 0.5, maxZoom: 1.1 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        minZoom={0.25}
        maxZoom={2}
        zoomOnDoubleClick={false}
        onNodeClick={(_, n) => {
          const rn = n.data as unknown as NetworkMapNode | undefined;
          if (rn) onNodeSelect(rn);
        }}
        defaultEdgeOptions={{
          type: "smoothstep",
          animated: true,
          markerEnd: { type: MarkerType.ArrowClosed, color: "#22d3ee", width: 16, height: 16 },
          style: { stroke: "rgba(34, 211, 238, 0.35)", strokeWidth: 1.5 },
        }}
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
      </ReactFlow>

      {/* Legend */}
      <div className="absolute top-3 left-3 text-xs bg-[var(--color-surface)]/95 backdrop-blur rounded-lg px-3 py-2.5 border border-[var(--color-border)]">
        <div className="font-semibold mb-2 text-[var(--color-text)] text-[11px] uppercase tracking-wide">Network Topology</div>
        <div className="space-y-1.5">
          <div className="flex items-center gap-2">
            <svg width="16" height="10" viewBox="0 0 48 32" fill="none">
              <path d="M12 26a6 6 0 01-.9-11.9A8 8 0 0127.5 10a6 6 0 012.5 11.6V26H12z"
                    fill="#334155" stroke="#64748b" strokeWidth="2" />
            </svg>
            <span className="text-[var(--color-text-muted)]">Internet</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-3 h-0.5 bg-[#8b5cf6] rounded" />
            <span className="text-[var(--color-text-muted)]">Ingress → Service</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-3 h-0.5 bg-[#06b6d4] rounded" />
            <span className="text-[var(--color-text-muted)]">Service → Pod</span>
          </div>
        </div>
      </div>

      <div className="absolute bottom-10 right-14 text-[10px] text-[var(--color-text-muted)] bg-[var(--color-surface)]/80 px-2 py-1 rounded border border-[var(--color-border)]">
        {data.nodes.length} resources · {data.edges.length} connections
      </div>
    </div>
  );
}

// ── Graph builder ────────────────────────────────────────────────────

function buildNetworkGraph(
  nodes: NetworkMapNode[],
  edges: NetworkMapEdge[],
): { flowNodes: Node[]; flowEdges: Edge[] } {
  const g = new dagre.graphlib.Graph({ directed: true, multigraph: false });
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "LR", nodesep: 60, ranksep: 100, marginx: 40, marginy: 36 });

  const flowNodes: Node[] = [];
  const flowEdges: Edge[] = [];

  // ── 1. Cloud node (entry point) ──
  const cloudUid = "__cloud__";
  g.setNode(cloudUid, { width: 60, height: 50 });
  flowNodes.push({
    id: cloudUid,
    type: "cloud",
    position: { x: 0, y: 0 },
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    data: { label: "Internet" },
  });

  // ── 2. Node IP ──
  const nodeIPs = new Set<string>();
  for (const n of nodes) {
    if (n.kind === "Ingress" || n.kind === "Service") {
      nodeIPs.add("10.0.0.1");
    }
  }
  if (nodeIPs.size === 0) nodeIPs.add("10.0.0.1");

  const nodeIPsArr = Array.from(nodeIPs);
  for (const ip of nodeIPsArr) {
    const uid = `__node__${ip}__`;
    g.setNode(uid, { width: 228, height: 88 });
    flowNodes.push({
      id: uid,
      type: "nodeIP",
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: { label: "Node", ip },
    });
    // Cloud → Node edge
    flowEdges.push({
      id: `${cloudUid}→${uid}`,
      source: cloudUid,
      target: uid,
    });
  }

  // ── 3. Index nodes by kind ──
  const byKind = new Map<string, NetworkMapNode[]>();
  for (const n of nodes) {
    if (!byKind.has(n.kind)) byKind.set(n.kind, []);
    byKind.get(n.kind)!.push(n);
  }

  const ingressNodes = byKind.get("Ingress") ?? [];
  const serviceNodes = byKind.get("Service") ?? [];
  const podNodes = byKind.get("Pod") ?? [];
  const policyNodes = byKind.get("NetworkPolicy") ?? [];

  // ── 4. Ingress nodes ──
  for (const n of ingressNodes) {
    g.setNode(n.uid, { width: 228, height: 88 });
    flowNodes.push({
      id: n.uid,
      type: "service",
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        kind: n.kind,
        name: n.name,
        health: n.health,
        sync: n.sync,
        raw: { uid: n.uid, kind: n.kind, name: n.name, labels: n.labels },
      },
    });
    // Node → Ingress edge
    const nodeIPUid = `__node__${nodeIPsArr[0]}__`;
    flowEdges.push({
      id: `${nodeIPUid}→${n.uid}`,
      source: nodeIPUid,
      target: n.uid,
    });
  }

  // ── 5. Service nodes ──
  for (const n of serviceNodes) {
    g.setNode(n.uid, { width: 228, height: 88 });
    flowNodes.push({
      id: n.uid,
      type: "service",
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        kind: n.kind,
        name: n.name,
        health: n.health,
        sync: n.sync,
        raw: { uid: n.uid, kind: n.kind, name: n.name, labels: n.labels },
      },
    });
  }

  // Ingress → Service edges
  for (const n of ingressNodes) {
    for (const svcName of n.ingressBackends ?? []) {
      const svc = serviceNodes.find((s) => s.name === svcName);
      if (svc) {
        flowEdges.push({
          id: `${n.uid}→${svc.uid}→routes`,
          source: n.uid,
          target: svc.uid,
        });
      }
    }
  }

  // ── 6. Pod nodes ──
  for (const n of podNodes) {
    g.setNode(n.uid, { width: 228, height: 88 });
    flowNodes.push({
      id: n.uid,
      type: "pod",
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        kind: n.kind,
        name: n.name,
        health: n.health,
        sync: n.sync,
        raw: { uid: n.uid, kind: n.kind, name: n.name, labels: n.labels },
      },
    });
  }

  // Service → Pod edges (via selector match)
  for (const n of serviceNodes) {
    if (n.selector && Object.keys(n.selector).length > 0) {
      for (const pod of podNodes) {
        if (pod.labels && matchesSelector(pod.labels, n.selector)) {
          flowEdges.push({
            id: `${n.uid}→${pod.uid}→selects`,
            source: n.uid,
            target: pod.uid,
          });
        }
      }
    }
  }

  // ── 7. NetworkPolicy nodes ──
  for (const n of policyNodes) {
    g.setNode(n.uid, { width: 228, height: 88 });
    flowNodes.push({
      id: n.uid,
      type: "policy",
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        kind: n.kind,
        name: n.name,
        health: n.health,
        sync: n.sync,
        raw: { uid: n.uid, kind: n.kind, name: n.name },
      },
    });
  }

  // Add all edges to dagre graph
  for (const e of flowEdges) {
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  for (const fn of flowNodes) {
    const node = g.node(fn.id);
    const w = node.width ?? 228;
    const h = node.height ?? 88;
    fn.position = { x: node.x - w / 2, y: node.y - h / 2 };
  }

  return { flowNodes, flowEdges };
}

function matchesSelector(labels: Record<string, string>, selector: Record<string, string>): boolean {
  for (const [k, v] of Object.entries(selector)) {
    if (labels[k] !== v) return false;
  }
  return true;
}
