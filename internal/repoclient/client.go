// Package repoclient provides an abstraction over the repo server so that
// callers (controller, API) can work with either an in-process reposerver
// (all-in-one mode) or a remote gRPC reposerver (scaled mode).
package repoclient

import (
	"context"

	"github.com/k8s-ui/k8s-ui/internal/git"
	"github.com/k8s-ui/k8s-ui/internal/reposerver"
)

// Client is the interface used by controller and API to interact with the
// repository server. It works for both in-process and gRPC modes.
type Client interface {
	Render(ctx context.Context, req RenderRequest) (*reposerver.RenderResult, error)
	RenderSHA(ctx context.Context, req RenderRequest, sha string) (*reposerver.RenderResult, error)
	ResolveRevision(ctx context.Context, req ResolveRequest) (string, error)
	ListCommits(ctx context.Context, req ListCommitsRequest) ([]git.CommitInfo, error)
	CommitForRevision(ctx context.Context, req CommitRequest) (*git.CommitInfo, error)
	DiffPaths(ctx context.Context, req DiffPathsRequest) (string, error)
	ReadRawFile(ctx context.Context, req ReadRawFileRequest) ([]byte, error)
}

// RenderRequest carries the information needed to render manifests.
type RenderRequest struct {
	RepoURL        string
	TargetRevision string
	Path           string
	AppName        string
	DestNamespace  string
	HelmValueFiles []string
	HelmValuesJSON string
	Credentials    *Credentials
}

// ResolveRequest resolves a symbolic ref to a SHA.
type ResolveRequest struct {
	RepoURL        string
	TargetRevision string
	Credentials    *Credentials
}

// ListCommitsRequest returns recent commits.
type ListCommitsRequest struct {
	RepoURL     string
	Path        string
	Limit       int
	Credentials *Credentials
}

// CommitRequest returns metadata for a specific SHA.
type CommitRequest struct {
	RepoURL     string
	SHA         string
	Credentials *Credentials
}

// DiffPathsRequest returns a unified diff between two SHAs.
type DiffPathsRequest struct {
	RepoURL     string
	Path        string
	FromSHA     string
	ToSHA       string
	Credentials *Credentials
}

// ReadRawFileRequest reads a single file from a repo.
type ReadRawFileRequest struct {
	RepoURL     string
	Revision    string
	RelPath     string
	Credentials *Credentials
}

// Credentials holds git authentication details.
type Credentials struct {
	Username            string
	Password            string
	SSHPrivKey       string
}
