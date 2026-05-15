package appcatalog

import (
	"testing"
)

func fixedCatalogResolver(name string) ArgoDestinationResolve {
	return func(_, _ string) (string, error) { return name, nil }
}

func TestParseCatalogYAML_MultiDoc(t *testing.T) {
	yaml := `apiVersion: k8s-ui.io/v1alpha1
kind: Application
metadata:
  name: app1
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    path: helm/app1
    targetRevision: main
  destination:
    name: in-cluster
    namespace: app1-ns
---
apiVersion: k8s-ui.io/v1alpha1
kind: AppProject
metadata:
  name: team-a
spec:
  description: team a
  sourceRepos:
    - "https://github.com/org/*"
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo2.git
    path: helm/app2
    targetRevision: HEAD
  destination:
    name: in-cluster
    namespace: app2-ns
`
	apps, projects, err := ParseCatalogYAML([]byte(yaml), fixedCatalogResolver("in-cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d: %v", len(apps), apps)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %v", len(projects), projects)
	}
	if apps[0].Name != "app1" || apps[1].Name != "app2" {
		t.Fatalf("app names: %v, %v", apps[0].Name, apps[1].Name)
	}
	if projects[0].Name != "team-a" {
		t.Fatalf("project name: %v", projects[0].Name)
	}
}

func TestParseCatalogYAML_LegacyFormat(t *testing.T) {
	yaml := `applications:
  - name: legacy-app
    project: default
    source:
      repoUrl: https://github.com/org/repo.git
      path: .
      targetRevision: HEAD
    destination:
      cluster: in-cluster
      namespace: ns
`
	apps, projects, err := ParseCatalogYAML([]byte(yaml), fixedCatalogResolver("in-cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 legacy app, got %d", len(apps))
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
	if apps[0].Name != "legacy-app" {
		t.Fatalf("name=%q", apps[0].Name)
	}
}

func TestParseCatalogYAML_Empty(t *testing.T) {
	apps, projects, err := ParseCatalogYAML([]byte(""), fixedCatalogResolver("in-cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 0 || len(projects) != 0 {
		t.Fatalf("expected empty, got %d apps %d projects", len(apps), len(projects))
	}
}

func TestParseCatalogYAML_SkipsNonCatalogDocs(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
data:
  key: value
---
apiVersion: k8s-ui.io/v1alpha1
kind: Application
metadata:
  name: real-app
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    path: .
    targetRevision: HEAD
  destination:
    name: in-cluster
    namespace: ns
`
	apps, _, err := ParseCatalogYAML([]byte(yaml), fixedCatalogResolver("in-cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "real-app" {
		t.Fatalf("name=%q", apps[0].Name)
	}
}
