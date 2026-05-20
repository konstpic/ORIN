package manifest

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func obj(apiVersion, kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": "x"},
	}}
}

func TestIsControlPlaneObject(t *testing.T) {
	cases := []struct {
		apiVersion string
		kind       string
		want       bool
	}{
		{"orin.io/v1alpha1", "Application", true},
		{"orin.io/v1alpha1", "AppProject", true},
		{"argoproj.io/v1alpha1", "Application", true},
		{"argoproj.io/v1alpha1", "AppProject", true},
		{"argoproj.io/v1alpha1", "ApplicationSet", false},
		{"v1", "ConfigMap", false},
		{"apps/v1", "Deployment", false},
		{"v1", "Secret", false},
	}
	for _, tc := range cases {
		u := obj(tc.apiVersion, tc.kind)
		if got := IsControlPlaneObject(u); got != tc.want {
			t.Errorf("IsControlPlaneObject(%q/%q) = %v, want %v", tc.apiVersion, tc.kind, got, tc.want)
		}
	}
	if IsControlPlaneObject(nil) {
		t.Error("nil should return false")
	}
}

func TestFilterApplicable(t *testing.T) {
	objs := []*unstructured.Unstructured{
		obj("apps/v1", "Deployment"),
		obj("orin.io/v1alpha1", "Application"),
		obj("v1", "ConfigMap"),
		obj("argoproj.io/v1alpha1", "AppProject"),
		obj("v1", "Secret"),
		obj("argoproj.io/v1alpha1", "Application"),
	}
	got := FilterApplicable(objs)
	if len(got) != 3 {
		t.Fatalf("expected 3 applicable objects, got %d", len(got))
	}
	for _, o := range got {
		if IsControlPlaneObject(o) {
			t.Errorf("control-plane object slipped through: %s/%s", o.GetAPIVersion(), o.GetKind())
		}
	}
}

func TestFilterApplicable_Empty(t *testing.T) {
	if got := FilterApplicable(nil); len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestFilterApplicable_AllApplicable(t *testing.T) {
	objs := []*unstructured.Unstructured{
		obj("apps/v1", "Deployment"),
		obj("v1", "Service"),
	}
	got := FilterApplicable(objs)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}
