package appcatalog

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// tryAppProjectEntry maps an unstructured AppProject object (orin.io or
// argoproj.io) to a ProjectEntry.  Both groups use the same spec shape
// (description, policies.sourceRepos, policies.destinations, etc.) mirroring
// Argo CD AppProject.
func tryAppProjectEntry(u *unstructured.Unstructured) (ProjectEntry, bool, error) {
	name := strings.TrimSpace(u.GetName())
	if name == "" {
		return ProjectEntry{}, false, fmt.Errorf("app project: missing metadata.name")
	}
	spec, ok, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil || !ok {
		// spec is optional; an AppProject with no spec is valid (no constraints).
		spec = map[string]interface{}{}
	}

	var pe ProjectEntry
	pe.Name = name
	pe.Description = strings.TrimSpace(strFromMap(spec, "description"))

	// sourceRepos
	if repos, ok2, _ := unstructured.NestedStringSlice(spec, "sourceRepos"); ok2 {
		pe.Policies.SourceRepos = append([]string(nil), repos...)
	}

	// destinations — array of {server, name, namespace}
	if dests, ok2, _ := unstructured.NestedSlice(spec, "destinations"); ok2 {
		for _, raw := range dests {
			m, ok3 := raw.(map[string]interface{})
			if !ok3 {
				continue
			}
			pe.Policies.Destinations = append(pe.Policies.Destinations, struct {
				Server    string `yaml:"server,omitempty"`
				Name      string `yaml:"name,omitempty"`
				Namespace string `yaml:"namespace"`
			}{
				Server:    strings.TrimSpace(strFromMap(m, "server")),
				Name:      strings.TrimSpace(strFromMap(m, "name")),
				Namespace: strings.TrimSpace(strFromMap(m, "namespace")),
			})
		}
	}

	// clusterResourceWhitelist — array of {group, kind}
	if wl, ok2, _ := unstructured.NestedSlice(spec, "clusterResourceWhitelist"); ok2 {
		for _, raw := range wl {
			m, ok3 := raw.(map[string]interface{})
			if !ok3 {
				continue
			}
			pe.Policies.ClusterResourceWhitelist = append(
				pe.Policies.ClusterResourceWhitelist,
				struct {
					Group string `yaml:"group"`
					Kind  string `yaml:"kind"`
				}{
					Group: strings.TrimSpace(strFromMap(m, "group")),
					Kind:  strings.TrimSpace(strFromMap(m, "kind")),
				},
			)
		}
	}

	// namespaceResourceBlacklist — array of {group, kind}
	if bl, ok2, _ := unstructured.NestedSlice(spec, "namespaceResourceBlacklist"); ok2 {
		for _, raw := range bl {
			m, ok3 := raw.(map[string]interface{})
			if !ok3 {
				continue
			}
			pe.Policies.NamespaceResourceBlacklist = append(
				pe.Policies.NamespaceResourceBlacklist,
				struct {
					Group string `yaml:"group"`
					Kind  string `yaml:"kind"`
				}{
					Group: strings.TrimSpace(strFromMap(m, "group")),
					Kind:  strings.TrimSpace(strFromMap(m, "kind")),
				},
			)
		}
	}

	return pe, true, nil
}
