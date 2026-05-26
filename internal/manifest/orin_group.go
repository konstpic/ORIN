package manifest

import "strings"

// IsOrinAPIGroup reports whether apiVersion belongs to an ORIN control-plane API group.
func IsOrinAPIGroup(apiVersion string) bool {
	av := strings.ToLower(apiVersion)
	return strings.HasPrefix(av, "orin.") || strings.HasPrefix(av, "k8s-ui.io")
}
