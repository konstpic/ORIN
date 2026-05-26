package reposerver

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/git"
	pb "github.com/orin/orin/proto/reposerver"
)

// remoteEngine delegates Git/Helm work to a standalone reposerver over gRPC.
type remoteEngine struct {
	conn   *grpc.ClientConn
	client pb.RepoServerClient
}

func dialRemoteEngine(ctx context.Context, addr string) (*remoteEngine, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &remoteEngine{conn: conn, client: pb.NewRepoServerClient(conn)}, nil
}

func (r *remoteEngine) Close() error { return r.conn.Close() }

func (r *remoteEngine) Render(ctx context.Context, opts RenderOpts) (*RenderResult, error) {
	resp, err := r.client.Render(ctx, &pb.RenderRequest{App: appRefFromOpts(opts)})
	if err != nil {
		return nil, err
	}
	objs, err := yamlStrsToObjects(resp.Manifests)
	if err != nil {
		return nil, err
	}
	return &RenderResult{Revision: resp.Revision, Objects: objs}, nil
}

func (r *remoteEngine) RenderSHA(ctx context.Context, opts RenderOpts, sha string) (*RenderResult, error) {
	resp, err := r.client.RenderSHA(ctx, &pb.RenderSHARequest{App: appRefFromOpts(opts), Sha: sha})
	if err != nil {
		return nil, err
	}
	objs, err := yamlStrsToObjects(resp.Manifests)
	if err != nil {
		return nil, err
	}
	return &RenderResult{Revision: resp.Revision, Objects: objs}, nil
}

func (r *remoteEngine) ResolveRevision(ctx context.Context, repoURL, targetRevision string, creds *domain.RepoCreds) (string, error) {
	resp, err := r.client.ResolveRevision(ctx, &pb.ResolveRequest{
		RepoUrl:        repoURL,
		TargetRevision: targetRevision,
		Username:       credsField(creds, func(c *domain.RepoCreds) string { return c.Username }),
		Password:       credsField(creds, func(c *domain.RepoCreds) string { return c.Password }),
		SshPrivateKey:  credsField(creds, func(c *domain.RepoCreds) string { return c.SSHPrivKey }),
	})
	if err != nil {
		return "", err
	}
	return resp.Sha, nil
}

func (r *remoteEngine) ListCommits(ctx context.Context, repoURL, path string, limit int, creds *domain.RepoCreds) ([]git.CommitInfo, error) {
	resp, err := r.client.ListCommits(ctx, &pb.ListCommitsRequest{
		RepoUrl:       repoURL,
		Path:          path,
		Limit:         int32(limit),
		Username:      credsField(creds, func(c *domain.RepoCreds) string { return c.Username }),
		Password:      credsField(creds, func(c *domain.RepoCreds) string { return c.Password }),
		SshPrivateKey: credsField(creds, func(c *domain.RepoCreds) string { return c.SSHPrivKey }),
	})
	if err != nil {
		return nil, err
	}
	out := make([]git.CommitInfo, 0, len(resp.Commits))
	for _, ci := range resp.Commits {
		out = append(out, git.CommitInfo{
			SHA:        ci.Sha,
			ShortSHA:   ci.ShortSha,
			Message:    ci.Message,
			Author:     ci.Author,
			AuthorDate: timeFromUnix(ci.AuthorDateUnix),
		})
	}
	return out, nil
}

func (r *remoteEngine) CommitByHash(ctx context.Context, repoURL, sha string, creds *domain.RepoCreds) (*git.CommitInfo, error) {
	resp, err := r.client.GetCommit(ctx, &pb.GetCommitRequest{
		RepoUrl:       repoURL,
		Sha:           sha,
		Username:      credsField(creds, func(c *domain.RepoCreds) string { return c.Username }),
		Password:      credsField(creds, func(c *domain.RepoCreds) string { return c.Password }),
		SshPrivateKey: credsField(creds, func(c *domain.RepoCreds) string { return c.SSHPrivKey }),
	})
	if err != nil {
		return nil, err
	}
	if resp.Commit == nil || resp.Commit.Sha == "" {
		return nil, nil
	}
	return &git.CommitInfo{
		SHA:        resp.Commit.Sha,
		ShortSHA:   resp.Commit.ShortSha,
		Message:    resp.Commit.Message,
		Author:     resp.Commit.Author,
		AuthorDate: timeFromUnix(resp.Commit.AuthorDateUnix),
	}, nil
}

func (r *remoteEngine) DiffCommitRange(ctx context.Context, repoURL, path, fromSHA, toSHA string, creds *domain.RepoCreds) (string, error) {
	resp, err := r.client.DiffPaths(ctx, &pb.DiffPathsRequest{
		RepoUrl:       repoURL,
		Path:          path,
		FromSha:       fromSHA,
		ToSha:         toSHA,
		Username:      credsField(creds, func(c *domain.RepoCreds) string { return c.Username }),
		Password:      credsField(creds, func(c *domain.RepoCreds) string { return c.Password }),
		SshPrivateKey: credsField(creds, func(c *domain.RepoCreds) string { return c.SSHPrivKey }),
	})
	if err != nil {
		return "", err
	}
	return resp.Diff, nil
}

func (r *remoteEngine) ReadRawFile(ctx context.Context, repoURL, revision, relPath string, creds *domain.RepoCreds) ([]byte, error) {
	resp, err := r.client.ReadRawFile(ctx, &pb.ReadRawFileRequest{
		RepoUrl:       repoURL,
		Revision:      revision,
		RelPath:       relPath,
		Username:      credsField(creds, func(c *domain.RepoCreds) string { return c.Username }),
		Password:      credsField(creds, func(c *domain.RepoCreds) string { return c.Password }),
		SshPrivateKey: credsField(creds, func(c *domain.RepoCreds) string { return c.SSHPrivKey }),
	})
	if err != nil {
		return nil, err
	}
	return resp.Content, nil
}

func appRefFromOpts(opts RenderOpts) *pb.AppRef {
	ref := &pb.AppRef{
		RepoUrl:        opts.RepoURL,
		TargetRevision: opts.TargetRevision,
		Path:           opts.Path,
		AppNameLabel:   opts.AppName,
		DestNamespace:  opts.DestNamespace,
		HelmValueFiles: opts.HelmValueFiles,
		HelmValuesJson: opts.HelmValuesJSON,
	}
	if opts.Credentials != nil {
		ref.Username = opts.Credentials.Username
		ref.Password = opts.Credentials.Password
		ref.SshPrivateKey = opts.Credentials.SSHPrivKey
	}
	return ref
}

func credsField(c *domain.RepoCreds, f func(*domain.RepoCreds) string) string {
	if c == nil {
		return ""
	}
	return f(c)
}

func timeFromUnix(unix int64) time.Time {
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

func yamlStrsToObjects(manifests []string) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	for _, yml := range manifests {
		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(yml), &m); err != nil {
			continue
		}
		if len(m) == 0 {
			continue
		}
		objs = append(objs, &unstructured.Unstructured{Object: m})
	}
	return objs, nil
}
