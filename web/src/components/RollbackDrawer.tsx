import { useCallback, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Check,
  Clock,
  Copy,
  ExternalLink,
  GitCommit,
  RotateCcw,
  Search,
  User,
} from "lucide-react";
import { api } from "../api/client";
import type { GitCommit as GitCommitType } from "../api/types";

export function RollbackDrawer({
  appName,
  open,
  onClose,
}: {
  appName: string;
  open: boolean;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [selected, setSelected] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [filter, setFilter] = useState("");

  const { data: revisions, isLoading: revLoading, error: revError } = useQuery({
    queryKey: ["app-revisions", appName],
    queryFn: () => api.appRevisions(appName),
    enabled: open && !!appName,
  });

  const { data: statusData } = useQuery({
    queryKey: ["app", appName],
    queryFn: () => api.getApp(appName),
    enabled: open && !!appName,
  });

  const currentRev = (statusData as { status?: { observedRevision?: string } })?.status?.observedRevision ?? "";

  const { data: diffData, isLoading: diffLoading } = useQuery({
    queryKey: ["rollback-diff", appName, currentRev, selected],
    queryFn: () => api.appRevisionDiff(appName, selected!, currentRev),
    enabled: open && !!selected && selected !== currentRev && previewing,
  });

  const filtered = useMemo(() => {
    const list = revisions?.commits ?? [];
    if (!filter.trim()) return list;
    const needle = filter.toLowerCase();
    return list.filter(
      (c) =>
        c.message.toLowerCase().includes(needle) ||
        c.author.toLowerCase().includes(needle) ||
        c.shortSha.toLowerCase().includes(needle),
    );
  }, [revisions?.commits, filter]);

  const rollbackMut = useMutation({
    mutationFn: (revision: string) => api.appRollback(appName, revision),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["app", appName] });
      qc.invalidateQueries({ queryKey: ["app-history", appName] });
      setSelected(null);
      setPreviewing(false);
      onClose();
    },
  });

  const handleCopy = useCallback((sha: string) => {
    void navigator.clipboard.writeText(sha);
  }, []);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end pointer-events-none">
      <button
        type="button"
        className="absolute inset-0 bg-black/50 pointer-events-auto transition-opacity"
        aria-label="Close rollback"
        onClick={onClose}
      />
      <aside className="relative pointer-events-auto h-full w-full max-w-2xl border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col">
        {/* Header */}
        <div className="shrink-0 px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text)] flex items-center gap-2">
              <RotateCcw className="size-4" strokeWidth={2} />
              Rollback
            </h2>
            <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
              Select a previous revision to revert to
              {currentRev && (
                <span className="ml-1 font-mono text-[var(--color-accent)]">@{currentRev.slice(0, 7)}</span>
              )}
            </p>
          </div>
          <button
            type="button"
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1 text-xs text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
            onClick={onClose}
          >
            Close
          </button>
        </div>

        {/* Search */}
        <div className="shrink-0 px-3 py-2 border-b border-[var(--color-border)]">
          <div className="relative">
            <Search className="size-3.5 absolute left-2 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)] pointer-events-none" />
            <input
              type="search"
              placeholder="Filter by message, author, or SHA…"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] pl-7 pr-2 py-1.5 text-xs text-[var(--color-text)] placeholder:text-[var(--color-text-muted)]"
            />
          </div>
        </div>

        <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
          {/* Commit list */}
          <div className="flex-1 min-h-0 overflow-y-auto p-3">
            {revLoading && <div className="text-sm text-[var(--color-text-muted)] animate-pulse">Loading revisions…</div>}
            {revError && <div className="text-sm text-red-400">{(revError as Error).message}</div>}
            {!revLoading && !revError && !filtered.length && (
              <div className="text-sm text-[var(--color-text-muted)]">
                {filter ? "No revisions match this filter." : "No revisions found."}
              </div>
            )}
            <div className="space-y-1.5">
              {filtered.map((commit) => {
                const isCurrent = commit.sha === currentRev;
                const isSelected = commit.sha === selected;
                return (
                  <button
                    key={commit.sha}
                    type="button"
                    onClick={() => {
                      setSelected(commit.sha);
                      setPreviewing(false);
                    }}
                    className={`w-full text-left rounded-lg border px-3 py-2.5 transition-all duration-150 hover:shadow-md active:scale-[0.995] ${
                      isSelected
                        ? "border-[var(--color-accent)] bg-[var(--color-accent-muted)] ring-1 ring-[var(--color-accent)]/30"
                        : isCurrent
                          ? "border-emerald-500/40 bg-emerald-500/5"
                          : "border-[var(--color-border)] bg-[var(--color-surface-muted)] hover:border-[var(--color-border-strong)]"
                    }`}
                  >
                    <div className="flex items-start gap-2.5">
                      {/* Selection indicator */}
                      <div className="mt-0.5 shrink-0">
                        {isSelected ? (
                          <div className="size-4 rounded-full bg-[var(--color-accent)] flex items-center justify-center">
                            <Check className="size-3 text-[#0a0e14]" strokeWidth={3} />
                          </div>
                        ) : isCurrent ? (
                          <div className="size-4 rounded-full border-2 border-emerald-400" />
                        ) : (
                          <div className="size-4 rounded-full border-2 border-[var(--color-border)]" />
                        )}
                      </div>

                      {/* Commit info */}
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <GitCommit className="size-3.5 shrink-0 text-[var(--color-text-muted)]" strokeWidth={2} />
                          <span className="font-mono text-xs text-[var(--color-accent)]">{commit.shortSha}</span>
                          <button
                            type="button"
                            className="p-0.5 rounded hover:bg-[var(--color-surface)] transition-colors"
                            title="Copy SHA"
                            onClick={(e) => { e.stopPropagation(); handleCopy(commit.sha); }}
                          >
                            <Copy className="size-3 text-[var(--color-text-muted)]" />
                          </button>
                          {isCurrent && (
                            <span className="text-[10px] font-semibold uppercase tracking-wide text-emerald-300 bg-emerald-500/20 px-1.5 py-0.5 rounded">
                              current
                            </span>
                          )}
                        </div>
                        <div className="text-xs text-[var(--color-text)] mb-1 truncate">{commit.message}</div>
                        <div className="flex items-center gap-3 text-[10px] text-[var(--color-text-muted)]">
                          <span className="flex items-center gap-1">
                            <User className="size-3" />
                            {commit.author}
                          </span>
                          <span className="flex items-center gap-1">
                            <Clock className="size-3" />
                            {new Date(commit.authorDate).toLocaleString()}
                          </span>
                        </div>
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Bottom action bar */}
          {selected && (
            <div className="shrink-0 border-t border-[var(--color-border)] bg-[var(--color-surface-muted)]">
              {/* Diff preview */}
              {previewing && (
                <div className="border-b border-[var(--color-border)]">
                  <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
                    <span className="text-xs font-semibold text-[var(--color-text-muted)] uppercase">
                      Diff preview
                    </span>
                    <button
                      type="button"
                      className="ml-auto text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline"
                      onClick={() => setPreviewing(false)}
                    >
                      Close
                    </button>
                  </div>
                  <div className="max-h-48 overflow-y-auto p-3">
                    {diffLoading && <div className="text-xs text-[var(--color-text-muted)] animate-pulse">Loading diff…</div>}
                    {diffData?.diff ? (
                      <pre className="text-[10px] font-mono text-[var(--color-text)] whitespace-pre-wrap break-all">
                        {diffData.diff.slice(0, 3000)}
                        {diffData.diff.length > 3000 && (
                          <span className="text-[var(--color-text-muted)]">… (truncated)</span>
                        )}
                      </pre>
                    ) : !diffLoading ? (
                      <div className="text-xs text-[var(--color-text-muted)]">No changes between revisions.</div>
                    ) : null}
                  </div>
                </div>
              )}

              {/* Actions */}
              <div className="px-3 py-2.5 flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-all duration-150 hover:scale-[1.02] active:scale-[0.98] ${
                    previewing
                      ? "border border-[var(--color-border)] bg-[var(--color-surface)] text-[var(--color-accent)]"
                      : "border border-[var(--color-border)] bg-[var(--color-input-bg)] text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                  }`}
                  onClick={() => {
                    if (selected !== currentRev) setPreviewing(!previewing);
                  }}
                  disabled={selected === currentRev}
                >
                  {previewing ? (
                    <ArrowLeft className="size-3.5" />
                  ) : (
                    <ExternalLink className="size-3.5" />
                  )}
                  {previewing ? "Hide diff" : "Preview diff"}
                </button>

                <div className="flex-1" />

                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-semibold text-[#0a0e14] hover:brightness-110 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                  onClick={() => rollbackMut.mutate(selected)}
                  disabled={rollbackMut.isPending || selected === currentRev}
                >
                  <RotateCcw className={`size-3.5 ${rollbackMut.isPending ? "animate-spin" : ""}`} />
                  {rollbackMut.isPending ? "Rolling back…" : `Rollback to ${selected.slice(0, 7)}`}
                </button>
              </div>
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
