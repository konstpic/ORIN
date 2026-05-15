package appcatalog

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/k8s-ui/k8s-ui/internal/domain"
)

// ProjectEntry is one row in a `projects:` list (Git catalog or embedded ConfigMap).
type ProjectEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Policies    struct {
		SourceRepos  []string `yaml:"sourceRepos,omitempty"`
		Destinations []struct {
			Server    string `yaml:"server,omitempty"`
			Name      string `yaml:"name,omitempty"`
			Namespace string `yaml:"namespace"`
		} `yaml:"destinations,omitempty"`
		ClusterResourceWhitelist []struct {
			Group string `yaml:"group"`
			Kind  string `yaml:"kind"`
		} `yaml:"clusterResourceWhitelist,omitempty"`
		NamespaceResourceBlacklist []struct {
			Group string `yaml:"group"`
			Kind  string `yaml:"kind"`
		} `yaml:"namespaceResourceBlacklist,omitempty"`
	} `yaml:"policies"`
}

// projectsDoc is the YAML document root for project lists.
type projectsDoc struct {
	Projects []ProjectEntry `yaml:"projects"`
}

// ParseProjectsListYAML parses a file/blob with top-level `projects:` list.
func ParseProjectsListYAML(data []byte) ([]ProjectEntry, error) {
	var d projectsDoc
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if len(d.Projects) == 0 {
		return nil, fmt.Errorf("projects: empty or missing list")
	}
	return d.Projects, nil
}

// DomainProjectFromEntry maps a catalog entry to a domain.Project.
func DomainProjectFromEntry(e ProjectEntry) (*domain.Project, error) {
	if e.Name == "" {
		return nil, fmt.Errorf("missing project name")
	}
	out := &domain.Project{
		Name:        e.Name,
		Description: e.Description,
	}
	out.Policies.SourceRepos = slices.Clone(e.Policies.SourceRepos)
	for _, d := range e.Policies.Destinations {
		out.Policies.Destinations = append(out.Policies.Destinations, domain.ProjectDestination{
			Server:    d.Server,
			Name:      d.Name,
			Namespace: d.Namespace,
		})
	}
	for _, r := range e.Policies.ClusterResourceWhitelist {
		out.Policies.ClusterResourceWhitelist = append(out.Policies.ClusterResourceWhitelist, domain.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	for _, r := range e.Policies.NamespaceResourceBlacklist {
		out.Policies.NamespaceResourceBlacklist = append(out.Policies.NamespaceResourceBlacklist, domain.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	return out, nil
}

// ProjectNeedsDBUpdate reports whether store.Projects.Update should run.
func ProjectNeedsDBUpdate(cur, want *domain.Project) bool {
	if cur.Description != want.Description {
		return true
	}
	a, b := cur.Policies, want.Policies
	if !slices.Equal(a.SourceRepos, b.SourceRepos) {
		return true
	}
	if len(a.Destinations) != len(b.Destinations) {
		return true
	}
	for i := range a.Destinations {
		if a.Destinations[i] != b.Destinations[i] {
			return true
		}
	}
	if !equalResourceRules(a.ClusterResourceWhitelist, b.ClusterResourceWhitelist) {
		return true
	}
	if !equalResourceRules(a.NamespaceResourceBlacklist, b.NamespaceResourceBlacklist) {
		return true
	}
	return false
}

func equalResourceRules(a, b []domain.ProjectResourceRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
