import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function HistoryView({ name }: { name: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["app-history", name],
    queryFn: () => api.appHistory(name),
    refetchInterval: 5000,
  });
  if (isLoading) return <div className="text-sm text-[var(--color-text-muted)]">Loading history…</div>;
  if (error) return <div className="text-sm text-red-400">{(error as Error).message}</div>;
  if (!data?.length) return <div className="text-sm text-[var(--color-text-muted)]">No sync history yet.</div>;
  return (
    <table className="w-full text-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg overflow-hidden">
      <thead className="text-left text-xs text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
        <tr>
          <th className="px-3 py-2">Started</th>
          <th className="px-3 py-2">Revision</th>
          <th className="px-3 py-2">Status</th>
          <th className="px-3 py-2">Initiated by</th>
          <th className="px-3 py-2">Resources</th>
        </tr>
      </thead>
      <tbody className="text-[var(--color-text)]">
        {data.map((op) => (
          <tr key={op.id} className="border-t border-[var(--color-border)]">
            <td className="px-3 py-2 text-xs">{new Date(op.startedAt).toLocaleString()}</td>
            <td className="px-3 py-2 font-mono text-xs">{op.revision?.slice(0, 8)}</td>
            <td className="px-3 py-2">{op.status}</td>
            <td className="px-3 py-2">{op.initiatedBy}</td>
            <td className="px-3 py-2">{op.resources.length}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
