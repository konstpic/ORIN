import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, XCircle, FileQuestion } from "lucide-react";
import { api } from "../api/client";

export function OutOfSyncDrawer({
  appName,
  open,
  onClose,
}: {
  appName: string;
  open: boolean;
  onClose: () => void;
}) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["app-diff", appName],
    queryFn: () => api.appDiff(appName),
    enabled: open && !!appName,
    refetchInterval: open ? 10000 : false,
  });

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end pointer-events-none">
      <button
        type="button"
        className="absolute inset-0 bg-black/50 pointer-events-auto"
        aria-label="Close diff details"
        onClick={onClose}
      />
      <aside className="relative pointer-events-auto h-full w-full max-w-3xl border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col">
        <div className="shrink-0 px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text)]">
              Out Of Sync Resources
            </h2>
            <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
              Resources that differ between Git and cluster
            </p>
          </div>
          <button
            type="button"
            className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline"
            onClick={onClose}
          >
            Close
          </button>
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto p-4">
          {isLoading && <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>}
          {error && <div className="text-sm text-red-400">{(error as Error).message}</div>}

          {!isLoading && !error && data && (
            <>
              {/* Summary */}
              <div className="mb-4 grid grid-cols-2 gap-3">
                <div className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2">
                  <div className="flex items-center gap-2 mb-1">
                    <XCircle className="size-4 text-red-400" strokeWidth={2} />
                    <span className="text-xs font-semibold text-red-200">Out Of Sync</span>
                  </div>
                  <div className="text-2xl font-bold text-red-300">{data.outOfSync}</div>
                </div>
                <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2">
                  <div className="flex items-center gap-2 mb-1">
                    <CheckCircle2 className="size-4 text-emerald-400" strokeWidth={2} />
                    <span className="text-xs font-semibold text-emerald-200">Synced</span>
                  </div>
                  <div className="text-2xl font-bold text-emerald-300">{data.synced}</div>
                </div>
              </div>

              {/* Out of sync resources */}
              {data.resources.filter((r) => r.sync === "OutOfSync").length > 0 && (
                <div className="space-y-3 mb-6">
                  <h3 className="text-xs font-semibold uppercase tracking-wide text-red-300 flex items-center gap-2">
                    <AlertTriangle className="size-4" strokeWidth={2} />
                    Resources with differences
                  </h3>
                  {data.resources
                    .filter((r) => r.sync === "OutOfSync")
                    .map((r, i) => (
                      <div
                        key={i}
                        className="rounded-lg border border-red-500/40 bg-red-500/5 overflow-hidden"
                      >
                        <div className="px-3 py-2 bg-red-500/10 border-b border-red-500/30 flex items-center gap-2">
                          <span className="font-mono text-xs text-red-300 font-semibold">
                            {r.kind}
                          </span>
                          <span className="text-xs text-red-200">/</span>
                          <span className="font-mono text-xs text-red-200">{r.name}</span>
                          {r.namespace && (
                            <>
                              <span className="text-xs text-red-300/60">in</span>
                              <span className="font-mono text-xs text-red-300/80">
                                {r.namespace}
                              </span>
                            </>
                          )}
                        </div>
                        {r.normalizedDiff && (
                          <div className="p-3">
                            <pre className="text-xs font-mono text-[var(--color-text)] bg-[var(--color-surface-muted)] rounded p-2 overflow-x-auto whitespace-pre-wrap break-all max-h-[300px] overflow-y-auto">
                              {r.normalizedDiff}
                            </pre>
                          </div>
                        )}
                      </div>
                    ))}
                </div>
              )}

              {/* Synced resources (collapsed) */}
              {data.resources.filter((r) => r.sync === "Synced").length > 0 && (
                <details className="group">
                  <summary className="cursor-pointer text-xs font-semibold uppercase tracking-wide text-emerald-300 flex items-center gap-2 mb-3 hover:text-emerald-200">
                    <CheckCircle2 className="size-4" strokeWidth={2} />
                    Synced resources ({data.resources.filter((r) => r.sync === "Synced").length})
                    <span className="ml-auto text-[var(--color-text-muted)] group-open:rotate-90 transition-transform">
                      ▶
                    </span>
                  </summary>
                  <div className="space-y-2 mb-4">
                    {data.resources
                      .filter((r) => r.sync === "Synced")
                      .map((r, i) => (
                        <div
                          key={i}
                          className="rounded border border-emerald-500/30 bg-emerald-500/5 px-3 py-2 flex items-center gap-2"
                        >
                          <CheckCircle2 className="size-3.5 text-emerald-400 shrink-0" strokeWidth={2} />
                          <span className="font-mono text-xs text-emerald-300 font-semibold">
                            {r.kind}
                          </span>
                          <span className="text-xs text-emerald-200">/</span>
                          <span className="font-mono text-xs text-emerald-200">{r.name}</span>
                          {r.namespace && (
                            <>
                              <span className="text-xs text-emerald-300/60">in</span>
                              <span className="font-mono text-xs text-emerald-300/80">
                                {r.namespace}
                              </span>
                            </>
                          )}
                        </div>
                      ))}
                  </div>
                </details>
              )}

              {/* Help section */}
              {data.outOfSync > 0 && (
                <div className="mt-6 rounded-lg border border-amber-500/40 bg-amber-500/5 p-4">
                  <div className="flex items-start gap-3">
                    <FileQuestion className="size-5 shrink-0 text-amber-400 mt-0.5" strokeWidth={2} />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-semibold text-amber-200 mb-2">
                        Common causes of persistent OutOfSync
                      </div>
                      <ul className="text-xs text-amber-300/90 space-y-1 list-disc list-inside">
                        <li>HPA managing replica counts</li>
                        <li>Cluster-assigned Service IPs or PVC volume names</li>
                        <li>Annotations added by controllers or webhooks</li>
                        <li>CRD default values applied server-side</li>
                      </ul>
                      <div className="mt-3 text-xs text-amber-300/80">
                        Use <code className="px-1 py-0.5 bg-black/20 rounded font-mono">ignoreDifferences</code> in your Application's syncPolicy to suppress false positives.
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </aside>
    </div>
  );
}
