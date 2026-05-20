// Package grpcserver implements the reposerver gRPC service. It runs as a
// standalone process (the `reposerver` subcommand) and exposes git rendering
// and introspection over gRPC so that it can be scaled independently.
package grpcserver

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/reposerver"
	pb "github.com/orin/orin/proto/reposerver"
)

// Server is the gRPC service implementation. It wraps Engine for all
// rendering logic and has no DB dependency.
type Server struct {
	pb.UnimplementedRepoServerServer
	engine *reposerver.Engine
}

// New constructs the gRPC server.
func New(engine *reposerver.Engine) *Server {
	return &Server{engine: engine}
}

// ListenAndServe starts the gRPC listener and blocks until the server stops.
func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterRepoServerServer(grpcServer, s)
	fmt.Printf("reposerver: listening on %s\n", addr)
	return grpcServer.Serve(lis)
}

func protoCreds(r *pb.AppRef) *domain.RepoCreds {
	if r == nil {
		return nil
	}
	return &domain.RepoCreds{
		Username:              r.Username,
		Password:              r.Password,
		SSHPrivKey:         r.SshPrivateKey,
	}
}

func simpleCreds(repoURL, username, password, sshKey string) *domain.RepoCreds {
	return &domain.RepoCreds{
		Username:    username,
		Password:    password,
		SSHPrivKey:  sshKey,
	}
}

func (s *Server) Render(ctx context.Context, req *pb.RenderRequest) (*pb.RenderResponse, error) {
	app := req.App
	if app == nil {
		return nil, status.Errorf(codes.InvalidArgument, "app is required")
	}
	result, err := s.engine.Render(ctx, reposerver.RenderOpts{
		RepoURL:        app.RepoUrl,
		TargetRevision: app.TargetRevision,
		Path:           app.Path,
		AppName:        app.AppNameLabel,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: app.HelmValuesJson,
		Credentials:    protoCreds(app),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "render: %v", err)
	}
	return &pb.RenderResponse{
		Revision:  result.Revision,
		Manifests: objsToYAMLs(result.Objects),
	}, nil
}

func (s *Server) RenderSHA(ctx context.Context, req *pb.RenderSHARequest) (*pb.RenderResponse, error) {
	app := req.App
	if app == nil {
		return nil, status.Errorf(codes.InvalidArgument, "app is required")
	}
	if req.Sha == "" {
		return nil, status.Errorf(codes.InvalidArgument, "sha is required")
	}
	result, err := s.engine.RenderSHA(ctx, reposerver.RenderOpts{
		RepoURL:        app.RepoUrl,
		Path:           app.Path,
		AppName:        app.AppNameLabel,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: app.HelmValuesJson,
		Credentials:    protoCreds(app),
	}, req.Sha)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "render: %v", err)
	}
	return &pb.RenderResponse{
		Revision:  result.Revision,
		Manifests: objsToYAMLs(result.Objects),
	}, nil
}

func (s *Server) ResolveRevision(ctx context.Context, req *pb.ResolveRequest) (*pb.ResolveResponse, error) {
	sha, err := s.engine.ResolveRevision(ctx, req.RepoUrl, req.TargetRevision,
		simpleCreds(req.RepoUrl, req.Username, req.Password, req.SshPrivateKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve: %v", err)
	}
	return &pb.ResolveResponse{Sha: sha}, nil
}

func (s *Server) ListCommits(ctx context.Context, req *pb.ListCommitsRequest) (*pb.ListCommitsResponse, error) {
	commits, err := s.engine.ListCommits(ctx, req.RepoUrl, req.Path, int(req.Limit),
		simpleCreds(req.RepoUrl, req.Username, req.Password, req.SshPrivateKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list commits: %v", err)
	}
	out := make([]*pb.CommitInfo, 0, len(commits))
	for _, c := range commits {
		out = append(out, &pb.CommitInfo{
			Sha:            c.SHA,
			ShortSha:       c.ShortSHA,
			Message:        c.Message,
			Author:         c.Author,
			AuthorDateUnix: c.AuthorDate.Unix(),
		})
	}
	return &pb.ListCommitsResponse{Commits: out}, nil
}

func (s *Server) GetCommit(ctx context.Context, req *pb.GetCommitRequest) (*pb.GetCommitResponse, error) {
	ci, err := s.engine.CommitByHash(ctx, req.RepoUrl, req.Sha,
		simpleCreds(req.RepoUrl, req.Username, req.Password, req.SshPrivateKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get commit: %v", err)
	}
	if ci == nil {
		return &pb.GetCommitResponse{}, nil
	}
	return &pb.GetCommitResponse{Commit: &pb.CommitInfo{
		Sha:            ci.SHA,
		ShortSha:       ci.ShortSHA,
		Message:        ci.Message,
		Author:         ci.Author,
		AuthorDateUnix: ci.AuthorDate.Unix(),
	}}, nil
}

func (s *Server) DiffPaths(ctx context.Context, req *pb.DiffPathsRequest) (*pb.DiffPathsResponse, error) {
	diff, err := s.engine.DiffCommitRange(ctx, req.RepoUrl, req.Path, req.FromSha, req.ToSha,
		simpleCreds(req.RepoUrl, req.Username, req.Password, req.SshPrivateKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "diff: %v", err)
	}
	return &pb.DiffPathsResponse{Diff: diff}, nil
}

func (s *Server) ReadRawFile(ctx context.Context, req *pb.ReadRawFileRequest) (*pb.ReadRawFileResponse, error) {
	content, err := s.engine.ReadRawFile(ctx, req.RepoUrl, req.Revision, req.RelPath,
		simpleCreds(req.RepoUrl, req.Username, req.Password, req.SshPrivateKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read file: %v", err)
	}
	return &pb.ReadRawFileResponse{Content: content}, nil
}

func objsToYAMLs(objs []*unstructured.Unstructured) []string {
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		yml, err := yaml.Marshal(o.Object)
		if err != nil {
			continue
		}
		out = append(out, string(yml))
	}
	return out
}
