package appcatalog

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTryArgoApplicationEntry(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name": "guestbook",
		},
		"spec": map[string]interface{}{
			"project": "demo",
			"source": map[string]interface{}{
				"repoURL":        "https://github.com/org/repo.git",
				"path":           "apps/guestbook",
				"targetRevision": "main",
			},
			"destination": map[string]interface{}{
				"server":    "https://kubernetes.default.svc",
				"namespace": "guestbook-ns",
			},
			"syncPolicy": map[string]interface{}{
				"syncOptions": []interface{}{"CreateNamespace=true"},
				"managedNamespaceMetadata": map[string]interface{}{
					"labels": map[string]interface{}{"team": "a"},
				},
			},
		},
	}}
	resolve := func(server, name string) (string, error) {
		if name == "in-cluster" {
			return "in-cluster", nil
		}
		return "in-cluster", nil
	}
	e, ok, err := TryArgoApplicationEntry(obj, resolve)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if e.Name != "guestbook" || e.Project != "demo" {
		t.Fatalf("meta %+v", e)
	}
	if e.Source.RepoURL != "https://github.com/org/repo.git" || e.Source.Path != "apps/guestbook" {
		t.Fatalf("source %+v", e.Source)
	}
	if e.Destination.Cluster != "in-cluster" || e.Destination.Namespace != "guestbook-ns" {
		t.Fatalf("dest %+v", e.Destination)
	}
	pol := syncPolicyFromEntry(e)
	if !pol.EffectiveCreateNamespace() {
		t.Fatal("expected EffectiveCreateNamespace")
	}
	if pol.ManagedNamespaceMetadata == nil || pol.ManagedNamespaceMetadata.Labels["team"] != "a" {
		t.Fatalf("managed ns %+v", pol.ManagedNamespaceMetadata)
	}
}

func TestTryArgoApplicationEntryNotArgo(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
	}}
	_, ok, err := TryArgoApplicationEntry(obj, func(_, _ string) (string, error) { return "in-cluster", nil })
	if err != nil || ok {
		t.Fatalf("want skip, ok=%v err=%v", ok, err)
	}
}
