package manifest

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IsControlPlaneObject reports whether an unstructured object belongs to a
// k8s-ui or Argo control-plane API group and is a child-declaration kind
// (Application or AppProject).
//
// These objects appear in rendered manifests when a parent chart uses the
// app-of-apps pattern to declare child applications or projects.  They are
// parsed by the controller to upsert rows into the database but must NOT be
// applied to the destination cluster (which has no CRD for them).
func IsControlPlaneObject(u *unstructured.Unstructured) bool {
	if u == nil {
		return false
	}
	av := strings.ToLower(u.GetAPIVersion())
	if !strings.HasPrefix(av, "k8s-ui.io") && !strings.Contains(av, "argoproj.io") {
		return false
	}
	kind := u.GetKind()
	return kind == "Application" || kind == "AppProject"
}

// FilterApplicable removes control-plane objects from a slice, returning only
// the objects that should be applied to the destination cluster.
func FilterApplicable(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	out := make([]*unstructured.Unstructured, 0, len(objs))
	for _, o := range objs {
		if !IsControlPlaneObject(o) {
			out = append(out, o)
		}
	}
	return out
}
