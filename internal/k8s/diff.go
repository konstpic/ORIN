package k8s

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

// ResourceDiff represents one normalized diff entry.
type ResourceDiff struct {
	Group       string
	Version     string
	Kind        string
	Namespace   string
	Name        string
	Synced      bool
	DesiredYAML string
	LiveYAML    string
	UnifiedDiff string
}

// DiffSet is the aggregated diff for an application.
type DiffSet struct {
	Items     []ResourceDiff
	OutOfSync int
	Synced    int
}

// Diff compares desired vs live and produces a normalized DiffSet.
// Live objects with no matching desired counterpart are ignored here (prune
// detection lives in the sync executor where the previous applied set is
// known).
//
// rules optionally suppresses diff on specific JSON pointer paths
// (Argo-compatible ignoreDifferences).
func Diff(desired []*unstructured.Unstructured, live []*unstructured.Unstructured, rules ...[]domain.IgnoreDifferenceRule) (*DiffSet, error) {
	var ignoreRules []domain.IgnoreDifferenceRule
	if len(rules) > 0 {
		ignoreRules = rules[0]
	}

	liveByKey := map[string]*unstructured.Unstructured{}
	for _, o := range live {
		liveByKey[objKey(o)] = o
	}

	ds := &DiffSet{}
	for _, d := range desired {
		key := objKey(d)
		l := liveByKey[key]

		dNorm := normalize(d)
		var lNorm *unstructured.Unstructured
		if l != nil {
			lNorm = normalize(l)
		}

		// Apply ignoreDifferences rules: strip matching JSON pointer paths from
		// both sides before comparison so those fields never cause OutOfSync.
		matching := matchingIgnoreRules(d, ignoreRules)
		if len(matching) > 0 {
			dNorm = applyIgnorePointers(dNorm, matching)
			if lNorm != nil {
				lNorm = applyIgnorePointers(lNorm, matching)
			}
		}

		desiredYAML, err := toYAML(dNorm)
		if err != nil {
			return nil, err
		}
		var liveYAML string
		if lNorm != nil {
			liveYAML, err = toYAML(lNorm)
			if err != nil {
				return nil, err
			}
		}

		synced := lNorm != nil && structuralEqual(dNorm.Object, lNorm.Object)
		item := ResourceDiff{
			Group:       d.GroupVersionKind().Group,
			Version:     d.GroupVersionKind().Version,
			Kind:        d.GetKind(),
			Namespace:   d.GetNamespace(),
			Name:        d.GetName(),
			Synced:      synced,
			DesiredYAML: desiredYAML,
			LiveYAML:    liveYAML,
		}
		if !synced {
			item.UnifiedDiff = simpleUnifiedDiff(liveYAML, desiredYAML)
			ds.OutOfSync++
		} else {
			ds.Synced++
		}
		ds.Items = append(ds.Items, item)
	}
	return ds, nil
}

// matchingIgnoreRules returns the subset of rules that apply to obj.
func matchingIgnoreRules(obj *unstructured.Unstructured, rules []domain.IgnoreDifferenceRule) []domain.IgnoreDifferenceRule {
	if len(rules) == 0 {
		return nil
	}
	group := obj.GroupVersionKind().Group
	kind := obj.GetKind()
	name := obj.GetName()
	ns := obj.GetNamespace()

	var out []domain.IgnoreDifferenceRule
	for _, r := range rules {
		if r.Group != group {
			continue
		}
		if r.Kind != kind {
			continue
		}
		if r.Name != "" && r.Name != name {
			continue
		}
		if r.Namespace != "" && r.Namespace != ns {
			continue
		}
		out = append(out, r)
	}
	return out
}

// applyIgnorePointers removes the listed JSON pointer paths from a deep copy of obj.
func applyIgnorePointers(obj *unstructured.Unstructured, rules []domain.IgnoreDifferenceRule) *unstructured.Unstructured {
	out := obj.DeepCopy()
	for _, r := range rules {
		for _, ptr := range r.JSONPointers {
			removeJSONPointer(out.Object, ptr)
		}
	}
	return out
}

