// Package appcatalog parses the shared YAML shape used by Git-driven catalog
// sync and by the app-of-apps rendering of parent applications.
package appcatalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/store"
)

// doc is the YAML document root.
type doc struct {
	Applications []Entry `yaml:"applications"`
}

// EntryIgnoreDifferenceRule mirrors domain.IgnoreDifferenceRule in YAML catalog form.
type EntryIgnoreDifferenceRule struct {
	Group        string   `yaml:"group"`
	Kind         string   `yaml:"kind"`
	Name         string   `yaml:"name,omitempty"`
	Namespace    string   `yaml:"namespace,omitempty"`
	JSONPointers []string `yaml:"jsonPointers,omitempty"`
}

// Entry is one application row (catalog file or embedded ConfigMap data).
type Entry struct {
	Name    string `yaml:"name"`
	Project string `yaml:"project"`
	Source  struct {
		RepoURL        string `yaml:"repoUrl"`
		Path           string `yaml:"path"`
		TargetRevision string `yaml:"targetRevision"`
		HelmValues     any    `yaml:"helmValues"`
		// HelmValueFiles are paths relative to the chart directory in the Git
		// checkout that are passed as extra -f layers to helm template.
		HelmValueFiles []string `yaml:"helmValueFiles,omitempty"`
	} `yaml:"source"`
	Destination struct {
		Cluster   string `yaml:"cluster"`
		Namespace string `yaml:"namespace"`
	} `yaml:"destination"`
	SyncPolicy *struct {
		Automated *struct {
			Prune    bool `yaml:"prune"`
			SelfHeal bool `yaml:"selfHeal"`
		} `yaml:"automated"`
		SyncOptions              []string `yaml:"syncOptions,omitempty"`
		ManagedNamespaceMetadata *struct {
			Labels      map[string]string `yaml:"labels,omitempty"`
			Annotations map[string]string `yaml:"annotations,omitempty"`
		} `yaml:"managedNamespaceMetadata,omitempty"`
		CreateNamespace   *bool                       `yaml:"createNamespace,omitempty"`
		IgnoreDifferences []EntryIgnoreDifferenceRule `yaml:"ignoreDifferences,omitempty"`
	} `yaml:"syncPolicy"`
}

// ParseApplicationsListYAML parses a file with top-level `applications:` list.
func ParseApplicationsListYAML(data []byte) ([]Entry, error) {
	var d doc
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if len(d.Applications) == 0 {
		return nil, fmt.Errorf("applications: empty or missing list")
	}
	return d.Applications, nil
}

func helmValuesJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := bytes.TrimSpace(b)
	if len(s) == 0 || string(s) == "null" {
		return nil, nil
	}
	return b, nil
}

func syncPolicyFromEntry(e Entry) domain.SyncPolicy {
	var pol domain.SyncPolicy
	if e.SyncPolicy != nil && e.SyncPolicy.Automated != nil {
		pol.Automated = &domain.AutomatedSync{
			Prune:    e.SyncPolicy.Automated.Prune,
			SelfHeal: e.SyncPolicy.Automated.SelfHeal,
		}
	}
	if e.SyncPolicy != nil {
		if e.SyncPolicy.CreateNamespace != nil {
			pol.CreateNamespace = *e.SyncPolicy.CreateNamespace
		}
		if len(e.SyncPolicy.SyncOptions) > 0 {
			pol.SyncOptions = slices.Clone(e.SyncPolicy.SyncOptions)
		}
		if m := e.SyncPolicy.ManagedNamespaceMetadata; m != nil && (len(m.Labels) > 0 || len(m.Annotations) > 0) {
			pol.ManagedNamespaceMetadata = &domain.ManagedNamespaceMetadata{
				Labels:      maps.Clone(m.Labels),
				Annotations: maps.Clone(m.Annotations),
			}
		}
		for _, r := range e.SyncPolicy.IgnoreDifferences {
			pol.IgnoreDifferences = append(pol.IgnoreDifferences, domain.IgnoreDifferenceRule{
				Group:        r.Group,
				Kind:         r.Kind,
				Name:         r.Name,
				Namespace:    r.Namespace,
				JSONPointers: slices.Clone(r.JSONPointers),
			})
		}
	}
	return pol
}

// DomainFromEntry resolves repository and cluster rows and builds a domain.Application.
func DomainFromEntry(ctx context.Context, st *store.Store, e Entry) (*domain.Application, error) {
	if e.Name == "" {
		return nil, fmt.Errorf("missing name")
	}
	if e.Source.RepoURL == "" {
		return nil, fmt.Errorf("missing source.repoUrl")
	}
	if e.Source.Path == "" {
		return nil, fmt.Errorf("missing source.path")
	}
	if e.Destination.Cluster == "" || e.Destination.Namespace == "" {
		return nil, fmt.Errorf("missing destination.cluster or destination.namespace")
	}
	repo, err := st.Repositories.GetByURL(ctx, e.Source.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("unknown repo %q: %w", e.Source.RepoURL, err)
	}
	cl, err := st.Clusters.GetByName(ctx, e.Destination.Cluster)
	if err != nil {
		return nil, fmt.Errorf("unknown cluster %q: %w", e.Destination.Cluster, err)
	}
	rev := e.Source.TargetRevision
	if rev == "" {
		rev = "HEAD"
	}
	proj := e.Project
	if proj == "" {
		proj = "default"
	}
	hv, err := helmValuesJSON(e.Source.HelmValues)
	if err != nil {
		return nil, fmt.Errorf("helmValues: %w", err)
	}
	app := &domain.Application{
		Name:           e.Name,
		Project:        proj,
		RepoID:         repo.ID,
		Path:           e.Source.Path,
		TargetRevision: rev,
		HelmValueFiles: slices.Clone(e.Source.HelmValueFiles),
		DestClusterID:  cl.ID,
		DestNamespace:  e.Destination.Namespace,
		SyncPolicy:     syncPolicyFromEntry(e),
	}
	if len(hv) > 0 {
		app.HelmValuesJSON = hv
	}
	return app, nil
}

