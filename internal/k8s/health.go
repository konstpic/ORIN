package k8s

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

// Health returns the health for one live object. Built-in support covers the
// most common workload kinds; everything else is Unknown.
func Health(obj *unstructured.Unstructured) domain.HealthStatus {
	if obj == nil {
		return domain.HealthMissing
	}
	switch obj.GetKind() {
	case "Deployment":
		return deploymentHealth(obj)
	case "ReplicaSet":
		return replicaSetHealth(obj)
	case "StatefulSet":
		return statefulSetHealth(obj)
	case "DaemonSet":
		return daemonSetHealth(obj)
	case "Pod":
		return podHealth(obj)
	case "Job":
		return jobHealth(obj)
	case "Service", "Ingress", "ConfigMap", "Secret",
		"ServiceAccount", "Role", "RoleBinding",
		"ClusterRole", "ClusterRoleBinding", "Namespace":
		return domain.HealthHealthy
	}
	return domain.HealthUnknown
}

// Aggregate returns the worst-case health across multiple resources.
func Aggregate(per []domain.HealthStatus) domain.HealthStatus {
	rank := map[domain.HealthStatus]int{
		domain.HealthHealthy:     0,
		domain.HealthSuspended:   1,
		domain.HealthProgressing: 2,
		domain.HealthMissing:     3,
		domain.HealthDegraded:    4,
		domain.HealthUnknown:     5,
	}
	worst := domain.HealthHealthy
	for _, h := range per {
		if rank[h] > rank[worst] {
			worst = h
		}
	}
	if len(per) == 0 {
		return domain.HealthUnknown
	}
	return worst
}

func deploymentHealth(o *unstructured.Unstructured) domain.HealthStatus {
	desired, _, _ := unstructured.NestedInt64(o.Object, "spec", "replicas")
	avail, _, _ := unstructured.NestedInt64(o.Object, "status", "availableReplicas")
	updated, _, _ := unstructured.NestedInt64(o.Object, "status", "updatedReplicas")
	gen, _, _ := unstructured.NestedInt64(o.Object, "metadata", "generation")
	obsGen, _, _ := unstructured.NestedInt64(o.Object, "status", "observedGeneration")
	if obsGen < gen {
		return domain.HealthProgressing
	}
	if updated < desired {
		return domain.HealthProgressing
	}
	if avail < desired {
		return domain.HealthDegraded
	}
	return domain.HealthHealthy
}

func replicaSetHealth(o *unstructured.Unstructured) domain.HealthStatus {
	desired, found, _ := unstructured.NestedInt64(o.Object, "spec", "replicas")
	if !found {
		desired = 1
	}
	ready, _, _ := unstructured.NestedInt64(o.Object, "status", "readyReplicas")
	if desired > 0 && ready < desired {
		return domain.HealthProgressing
	}
	return domain.HealthHealthy
}

func statefulSetHealth(o *unstructured.Unstructured) domain.HealthStatus {
	desired, _, _ := unstructured.NestedInt64(o.Object, "spec", "replicas")
	ready, _, _ := unstructured.NestedInt64(o.Object, "status", "readyReplicas")
	if ready < desired {
		return domain.HealthProgressing
	}
	return domain.HealthHealthy
}

func daemonSetHealth(o *unstructured.Unstructured) domain.HealthStatus {
	desired, _, _ := unstructured.NestedInt64(o.Object, "status", "desiredNumberScheduled")
	ready, _, _ := unstructured.NestedInt64(o.Object, "status", "numberReady")
	if ready < desired {
		return domain.HealthProgressing
	}
	return domain.HealthHealthy
}

func podHealth(o *unstructured.Unstructured) domain.HealthStatus {
	phase, _, _ := unstructured.NestedString(o.Object, "status", "phase")
	switch phase {
	case "Running":
		return domain.HealthHealthy
	case "Succeeded":
		return domain.HealthHealthy
	case "Pending":
		return domain.HealthProgressing
	case "Failed":
		return domain.HealthDegraded
	}
	return domain.HealthUnknown
}

func jobHealth(o *unstructured.Unstructured) domain.HealthStatus {
	failed, _, _ := unstructured.NestedInt64(o.Object, "status", "failed")
	succ, _, _ := unstructured.NestedInt64(o.Object, "status", "succeeded")
	if failed > 0 {
		return domain.HealthDegraded
	}
	if succ > 0 {
		return domain.HealthHealthy
	}
	return domain.HealthProgressing
}
