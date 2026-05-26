package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orin/orin/internal/domain"
)

func newDeployment(replicas, avail, updated, gen, obsGen int64, conditions []map[string]interface{}) *unstructured.Unstructured {
	o := &unstructured.Unstructured{}
	o.SetAPIVersion("apps/v1")
	o.SetKind("Deployment")
	o.SetName("demo")
	o.SetNamespace("default")
	o.Object["spec"] = map[string]interface{}{
		"replicas": replicas,
	}
	status := map[string]interface{}{
		"observedGeneration": obsGen,
		"replicas":           replicas,
		"updatedReplicas":    updated,
		"availableReplicas":  avail,
	}
	if conditions != nil {
		slice := make([]interface{}, len(conditions))
		for i, c := range conditions {
			slice[i] = c
		}
		status["conditions"] = slice
	}
	o.Object["status"] = status
	o.Object["metadata"] = map[string]interface{}{
		"generation": gen,
	}
	return o
}

func TestDeploymentHealth_AvailableLagIsProgressing(t *testing.T) {
	d := newDeployment(10, 7, 10, 5, 5, nil)
	if got := deploymentHealth(d); got != domain.HealthProgressing {
		t.Fatalf("avail < desired with updated caught up: got %q, want Progressing", got)
	}
}

func TestDeploymentHealth_FullyAvailable(t *testing.T) {
	d := newDeployment(3, 3, 3, 2, 2, nil)
	if got := deploymentHealth(d); got != domain.HealthHealthy {
		t.Fatalf("got %q, want Healthy", got)
	}
}

func TestDeploymentConditionDegraded_ReadsConditions(t *testing.T) {
	d := newDeployment(3, 0, 3, 2, 2, []map[string]interface{}{
		{
			"type":   "Progressing",
			"status": "False",
			"reason": "ProgressDeadlineExceeded",
		},
	})
	if !deploymentConditionDegraded(d) {
		conds, found, _ := unstructured.NestedSlice(d.Object, "status", "conditions")
		typ, _, _ := unstructured.NestedString(d.Object, "status", "conditions", "0", "type")
		t.Fatalf("deploymentConditionDegraded=false found=%v len=%d typ=%q conds=%#v", found, len(conds), typ, conds)
	}
}

func TestDeploymentHealth_ProgressDeadlineExceeded(t *testing.T) {
	d := newDeployment(3, 0, 3, 2, 2, []map[string]interface{}{
		{
			"type":   "Progressing",
			"status": "False",
			"reason": "ProgressDeadlineExceeded",
		},
	})
	if got := deploymentHealth(d); got != domain.HealthDegraded {
		t.Fatalf("got %q, want Degraded", got)
	}
}

func TestDeploymentHealth_ReplicaFailure(t *testing.T) {
	d := newDeployment(3, 2, 3, 2, 2, []map[string]interface{}{
		{
			"type":   "ReplicaFailure",
			"status": "True",
		},
	})
	if got := deploymentHealth(d); got != domain.HealthDegraded {
		t.Fatalf("got %q, want Degraded", got)
	}
}
