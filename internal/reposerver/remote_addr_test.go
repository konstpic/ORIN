package reposerver

import "testing"

func TestIsRemoteRepoAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"", false},
		{":50051", false},
		{"orin-reposerver:50051", true},
		{"127.0.0.1:50051", true},
	}
	for _, tc := range tests {
		if got := isRemoteRepoAddr(tc.addr); got != tc.want {
			t.Errorf("isRemoteRepoAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
