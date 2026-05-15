// Package project implements AppProject-style policy enforcement.
// Policies are checked on Application create, update, and sync.
package project

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

// Enforcer validates GitOps operations against project policies.
type Enforcer struct {
	project domain.Project
}

// NewEnforcer returns an Enforcer for the given project.
func NewEnforcer(p domain.Project) *Enforcer { return &Enforcer{project: p} }

// CheckSource returns an error if repoURL is not allowed by the project's
// sourceRepos whitelist. An empty whitelist (default project) allows any repo.
func (e *Enforcer) CheckSource(repoURL string) error {
	pol := e.project.Policies
	if len(pol.SourceRepos) == 0 {
		return nil
	}
	for _, allowed := range pol.SourceRepos {
		if allowed == "*" || matchGlob(allowed, repoURL) {
			return nil
		}
	}
	return fmt.Errorf("project %q: repository %q is not in sourceRepos whitelist", e.project.Name, repoURL)
}

// CheckDestination returns an error if the cluster/namespace pair is not
// permitted by the project's destinations list. An empty list allows any.
func (e *Enforcer) CheckDestination(clusterName, clusterServerURL, namespace string) error {
	pol := e.project.Policies
	if len(pol.Destinations) == 0 {
		return nil
	}
	for _, d := range pol.Destinations {
		clusterOK := d.Server == "*" || d.Name == "*" ||
			matchGlob(d.Name, clusterName) ||
			matchGlob(d.Server, clusterServerURL)
		nsOK := d.Namespace == "*" || matchGlob(d.Namespace, namespace)
		if clusterOK && nsOK {
			return nil
		}
	}
	return fmt.Errorf("project %q: destination cluster=%q namespace=%q is not in destinations whitelist", e.project.Name, clusterName, namespace)
}

// CheckManifests inspects a list of rendered objects for policy violations:
//   - cluster-scoped resources must be in clusterResourceWhitelist (if non-empty)
//   - namespace-scoped resources must not match namespaceResourceBlacklist
//
// Returns a slice of one error per violation (empty = OK).
func (e *Enforcer) CheckManifests(objs []*unstructured.Unstructured) []error {
	pol := e.project.Policies
	var errs []error
	for _, obj := range objs {
		ns := obj.GetNamespace()
		group := obj.GroupVersionKind().Group
		kind := obj.GetKind()

		if ns == "" {
			// Cluster-scoped resource.
			if len(pol.ClusterResourceWhitelist) > 0 {
				if !matchesResourceRules(pol.ClusterResourceWhitelist, group, kind) {
					errs = append(errs, fmt.Errorf("project %q: cluster-scoped %s/%s %q is not in clusterResourceWhitelist",
						e.project.Name, group, kind, obj.GetName()))
				}
			}
		} else {
			// Namespace-scoped resource.
			if matchesResourceRules(pol.NamespaceResourceBlacklist, group, kind) {
				errs = append(errs, fmt.Errorf("project %q: namespace-scoped %s/%s %q/%q is in namespaceResourceBlacklist",
					e.project.Name, group, kind, ns, obj.GetName()))
			}
		}
	}
	return errs
}

func matchesResourceRules(rules []domain.ProjectResourceRule, group, kind string) bool {
	for _, r := range rules {
		groupOK := r.Group == "*" || r.Group == group
		kindOK := r.Kind == "*" || r.Kind == kind
		if groupOK && kindOK {
			return true
		}
	}
	return false
}

// matchGlob is a minimal glob that supports a leading/trailing "*" wildcard.
func matchGlob(pattern, s string) bool {
	pattern = strings.TrimSpace(pattern)
	s = strings.TrimSpace(s)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(s, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(s, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return pattern == s
}
