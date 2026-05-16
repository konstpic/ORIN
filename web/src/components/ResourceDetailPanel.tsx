import { useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DiffEditor, Editor } from "@monaco-editor/react";
import { ExternalLink, FileDiff, ScrollText, Copy, FileCode2, Save, RotateCcw } from "lucide-react";
import { api } from "../api/client";
import type { Application, PodEvent, ResourceDiff, ResourceNode } from "../api/types";
import { HealthBadge, SyncBadge } from "./Badges";
import { iconForKind, kindIconTileClass } from "../k8s/kindMeta";
import { stripManagedFieldsYaml } from "../utils/yamlManagedFields";
import { podTileChar, podTileClass, podTileTitle } from "../k8s/podTile";

function resourceEventRowClass(ev: PodEvent): string {
  if (ev.type === "Warning") return "bg-amber-500/5 hover:bg-amber-500/10";
  return "hover:bg-[var(--color-accent-muted)]/15";
}

function getResourceCategoryBadgeColor(category?: string): string {
  if (!category) return "bg-gray-500/20 text-gray-300";
  if (category.includes("Crash") || category.includes("Failed") || category.includes("Error"))
    return "bg-red-500/20 text-red-300";
  if (category.includes("Probe"))
    return "bg-orange-500/20 text-orange-300";
  if (category.includes("ImagePull"))
    return "bg-blue-500/20 text-blue-300";
  if (category.includes("Success") || category.includes("Pulled") || category.includes("Started") || category.includes("Mount"))
    return "bg-green-500/20 text-green-300";
  if (category.includes("Starting") || category.includes("Created"))
    return "bg-cyan-500/20 text-cyan-300";
  if (category.includes("Stopping"))
    return "bg-purple-500/20 text-purple-300";
  return "bg-gray-500/20 text-gray-300";
}

function formatResourceEventTime(t: string | null | undefined): string {
  if (!t) return "—";
  try {
    return new Date(t).toLocaleString(undefined, { dateStyle: "short", timeStyle: "medium" });
  } catch {
    return t;
  }
}

function deriveEventCategory(reason: string): string {
  if (reason.includes("BackOff") || reason.includes("CrashLoop")) return "ContainerCrash";
  if (reason.includes("Pulling") || reason.includes("Pulled")) return "ImagePull";
  if (reason.includes("Created") || reason.includes("Started")) return "ContainerStarting";
  if (reason.includes("Failed")) return "OperationFailed";
  if (reason.includes("Killing") || reason.includes("Stopping")) return "ContainerStopping";
  if (reason.includes("Scaling") || reason.includes("Successful")) return "Scaling";
  return reason;
}

type TabId = "summary" | "manifest" | "diff" | "events" | "logs";

const TAB_LABEL: Record<TabId, string> = {
  summary: "SUMMARY",
  manifest: "MANIFEST",
  diff: "DIFF",
  events: "EVENTS",
  logs: "LOGS",
};

function findResourceDiff(node: ResourceNode, resources: ResourceDiff[] | undefined) {
  if (!resources) return undefined;
  const ns = node.namespace ?? "";
  return resources.find((d) => d.kind === node.kind && d.name === node.name && (d.namespace ?? "") === ns);
}

function CopyInline({ text }: { text: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className="inline-flex items-center justify-center p-1 rounded-md border border-transparent hover:border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-accent)]"
      title="Copy"
      onClick={() => {
        void navigator.clipboard.writeText(text).then(() => {
          setDone(true);
          setTimeout(() => setDone(false), 1200);
        });
      }}
    >
      <Copy className="size-3.5" strokeWidth={2} />
      {done && <span className="sr-only">Copied</span>}
    </button>
  );
}

