import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function HistoryDrawer({
  appName,
  open,
  onClose,
}: {
  appName: string;
  open: boolean;
  onClose: () => void;
}) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["app-history", appName],
    queryFn: () => api.appHistory(appName),
    enabled: open && !!appName,
    refetchInterval: open ? 5000 : false,
  });

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end pointer-events-none">
      <button type="button" className="absolute inset-0 bg-black/50 pointer-events-auto" aria-label="Close history" onClick={onClose} />
      <aside className="relative pointer-events-auto h-full w-full max-w-2xl border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col">
        <div className="shrink-0 px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text)]">Sync history</h2>
            <p className="text-xs text-[var(--color-text-muted)] mt-0.5">Recent operations and per-resource results</p>
          </div>
          <button type="button" className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline" onClick={onClose}>
            Close
          </button>
        </div>
        <div className="flex-1 min-h-0 overflow-y-auto p-4">
          {isLoading && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
          {error && <div className="text-sm text-red-400">{(error as Error).message}</div>}
          {!isLoading && !error && !data?.length && (
            <div className="text-sm text-[var(--color-text-muted)]">No sync history yet.</div>
          )}
          {!!data?.length && (
            <div className="space-y-6">
              {data.map((op) => (
                <div key={op.id} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] overflow-hidden">
                  <div className="px-3 py-2 text-xs border-b border-[var(--color-border)] bg-[var(--color-surface)] flex flex-wrap gap-x-4 gap-y-1 text-[var(--color-text)]">
                    <span>
                      <span className="text-[var(--color-text-muted)]">Started:</span> {new Date(op.startedAt).toLocaleString()}
                    </span>
                    <span className="font-mono">@{op.revision?.slice(0, 8)}</span>
                    <span className="font-medium">{op.status}</span>
                    <span className="text-[var(--color-text-muted)]">by {op.initiatedBy}</span>
                  </div>
                  {op.message && <div className="px-3 py-2 text-xs text-amber-300/90 border-b border-[var(--color-border)]">{op.message}</div>}
                  {op.resources.length > 0 && (
                    <table className="w-full text-left text-xs">
                      <thead className="text-[var(--color-text-muted)] uppercase tracking-wide bg-[var(--color-surface)]">
                        <tr>
                          <th className="px-2 py-1.5">Kind</th>
                          <th className="px-2 py-1.5">Name</th>
                          <th className="px-2 py-1.5">Status</th>
                          <th className="px-2 py-1.5">Message</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[var(--color-border)] text-[var(--color-text)]">
                        {op.resources.map((r, i) => (
                          <tr key={`${r.kind}-${r.name}-${i}`} className="align-top">
                            <td className="px-2 py-1 font-mono text-[var(--color-accent)]">{r.kind}</td>
                            <td className="px-2 py-1 font-mono">{r.name}</td>
                            <td className="px-2 py-1 whitespace-nowrap">
                              <span
                                className={
                                  r.status === "Failed"
                                    ? "text-red-400 font-medium"
                                    : r.status === "Applied" || r.status === "DryRun" || r.status === "Pruned"
                                      ? "text-emerald-400"
                                      : "text-[var(--color-text-muted)]"
                                }
                              >
                                {r.status}
                              </span>
                            </td>
                            <td className="px-2 py-1 text-[var(--color-text-muted)] break-all">{r.message || "—"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
