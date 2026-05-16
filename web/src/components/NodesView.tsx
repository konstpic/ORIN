import { useState } from "react";
import type { ClusterHealth, NodeInfo } from "../api/types";

export function NodesView({
  cluster,
  nodes,
  loading,
  onBack,
}: {
  cluster: ClusterHealth;
  nodes: NodeInfo[];
  loading: boolean;
  onBack: () => void;
}) {
  return (
    <div className="flex flex-col h-full">
      <header className="shrink-0 px-8 py-5 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
        <div className="flex items-center gap-3">
          <button onClick={onBack} className="text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)] transition">
            ← Clusters
          </button>
          <span className="text-[var(--color-text-muted)]">/</span>
          <h1 className="text-lg font-semibold text-[var(--color-text)]">{cluster.clusterName}</h1>
          <span className="text-sm text-[var(--color-text-muted)]">{cluster.k8sVersion}</span>
        </div>
        <div className="flex gap-4 mt-1 text-sm text-[var(--color-text-muted)]">
          <span>{nodes.length} node{nodes.length !== 1 ? "s" : ""}</span>
          <span>{nodes.reduce((s, n) => s + n.podCount, 0)} pods</span>
        </div>
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto p-6">
        {loading && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
        <div className="grid gap-5">
          {nodes.map((n) => (
            <NodeCard key={n.name} node={n} />
          ))}
        </div>
      </div>
    </div>
  );
}

function NodeCard({ node }: { node: NodeInfo }) {
  const [expanded, setExpanded] = useState(false);
  const isCP = node.roles.includes("control-plane") || node.roles.includes("master");
  const statusColor = node.status === "Ready" ? "text-green-400" : "text-red-400";
  const statusDot = node.status === "Ready" ? "bg-green-400" : "bg-red-400";

  return (
    <div className={`bg-[var(--color-surface)] border ${isCP ? "border-[var(--color-accent)]/30" : "border-[var(--color-border)]"} rounded-lg overflow-hidden`}>
      <div
        onClick={() => setExpanded(!expanded)}
        className="px-5 py-4 cursor-pointer hover:bg-[var(--color-sidebar-hover)]/50 transition"
      >
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <span className={`size-2.5 rounded-full ${statusDot} shrink-0`} />
            <span className="font-mono text-sm font-semibold text-[var(--color-text)]">{node.name}</span>
            {isCP && (
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-accent)]/10 text-[var(--color-accent)] font-medium">
                control-plane
              </span>
            )}
          </div>
          <div className="flex items-center gap-6 text-sm text-[var(--color-text-muted)] shrink-0">
            <span className={statusColor}>{node.status}</span>
            <span>{node.podCount} pods</span>
            <span className="w-32">
              <ResourceBar label="CPU" used={node.cpuUsedPercent} />
            </span>
            <span className="w-32">
              <ResourceBar label="Mem" used={node.memUsedPercent} />
            </span>
            <span className="text-[var(--color-text-muted)] text-xs">{expanded ? "▾" : "▸"}</span>
          </div>
        </div>
        <div className="flex gap-4 mt-2 text-xs text-[var(--color-text-muted)]">
          <span>{node.kubeletVersion}</span>
          <span>{node.os} / {node.arch}</span>
          <span>Alloc: CPU {node.cpuAllocatable}, Mem {node.memAllocatable}</span>
        </div>
      </div>
      {expanded && node.pods.length > 0 && (
        <div className="border-t border-[var(--color-border)] bg-[var(--color-bg)]/50">
          <table className="w-full text-xs">
            <thead className="text-left text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
              <tr>
                <th className="px-4 py-1.5 font-medium">Pod</th>
                <th className="px-4 py-1.5 font-medium">Namespace</th>
                <th className="px-4 py-1.5 font-medium">Owner</th>
                <th className="px-4 py-1.5 font-medium">CPU Req</th>
                <th className="px-4 py-1.5 font-medium">Mem Req</th>
                <th className="px-4 py-1.5 font-medium">Status</th>
              </tr>
            </thead>
            <tbody className="text-[var(--color-text)] divide-y divide-[var(--color-border)]/30">
              {node.pods.map((p) => (
                <tr key={p.name} className="hover:bg-[var(--color-sidebar-hover)]/30">
                  <td className="px-4 py-1.5 font-mono truncate max-w-[200px]">{p.name}</td>
                  <td className="px-4 py-1.5 text-[var(--color-text-muted)]">{p.namespace}</td>
                  <td className="px-4 py-1.5">
                    {p.owner ? (
                      <span className="text-[var(--color-text-muted)]">{p.kind}/{p.owner}</span>
                    ) : (
                      <span className="text-[var(--color-text-muted)]">—</span>
                    )}
                  </td>
                  <td className="px-4 py-1.5 text-[var(--color-text-muted)] font-mono">{p.cpuReq || "—"}</td>
                  <td className="px-4 py-1.5 text-[var(--color-text-muted)] font-mono">{p.memReq || "—"}</td>
                  <td className="px-4 py-1.5">
                    <span className={podStatusColor(p.status)}>{p.status}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function ResourceBar({ label, used }: { label: string; used: number }) {
  const pct = Math.min(100, Math.max(0, used));
  const barColor = pct > 90 ? "bg-red-400" : pct > 70 ? "bg-yellow-400" : "bg-green-400";
  return (
    <span className="flex items-center gap-1.5" title={`${label}: ${pct.toFixed(0)}%`}>
      <span className="text-[10px] w-6 shrink-0">{label}</span>
      <span className="flex-1 h-1.5 bg-[var(--color-border)] rounded-full overflow-hidden">
        <span className={`h-full rounded-full ${barColor} transition-all`} style={{ width: `${pct}%` }} />
      </span>
      <span className="text-[10px] w-8 text-right">{pct.toFixed(0)}%</span>
    </span>
  );
}

function podStatusColor(status: string): string {
  switch (status) {
    case "Running":
    case "Succeeded":
      return "text-green-400";
    case "Pending":
      return "text-yellow-400";
    case "Failed":
      return "text-red-400";
    default:
      return "text-[var(--color-text-muted)]";
  }
}
