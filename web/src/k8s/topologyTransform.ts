import type { HealthStatus, ResourceNode, SyncStatus } from "../api/types";

/** Group pods under ReplicaSet or Deployment when there are at least this many pods (compact topology). */
export const POD_GROUP_MIN_PODS = 1;

/** Group same-kind siblings (e.g. ConfigMap, Secret, Service) when count >= this threshold (topology view). */
export const GENERIC_GROUP_MIN = 3;

/** In list view, group same-kind siblings when count >= this threshold. */
export const LIST_GROUP_MIN = 1;

/** Kinds eligible for generic same-kind grouping under any parent. */
const GROUPABLE_KINDS = new Set([
  "ConfigMap",
  "Secret",
  "ServiceAccount",
  "Service",
  "Ingress",
  "PersistentVolumeClaim",
  "Endpoints",
  "EndpointSlice",
  "Role",
  "RoleBinding",
  "ClusterRole",
  "ClusterRoleBinding",
  "NetworkPolicy",
  "PodDisruptionBudget",
  "HorizontalPodAutoscaler",
  "Job",
  "CronJob",
]);

function aggregatePodHealth(pods: ResourceNode[]): HealthStatus {
  if (!pods.length) return "Unknown";
  if (pods.some((p) => p.health === "Degraded")) return "Degraded";
  if (pods.some((p) => p.health === "Progressing")) return "Progressing";
  if (pods.some((p) => p.health === "Missing")) return "Missing";
  if (pods.some((p) => p.health === "Suspended")) return "Suspended";
  if (pods.every((p) => p.health === "Healthy")) return "Healthy";
  return "Unknown";
}

function aggregateSync(items: ResourceNode[]): SyncStatus {
  if (!items.length) return "Unknown";
  if (items.some((i) => i.sync === "OutOfSync")) return "OutOfSync";
  if (items.every((i) => i.sync === "Synced")) return "Synced";
  return "Unknown";
}

/** Build a synthetic group node for >=N same-kind sibling resources. */
function makeKindGroup(kind: string, members: ResourceNode[], parentUid: string): ResourceNode {
  const uid = `synthetic:group:${parentUid}:${kind}`;
  return {
    group: "ui.k8s-ui",
    version: "v1",
    kind,
    name: `${members.length} ${kind}`,
    uid,
    health: aggregatePodHealth(members),
    sync: aggregateSync(members),
    parentUid,
    isKindGroup: true,
    groupedMembers: members,
  };
}

/** Collapse pod children into the parent ReplicaSet / Deployment node (no separate PodGroup graph node). */
function compactManyPodsUnderNode(
  node: ResourceNode,
  expandedGroupParentUids: Set<string>,
  groupOtherKinds: boolean,
): ResourceNode {
  const mappedChildren = (node.children ?? []).map((c) =>
    compactManyPodsUnderNode(c, expandedGroupParentUids, groupOtherKinds),
  );

  const isPodParent = node.kind === "ReplicaSet" || node.kind === "Deployment";
  const pods = mappedChildren.filter((c) => c.kind === "Pod");
  const nonPods = mappedChildren.filter((c) => c.kind !== "Pod");

  if (isPodParent && pods.length >= POD_GROUP_MIN_PODS && !expandedGroupParentUids.has(node.uid)) {
    const childrenAfterPodGroup = nonPods.length > 0 ? nonPods : undefined;
    return {
      ...node,
      children: childrenAfterPodGroup
        ? groupSameKindSiblings(childrenAfterPodGroup, node.uid, expandedGroupParentUids, groupOtherKinds)
        : undefined,
      groupedPods: pods,
      health: aggregatePodHealth(pods),
    };
  }

  return {
    ...node,
    children: groupSameKindSiblings(mappedChildren, node.uid, expandedGroupParentUids, groupOtherKinds),
  };
}

/** Bucket siblings of the same groupable kind into a synthetic group node. */
function groupSameKindSiblings(
  siblings: ResourceNode[],
  parentUid: string,
  expandedGroupParentUids: Set<string>,
  groupOtherKinds: boolean,
  minCount = GENERIC_GROUP_MIN,
): ResourceNode[] {
  if (!groupOtherKinds || siblings.length === 0) return siblings;

  const byKind = new Map<string, ResourceNode[]>();
  const order: string[] = [];
  for (const s of siblings) {
    if (!byKind.has(s.kind)) {
      order.push(s.kind);
      byKind.set(s.kind, []);
    }
    byKind.get(s.kind)!.push(s);
  }

  const out: ResourceNode[] = [];
  for (const kind of order) {
    const members = byKind.get(kind)!;
    const eligible =
      GROUPABLE_KINDS.has(kind) &&
      members.length >= minCount &&
      !expandedGroupParentUids.has(`synthetic:group:${parentUid}:${kind}`);
    if (eligible) {
      out.push(makeKindGroup(kind, members, parentUid));
    } else {
      out.push(...members);
    }
  }
  return out;
}

/** Recursively group same-kind siblings in the list view (lower threshold). */
function groupForListView(node: ResourceNode, expandedGroupUids: Set<string>): ResourceNode {
  const mappedChildren = (node.children ?? []).map((c) => groupForListView(c, expandedGroupUids));
  return {
    ...node,
    children: groupSameKindSiblings(mappedChildren, node.uid, expandedGroupUids, true, LIST_GROUP_MIN),
  };
}

export function buildSyntheticApplicationRoot(
  appName: string,
  appHealth: HealthStatus,
  appSync: SyncStatus,
  resourceRoots: ResourceNode[],
): ResourceNode {
  return {
    group: "ui.k8s-ui",
    version: "v1",
    kind: "Application",
    name: appName,
    uid: `synthetic:app:${appName}`,
    health: appHealth,
    sync: appSync,
    children: resourceRoots,
  };
}

export function prepareTopologyRoots(
  apiRoots: ResourceNode[],
  options: {
    appName: string;
    appHealth: HealthStatus;
    appSync: SyncStatus;
    compactPods: boolean;
    groupOtherKinds: boolean;
    expandedReplicaSetUids: Set<string>;
    expandedGroupUids: Set<string>;
  },
): ResourceNode[] {
  const expanded = new Set<string>();
  for (const uid of options.expandedReplicaSetUids) expanded.add(uid);
  for (const uid of options.expandedGroupUids) expanded.add(uid);

  let forest: ResourceNode[];
  if (options.compactPods || options.groupOtherKinds) {
    forest = apiRoots.map((r) => compactManyPodsUnderNode(r, expanded, options.groupOtherKinds));
    if (options.groupOtherKinds) {
      forest = groupSameKindSiblings(forest, `synthetic:app:${options.appName}`, expanded, true);
    }
  } else {
    forest = apiRoots;
  }
  return [
    buildSyntheticApplicationRoot(
      options.appName,
      options.appHealth,
      options.appSync,
      forest,
    ),
  ];
}

export function prepareListRoots(
  apiRoots: ResourceNode[],
  options: {
    appName: string;
    appHealth: HealthStatus;
    appSync: SyncStatus;
    expandedGroupUids?: Set<string>;
  },
): ResourceNode[] {
  const expanded = options.expandedGroupUids ?? new Set<string>();
  const appUid = `synthetic:app:${options.appName}`;

  // Group top-level siblings by kind
  const groupedRoots = groupSameKindSiblings(
    apiRoots.map((r) => groupForListView(r, expanded)),
    appUid,
    expanded,
    true,
    LIST_GROUP_MIN,
  );

  return [
    buildSyntheticApplicationRoot(
      options.appName,
      options.appHealth,
      options.appSync,
      groupedRoots,
    ),
  ];
}
