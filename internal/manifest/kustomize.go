package manifest

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Kustomize renders using `kustomize build` (kustomize v5+ binary on PATH).
type Kustomize struct{}

// Render runs `kustomize build` in dir (root containing kustomization.yaml).
func (k *Kustomize) Render(dir string) ([]*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kustomize", "build", ".")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("kustomize build: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("kustomize build: %w (is kustomize installed and on PATH?)", err)
	}
	return SplitYAMLDocs(out)
}