// removeJSONPointer removes the field at the RFC 6901 path from the map in-place.
// Silently does nothing if the path does not exist.
func removeJSONPointer(obj map[string]interface{}, pointer string) {
	// Unescape per RFC 6901: ~1 → "/" and ~0 → "~"
	unescape := func(s string) string {
		s = strings.ReplaceAll(s, "~1", "/")
		s = strings.ReplaceAll(s, "~0", "~")
		return s
	}

	pointer = strings.TrimPrefix(pointer, "/")
	if pointer == "" {
		return
	}
	parts := strings.Split(pointer, "/")
	for i, p := range parts {
		parts[i] = unescape(p)
	}

	cur := obj
	for i, key := range parts[:len(parts)-1] {
		next, ok := cur[key]
		if !ok {
			return
		}
		switch v := next.(type) {
		case map[string]interface{}:
			cur = v
		default:
			_ = i
			return
		}
	}
	delete(cur, parts[len(parts)-1])
}

func objKey(o *unstructured.Unstructured) string {
	g := o.GroupVersionKind().Group
	k := o.GetKind()
	ns := o.GetNamespace()
	name := o.GetName()
	return fmt.Sprintf("%s/%s/%s/%s", g, k, ns, name)
}

// normalize strips server-managed and irrelevant fields so that a structural
// equality check answers "did the user's intent change?" rather than "did
// the cluster mutate any byte?".
func normalize(u *unstructured.Unstructured) *unstructured.Unstructured {
	out := u.DeepCopy()
	// Remove server-managed metadata.
	unstructured.RemoveNestedField(out.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(out.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(out.Object, "metadata", "uid")
	unstructured.RemoveNestedField(out.Object, "metadata", "generation")
	unstructured.RemoveNestedField(out.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(out.Object, "metadata", "selfLink")
	// Status is owned by the cluster.
	unstructured.RemoveNestedField(out.Object, "status")
	// Annotations injected by the apiserver.
	if anns, ok, _ := unstructured.NestedStringMap(out.Object, "metadata", "annotations"); ok {
		delete(anns, "kubectl.kubernetes.io/last-applied-configuration")
		delete(anns, "deployment.kubernetes.io/revision")
		if len(anns) == 0 {
			unstructured.RemoveNestedField(out.Object, "metadata", "annotations")
		} else {
			_ = unstructured.SetNestedStringMap(out.Object, anns, "metadata", "annotations")
		}
	}
	// Clean up nil/empty maps that the apiserver fills in.
	if creationTimestamp, ok := out.Object["metadata"].(map[string]any); ok {
		if v, ok := creationTimestamp["creationTimestamp"]; ok && v == nil {
			delete(creationTimestamp, "creationTimestamp")
		}
	}
	return out
}

func structuralEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(canonicalize(a), canonicalize(b))
}

// canonicalize recursively returns a comparable form: nil-empty maps/slices
// collapse to nil, slices are returned in stable order if all entries are
// scalars, and nested maps are recursed.
func canonicalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			return nil
		}
		out := make(map[string]any, len(t))
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c := canonicalize(t[k])
			if c == nil {
				continue
			}
			out[k] = c
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		if len(t) == 0 {
			return nil
		}
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = canonicalize(e)
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return t
	case nil:
		return nil
	default:
		return t
	}
}

func toYAML(u *unstructured.Unstructured) (string, error) {
	b, err := yaml.Marshal(u.Object)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// simpleUnifiedDiff is a tiny line-based diff suitable for the UI; it isn't
// a real Myers diff but it shows added/removed/changed lines clearly enough
// for human eyeballing. The frontend renders the real diff via Monaco; this
// is mostly for CLI/log debugging.
func simpleUnifiedDiff(a, b string) string {
	var buf bytes.Buffer
	la, lb := splitLines(a), splitLines(b)
	max := len(la)
	if len(lb) > max {
		max = len(lb)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(la) {
			av = la[i]
		}
		if i < len(lb) {
			bv = lb[i]
		}
		if av == bv {
			fmt.Fprintf(&buf, "  %s\n", av)
			continue
		}
		if av != "" {
			fmt.Fprintf(&buf, "- %s\n", av)
		}
		if bv != "" {
			fmt.Fprintf(&buf, "+ %s\n", bv)
		}
	}
	return buf.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
