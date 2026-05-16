// Package reposerver renders Git repository contents into a list of
// unstructured Kubernetes manifests at a pinned revision.
//
// Server wraps Engine + DB-backed credential resolution for in-process use.
// Engine can also be used standalone (gRPC repo server).
package reposerver

import (
	"context"
	"fmt"
	"time"

	"github.com/k8s-ui/k8s-ui/internal/config"
	"github.com/k8s-ui/k8s-ui/internal/crypto"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/git"
	"github.com/k8s-ui/k8s-ui/internal/store"
)

// Server renders manifests for an application, loading credentials from the
// database. It delegates the actual Git/rendering work to Engine.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	cipher *crypto.Cipher
	engine *Engine
}

// New constructs the Server.
func New(cfg *config.Config, st *store.Store, cipher *crypto.Cipher) (*Server, error) {
	engine, err := NewEngine(cfg.RepoCacheDir)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, store: st, cipher: cipher, engine: engine}, nil
}

// Engine returns the underlying rendering engine for direct use.
func (s *Server) Engine() *Engine { return s.engine }

// ResolveRevision returns the commit SHA for the application's targetRevision
// after fetching latest refs.
func (s *Server) ResolveRevision(ctx context.Context, app *domain.Application) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	return s.engine.ResolveRevision(ctx, repo.URL, app.TargetRevision, repo.Credentials)
}

// RenderForApp returns the rendered manifest list for the application at the
// pinned revision (target ref resolved to a SHA), with tracking labels
// applied.
func (s *Server) RenderForApp(ctx context.Context, app *domain.Application) (*RenderResult, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.engine.Render(ctx, RenderOpts{
		RepoURL:        repo.URL,
		TargetRevision: app.TargetRevision,
		Path:           app.Path,
		AppName:        app.Name,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: string(app.HelmValuesJSON),
		Credentials:    repo.Credentials,
	})
}

// RenderForAppSHA renders manifests at an explicit commit SHA.
func (s *Server) RenderForAppSHA(ctx context.Context, app *domain.Application, sha string) (*RenderResult, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.engine.RenderSHA(ctx, RenderOpts{
		RepoURL:        repo.URL,
		Path:           app.Path,
		AppName:        app.Name,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: string(app.HelmValuesJSON),
		Credentials:    repo.Credentials,
	}, sha)
}

// ListCommitsForApp returns recent commits optionally scoped to app.Path.
func (s *Server) ListCommitsForApp(ctx context.Context, app *domain.Application, limit int) ([]git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.engine.ListCommits(ctx, repo.URL, app.Path, limit, repo.Credentials)
}

// CommitForRevision resolves app.TargetRevision to a SHA and returns the
// commit metadata. Returns nil commit (no error) when the SHA is not found.
func (s *Server) CommitForRevision(ctx context.Context, app *domain.Application, sha string) (*git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	return s.engine.CommitByHash(ctx, repo.URL, sha, repo.Credentials)
}

// DiffGitPaths returns a unified diff for app.Path between two commits.
func (s *Server) DiffGitPaths(ctx context.Context, app *domain.Application, fromSHA, toSHA string) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	return s.engine.DiffCommitRange(ctx, repo.URL, app.Path, fromSHA, toSHA, repo.Credentials)
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
	return s.engine.ReadRawFile(ctx, repo.URL, revision, relPath, repo.Credentials)
}

// CacheAge is for tests/diagnostics.
func (s *Server) CacheAge() time.Duration { return 0 }