export function ResourceDetailPanel({
  appName,
  node,
  app,
  onClose,
  onOpenPod,
  onSelectMember,
  onExpandCompactPods,
  onExpandKindGroup,
}: {
  appName: string;
  node: ResourceNode;
  app?: Application;
  onClose: () => void;
  onOpenPod?: (n: ResourceNode) => void;
  onSelectMember?: (n: ResourceNode) => void;
  /** Expand compacted pods as separate nodes on the topology map. */
  onExpandCompactPods?: (parentUid: string) => void;
  /** Expand a kind-group node into individual resource nodes on the map. */
  onExpandKindGroup?: (groupUid: string) => void;
}) {
  const [tab, setTab] = useState<TabId>("summary");
  const [hideManaged, setHideManaged] = useState(true);
  const [inlineDiff, setInlineDiff] = useState(false);
  const [editedYaml, setEditedYaml] = useState<string | null>(null);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [eventFilter, setEventFilter] = useState<"all" | "warning">("all");
  const qc = useQueryClient();

  const isSyntheticApp = node.kind === "Application" && node.uid.startsWith("synthetic:app:");
  const isKindGroup = !!node.isKindGroup;
  const isPod = node.kind === "Pod";
  const hasEvents = isPod || node.kind === "ReplicaSet" || node.kind === "Deployment" || node.kind === "StatefulSet" || node.kind === "DaemonSet";

  const { data: podEvents, refetch: refetchPodEvents } = useQuery({
    queryKey: ["pod-events", appName, node.name],
    queryFn: () => api.getPodEvents(appName, node.name),
    enabled: isPod && tab === "events",
    refetchInterval: isPod && tab === "events" ? 5000 : false,
    retry: 0,
  });

  const { data: resourceEvents, refetch: refetchResourceEvents } = useQuery({
    queryKey: ["resource-events", appName, node.kind, node.name, node.namespace],
    queryFn: () => api.getResourceEvents(appName, node.kind, node.name, node.namespace),
    enabled: !isPod && hasEvents && tab === "events",
    refetchInterval: !isPod && hasEvents && tab === "events" ? 5000 : false,
    retry: 0,
  });

  const rawEvents = isPod ? podEvents : resourceEvents;
  const eventsLoading = isPod
    ? podEvents === undefined
    : resourceEvents === undefined;
  const refetchEventsFn = isPod ? refetchPodEvents : refetchResourceEvents;
  const filteredEvents = useMemo(() => {
    const list = rawEvents ?? [];
    if (eventFilter === "warning") return list.filter((e) => e.type === "Warning");
    return list;
  }, [rawEvents, eventFilter]);

  const { data: diffData } = useQuery({
    queryKey: ["app-diff", appName],
    queryFn: () => api.appDiff(appName),
    enabled: !isSyntheticApp && !isKindGroup,
    staleTime: 10_000,
  });

  const diffEntry = useMemo(
    () => (!isSyntheticApp && !isKindGroup && diffData ? findResourceDiff(node, diffData.resources) : undefined),
    [node, diffData, isSyntheticApp, isKindGroup],
  );

  const liveYaml = diffEntry?.liveYaml ?? "";
  const desiredYaml = diffEntry?.desiredYaml ?? "";
  const displayLive = hideManaged ? stripManagedFieldsYaml(liveYaml || "# (empty)") : liveYaml || "# (empty)";
  const displayDesired = hideManaged ? stripManagedFieldsYaml(desiredYaml || "# (empty)") : desiredYaml || "# (empty)";

  const Icon = iconForKind(node.kind);

  const tabIds: TabId[] = isSyntheticApp
    ? ["summary", "events", "logs"]
    : isKindGroup
      ? ["summary"]
      : ["summary", "manifest", "diff", "events", "logs"];

  const applyMut = useMutation({
    mutationFn: (yaml: string) => api.applyLiveResource(appName, yaml),
    onSuccess: () => {
      setEditedYaml(null);
      setApplyError(null);
      qc.invalidateQueries({ queryKey: ["app-diff", appName] });
      qc.invalidateQueries({ queryKey: ["app", appName] });
    },
    onError: (e) => setApplyError((e as Error).message),
  });

  useEffect(() => {
    setTab("summary");
    setEditedYaml(null);
    setApplyError(null);
  }, [node.uid]);

  return (
    <aside className="w-full h-full flex flex-col border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl overflow-hidden">
      <div className="shrink-0 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] px-4 py-3 flex items-start justify-between gap-3">
        <div className="flex gap-3 min-w-0">
          <span
            className={`inline-flex shrink-0 items-center justify-center rounded-xl size-11 [&_svg]:size-5 ${kindIconTileClass(node.kind)}`}
          >
            <Icon strokeWidth={1.65} />
          </span>
          <div className="min-w-0">
            <div className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] font-semibold">
              {isKindGroup ? `${node.kind} group` : node.kind}
            </div>
            {!isSyntheticApp && !isKindGroup && (
              <div className="text-sm font-semibold text-[var(--color-text)] truncate">{node.name}</div>
            )}
            {isKindGroup && (
              <div className="text-sm font-semibold text-[var(--color-text)] truncate">
                {node.groupedMembers?.length ?? 0} {node.kind}s
              </div>
            )}
            {isSyntheticApp && app && <div className="text-sm font-semibold text-[var(--color-text)] truncate">{app.name}</div>}
          </div>
        </div>
        <button type="button" className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1 text-xs text-[var(--color-text)] hover:border-[var(--color-border-strong)] shrink-0" onClick={onClose}>
          Close
        </button>
      </div>

      <div className="shrink-0 flex border-b border-[var(--color-border)] px-1 pt-1 gap-0.5 overflow-x-auto">
        {tabIds.map((id) => (
          <button
            key={id}
            type="button"
            onClick={() => setTab(id)}
            className={`px-3 py-2 text-xs font-semibold uppercase tracking-wide rounded-t-md border-b-2 -mb-px whitespace-nowrap transition-all duration-150 hover:translate-y-[-1px] ${
              tab === id
                ? "border-[var(--color-accent)] text-[var(--color-accent)] bg-[var(--color-surface)]"
                : "border-transparent text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            }`}
          >
            {TAB_LABEL[id]}
          </button>
        ))}
      </div>

      <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
        {tab === "summary" && (
          <div className="p-4 space-y-4 text-sm overflow-y-auto flex-1 min-h-0">
            {isKindGroup && (
              <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-3 space-y-3">
                <div className="flex items-center justify-between gap-2">
                  <div className="text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide">
                    Members ({node.groupedMembers?.length ?? 0})
                  </div>
                  {onExpandKindGroup && (
                    <button
                      type="button"
                      className="rounded-md border border-[var(--color-border-strong)] bg-[var(--color-accent-muted)] px-2.5 py-1 text-[11px] font-medium text-[var(--color-accent)] hover:brightness-110"
                      onClick={() => onExpandKindGroup(node.uid)}
                    >
                      Expand on map
                    </button>
                  )}
                </div>
                <ul className="space-y-1 max-h-[360px] overflow-y-auto">
                  {node.groupedMembers?.map((m) => (
                    <li key={m.uid}>
                      <button
                        type="button"
                        onClick={() => onSelectMember?.(m)}
                        className="w-full flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-2.5 py-2 text-left text-xs hover:border-[var(--color-border-strong)]"
                      >
                        <span className="font-mono text-[var(--color-text)] truncate flex-1">{m.name}</span>
                        <span className="shrink-0"><HealthBadge status={m.health} /></span>
                        <span className="shrink-0"><SyncBadge status={m.sync} /></span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {!isSyntheticApp && !isKindGroup && (
              <dl className="grid grid-cols-[120px_1fr] gap-x-2 gap-y-2 text-[var(--color-text)]">
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Kind</dt>
                <dd className="font-mono text-xs">{node.kind}</dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Name</dt>
                <dd className="font-mono text-xs break-all flex items-center gap-1">
                  {node.name}
                  <CopyInline text={node.name} />
                </dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Namespace</dt>
                <dd className="font-mono text-xs flex items-center gap-1">
                  {node.namespace || "—"}
                  {node.namespace ? <CopyInline text={node.namespace} /> : null}
                </dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Created at</dt>
                <dd className="text-[var(--color-text-muted)]">—</dd>
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Sync</dt>
                <dd className="flex flex-wrap gap-2 items-center">
                  <SyncBadge status={node.sync} />
                </dd>
                {node.syncMessage ? (
                  <>
                    <dt className="text-[var(--color-text-muted)] text-xs uppercase">Sync detail</dt>
                    <dd className="text-xs text-amber-300/95 break-words">{node.syncMessage}</dd>
                  </>
                ) : null}
                <dt className="text-[var(--color-text-muted)] text-xs uppercase">Health</dt>
                <dd>
                  <HealthBadge status={node.health} />
                </dd>
              </dl>
            )}
            {!isSyntheticApp &&
              (node.kind === "ReplicaSet" || node.kind === "Deployment") &&
              !!node.groupedPods?.length && (
                <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-3 space-y-3">
                  <div className="text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide">
                    Pods ({node.groupedPods.length})
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {node.groupedPods.map((p) => (
                      <button
                        key={p.uid}
                        type="button"
                        title={podTileTitle(p)}
                        onClick={() => onOpenPod?.(p)}
                        className={`inline-flex size-7 shrink-0 items-center justify-center rounded-[4px] border text-[10px] font-bold leading-none hover:brightness-110 ${podTileClass(p)}`}
                      >
                        {podTileChar(p)}
                      </button>
                    ))}
                  </div>
                  {onExpandCompactPods && (
                    <button
                      type="button"
                      className="w-full rounded-md border border-[var(--color-border-strong)] bg-[var(--color-accent-muted)] px-3 py-2 text-xs font-medium text-[var(--color-accent)] hover:brightness-110"
                      onClick={() => onExpandCompactPods(node.uid)}
                    >
                      Show each pod on map
                    </button>
                  )}
                </div>
              )}
            {isSyntheticApp && app && (
              <div className="space-y-2 text-[var(--color-text-muted)]">
                <p className="text-[var(--color-text)] text-sm">
                  GitOps application grouping live resources with label{" "}
                  <code className="text-[var(--color-accent)]">app.kubernetes.io/instance</code>.
                </p>
                <p>
                  <span className="text-[var(--color-text-muted)]">Destination:</span>{" "}
                  <span className="font-mono text-[var(--color-text)]">
                    {app.destination.cluster}/{app.destination.namespace}
                  </span>
                </p>
              </div>
            )}

            <div>
              <div className="text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide mb-2">Links</div>
              <div className="flex flex-wrap gap-2">
                {node.kind === "Pod" ? (
                  <button
                    type="button"
                    className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2.5 py-1.5 text-xs text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
                    onClick={() => onOpenPod?.(node)}
                  >
                    <ScrollText className="size-3.5" />
                    Logs & terminal
                  </button>
                ) : (
                  <>
                    <span className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] px-2.5 py-1.5 text-xs text-[var(--color-text-muted)] cursor-not-allowed" title="Select a pod to stream logs">
                      <ScrollText className="size-3.5" />
                      Logs
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] px-2.5 py-1.5 text-xs text-[var(--color-text-muted)] cursor-not-allowed">
                      Error logs
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-border)] px-2.5 py-1.5 text-xs text-[var(--color-text-muted)] cursor-not-allowed">
                      <ExternalLink className="size-3.5" />
                      Metrics
                    </span>
                  </>
                )}
              </div>
            </div>
          </div>
        )}

        {tab === "manifest" && !isSyntheticApp && (
          <div className="flex flex-col flex-1 min-h-0">
            <div className="shrink-0 px-3 py-2 flex flex-wrap items-center gap-2 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)]">
              <span className="flex items-center gap-1.5 text-xs text-[var(--color-text-muted)] flex-1 min-w-0">
                <FileCode2 className="size-3.5 shrink-0" />
                Live manifest — edit &amp; apply to cluster
              </span>
              {diffEntry?.liveYaml && (
                <div className="flex items-center gap-1.5 shrink-0">
                  {editedYaml !== null && (
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 rounded px-2 py-1 text-[11px] border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                      onClick={() => { setEditedYaml(null); setApplyError(null); }}
                      title="Reset to live"
                    >
                      <RotateCcw className="size-3" />
                      Reset
                    </button>
                  )}
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded px-2 py-1 text-[11px] bg-[var(--color-accent)] text-[#0a0e14] font-semibold hover:brightness-110 disabled:opacity-50"
                    onClick={() => applyMut.mutate(editedYaml ?? displayLive)}
                    disabled={applyMut.isPending}
                    title="Apply to cluster (server-side apply)"
                  >
                    <Save className="size-3" />
                    {applyMut.isPending ? "Applying…" : "Apply to cluster"}
                  </button>
                </div>
              )}
            </div>
            {applyError && (
              <div className="shrink-0 px-3 py-1.5 bg-red-500/10 border-b border-red-500/30 text-[11px] text-red-300 break-words">
                {applyError}
              </div>
            )}
            {applyMut.isSuccess && (
              <div className="shrink-0 px-3 py-1.5 bg-green-500/10 border-b border-green-500/30 text-[11px] text-green-300">
                Applied successfully.
              </div>
            )}
            <div className="flex-1 min-h-0">
              {diffEntry?.liveYaml ? (
                <Editor
                  height="100%"
                  theme="vs-dark"
                  language="yaml"
                  value={editedYaml ?? displayLive}
                  onChange={(v) => { setEditedYaml(v ?? ""); setApplyError(null); }}
                  options={{ minimap: { enabled: false }, wordWrap: "on", scrollBeyondLastLine: false }}
                />
              ) : (
                <div className="p-4 text-xs text-[var(--color-text-muted)]">
                  No live manifest available for this object (not yet deployed or diff not ready).
                </div>
              )}
            </div>
          </div>
        )}

        {tab === "diff" && !isSyntheticApp && (
          <div className="flex flex-col flex-1 min-h-0">
            <div className="shrink-0 px-3 py-2 flex flex-wrap items-center gap-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)]">
              <span className="text-xs font-semibold text-[var(--color-text-muted)] uppercase flex items-center gap-1">
                <FileDiff className="size-3.5" /> Live vs desired
              </span>
              <label className="inline-flex items-center gap-1.5 text-xs text-[var(--color-text)] cursor-pointer">
                <input type="checkbox" checked={hideManaged} onChange={(e) => setHideManaged(e.target.checked)} />
                Hide managed fields
              </label>
              <label className="inline-flex items-center gap-1.5 text-xs text-[var(--color-text)] cursor-pointer">
                <input type="checkbox" checked={inlineDiff} onChange={(e) => setInlineDiff(e.target.checked)} />
                Inline (compact) diff
              </label>
            </div>
            <div className="flex-1 min-h-0">
              {diffEntry ? (
                <DiffEditor
                  height="100%"
                  theme="vs-dark"
                  language="yaml"
                  original={displayLive}
                  modified={displayDesired}
                  options={{
                    readOnly: true,
                    renderSideBySide: !inlineDiff,
                    minimap: { enabled: false },
                  }}
                />
              ) : (
                <div className="p-4 text-xs text-[var(--color-text-muted)]">
                  No diff entry for this object (not in desired set or not yet rendered).
                </div>
              )}
            </div>
          </div>
        )}

        {tab === "events" && (
          <div className="flex-1 flex flex-col min-h-[300px] p-3 gap-2">
            {hasEvents ? (
              <>
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-2.5 py-1.5 text-xs font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
                    onClick={() => void refetchEventsFn()}
                    disabled={eventsLoading}
                  >
                    <RotateCcw className="size-3.5" /> Refresh
                  </button>
                  <div className="inline-flex rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-0.5">
                    {(["all", "warning"] as const).map((opt) => (
                      <button
                        key={opt}
                        type="button"
                        className={`rounded-sm px-2 py-1 text-xs font-medium capitalize ${
                          eventFilter === opt
                            ? "bg-[var(--color-surface)] text-[var(--color-accent)] border border-[var(--color-border)]"
                            : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                        }`}
                        onClick={() => setEventFilter(opt)}
                      >
                        {opt}
                      </button>
                    ))}
                  </div>
                  <span className="text-[10px] text-[var(--color-text-muted)] ml-auto">
                    Auto-refresh every 5 s
                  </span>
                </div>
                <div className="flex-1 overflow-auto rounded-md border border-[var(--color-border)] bg-[var(--color-surface-muted)]">
                  {!filteredEvents.length && !eventsLoading && (
                    <div className="p-4 text-xs text-[var(--color-text-muted)]">
                      {eventFilter === "warning" ? "No warnings." : "No events found."}
                    </div>
                  )}
                  {!!filteredEvents.length && (
                    <table className="w-full text-left text-xs">
                      <thead className="sticky top-0 bg-[var(--color-surface)] border-b border-[var(--color-border)] z-10">
                        <tr className="text-[var(--color-text-muted)] uppercase tracking-wide">
                          <th className="px-2 py-2 w-16">Type</th>
                          <th className="px-2 py-2 w-28">Category</th>
                          <th className="px-2 py-2 w-28">Reason</th>
                          <th className="px-2 py-2">Message</th>
                          <th className="px-2 py-2 w-40 hidden md:table-cell">Last seen</th>
                          <th className="px-2 py-2 w-10 text-right">#</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[var(--color-border)] text-[var(--color-text)]">
                        {filteredEvents.map((ev, i) => (
                          <tr key={`${ev.reason}-${i}`} className={`align-top ${resourceEventRowClass(ev)}`}>
                            <td className={`px-2 py-1.5 whitespace-nowrap font-mono ${ev.type === "Warning" ? "text-amber-300" : ""}`}>{ev.type}</td>
                            <td className="px-2 py-1.5 whitespace-nowrap">
                              <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-semibold ${getResourceCategoryBadgeColor(ev.category ?? deriveEventCategory(ev.reason))}`}>
                                {ev.category ?? deriveEventCategory(ev.reason)}
                              </span>
                            </td>
                            <td className="px-2 py-1.5 whitespace-nowrap text-[var(--color-text-muted)]">{ev.reason}</td>
                            <td className="px-2 py-1.5 break-words">{ev.message}</td>
                            <td className="px-2 py-1.5 text-[var(--color-text-muted)] hidden md:table-cell whitespace-nowrap">
                              {formatResourceEventTime((ev as any).lastTime ?? ev.firstTime)}
                            </td>
                            <td className="px-2 py-1.5 text-right text-[var(--color-text-muted)]">{ev.count}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              </>
            ) : (
              <div className="text-sm text-[var(--color-text-muted)]">
                Events are surfaced for <strong>Pod</strong> resources. For other kinds, use <code>kubectl describe</code> or inspect the owning pods.
              </div>
            )}
          </div>
        )}

        {tab === "logs" && (
          <div className="p-4 text-sm text-[var(--color-text-muted)] overflow-y-auto flex-1 min-h-0">
            {node.kind === "Pod" ? (
              <button type="button" className="text-[var(--color-accent)] underline" onClick={() => onOpenPod?.(node)}>
                Open logs & terminal
              </button>
            ) : (
              <>Logs are available for <strong className="text-[var(--color-text)]">Pod</strong> resources only.</>
            )}
          </div>
        )}
      </div>
    </aside>
  );
}
