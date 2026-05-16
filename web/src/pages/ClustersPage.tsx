import { useCallback, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";
import type { ClusterHealth, NodeInfo } from "../api/types";
import { NodesView } from "../components/NodesView";

export function ClustersPage() {
  const { data: clusters, isLoading: loadingClusters, error: clustersError } = useQuery({
    queryKey: ["clusters"],
    queryFn: api.listClusters,
  });
  const { data: health, isLoading: loadingHealth } = useQuery({
    queryKey: ["clusters", "health"],
    queryFn: api.listClusterHealth,
    refetchInterval: 30000,
  });
  const [selectedCluster, setSelectedCluster] = useState<ClusterHealth | null>(null);
  const { data: nodes, isLoading: loadingNodes } = useQuery({
    queryKey: ["clusters", selectedCluster?.clusterId, "nodes"],
    queryFn: () => api.listClusterNodes(selectedCluster!.clusterId),
    enabled: !!selectedCluster,
    refetchInterval: 15000,
  });

  const handleBack = useCallback(() => {
    setSelectedCluster(null);
  }, []);

  if (selectedCluster) {
    return (
      <div className="flex-1 min-h-0 overflow-hidden">
        <NodesView
          cluster={selectedCluster}
          nodes={nodes ?? []}
          loading={loadingNodes}
          onBack={handleBack}
        />
      </div>
    );
  }

  if (clustersError) {
    return (
      <div className="p-6">
        <h1 className="text-xl font-semibold mb-4 text-[var(--color-text)]">Clusters</h1>
        <div className="text-sm text-red-400">{(clustersError as Error).message}</div>
      </div>
    );
  }

  return (
    <div className="p-6 max-w-5xl flex-1 min-h-0 overflow-y-auto w-full">
      <h1 className="text-xl font-semibold mb-4 text-[var(--color-text)]">Clusters</h1>
      {(loadingClusters || loadingHealth) && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
      {clusters && health && (
        <div className="grid gap-4">
          {health.map((h) => {
            const cluster = clusters.find((c) => c.id === h.clusterId);
            return (
              <ClusterCard
                key={h.clusterId}
                health={h}
                cluster={cluster}
                onSelect={() => setSelectedCluster(h)}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}

function ClusterCard({
  health,
  cluster,
  onSelect,
}: {
  health: ClusterHealth;
  cluster?: { name: string; serverUrl: string; inCluster: boolean };
  onSelect: () => void;
}) {
  const statusColor =
    health.status === "Ready"
      ? "text-green-400"
      : health.status === "Degraded"
        ? "text-yellow-400"
        : "text-red-400";

  const statusDot =
    health.status === "Ready"
      ? "bg-green-400"
      : health.status === "Degraded"
        ? "bg-yellow-400"
        : "bg-red-400";

  return (
    <div
      onClick={onSelect}
      className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg p-5 cursor-pointer hover:border-[var(--color-accent)]/40 transition group"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 mb-2">
            <span className={`size-2.5 rounded-full ${statusDot} shrink-0`} />
            <span className="text-base font-semibold text-[var(--color-text)]">{health.clusterName}</span>
            {cluster?.inCluster && (
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-accent)]/10 text-[var(--color-accent)] font-medium">
                in-cluster
              </span>
            )}
          </div>
          <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-[var(--color-text-muted)]">
            <span className={statusColor}>{health.status}</span>
            {health.k8sVersion && <span>K8s {health.k8sVersion}</span>}
            <span>{health.nodeCount} node{health.nodeCount !== 1 ? "s" : ""}</span>
            <span>{health.appCount} app{health.appCount !== 1 ? "s" : ""}</span>
          </div>
          {health.error && (
            <div className="mt-2 text-xs text-red-400 bg-red-400/10 rounded px-2 py-1">
              {health.error}
            </div>
          )}
        </div>
        <div className="text-[var(--color-text-muted)] group-hover:text-[var(--color-text)] transition text-sm shrink-0">
          View nodes →
        </div>
      </div>
    </div>
  );
}
