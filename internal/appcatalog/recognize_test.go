package appcatalog

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func fixedResolver(clusterName string) ArgoDestinationResolve {
	return func(_, _ string) (string, error) { return clusterName, nil }
}

// applicationObj returns a minimal Application unstructured object for the given group.
func applicationObj(apiVersion, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       "Application",
		"metadata":   map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"project": "default",
			"source": map[string]interface{}{
				"repoURL":        "https://github.com/org/repo.git",
				"path":           "helm/app",
				"targetRevision": "main",
			},
			"destination": map[string]interface{}{
				"name":      "in-cluster",
				"namespace": "test-ns",
			},
		},
	}}
}

// appProjectObj returns a minimal AppProject unstructured object.
func appProjectObj(apiVersion, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       "AppProject",
		"metadata":   map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"description": "test project",
			"sourceRepos": []interface{}{"https://github.com/org/*"},
			"destinations": []interface{}{
				map[string]interface{}{"name": "in-cluster", "namespace": "test-*"},
			},
		},
	}}
}

// ---- k8s-ui.io group ----

func TestTryEntryFromObject_KuiApplication(t *testing.T) {
	u := applicationObj("k8s-ui.io/v1alpha1", "my-app")
	app, _, kind, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if kind != EntryKindApplication {
		t.Fatalf("kind=%v", kind)
	}
	if app.Name != "my-app" {
		t.Fatalf("name=%q", app.Name)
	}
	if app.Source.RepoURL != "https://github.com/org/repo.git" {
		t.Fatalf("repoURL=%q", app.Source.RepoURL)
	}
	if app.Destination.Cluster != "in-cluster" || app.Destination.Namespace != "test-ns" {
		t.Fatalf("dest=%+v", app.Destination)
	}
}

func TestTryEntryFromObject_KuiAppProject(t *testing.T) {
	u := appProjectObj("k8s-ui.io/v1alpha1", "my-project")
	_, proj, kind, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if kind != EntryKindAppProject {
		t.Fatalf("kind=%v", kind)
	}
	if proj.Name != "my-project" {
		t.Fatalf("name=%q", proj.Name)
	}
	if proj.Description != "test project" {
		t.Fatalf("desc=%q", proj.Description)
	}
	if len(proj.Policies.SourceRepos) != 1 || proj.Policies.SourceRepos[0] != "https://github.com/org/*" {
		t.Fatalf("sourceRepos=%v", proj.Policies.SourceRepos)
	}
	if len(proj.Policies.Destinations) != 1 || proj.Policies.Destinations[0].Namespace != "test-*" {
		t.Fatalf("destinations=%v", proj.Policies.Destinations)
	}
}

// ---- argoproj.io compat group ----

func TestTryEntryFromObject_ArgoApplication(t *testing.T) {
	u := applicationObj("argoproj.io/v1alpha1", "argo-app")
	app, _, kind, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if kind != EntryKindApplication {
		t.Fatalf("kind=%v", kind)
	}
	if app.Name != "argo-app" {
		t.Fatalf("name=%q", app.Name)
	}
}

func TestTryEntryFromObject_ArgoAppProject(t *testing.T) {
	u := appProjectObj("argoproj.io/v1alpha1", "argo-proj")
	_, proj, kind, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if kind != EntryKindAppProject {
		t.Fatalf("kind=%v", kind)
	}
	if proj.Name != "argo-proj" {
		t.Fatalf("name=%q", proj.Name)
	}
}

// ---- non-control-plane objects are ignored ----

func TestTryEntryFromObject_ConfigMap_Ignored(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "cm"},
	}}
	_, _, _, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || ok {
		t.Fatalf("expected skip, ok=%v err=%v", ok, err)
	}
}

func TestTryEntryFromObject_UnknownKind_Ignored(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "k8s-ui.io/v1alpha1",
		"kind":       "SomethingElse",
		"metadata":   map[string]interface{}{"name": "x"},
	}}
	_, _, _, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || ok {
		t.Fatalf("expected skip, ok=%v err=%v", ok, err)
	}
}

// ---- edge cases ----

func TestTryEntryFromObject_Application_MissingName(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "k8s-ui.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]interface{}{},
		"spec": map[string]interface{}{
			"source":      map[string]interface{}{"repoURL": "https://x.git"},
			"destination": map[string]interface{}{"name": "in-cluster", "namespace": "ns"},
		},
	}}
	_, _, _, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if ok || err == nil {
		t.Fatalf("expected error for missing name, ok=%v err=%v", ok, err)
	}
}

func TestTryEntryFromObject_Application_IgnoreDifferences(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "k8s-ui.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]interface{}{"name": "idf-app"},
		"spec": map[string]interface{}{
			"project": "p",
			"source": map[string]interface{}{
				"repoURL": "https://github.com/org/repo.git",
				"path":    ".",
			},
			"destination": map[string]interface{}{
				"name":      "in-cluster",
				"namespace": "ns",
			},
			// Argo CD puts ignoreDifferences at spec level
			"ignoreDifferences": []interface{}{
				map[string]interface{}{
					"group":        "apps",
					"kind":         "Deployment",
					"jsonPointers": []interface{}{"/spec/replicas"},
				},
			},
		},
	}}
	app, _, _, ok, err := TryEntryFromObject(u, fixedResolver("in-cluster"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if app.SyncPolicy == nil || len(app.SyncPolicy.IgnoreDifferences) != 1 {
		t.Fatalf("ignoreDifferences: %+v", app.SyncPolicy)
	}
	r := app.SyncPolicy.IgnoreDifferences[0]
	if r.Group != "apps" || r.Kind != "Deployment" {
		t.Fatalf("rule: %+v", r)
	}
}

func TestTryEntryFromObject_Nil(t *testing.T) {
	_, _, _, ok, err := TryEntryFromObject(nil, fixedResolver("in-cluster"))
	if err != nil || ok {
		t.Fatalf("expected skip on nil, ok=%v err=%v", ok, err)
	}
}

func TestTryEntryFromObject_NilResolver(t *testing.T) {
	u := applicationObj("k8s-ui.io/v1alpha1", "app")
	_, _, _, ok, err := TryEntryFromObject(u, nil)
	if err != nil || ok {
		t.Fatalf("expected skip on nil resolver, ok=%v err=%v", ok, err)
	}
}

// ---- IsControlPlaneObject ----

func TestIsControlPlaneObject(t *testing.T) {
	cases := []struct {
		apiVersion string
		kind       string
		want       bool
	}{
		{"k8s-ui.io/v1alpha1", "Application", true},
		{"k8s-ui.io/v1alpha1", "AppProject", true},
		{"argoproj.io/v1alpha1", "Application", true},
		{"argoproj.io/v1alpha1", "AppProject", true},
		{"argoproj.io/v1alpha1", "ApplicationSet", false}, // not a managed kind
		{"v1", "ConfigMap", false},
		{"apps/v1", "Deployment", false},
	}
	for _, tc := range cases {
		u := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": tc.apiVersion,
			"kind":       tc.kind,
		}}
		if got := IsControlPlaneObject(u); got != tc.want {
			t.Errorf("IsControlPlaneObject(%q/%q) = %v, want %v", tc.apiVersion, tc.kind, got, tc.want)
		}
	}
}
