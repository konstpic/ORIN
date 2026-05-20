package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/orin/orin/internal/appcatalog"
	"github.com/orin/orin/internal/store"
)

// ArgoImportResult is one line of the import response.
type ArgoImportResult struct {
	Name     string   `json:"name"`
	Action   string   `json:"action"` // created | updated | skipped | error
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// importArgoApplications handles POST /api/v1/argo-import.
//
// The request body must be a Kubernetes YAML document produced by:
//
//	kubectl get applications.argoproj.io -A -o yaml
//
// or any number of individual Application manifest files concatenated with "---".
//
// Query parameter ?apply=true commits the changes. Without it the call is a
// dry-run that returns only the plan and warnings.
func (s *Server) importArgoApplications(w http.ResponseWriter, r *http.Request) {
	apply := strings.EqualFold(r.URL.Query().Get("apply"), "true")

	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}

	objs, err := splitArgoYAML(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse_yaml", err.Error())
		return
	}
	if len(objs) == 0 {
		writeError(w, http.StatusBadRequest, "no_applications", "no argoproj.io Application manifests found in request body")
		return
	}

	resolve := s.argoImportResolver(r)

	var results []ArgoImportResult
	for _, obj := range objs {
		result := s.importOneArgoApp(r, obj, resolve, apply)
		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) importOneArgoApp(
	r *http.Request,
	obj *unstructured.Unstructured,
	resolve appcatalog.ArgoDestinationResolve,
	apply bool,
) ArgoImportResult {
	entry, ok, err := appcatalog.TryArgoApplicationEntry(obj, resolve)
	if !ok || err != nil {
		name := obj.GetName()
		if name == "" {
			name = "<unknown>"
		}
		msg := "not an Argo Application"
		if err != nil {
			msg = err.Error()
		}
		return ArgoImportResult{Name: name, Action: "error", Error: msg}
	}

	var warnings []string

	// Detect multi-source and record a warning.
	if spec, ok2, _ := unstructured.NestedMap(obj.Object, "spec"); ok2 {
		if arr, ok3 := spec["sources"].([]interface{}); ok3 && len(arr) > 1 {
			warnings = append(warnings, fmt.Sprintf(
				"spec.sources has %d entries — only the first source was imported; create separate Applications for the remaining sources",
				len(arr),
			))
		}
	}

	// Detect unsupported source features.
	warnings = append(warnings, argoSourceWarnings(obj)...)

	if !apply {
		return ArgoImportResult{Name: entry.Name, Action: "dry-run", Warnings: warnings}
	}

	want, err := appcatalog.DomainFromEntry(r.Context(), s.opts.Store, entry)
	if err != nil {
		return ArgoImportResult{Name: entry.Name, Action: "error", Error: err.Error(), Warnings: warnings}
	}

	cur, err := s.opts.Store.Applications.GetByName(r.Context(), want.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return ArgoImportResult{Name: entry.Name, Action: "error", Error: err.Error(), Warnings: warnings}
		}
		if err := s.opts.Store.Applications.Create(r.Context(), want); err != nil {
			return ArgoImportResult{Name: entry.Name, Action: "error", Error: err.Error(), Warnings: warnings}
		}
		s.opts.Controller.EnqueueStatus(want.Name)
		return ArgoImportResult{Name: entry.Name, Action: "created", Warnings: warnings}
	}

	up := *cur
	up.RepoID = want.RepoID
	up.Path = want.Path
	up.TargetRevision = want.TargetRevision
	up.DestClusterID = want.DestClusterID
	up.DestNamespace = want.DestNamespace
	up.HelmValuesJSON = want.HelmValuesJSON
	up.SyncPolicy = want.SyncPolicy

	if !appcatalog.NeedsDBUpdate(cur, &up) {
		return ArgoImportResult{Name: entry.Name, Action: "skipped", Warnings: warnings}
	}
	if err := s.opts.Store.Applications.Update(r.Context(), &up); err != nil {
		return ArgoImportResult{Name: entry.Name, Action: "error", Error: err.Error(), Warnings: warnings}
	}
	s.opts.Controller.EnqueueStatus(want.Name)
	return ArgoImportResult{Name: entry.Name, Action: "updated", Warnings: warnings}
}

