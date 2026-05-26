package reposerver

import "net"

// isRemoteRepoAddr reports whether REPO_SERVER_ADDR points at a remote gRPC
// reposerver (scaled mode). Bare ":port" means this process listens locally.
func isRemoteRepoAddr(addr string) bool {
	if addr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	return host != ""
}
