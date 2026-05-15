package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newCM(name string, data map[string]string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{}
	o.SetAPIVersion("v1")
	o.SetKind("ConfigMap")
	o.SetName(name)
	o.SetNamespace("default")
	d := map[string]any{}
	for k, v := range data {
		d[k] = v
	}
	o.Object["data"] = d
	return o
}

func TestDiff_Equal(t *testing.T) {
	d := newCM("a", map[string]string{"k": "v"})
	l := newCM("a", map[string]string{"k": "v"})
	// Pretend the cluster set status / managedFields / RV.
	l.Object["status"] = map[string]any{"foo": "bar"}
	l.SetResourceVersion("12345")
	l.SetUID("test")

	ds, err := Diff([]*unstructured.Unstructured{d}, []*unstructured.Unstructured{l})
	if err != nil {
		t.Fatal(err)
	}
	if ds.OutOfSync != 0 || ds.Synced != 1 {
		t.Fatalf("expected synced (1/0), got %d synced %d outOfSync", ds.Synced, ds.OutOfSync)
	}
}

func TestDiff_Drift(t *testing.T) {
	d := newCM("a", map[string]string{"k": "v"})
	l := newCM("a", map[string]string{"k": "v2"})
	ds, err := Diff([]*unstructured.Unstructured{d}, []*unstructured.Unstructured{l})
	if err != nil {
		t.Fatal(err)
	}
	if ds.OutOfSync != 1 || ds.Synced != 0 {
		t.Fatalf("expected OutOfSync, got %d synced %d outOfSync", ds.Synced, ds.OutOfSync)
	}
}

func TestDiff_Missing(t *testing.T) {
	d := newCM("a", map[string]string{"k": "v"})
	ds, err := Diff([]*unstructured.Unstructured{d}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ds.OutOfSync != 1 {
		t.Fatalf("missing live should be OutOfSync, got %d", ds.OutOfSync)
	}
	if ds.Items[0].LiveYAML != "" {
		t.Fatal("LiveYAML should be empty when live is missing")
	}
}

func TestNormalize_StripsServerFields(t *testing.T) {
	o := newCM("a", map[string]string{"k": "v"})
	o.SetResourceVersion("9")
	o.SetUID("test-uid")
	o.SetGeneration(3)
	_ = unstructured.SetNestedField(o.Object, "blah", "metadata", "managedFields", "0")
	o.Object["status"] = map[string]any{"foo": "bar"}

	n := normalize(o)
	if v, ok, _ := unstructured.NestedString(n.Object, "metadata", "resourceVersion"); ok && v != "" {
		t.Fatal("resourceVersion should be stripped")
	}
	if _, found := n.Object["status"]; found {
		t.Fatal("status should be stripped")
	}
}
