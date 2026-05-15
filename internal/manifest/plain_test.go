package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlainRenderer(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "deployment.yaml"), `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector:
    app: web
  ports:
    - port: 80
`)
	mustWrite(t, filepath.Join(dir, "configmap.yml"), `apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  k: v
`)

	r, err := Detect(dir, RenderContext{AppName: "my-app", DestNamespace: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	objs, err := r.Render(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}
	ApplyTracking(objs, "my-app", "demo")
	for _, o := range objs {
		if got := o.GetLabels()[TrackingLabel]; got != "my-app" {
			t.Fatalf("missing tracking label on %s", o.GetName())
		}
		if o.GetNamespace() != "demo" {
			t.Fatalf("expected namespace demo, got %s", o.GetNamespace())
		}
	}
}

func TestDetectHelmChart(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Chart.yaml"), "name: x\nversion: 1.0.0\n")
	r, err := Detect(dir, RenderContext{AppName: "x", DestNamespace: "ns"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(*Helm); !ok {
		t.Fatalf("expected *Helm renderer, got %T", r)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
