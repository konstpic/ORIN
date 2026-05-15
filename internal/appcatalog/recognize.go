package appcatalog

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EntryKind distinguishes Application entries from AppProject entries.
type EntryKind int

const (
	EntryKindApplication EntryKind = iota
	EntryKindAppProject
)

// isControlPlaneGroup returns true for API groups that k8s-ui owns and which
// must never be applied to the destination Kubernetes cluster.
func isControlPlaneGroup(apiVersion string) bool {
	av := strings.ToLower(apiVersion)
	return strings.HasPrefix(av, "k8s-ui.io") || strings.Contains(av, "argoproj.io")
}

// IsControlPlaneObject reports whether an unstructured object belongs to a
// k8s-ui or Argo control-plane group (Application or AppProject).  These
// objects are used to declare child resources but must not be applied to the
// destination cluster.
func IsControlPlaneObject(u *unstructured.Unstructured) bool {
	if u == nil {
		return false
	}
	if !isControlPlaneGroup(u.GetAPIVersion()) {
		return false
	}
	kind := u.GetKind()
	return kind == "Application" || kind == "AppProject"
}

// TryEntryFromObject attempts to parse u as either an Application entry or an
// AppProject entry.  It accepts objects from both the canonical k8s-ui.io
// group and the argoproj.io compat group.
//
// ok is false when the object is not a recognised control-plane kind.
// An error is returned only when the object matches a known kind but is
// structurally invalid (e.g. missing required fields).
func TryEntryFromObject(
	u *unstructured.Unstructured,
	resolve ArgoDestinationResolve,
) (appEntry Entry, projEntry ProjectEntry, kind EntryKind, ok bool, err error) {
	if u == nil || resolve == nil {
		return
	}
	av := strings.ToLower(u.GetAPIVersion())
	isKuiGroup := strings.HasPrefix(av, "k8s-ui.io")
	isArgoGroup := strings.Contains(av, "argoproj.io")
	if !isKuiGroup && !isArgoGroup {
		return
	}

	switch u.GetKind() {
	case "Application":
		e, eOk, eErr := tryApplicationEntry(u, resolve, isKuiGroup)
		return e, ProjectEntry{}, EntryKindApplication, eOk, eErr
	case "AppProject":
		pe, peOk, peErr := tryAppProjectEntry(u)
		return Entry{}, pe, EntryKindAppProject, peOk, peErr
	}
	return
}

// tryApplicationEntry parses an Application object from either the k8s-ui.io
// or argoproj.io group.  The k8s-ui.io spec uses spec.source.repoUrl (camelCase)
// while argoproj.io uses spec.source.repoURL.  The existing TryArgoApplicationEntry
// helper handles both cases via firstString, so we reuse it for both groups.
func tryApplicationEntry(u *unstructured.Unstructured, resolve ArgoDestinationResolve, isKuiGroup bool) (Entry, bool, error) {
	// For k8s-ui.io/Application the spec shape is identical to argoproj.io/Application
	// (same source/destination/syncPolicy layout).  We temporarily rewrite the
	// apiVersion so TryArgoApplicationEntry recognises it, then restore.
	if isKuiGroup {
		orig := u.GetAPIVersion()
		u.SetAPIVersion("argoproj.io/v1alpha1")
		e, ok, err := TryArgoApplicationEntry(u, resolve)
		u.SetAPIVersion(orig)
		return e, ok, err
	}
	return TryArgoApplicationEntry(u, resolve)
}
