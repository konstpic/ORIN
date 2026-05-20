package repoclient

import (
	"context"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/git"
	"github.com/orin/orin/internal/reposerver"
)

// InProcessClient wraps Engine for in-process use.
// This is used in all-in-one mode where the repo server runs in the same process.
type InProcessClient struct {
	engine *reposerver.Engine
}

// NewInProcessClient wraps an Engine.
func NewInProcessClient(engine *reposerver.Engine) *InProcessClient {
	return &InProcessClient{engine: engine}
}

func (c *InProcessClient) Render(ctx context.Context, req RenderRequest) (*reposerver.RenderResult, error) {
	return c.engine.Render(ctx, reposerver.RenderOpts{
		RepoURL:        req.RepoURL,
		TargetRevision: req.TargetRevision,
		Path:           req.Path,
		AppName:        req.AppName,
		DestNamespace:  req.DestNamespace,
		HelmValueFiles: req.HelmValueFiles,
		HelmValuesJSON: req.HelmValuesJSON,
		Credentials:    domainCreds(req.Credentials),
	})
}

func (c *InProcessClient) RenderSHA(ctx context.Context, req RenderRequest, sha string) (*reposerver.RenderResult, error) {
	return c.engine.RenderSHA(ctx, reposerver.RenderOpts{
		RepoURL:        req.RepoURL,
		Path:           req.Path,
		AppName:        req.AppName,
		DestNamespace:  req.DestNamespace,
		HelmValueFiles: req.HelmValueFiles,
		HelmValuesJSON: req.HelmValuesJSON,
		Credentials:    domainCreds(req.Credentials),
	}, sha)
}

func (c *InProcessClient) ResolveRevision(ctx context.Context, req ResolveRequest) (string, error) {
	return c.engine.ResolveRevision(ctx, req.RepoURL, req.TargetRevision, domainCreds(req.Credentials))
}

func (c *InProcessClient) ListCommits(ctx context.Context, req ListCommitsRequest) ([]git.CommitInfo, error) {
	return c.engine.ListCommits(ctx, req.RepoURL, req.Path, req.Limit, domainCreds(req.Credentials))
}

func (c *InProcessClient) CommitForRevision(ctx context.Context, req CommitRequest) (*git.CommitInfo, error) {
	return c.engine.CommitByHash(ctx, req.RepoURL, req.SHA, domainCreds(req.Credentials))
}

func (c *InProcessClient) DiffPaths(ctx context.Context, req DiffPathsRequest) (string, error) {
	return c.engine.DiffCommitRange(ctx, req.RepoURL, req.Path, req.FromSHA, req.ToSHA, domainCreds(req.Credentials))
}

func (c *InProcessClient) ReadRawFile(ctx context.Context, req ReadRawFileRequest) ([]byte, error) {
	return c.engine.ReadRawFile(ctx, req.RepoURL, req.Revision, req.RelPath, domainCreds(req.Credentials))
}

func domainCreds(c *Credentials) *domain.RepoCreds {
	if c == nil {
		return nil
	}
	return &domain.RepoCreds{
		Username:              c.Username,
		Password:              c.Password,
		SSHPrivKey:         c.SSHPrivKey,
	}
}
