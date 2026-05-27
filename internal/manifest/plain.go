// Package manifest renders Kubernetes manifests from a checked-out repo
// directory. The MVP supports plain YAML only; Helm/Kustomize plug into the
// same Renderer interface later.
package manifest

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Renderer turns a directory of files into a flat list of unstructured objects.
type Renderer interface {
	Render(dir string) ([]*unstructured.Unstructured, error)
}

// chartDir reports whether dir is a Helm chart root.
func chartDir(dir string) (string, bool) {
	for _, name := range []string{"Chart.yaml", "Chart.yml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir, true
		}
	}
	return "", false
}

// Detect returns the renderer appropriate for the directory.
// Helm and Kustomize require the respective CLIs on PATH.
//
// When ctx.Plugin is non-nil the PluginRenderer is returned unconditionally,
// bypassing all filesystem detection heuristics.
func Detect(dir string, ctx RenderContext) (Renderer, error) {
	if ctx.Plugin != nil {
		return &PluginRenderer{Config: *ctx.Plugin}, nil
	}
	if _, ok := chartDir(dir); ok {
		return &Helm{
			ReleaseName:     sanitizeHelmRelease(ctx.AppName),
			Namespace:       ctx.DestNamespace,
			ExtraValueFiles: ctx.HelmValueFiles,
			ExtraValuesYAML: ctx.HelmValuesJSON,
		}, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "kustomization.yaml")); err == nil {
		return &Kustomize{}, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "kustomization.yml")); err == nil {
		return &Kustomize{}, nil
	}
	return &Plain{}, nil
}

// Plain renders a directory of .yaml/.yml files (recursively).
type Plain struct{}

// Render reads every YAML doc under dir and returns one unstructured per doc.
func (p *Plain) Render(dir string) ([]*unstructured.Unstructured, error) {
	var out []*unstructured.Unstructured
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		objs, err := SplitYAMLDocs(content)
		if err != nil {
			slog.Warn("skipping manifest file", "path", path, "err", err)
			return nil
		}
		out = append(out, objs...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SplitYAMLDocs splits a multi-doc YAML stream into unstructured objects,
// skipping empty docs.
func SplitYAMLDocs(content []byte) ([]*unstructured.Unstructured, error) {
	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)
	var out []*unstructured.Unstructured
	for {
		var raw map[string]any
		if err := reader.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: raw}
		if u.GetKind() == "" || u.GetAPIVersion() == "" {
			return nil, fmt.Errorf("object missing apiVersion/kind: %v", raw)
		}
		out = append(out, u)
	}
	return out, nil
}

// ParseYAMLToUnstructured parses a single YAML document and returns the first
// object it finds. It is used by the live-resource apply API endpoint.
func ParseYAMLToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	objs, err := SplitYAMLDocs(data)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("no Kubernetes object found in the provided YAML")
	}
	return objs[0], nil
}

// InjectTrackingLabel adds the tracking label that ties an object to an
// application instance.
const TrackingLabel = "app.kubernetes.io/instance"

// ApplyTracking sets the tracking label and defaults namespace on each object.
// For Pod-spawning workloads (Deployment, StatefulSet, DaemonSet, Job) it also
// injects the label into spec.template.metadata.labels so that the pods
// produced by the workload inherit the tracking label and can be looked up.
func ApplyTracking(objs []*unstructured.Unstructured, appName, defaultNamespace string) {
	for _, o := range objs {
		labels := o.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[TrackingLabel] = appName
		o.SetLabels(labels)
		if o.GetNamespace() == "" && isNamespaced(o.GroupVersionKind().Kind) {
			o.SetNamespace(defaultNamespace)
		}
		injectPodTemplateTrackingLabel(o, appName)
	}
}

// podTemplateKinds are workload kinds that carry a spec.template (PodTemplateSpec).
var podTemplateKinds = map[string]bool{
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"Job":         true,
}

// injectPodTemplateTrackingLabel adds the tracking label into
// spec.template.metadata.labels for workload objects that create pods.
func injectPodTemplateTrackingLabel(o *unstructured.Unstructured, appName string) {
	if !podTemplateKinds[o.GetKind()] {
		return
	}
	tplLabels, found, err := unstructured.NestedStringMap(o.Object, "spec", "template", "metadata", "labels")
	if err != nil || !found {
		tplLabels = map[string]string{}
	}
	if tplLabels == nil {
		tplLabels = map[string]string{}
	}
	tplLabels[TrackingLabel] = appName
	_ = unstructured.SetNestedStringMap(o.Object, tplLabels, "spec", "template", "metadata", "labels")
}

// isNamespaced is a best-effort fast path; the truth is in the discovery
// client at apply time. We default cluster-scoped Kinds to "" here.
func isNamespaced(kind string) bool {
	switch kind {
	case "Namespace", "Node", "PersistentVolume",
		"ClusterRole", "ClusterRoleBinding",
		"CustomResourceDefinition", "StorageClass",
		"PriorityClass", "ValidatingWebhookConfiguration",
		"MutatingWebhookConfiguration", "APIService":
		return false
	}
	return true
}
