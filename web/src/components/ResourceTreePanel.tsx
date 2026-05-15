import { useCallback, useMemo, useState, type MouseEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Layers, Boxes } from "lucide-react";
import { ResourceTreeView } from "./ResourceTreeView";
import { ResourceTopologyView } from "./ResourceTopologyView";
import { PodDrawer } from "./PodDrawer";
import { ResourceDetailPanel } from "./ResourceDetailPanel";
import { ResourceContextMenu, type ContextMenuState, type ResourceAction } from "./ResourceContextMenu";
import { ConfirmDialog } from "./ConfirmDialog";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";
import type { Application, ResourceNode, ResourceTree } from "../api/types";
import { prepareListRoots, prepareTopologyRoots } from "../k8s/topologyTransform";
import { filterResourceForest } from "../k8s/treeFilter";

type PendingAction = {
  action: ResourceAction;
  node: ContextMenuState["node"];
};

export function ResourceTreePanel({ name, app }: { name: string; app: Application }) {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [mode, setMode] = useState<"list" | "topology">("topology");
  const [compactTopology, setCompactTopology] = useState(true);
  const [groupOtherKinds, setGroupOtherKinds] = useState(true);
  const [expandedReplicaSetUids, setExpandedReplicaSetUids] = useState<Set<string>>(() => new Set());
  const [expandedGroupUids, setExpandedGroupUids] = useState<Set<string>>(() => new Set());
  const [selected, setSelected] = useState<ResourceNode | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const [resourceFilter, setResourceFilter] = useState("");

  const deleteMut = useMutation({
    mutationFn: (node: ContextMenuState["node"]) =>
      node.kind === "Pod"
        ? api.deletePod(name, node.name)
        : api.deleteLiveResource(name, node),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["app-tree", name] });
      if (selected?.name === pendingAction?.node.name) setSelected(null);
    },
    onError: (err: Error) => setActionError(err.message),
  });

  const syncMut = useMutation({
    mutationFn: (node: ContextMenuState["node"]) => api.syncLiveResource(name, node),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["app-tree", name] }),
    onError: (err: Error) => setActionError(err.message),
  });

  const handleAction = useCallback(
    (action: ResourceAction, node: ContextMenuState["node"]) => {
      if (action === "sync") {
        syncMut.mutate(node);
      } else {
        // delete and restart both need confirmation
        setPendingAction({ action, node });
      }
    },
    [syncMut],
  );

  const confirmAction = useCallback(() => {
    if (!pendingAction) return;
    deleteMut.mutate(pendingAction.node);
    setPendingAction(null);
  }, [pendingAction, deleteMut]);

  const onContextMenu = useCallback((e: MouseEvent, node: ResourceNode) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({
      x: e.clientX,
      y: e.clientY,
      node: {
        kind: node.kind,
        group: node.group,
        version: node.version,
        namespace: node.namespace,
        name: node.name,
        uid: node.uid,
      },
    });
  }, []);

  const { data, isLoading, error } = useQuery({
    queryKey: ["app-tree", name],
    queryFn: () => api.appTree(name),
    refetchInterval: (q) => {
      const tree = q.state.data as ResourceTree | undefined;
      return tree?.activeSync ? 2000 : 7000;
    },
  });

  const onSelect = useCallback((n: ResourceNode) => {
    setSelected(n);
  }, []);

  const filteredNodes = useMemo(
    () => (data?.nodes?.length ? filterResourceForest(data.nodes, resourceFilter) : []),
    [data?.nodes, resourceFilter],
  );

  const topologyRoots = useMemo(
    () =>
      filteredNodes.length
        ? prepareTopologyRoots(filteredNodes, {
            appName: app.name,
            appHealth: app.status.health,
            appSync: app.status.sync,
            compactPods: compactTopology,
            groupOtherKinds,
            expandedReplicaSetUids,
            expandedGroupUids,
          })
        : [],
    [
      filteredNodes,
      app.name,
      app.status.health,
      app.status.sync,
      compactTopology,
      groupOtherKinds,
      expandedReplicaSetUids,
      expandedGroupUids,
    ],
  );

  const listRoots = useMemo(
    () =>
      filteredNodes.length
        ? prepareListRoots(filteredNodes, {
            appName: app.name,
            appHealth: app.status.health,
            appSync: app.status.sync,
          })
        : [],
    [filteredNodes, app.name, app.status.health, app.status.sync],
  );

  const expandReplicaSetOnMap = useCallback((rsUid: string | undefined) => {
    if (!rsUid) return;
    setExpandedReplicaSetUids((prev) => {
      const next = new Set(prev);
      next.add(rsUid);
      return next;
    });
    setMode("topology");
  }, []);

  const expandKindGroupOnMap = useCallback((groupUid: string | undefined) => {
    if (!groupUid) return;
    setExpandedGroupUids((prev) => {
      const next = new Set(prev);
      next.add(groupUid);
      return next;
    });
    setMode("topology");
  }, []);

  if (isLoading) return <div className="text-sm text-[var(--color-text-muted)]">Loading resources…</div>;
  if (error) return <div className="text-sm text-red-400">{(error as Error).message}</div>;
  if (!data?.nodes?.length) {
    return <div className="text-sm text-[var(--color-text-muted)]">No resources yet — try syncing.</div>;
  }

  return (
    <div className="relative w-full flex-1 flex flex-col min-h-0 gap-3">
      {data?.activeSync ? (
        <div
          className="shrink-0 rounded-lg border border-cyan-500/25 bg-cyan-500/10 px-3 py-2 text-xs text-[var(--color-text)]"
          role="status"
        >
          <span className="font-semibold text-cyan-200">{data.activeSync.status}</span>
          {data.activeSync.message ? (
            <span className="ml-2 text-[var(--color-text-muted)]">{data.activeSync.message}</span>
          ) : null}
          {data.activeSync.resources?.length ? (
            <span className="ml-2 text-[var(--color-text-muted)]">
              ({data.activeSync.resources.length} resource result{data.activeSync.resources.length === 1 ? "" : "s"})
            </span>
          ) : null}
        </div>
      ) : null}

      <div className="mb-0 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between shrink-0">
        <div>
          <h2 className="text-base font-semibold text-[var(--color-text)]">Resources</h2>
          <p className="text-xs text-[var(--color-text-muted)]">
            {mode === "topology"
              ? "Pan, zoom, click a node. Group pods folds replicas into their controller; Group kinds collapses ConfigMap/Secret/Service clusters."
              : "List view — hierarchical live objects."}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2 justify-end">
          {mode === "topology" && (
            <div className="inline-flex shrink-0 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-0.5">
              <button
                type="button"
                title={compactTopology ? "Pods are grouped under their controller (ReplicaSet / Deployment)" : "Show every pod as a separate map node"}
                className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors ${
                  compactTopology
                    ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                    : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                }`}
                onClick={() => {
                  setCompactTopology((c) => {
                    if (c) setExpandedReplicaSetUids(new Set());
                    return !c;
                  });
                }}
              >
                <Boxes className="size-3.5 shrink-0" strokeWidth={2} />
                Group pods
              </button>
              <button
                type="button"
                title={groupOtherKinds ? "ConfigMap/Secret/Service/etc. clusters get a single group tile" : "Show every resource individually"}
                className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors ${
                  groupOtherKinds
                    ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                    : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
                }`}
                onClick={() => {
                  setGroupOtherKinds((g) => {
                    if (g) setExpandedGroupUids(new Set());
                    return !g;
                  });
                }}
              >
                <Layers className="size-3.5 shrink-0" strokeWidth={2} />
                Group kinds
              </button>
            </div>
          )}
          <div className="inline-flex shrink-0 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-0.5">
            <button
              type="button"
              className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                mode === "topology"
                  ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                  : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              }`}
              onClick={() => setMode("topology")}
            >
              Tree
            </button>
            <button
              type="button"
              className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                mode === "list"
                  ? "bg-[var(--color-surface)] text-[var(--color-accent)] shadow-sm border border-[var(--color-border)]"
                  : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              }`}
              onClick={() => setMode("list")}
            >
              List
            </button>
          </div>
        </div>
      </div>

      <div className="flex flex-1 min-h-0 flex-col lg:flex-row gap-4 w-full">
        <aside className="w-full lg:w-56 shrink-0 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-muted)] p-3 space-y-3 self-start lg:max-h-[min(100%,480px)] lg:overflow-y-auto">
          <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">Filters</div>
          <div>
            <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Name</label>
            <input
              type="search"
              placeholder="Filter by name…"
              value={resourceFilter}
              onChange={(e) => setResourceFilter(e.target.value)}
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)]"
            />
          </div>
          {resourceFilter.trim() && !filteredNodes.length && (
            <p className="text-xs text-amber-400/90">No resources match this filter.</p>
          )}
        </aside>
        <div className="flex-1 min-h-0 min-w-0 flex flex-col">
          {mode === "topology" ? (
            <ResourceTopologyView
              roots={topologyRoots}
              onNodeSelect={onSelect}
              onNodeContextMenu={onContextMenu}
              onNavigateToApp={(appName) => navigate(`/applications/${encodeURIComponent(appName)}`)}
            />
          ) : (
            <div className="flex-1 min-h-0 overflow-y-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]">
              <ResourceTreeView roots={listRoots} onNodeSelect={onSelect} onNodeContextMenu={onContextMenu} />
            </div>
          )}
        </div>
      </div>

      {contextMenu && (
        <ResourceContextMenu
          state={contextMenu}
          onAction={handleAction}
          onClose={() => setContextMenu(null)}
        />
      )}

      {pendingAction && (
        <ConfirmDialog
          title={
            pendingAction.action === "restart"
              ? `Restart pod "${pendingAction.node.name}"?`
              : `Delete ${pendingAction.node.kind} "${pendingAction.node.name}"?`
          }
          description={
            pendingAction.action === "restart"
              ? "The pod will be deleted and Kubernetes will restart it via its controller."
              : "This will delete the live resource from the cluster. It may be re-created on the next sync."
          }
          confirmLabel={pendingAction.action === "restart" ? "Restart" : "Delete"}
          danger
          onConfirm={confirmAction}
          onCancel={() => setPendingAction(null)}
        />
      )}

      {actionError && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-[10001] rounded-lg border border-red-500/40 bg-red-950/80 px-4 py-2.5 text-xs text-red-300 shadow-xl backdrop-blur-sm max-w-sm text-center">
          {actionError}
          <button
            type="button"
            className="ml-3 underline opacity-70 hover:opacity-100"
            onClick={() => setActionError(null)}
          >
            Dismiss
          </button>
        </div>
      )}

      {selected?.kind === "Pod" && (
        <div className="fixed inset-0 z-40 flex justify-end sm:pl-12 pointer-events-none">
          <div
            className="pointer-events-auto h-full w-full max-w-[min(100vw,720px)] border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl overflow-hidden"
            style={{ marginTop: "env(safe-area-inset-top, 0)" }}
          >
            <PodDrawer appName={name} node={selected} onClose={() => setSelected(null)} />
          </div>
        </div>
      )}
      {selected && selected.kind !== "Pod" && (
        <div className="fixed inset-0 z-40 flex justify-end sm:pl-24 pointer-events-none">
          <div className="pointer-events-auto h-full w-full max-w-[min(100vw,640px)] border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl p-0 overflow-hidden">
            <ResourceDetailPanel
              appName={name}
              node={selected}
              app={app}
              onClose={() => setSelected(null)}
              onOpenPod={onSelect}
              onSelectMember={(child) => setSelected(child)}
              onExpandCompactPods={(uid) => {
                expandReplicaSetOnMap(uid);
                setSelected(null);
              }}
              onExpandKindGroup={(uid) => {
                expandKindGroupOnMap(uid);
                setSelected(null);
              }}
            />
          </div>
        </div>
      )}
    </div>
  );
}
