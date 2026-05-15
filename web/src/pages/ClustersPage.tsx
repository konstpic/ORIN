import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function ClustersPage() {
  const { data, isLoading, error } = useQuery({ queryKey: ["clusters"], queryFn: api.listClusters });
  return (
    <div className="p-6 max-w-3xl flex-1 min-h-0 overflow-y-auto w-full">
      <h1 className="text-xl font-semibold mb-4 text-[var(--color-text)]">Clusters</h1>
      <p className="text-sm text-[var(--color-text-muted)] mb-4">
        MVP supports only the in-cluster destination. Multi-cluster is on the roadmap.
      </p>
      {isLoading && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
      {error && <div className="text-sm text-red-400">{(error as Error).message}</div>}
      {data && (
        <table className="w-full text-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg overflow-hidden">
          <thead className="text-left text-xs text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Server URL</th>
              <th className="px-3 py-2">In-cluster</th>
            </tr>
          </thead>
          <tbody className="text-[var(--color-text)]">
            {data.map((c) => (
              <tr key={c.id} className="border-t border-[var(--color-border)]">
                <td className="px-3 py-2 font-medium">{c.name}</td>
                <td className="px-3 py-2 break-all">{c.serverUrl}</td>
                <td className="px-3 py-2">{c.inCluster ? "yes" : "no"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
