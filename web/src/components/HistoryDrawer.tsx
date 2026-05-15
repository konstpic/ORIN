import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Clock, User, XCircle } from "lucide-react";
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
              {data.map((op) => {
                const isFailed = op.status === "Failed";
                const isSuccess = op.status === "Succeeded" || op.status === "Applied";
                const failedResources = op.resources.filter((r) => r.status === "Failed");
                const successResources = op.resources.filter((r) => r.status === "Applied" || r.status === "Pruned" || r.status === "DryRun");
                
                return (
                  <div 
                    key={op.id} 
                    className={`rounded-lg border overflow-hidden ${
                      isFailed 
                        ? "border-red-500/50 bg-red-500/5" 
                        : isSuccess 
                        ? "border-emerald-500/30 bg-emerald-500/5" 
                        : "border-[var(--color-border)] bg-[var(--color-surface-muted)]"
                    }`}
                  >
                    {/* Header with status icon */}
                    <div className={`px-3 py-2.5 border-b flex items-center gap-3 ${
                      isFailed 
                        ? "border-red-500/30 bg-red-500/10" 
                        : isSuccess 
                        ? "border-emerald-500/20 bg-emerald-500/10" 
                        : "border-[var(--color-border)] bg-[var(--color-surface)]"
                    }`}>
                      {isFailed ? (
                        <XCircle className="size-5 shrink-0 text-red-400" strokeWidth={2} />
                      ) : isSuccess ? (
                        <CheckCircle2 className="size-5 shrink-0 text-emerald-400" strokeWidth={2} />
                      ) : (
                        <AlertTriangle className="size-5 shrink-0 text-amber-400" strokeWidth={2} />
                      )}
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
                          <span className={`font-semibold ${
                            isFailed ? "text-red-200" : isSuccess ? "text-emerald-200" : "text-[var(--color-text)]"
                          }`}>
                            {op.status}
                          </span>
                          <span className="flex items-center gap-1 text-[var(--color-text-muted)]">
                            <Clock className="size-3" />
                            {new Date(op.startedAt).toLocaleString()}
                          </span>
                          <span className="font-mono text-[var(--color-text-muted)]">@{op.revision?.slice(0, 8)}</span>
                          <span className="flex items-center gap-1 text-[var(--color-text-muted)]">
                            <User className="size-3" />
                            {op.initiatedBy}
                          </span>
                        </div>
                      </div>
                    </div>
                    
                    {/* Operation message */}
                    {op.message && (
                      <div className={`px-3 py-2 text-xs border-b ${
                        isFailed 
                          ? "text-red-300 bg-red-500/10 border-red-500/30" 
                          : "text-amber-300/90 border-[var(--color-border)]"
                      }`}>
                        {op.message}
                      </div>
                    )}
                    
                    {/* Failed resources - show prominently */}
                    {failedResources.length > 0 && (
                      <div className="px-3 py-2 bg-red-500/10 border-b border-red-500/30">
                        <div className="text-xs font-semibold text-red-200 mb-2 flex items-center gap-2">
                          <XCircle className="size-4" strokeWidth={2} />
                          Failed Resources ({failedResources.length})
                        </div>
                        <div className="space-y-2">
                          {failedResources.map((r, i) => (
                            <div key={`${r.kind}-${r.name}-${i}`} className="rounded border border-red-500/40 bg-red-500/5 p-2">
                              <div className="flex items-start gap-2 mb-1">
                                <span className="font-mono text-xs text-red-300 font-semibold">{r.kind}</span>
                                <span className="font-mono text-xs text-red-200">{r.name}</span>
                              </div>
                              {r.message && (
                                <div className="text-xs text-red-300/90 leading-relaxed break-words pl-0">
                                  {r.message}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                    
                    {/* Successful resources - collapsible table */}
                    {successResources.length > 0 && (
                      <details className="group" open={failedResources.length === 0}>
                        <summary className="px-3 py-2 text-xs font-semibold text-emerald-300 cursor-pointer hover:bg-emerald-500/5 flex items-center gap-2 border-b border-[var(--color-border)]">
                          <CheckCircle2 className="size-4" strokeWidth={2} />
                          Successful Resources ({successResources.length})
                          <span className="ml-auto text-[var(--color-text-muted)] group-open:rotate-90 transition-transform">▶</span>
                        </summary>
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
                            {successResources.map((r, i) => (
                              <tr key={`${r.kind}-${r.name}-${i}`} className="align-top">
                                <td className="px-2 py-1 font-mono text-[var(--color-accent)]">{r.kind}</td>
                                <td className="px-2 py-1 font-mono">{r.name}</td>
                                <td className="px-2 py-1 whitespace-nowrap">
                                  <span className="text-emerald-400">{r.status}</span>
                                </td>
                                <td className="px-2 py-1 text-[var(--color-text-muted)] break-all">{r.message || "—"}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </details>
                    )}
                    
                    {/* Other resources */}
                    {op.resources.length > 0 && failedResources.length === 0 && successResources.length === 0 && (
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
                                <span className="text-[var(--color-text-muted)]">{r.status}</span>
                              </td>
                              <td className="px-2 py-1 text-[var(--color-text-muted)] break-all">{r.message || "—"}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
