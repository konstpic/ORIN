package repoclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/k8s-ui/k8s-ui/internal/git"
	"github.com/k8s-ui/k8s-ui/internal/reposerver"
	pb "github.com/k8s-ui/k8s-ui/proto/reposerver"
)

// GRPCClient calls a remote reposerver over gRPC.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.RepoServerClient
}

// NewGRPCClient connects to a remote reposerver.
func NewGRPCClient(ctx context.Context, addr string) (*GRPCClient, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &GRPCClient{
		conn:   conn,
		client: pb.NewRepoServerClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error { return c.conn.Close() }

func (c *GRPCClient) Render(ctx context.Context, req RenderRequest) (*reposerver.RenderResult, error) {
	resp, err := c.client.Render(ctx, &pb.RenderRequest{
		App: toAppRef(req),
	})
	if err != nil {
		return nil, err
	}
	objs, err := yamlStrsToObjects(resp.Manifests)
	if err != nil {
		return nil, err
	}
	return &reposerver.RenderResult{Revision: resp.Revision, Objects: objs}, nil
}

func (c *GRPCClient) RenderSHA(ctx context.Context, req RenderRequest, sha string) (*reposerver.RenderResult, error) {
	resp, err := c.client.RenderSHA(ctx, &pb.RenderSHARequest{
		App: toAppRef(req),
		Sha: sha,
	})
	if err != nil {
		return nil, err
	}
	objs, err := yamlStrsToObjects(resp.Manifests)
	if err != nil {
		return nil, err
	}
	return &reposerver.RenderResult{Revision: resp.Revision, Objects: objs}, nil
}

func (c *GRPCClient) ResolveRevision(ctx context.Context, req ResolveRequest) (string, error) {
	resp, err := c.client.ResolveRevision(ctx, &pb.ResolveRequest{
		RepoUrl:           req.RepoURL,
		TargetRevision:    req.TargetRevision,
		Username:          grpcCreds(req.Credentials).Username,
		Password:          grpcCreds(req.Credentials).Password,
		SshPrivateKey:     grpcCreds(req.Credentials).SSHPrivKey,
	})
	if err != nil {
		return "", err
	}
	return resp.Sha, nil
}

func (c *GRPCClient) ListCommits(ctx context.Context, req ListCommitsRequest) ([]git.CommitInfo, error) {
	resp, err := c.client.ListCommits(ctx, &pb.ListCommitsRequest{
		RepoUrl:               req.RepoURL,
		Path:                  req.Path,
		Limit:                 int32(req.Limit),
		Username:              grpcCreds(req.Credentials).Username,
		Password:              grpcCreds(req.Credentials).Password,
		SshPrivateKey:         grpcCreds(req.Credentials).SSHPrivKey,
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

func (c *GRPCClient) CommitForRevision(ctx context.Context, req CommitRequest) (*git.CommitInfo, error) {
	resp, err := c.client.GetCommit(ctx, &pb.GetCommitRequest{
		RepoUrl:               req.RepoURL,
		Sha:                   req.SHA,
		Username:              grpcCreds(req.Credentials).Username,
		Password:              grpcCreds(req.Credentials).Password,
		SshPrivateKey:         grpcCreds(req.Credentials).SSHPrivKey,
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

func (c *GRPCClient) DiffPaths(ctx context.Context, req DiffPathsRequest) (string, error) {
	resp, err := c.client.DiffPaths(ctx, &pb.DiffPathsRequest{
		RepoUrl:               req.RepoURL,
		Path:                  req.Path,
		FromSha:               req.FromSHA,
		ToSha:                 req.ToSHA,
		Username:              grpcCreds(req.Credentials).Username,
		Password:              grpcCreds(req.Credentials).Password,
		SshPrivateKey:         grpcCreds(req.Credentials).SSHPrivKey,
	})
	if err != nil {
		return "", err
	}
	return resp.Diff, nil
}

func (c *GRPCClient) ReadRawFile(ctx context.Context, req ReadRawFileRequest) ([]byte, error) {
	resp, err := c.client.ReadRawFile(ctx, &pb.ReadRawFileRequest{
		RepoUrl:               req.RepoURL,
		Revision:              req.Revision,
		RelPath:               req.RelPath,
		Username:              grpcCreds(req.Credentials).Username,
		Password:              grpcCreds(req.Credentials).Password,
		SshPrivateKey:         grpcCreds(req.Credentials).SSHPrivKey,
	})
	if err != nil {
		return nil, err
	}
	return resp.Content, nil
}

func toAppRef(req RenderRequest) *pb.AppRef {
	ref := &pb.AppRef{
		RepoUrl:         req.RepoURL,
		TargetRevision:  req.TargetRevision,
		Path:            req.Path,
		AppNameLabel:    req.AppName,
		DestNamespace:   req.DestNamespace,
		HelmValueFiles:  req.HelmValueFiles,
		HelmValuesJson:  req.HelmValuesJSON,
	}
	if req.Credentials != nil {
		ref.Username = req.Credentials.Username
		ref.Password = req.Credentials.Password
		ref.SshPrivateKey = req.Credentials.SSHPrivKey
	}
	return ref
}

func grpcCreds(c *Credentials) *Credentials {
	if c == nil {
		return &Credentials{}
	}
	return c
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
