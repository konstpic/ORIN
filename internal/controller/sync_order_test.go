package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSortForApply_namespaceBeforeDeployment(t *testing.T) {
	objs := []*unstructured.Unstructured{
		{Object: map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "web", "namespace": "demo"}}},
		{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "demo"}}},
	}
	sortForApply(objs)
	if objs[0].GetKind() != "Namespace" {
		t.Fatalf("want Namespace first, got %s", objs[0].GetKind())
	}
	if objs[1].GetKind() != "Deployment" {
		t.Fatalf("want Deployment second, got %s", objs[1].GetKind())
	}
}
