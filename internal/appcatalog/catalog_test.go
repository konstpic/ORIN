package appcatalog

import (
	"strings"
	"testing"
)

func TestParseApplicationsListYAML(t *testing.T) {
	y := `
applications:
  - name: a
    project: p1
    source:
      repoUrl: https://example.com/r.git
      path: charts/x
      targetRevision: main
      helmValues:
        replicaCount: 2
    destination:
      cluster: in-cluster
      namespace: ns1
    syncPolicy:
      automated:
        prune: true
        selfHeal: false
      createNamespace: true
      materializeChildApps: true
`
	entries, err := ParseApplicationsListYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len=%d", len(entries))
	}
	e := entries[0]
	if e.Name != "a" || e.Project != "p1" {
		t.Fatalf("meta: %+v", e)
	}
	if e.SyncPolicy == nil || e.SyncPolicy.CreateNamespace == nil || !*e.SyncPolicy.CreateNamespace {
		t.Fatalf("createNamespace: %+v", e.SyncPolicy)
	}
}

func TestParseApplicationsListYAML_SyncOptionsCreateNamespace(t *testing.T) {
	y := `
applications:
  - name: opt-ns
    project: default
    source:
      repoUrl: https://example.com/r.git
      path: "."
      targetRevision: HEAD
    destination:
      cluster: in-cluster
      namespace: ns-x
    syncPolicy:
      syncOptions:
        - CreateNamespace=true
`
	entries, err := ParseApplicationsListYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	pol := syncPolicyFromEntry(entries[0])
	if !pol.EffectiveCreateNamespace() {
		t.Fatalf("want EffectiveCreateNamespace, got %+v", pol)
	}
}

func TestParseApplicationsListYAMLEmpty(t *testing.T) {
	_, err := ParseApplicationsListYAML([]byte("applications: []"))
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want empty err, got %v", err)
	}
}