// NeedsDBUpdate reports whether store.Applications.Update should run.
func NeedsDBUpdate(cur, want *domain.Application) bool {
	if cur.RepoID != want.RepoID ||
		cur.Path != want.Path ||
		cur.TargetRevision != want.TargetRevision ||
		cur.DestClusterID != want.DestClusterID ||
		cur.DestNamespace != want.DestNamespace ||
		cur.ParentApp != want.ParentApp {
		return true
	}
	if !bytes.Equal(bytes.TrimSpace(cur.HelmValuesJSON), bytes.TrimSpace(want.HelmValuesJSON)) {
		return true
	}
	if !slices.Equal(cur.HelmValueFiles, want.HelmValueFiles) {
		return true
	}
	if !syncPolicyEqual(cur.SyncPolicy, want.SyncPolicy) {
		return true
	}
	return false
}

func syncPolicyEqual(a, b domain.SyncPolicy) bool {
	if a.CreateNamespace != b.CreateNamespace {
		return false
	}
	if !slices.Equal(a.SyncOptions, b.SyncOptions) {
		return false
	}
	if !managedNSMetaEqual(a.ManagedNamespaceMetadata, b.ManagedNamespaceMetadata) {
		return false
	}
	if !ignoreDiffRulesEqual(a.IgnoreDifferences, b.IgnoreDifferences) {
		return false
	}
	aA, bA := a.Automated, b.Automated
	switch {
	case aA == nil && bA == nil:
	case aA == nil || bA == nil:
		return false
	default:
		if aA.Prune != bA.Prune || aA.SelfHeal != bA.SelfHeal {
			return false
		}
	}
	return true
}

func ignoreDiffRulesEqual(a, b []domain.IgnoreDifferenceRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ra, rb := a[i], b[i]
		if ra.Group != rb.Group || ra.Kind != rb.Kind || ra.Name != rb.Name || ra.Namespace != rb.Namespace {
			return false
		}
		if !slices.Equal(ra.JSONPointers, rb.JSONPointers) {
			return false
		}
	}
	return true
}

func managedNSMetaEqual(a, b *domain.ManagedNamespaceMetadata) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}
	return maps.Equal(a.Labels, b.Labels) && maps.Equal(a.Annotations, b.Annotations)
}

// ParseCatalogYAML parses a Git catalog file.
//
// The canonical format is a multi-document YAML file where each document is
// a orin.io/v1alpha1 or argoproj.io/v1alpha1 object (Application or
// AppProject).
//
// For backward compatibility, a single-document file with a top-level
// "applications:" or "projects:" list is also accepted, but its use is
// deprecated and a warning is logged on every parse.
//
// resolve is used to map Argo-style destination.server to a cluster name;
// it may be nil if the caller only works with orin.io-style destinations
// that use a cluster name directly.
func ParseCatalogYAML(data []byte, resolve ArgoDestinationResolve) (apps []Entry, projects []ProjectEntry, err error) {
	if resolve == nil {
		resolve = func(_, _ string) (string, error) { return "", nil }
	}
	docs := bytes.Split(data, []byte("\n---"))
	for _, doc := range docs {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		// Try to parse as a typed CRD object first.
		jsonBytes, convErr := k8syaml.YAMLToJSON(doc)
		if convErr != nil {
			continue
		}
		var raw map[string]interface{}
		if unmarshalErr := json.Unmarshal(jsonBytes, &raw); unmarshalErr != nil {
			continue
		}
		u := &unstructured.Unstructured{Object: raw}
		av := strings.ToLower(u.GetAPIVersion())
		isTyped := strings.HasPrefix(av, "orin.io") || strings.Contains(av, "argoproj.io")
		if isTyped {
			appEntry, projEntry, kind, ok, tryErr := TryEntryFromObject(u, resolve)
			if tryErr != nil || !ok {
				continue
			}
			switch kind {
			case EntryKindApplication:
				apps = append(apps, appEntry)
			case EntryKindAppProject:
				projects = append(projects, projEntry)
			}
			continue
		}

		// Backward-compat: top-level "applications:" or "projects:" list.
		var legacyDoc struct {
			Applications []Entry        `yaml:"applications"`
			Projects     []ProjectEntry `yaml:"projects"`
		}
		if unmarshalErr := yaml.Unmarshal(doc, &legacyDoc); unmarshalErr != nil {
			continue
		}
		if len(legacyDoc.Applications) > 0 || len(legacyDoc.Projects) > 0 {
			slog.Warn("appcatalog: legacy 'applications:'/'projects:' list format is deprecated; " +
				"migrate to orin.io/v1alpha1 Application and AppProject objects")
			apps = append(apps, legacyDoc.Applications...)
			projects = append(projects, legacyDoc.Projects...)
		}
	}
	return apps, projects, nil
}
