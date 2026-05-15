package appcatalog

import (
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gopkg.in/yaml.v3"
)

// ArgoDestinationResolve maps Argo spec.destination.{server,name} to a k8s-ui cluster name.
type ArgoDestinationResolve func(server, name string) (clusterName string, err error)

// TryArgoApplicationEntry maps one unstructured Application (argoproj.io) to a catalog Entry.
// ok is false if the object is not an Argo Application or has no usable source.
func TryArgoApplicationEntry(u *unstructured.Unstructured, resolve ArgoDestinationResolve) (Entry, bool, error) {
	if u == nil || resolve == nil {
		return Entry{}, false, nil
	}
	av := strings.ToLower(u.GetAPIVersion())
	if !strings.Contains(av, "argoproj.io") || u.GetKind() != "Application" {
		return Entry{}, false, nil
	}
	name := strings.TrimSpace(u.GetName())
	if name == "" {
		return Entry{}, false, fmt.Errorf("argo application: missing metadata.name")
	}
	spec, ok, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil || !ok {
		return Entry{}, false, nil
	}
	source, ok := pickArgoSource(spec)
	if !ok {
		return Entry{}, false, nil
	}
	repoURL := firstString(source, "repoURL", "repoUrl")
	if repoURL == "" {
		return Entry{}, false, nil
	}
	dest, ok, err := unstructured.NestedMap(spec, "destination")
	if err != nil || !ok {
		return Entry{}, false, nil
	}
	destNS := strings.TrimSpace(strFromMap(dest, "namespace"))
	if destNS == "" {
		return Entry{}, false, nil
	}
	server := strings.TrimSpace(strFromMap(dest, "server"))
	destName := strings.TrimSpace(strFromMap(dest, "name"))
	clusterName, err := resolve(server, destName)
	if err != nil {
		return Entry{}, false, err
	}
	var e Entry
	e.Name = name
	e.Project = strings.TrimSpace(strFromMap(spec, "project"))
	if e.Project == "" {
		e.Project = "default"
	}
	e.Source.RepoURL = repoURL
	e.Source.Path = strings.TrimSpace(strFromMap(source, "path"))
	if e.Source.Path == "" {
		e.Source.Path = "."
	}
	e.Source.TargetRevision = strings.TrimSpace(strFromMap(source, "targetRevision"))
	if e.Source.TargetRevision == "" {
		e.Source.TargetRevision = "HEAD"
	}
	if hv := helmValuesFromArgoSource(source); hv != nil {
		e.Source.HelmValues = hv
	}
	if helm, ok2, _ := unstructured.NestedMap(source, "helm"); ok2 {
		if vfs, ok3, _ := unstructured.NestedStringSlice(helm, "valueFiles"); ok3 && len(vfs) > 0 {
			e.Source.HelmValueFiles = append([]string(nil), vfs...)
		}
	}
	e.Destination.Cluster = clusterName
	e.Destination.Namespace = destNS

	sp, ok, err := unstructured.NestedMap(spec, "syncPolicy")
	if err == nil && ok && len(sp) > 0 {
		e.SyncPolicy = &struct {
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
		}{}
		if auto, ok, _ := unstructured.NestedMap(sp, "automated"); ok && len(auto) > 0 {
			enabled := true
			if v, ok := auto["enabled"]; ok {
				if b, ok := v.(bool); ok {
					enabled = b
				}
			}
			if enabled {
				prune, _ := boolFromMap(auto, "prune")
				selfHeal, _ := boolFromMap(auto, "selfHeal")
				e.SyncPolicy.Automated = &struct {
					Prune    bool `yaml:"prune"`
					SelfHeal bool `yaml:"selfHeal"`
				}{Prune: prune, SelfHeal: selfHeal}
			}
		}
		if so, ok, _ := unstructured.NestedStringSlice(sp, "syncOptions"); ok && len(so) > 0 {
			e.SyncPolicy.SyncOptions = append([]string(nil), so...)
		}
		if mns, ok, _ := unstructured.NestedMap(sp, "managedNamespaceMetadata"); ok {
			labels, _, _ := unstructured.NestedStringMap(mns, "labels")
			ann, _, _ := unstructured.NestedStringMap(mns, "annotations")
			if len(labels) > 0 || len(ann) > 0 {
				e.SyncPolicy.ManagedNamespaceMetadata = &struct {
					Labels      map[string]string `yaml:"labels,omitempty"`
					Annotations map[string]string `yaml:"annotations,omitempty"`
				}{Labels: labels, Annotations: ann}
			}
		}
	}

	// Argo CD places ignoreDifferences at spec level (not inside syncPolicy).
	// Map them into the entry syncPolicy so they round-trip through DomainFromEntry.
	if idRaw, ok, _ := unstructured.NestedSlice(spec, "ignoreDifferences"); ok && len(idRaw) > 0 {
		if e.SyncPolicy == nil {
			e.SyncPolicy = &struct {
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
			}{}
		}
		for _, item := range idRaw {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			rule := EntryIgnoreDifferenceRule{
				Group:     strings.TrimSpace(strFromMap(m, "group")),
				Kind:      strings.TrimSpace(strFromMap(m, "kind")),
				Name:      strings.TrimSpace(strFromMap(m, "name")),
				Namespace: strings.TrimSpace(strFromMap(m, "namespace")),
			}
			if ptrs, ok2, _ := unstructured.NestedStringSlice(m, "jsonPointers"); ok2 {
				rule.JSONPointers = append([]string(nil), ptrs...)
			}
			e.SyncPolicy.IgnoreDifferences = append(e.SyncPolicy.IgnoreDifferences, rule)
		}
	}

	// Set legacy createNamespace when Argo uses only syncOptions (helps DB diff / older UI).
	if e.SyncPolicy != nil {
		pol := syncPolicyFromEntry(e)
		if pol.EffectiveCreateNamespace() && e.SyncPolicy.CreateNamespace == nil {
			t := true
			e.SyncPolicy.CreateNamespace = &t
		}
	}
	return e, true, nil
}

func pickArgoSource(spec map[string]interface{}) (map[string]interface{}, bool) {
	if arr, ok := spec["sources"].([]interface{}); ok && len(arr) > 0 {
		if len(arr) > 1 {
			// Only the first source is used. Log a structured warning so operators
			// can identify applications that need manual attention after migration.
			slog.Warn("argo application: spec.sources has multiple entries — only the first source is used; create separate k8s-ui Applications for the remaining sources",
				"source_count", len(arr))
		}
		if m, ok := arr[0].(map[string]interface{}); ok {
			return m, true
		}
	}
	if m, ok := spec["source"].(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v := strFromMap(m, k); v != "" {
			return v
		}
	}
	return ""
}

func strFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func boolFromMap(m map[string]interface{}, key string) (bool, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	switch x := v.(type) {
	case bool:
		return x, true
	default:
		return false, false
	}
}

func helmValuesFromArgoSource(source map[string]interface{}) interface{} {
	helm, ok, err := unstructured.NestedMap(source, "helm")
	if err != nil || !ok {
		return nil
	}
	if vo, ok := helm["valuesObject"]; ok && vo != nil {
		switch x := vo.(type) {
		case map[string]interface{}:
			return x
		default:
			return x
		}
	}
	if s, ok := helm["values"].(string); ok && strings.TrimSpace(s) != "" {
		var out interface{}
		if err := yaml.Unmarshal([]byte(s), &out); err == nil {
			return out
		}
	}
	return nil
}
