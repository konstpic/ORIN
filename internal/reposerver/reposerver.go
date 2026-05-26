// Package reposerver renders Git repository contents into a list of
// unstructured Kubernetes manifests at a pinned revision.
//
// Server wraps Engine + DB-backed credential resolution for in-process use,
// or delegates to a remote gRPC reposerver when REPO_SERVER_ADDR is set.
package reposerver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/orin/orin/internal/config"
	"github.com/orin/orin/internal/crypto"
	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/git"
	"github.com/orin/orin/internal/store"
)

// Server renders manifests for an application, loading credentials from the
// database. Git work runs in-process (all-in-one) or on a remote reposerver.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	cipher *crypto.Cipher
	engine *Engine
	remote *remoteEngine
}

// New constructs the Server. When REPO_SERVER_ADDR is a remote host (e.g.
// orin-reposerver:50051), all Git/Helm work is delegated over gRPC.
func New(ctx context.Context, cfg *config.Config, st *store.Store, cipher *crypto.Cipher) (*Server, error) {
	addr := strings.TrimSpace(os.Getenv("REPO_SERVER_ADDR"))
	if isRemoteRepoAddr(addr) {
		remote, err := dialRemoteEngine(ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("connect repo server %s: %w", addr, err)
		}
		return &Server{cfg: cfg, store: st, cipher: cipher, remote: remote}, nil
	}
	engine, err := NewEngine(cfg.RepoCacheDir)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, store: st, cipher: cipher, engine: engine}, nil
}

// Close releases a remote gRPC connection when used.
func (s *Server) Close() error {
	if s.remote != nil {
		return s.remote.Close()
	}
	return nil
}

func (s *Server) backend() interface {
	Render(ctx context.Context, opts RenderOpts) (*RenderResult, error)
	RenderSHA(ctx context.Context, opts RenderOpts, sha string) (*RenderResult, error)
	ResolveRevision(ctx context.Context, repoURL, targetRevision string, creds *domain.RepoCreds) (string, error)
	ListCommits(ctx context.Context, repoURL, path string, limit int, creds *domain.RepoCreds) ([]git.CommitInfo, error)
	CommitByHash(ctx context.Context, repoURL, sha string, creds *domain.RepoCreds) (*git.CommitInfo, error)
	DiffCommitRange(ctx context.Context, repoURL, path, fromSHA, toSHA string, creds *domain.RepoCreds) (string, error)
	ReadRawFile(ctx context.Context, repoURL, revision, relPath string, creds *domain.RepoCreds) ([]byte, error)
} {
	if s.remote != nil {
		return s.remote
	}
	return s.engine
}

// ResolveRevision returns the commit SHA for the application's targetRevision
// after fetching latest refs.
func (s *Server) ResolveRevision(ctx context.Context, app *domain.Application) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	return s.backend().ResolveRevision(ctx, repo.URL, app.TargetRevision, repo.Credentials)
}

// RenderForApp returns the rendered manifest list for the application at the
// pinned revision (target ref resolved to a SHA), with tracking labels
// applied.
func (s *Server) RenderForApp(ctx context.Context, app *domain.Application) (*RenderResult, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.backend().Render(ctx, renderOpts(repo, app))
}

// RenderForAppSHA renders manifests at an explicit commit SHA.
func (s *Server) RenderForAppSHA(ctx context.Context, app *domain.Application, sha string) (*RenderResult, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.backend().RenderSHA(ctx, renderOpts(repo, app), sha)
}

// ListCommitsForApp returns recent commits optionally scoped to app.Path.
func (s *Server) ListCommitsForApp(ctx context.Context, app *domain.Application, limit int) ([]git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.backend().ListCommits(ctx, repo.URL, app.Path, limit, repo.Credentials)
}

// CommitForRevision resolves app.TargetRevision to a SHA and returns the
// commit metadata. Returns nil commit (no error) when the SHA is not found.
func (s *Server) CommitForRevision(ctx context.Context, app *domain.Application, sha string) (*git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.backend().CommitByHash(ctx, repo.URL, sha, repo.Credentials)
}

// DiffGitPaths returns a unified diff for app.Path between two commits.
func (s *Server) DiffGitPaths(ctx context.Context, app *domain.Application, fromSHA, toSHA string) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	return s.backend().DiffCommitRange(ctx, repo.URL, app.Path, fromSHA, toSHA, repo.Credentials)
}

func (s *Server) loadRepo(ctx context.Context, repoID string) (*domain.Repository, error) {
	repo, err := s.store.Repositories.GetByID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if len(repo.CredentialsEncrypted) > 0 {
		plain, err := s.cipher.Decrypt(repo.CredentialsEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt repo creds: %w", err)
		}
		creds := &domain.RepoCreds{}
		if err := decodeCreds(plain, creds); err != nil {
			return nil, err
		}
		repo.Credentials = creds
	}
	return repo, nil
}

// ReadRawFile returns the bytes of relPath (relative to the repository root,
// using slash-separated components) at the given symbolic revision. repoURL
// must match a registered repository row.
func (s *Server) ReadRawFile(ctx context.Context, repoURL, revision, relPath string) ([]byte, error) {
	repoRec, err := s.store.Repositories.GetByURL(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	repo, err := s.loadRepo(ctx, repoRec.ID)
	if err != nil {
		return nil, err
	}
	return s.backend().ReadRawFile(ctx, repo.URL, revision, relPath, repo.Credentials)
}

// CacheAge is for tests/diagnostics.
func (s *Server) CacheAge() time.Duration { return 0 }

func renderOpts(repo *domain.Repository, app *domain.Application) RenderOpts {
	return RenderOpts{
		RepoURL:        repo.URL,
		TargetRevision: app.TargetRevision,
		Path:           app.Path,
		AppName:        app.Name,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: string(app.HelmValuesJSON),
		Credentials:    repo.Credentials,
	}
}
