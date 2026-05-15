package controller

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	annSyncWaveArgo = "argocd.argoproj.io/sync-wave"
	annSyncWave     = "gitops.k8s-ui.dev/sync-wave"
	annHookArgo     = "argocd.argoproj.io/hook"
	annHook         = "gitops.k8s-ui.dev/hook"
)

func syncWave(o *unstructured.Unstructured) int {
	a := o.GetAnnotations()
	if a == nil {
		return 0
	}
	for _, k := range []string{annSyncWaveArgo, annSyncWave} {
		if v, ok := a[k]; ok {
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return i
			}
		}
	}
	return 0
}

func objectSortKey(o *unstructured.Unstructured) string {
	ns := o.GetNamespace()
	if ns == "" {
		ns = "_"
	}
	return fmt.Sprintf("%s/%s/%s/%s", o.GetAPIVersion(), o.GetKind(), ns, o.GetName())
}

// hookPhase groups resources like Argo CD: PreSync (0), Sync (1), PostSync (2),
// Fail/Skip (4) last.
func hookPhase(o *unstructured.Unstructured) int {
	a := o.GetAnnotations()
	if a == nil {
		return 1
	}
	var v string
	for _, k := range []string{annHookArgo, annHook} {
		if x, ok := a[k]; ok {
			v = strings.TrimSpace(x)
			break
		}
	}
	switch strings.ToLower(v) {
	case "presync":
		return 0
	case "postsync":
		return 2
	case "fail", "skip":
		return 4
	default:
		return 1
	}
}

// sortForApply orders manifests for a successful first sync: hook phase,
// sync-wave, then a coarse kind order (Namespace and RBAC-ish objects before
// workloads — avoids plain repos where `deployment.yaml` sorts before `namespace.yaml`).
func sortForApply(objs []*unstructured.Unstructured) {
	sort.SliceStable(objs, func(i, j int) bool {
		a, b := objs[i], objs[j]
		hi, hj := hookPhase(a), hookPhase(b)
		if hi != hj {
			return hi < hj
		}
		wi, wj := syncWave(a), syncWave(b)
		if wi != wj {
			return wi < wj
		}
		oi, oj := resourceKindOrder(a.GetKind()), resourceKindOrder(b.GetKind())
		if oi != oj {
			return oi < oj
		}
		return objectSortKey(a) < objectSortKey(b)
	})
}

func resourceKindOrder(kind string) int {
	switch kind {
	case "Namespace":
		return -100
	case "ResourceQuota", "LimitRange":
		return -90
	case "ServiceAccount":
		return -80
	case "Secret", "ConfigMap":
		return -70
	case "Role", "RoleBinding":
		return -60
	case "ClusterRole", "ClusterRoleBinding":
		return -50
	case "PersistentVolumeClaim":
		return -40
	case "Service":
		return -10
	case "PodDisruptionBudget":
		return 0
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return 20
	case "Ingress", "NetworkPolicy", "HorizontalPodAutoscaler":
		return 30
	default:
		return 5
	}
}

// sortBySyncWave orders manifests like Argo CD: lower sync-wave first; ties
// broken by a stable apiVersion/kind/namespace/name key.
func sortBySyncWave(objs []*unstructured.Unstructured) {
	sort.SliceStable(objs, func(i, j int) bool {
		wi, wj := syncWave(objs[i]), syncWave(objs[j])
		if wi != wj {
			return wi < wj
		}
		return objectSortKey(objs[i]) < objectSortKey(objs[j])
	})
}