// argoImportResolver returns an ArgoDestinationResolve backed by registered clusters.
func (s *Server) argoImportResolver(r *http.Request) appcatalog.ArgoDestinationResolve {
	return func(server, name string) (string, error) {
		name = strings.TrimSpace(name)
		server = strings.TrimSpace(server)
		if name != "" {
			if _, err := s.opts.Store.Clusters.GetByName(r.Context(), name); err == nil {
				return name, nil
			}
			return "", fmt.Errorf("unknown cluster name %q (Argo destination.name); register the cluster first", name)
		}
		if server == "" {
			return "", fmt.Errorf("Argo destination needs server or name")
		}
		clusters, err := s.opts.Store.Clusters.List(r.Context())
		if err != nil {
			return "", err
		}
		norm := strings.TrimSuffix(server, "/")
		for _, cl := range clusters {
			if strings.TrimSuffix(cl.ServerURL, "/") == norm {
				return cl.Name, nil
			}
		}
		if strings.Contains(norm, "kubernetes.default") {
			if _, err := s.opts.Store.Clusters.GetByName(r.Context(), "in-cluster"); err == nil {
				return "in-cluster", nil
			}
		}
		return "", fmt.Errorf("no registered cluster matches Argo destination.server %q; register the cluster first", server)
	}
}

// argoSourceWarnings inspects an Argo Application and returns human-readable
// warnings for source features that orin does not support.
func argoSourceWarnings(obj *unstructured.Unstructured) []string {
	var w []string
	spec, ok, _ := unstructured.NestedMap(obj.Object, "spec")
	if !ok {
		return nil
	}

	checkSource := func(src map[string]interface{}) {
		if plugin, ok2, _ := unstructured.NestedMap(src, "plugin"); ok2 && len(plugin) > 0 {
			w = append(w, "spec.source.plugin (Config Management Plugin) is not supported — pre-render manifests in CI")
		}
		if helm, ok2, _ := unstructured.NestedMap(src, "helm"); ok2 {
			// repoURL pointing to a Helm repository instead of a Git path
			if repoURL := strings.TrimSpace(strOrEmpty(spec, "repoURL")); repoURL != "" {
				if looksLikeHelmRepo(repoURL) {
					w = append(w, "source appears to be a Helm repository URL (not a Git URL); Helm repo / OCI sources are not yet supported — mirror the chart into Git")
				}
			}
			if _, ok3, _ := unstructured.NestedString(helm, "version"); ok3 {
				w = append(w, "spec.source.helm.version is ignored — pin the chart in Git")
			}
			if _, ok3, _ := unstructured.NestedString(helm, "releaseName"); ok3 {
				w = append(w, "spec.source.helm.releaseName is ignored — release name is derived from the Application name")
			}
		}
		if kustomize, ok2, _ := unstructured.NestedMap(src, "kustomize"); ok2 && len(kustomize) > 0 {
			w = append(w, "spec.source.kustomize overlays (images, patches, namePrefix, etc.) are not supported — commit the overlay into the repository")
		}
	}

	if arr, ok2 := spec["sources"].([]interface{}); ok2 {
		for _, item := range arr {
			if m, ok3 := item.(map[string]interface{}); ok3 {
				checkSource(m)
			}
		}
	} else if src, ok2, _ := unstructured.NestedMap(spec, "source"); ok2 {
		checkSource(src)
	}
	return w
}

func strOrEmpty(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func looksLikeHelmRepo(url string) bool {
	return strings.HasPrefix(url, "oci://") ||
		(!strings.HasSuffix(url, ".git") &&
			!strings.Contains(url, "github.com") &&
			!strings.Contains(url, "gitlab.com") &&
			!strings.Contains(url, "bitbucket.org"))
}

// splitArgoYAML splits a multi-document YAML byte slice and returns only the
// objects that are argoproj.io Applications (or are wrapped in a List).
func splitArgoYAML(data []byte) ([]*unstructured.Unstructured, error) {
	var out []*unstructured.Unstructured
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var raw map[string]interface{}
		jsonBytes, err := yaml.YAMLToJSON(doc)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(jsonBytes, &raw); err != nil {
			continue
		}
		u := &unstructured.Unstructured{Object: raw}

		// Unwrap a List (kubectl get -o yaml produces an ItemList).
		if u.GetKind() == "List" {
			items, _, _ := unstructured.NestedSlice(u.Object, "items")
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					child := &unstructured.Unstructured{Object: m}
					if isArgoApplication(child) {
						out = append(out, child)
					}
				}
			}
			continue
		}

		if isArgoApplication(u) {
			out = append(out, u)
		}
	}
	return out, nil
}

func isArgoApplication(u *unstructured.Unstructured) bool {
	av := strings.ToLower(u.GetAPIVersion())
	return strings.Contains(av, "argoproj.io") && u.GetKind() == "Application"
}
