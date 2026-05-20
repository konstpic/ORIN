package manifest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helm renders a local Helm chart directory via `helm template` (Helm v3
// binary must be on PATH).
type Helm struct {
	ReleaseName string
	Namespace   string
	// ExtraValueFiles are paths relative to dir passed as -f layers before ExtraValuesYAML.
	// Equivalent to Argo CD spec.source.helm.valueFiles.
	ExtraValueFiles []string
	ExtraValuesYAML []byte // optional last values file (app-level JSON/YAML overrides)
}

// Render runs `helm template` in dir (chart root).
func (h *Helm) Render(dir string) ([]*unstructured.Unstructured, error) {
	if h.ReleaseName == "" {
		h.ReleaseName = "release"
	}
	ns := h.Namespace
	if ns == "" {
		ns = "default"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	args := []string{"template", h.ReleaseName, ".", "--namespace", ns}

	// Extra value files from the application (Argo-style valueFiles). Paths are
	// relative to the chart directory so we pass them as-is — helm resolves them
	// against cmd.Dir.
	for _, vf := range h.ExtraValueFiles {
		if strings.TrimSpace(vf) == "" {
			continue
		}
		args = append(args, "-f", vf)
	}

	// App-level JSON/YAML override (equivalent to an inline valuesObject / values string).
	var valuesFile string
	if len(bytes.TrimSpace(h.ExtraValuesYAML)) > 0 && string(bytes.TrimSpace(h.ExtraValuesYAML)) != "null" {
		f, err := os.CreateTemp("", "orin-helm-values-*.yaml")
		if err != nil {
			return nil, fmt.Errorf("helm values temp file: %w", err)
		}
		valuesFile = f.Name()
		defer func() { _ = os.Remove(valuesFile) }()
		if _, err := f.Write(h.ExtraValuesYAML); err != nil {
			_ = f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		args = append(args, "-f", valuesFile)
	}
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("helm template: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("helm template: %w (is helm v3 installed and on PATH?)", err)
	}
	return SplitYAMLDocs(out)
}

func sanitizeHelmRelease(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "release"
	}
	return out
}
