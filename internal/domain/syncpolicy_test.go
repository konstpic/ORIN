package domain

import "testing"

func TestEffectiveCreateNamespace(t *testing.T) {
	cases := []struct {
		name string
		p    SyncPolicy
		want bool
	}{
		{"empty", SyncPolicy{}, false},
		{"legacy true", SyncPolicy{CreateNamespace: true}, true},
		{"option true", SyncPolicy{SyncOptions: []string{"CreateNamespace=true"}}, true},
		{"option case", SyncPolicy{SyncOptions: []string{"  createnamespace=TRUE  "}}, true},
		{"option false", SyncPolicy{SyncOptions: []string{"CreateNamespace=false"}}, false},
		{"other option", SyncPolicy{SyncOptions: []string{"Validate=false"}}, false},
		{"legacy wins with false option", SyncPolicy{CreateNamespace: true, SyncOptions: []string{"CreateNamespace=false"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if g := tc.p.EffectiveCreateNamespace(); g != tc.want {
				t.Fatalf("EffectiveCreateNamespace() = %v, want %v", g, tc.want)
			}
		})
	}
}
