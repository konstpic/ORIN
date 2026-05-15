package domain

import (
	"strings"
)

// ManagedNamespaceMetadata is merged into the Namespace object when sync creates it
// (Argo CD spec.syncPolicy.managedNamespaceMetadata).
type ManagedNamespaceMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// EffectiveCreateNamespace is true if the legacy bool is set or syncOptions contains
// CreateNamespace=true (Argo CD sync option, case-insensitive key).
func (p SyncPolicy) EffectiveCreateNamespace() bool {
	if p.CreateNamespace {
		return true
	}
	for _, raw := range p.SyncOptions {
		if parseSyncOptionBool(raw, "CreateNamespace") {
			return true
		}
	}
	return false
}

func parseSyncOptionBool(option, key string) bool {
	option = strings.TrimSpace(option)
	if option == "" {
		return false
	}
	k, v, cut := strings.Cut(option, "=")
	if !cut {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(k), key) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(v), "true")
}
