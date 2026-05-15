// Package reposerver renders Git repository contents into a list of
// unstructured Kubernetes manifests at a pinned revision. In the MVP it is
// in-process; a future split moves this behind gRPC.
package reposerver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-ui/k8s-ui/internal/config"
	"github.com/k8s-ui/k8s-ui/internal/crypto"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/git"
	"github.com/k8s-ui/k8s-ui/internal/manifest"
	"github.com/k8s-ui/k8s-ui/internal/store"
)

// Server renders manifests for an application at a target revision, caching
// rendered output by (repoURL, revisionSHA, path).
type Server struct {
	cfg    *config.Config
	store  *store.Store
	cipher *crypto.Cipher
	cache  *git.Cache
	render *lru.Cache[string, []*unstructured.Unstructured]
	mu     sync.Mutex
}

// New constructs the Server.
func New(cfg *config.Config, st *store.Store, cipher *crypto.Cipher) (*Server, error) {
	c, err := git.NewCache(cfg.RepoCacheDir)
	if err != nil {
		return nil, fmt.Errorf("git cache: %w", err)
	}
	renderCache, err := lru.New[string, []*unstructured.Unstructured](256)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, store: st, cipher: cipher, cache: c, render: renderCache}, nil
}

// RenderResult describes one rendered revision.
type RenderResult struct {
	Revision string
	Objects  []*unstructured.Unstructured
}

// ResolveRevision returns the commit SHA for the application's targetRevision
// after fetching latest refs.
func (s *Server) ResolveRevision(ctx context.Context, app *domain.Application) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return "", err
	}
	return s.cache.Resolve(bareDir, app.TargetRevision)
}

// RenderForApp returns the rendered manifest list for the application at the
// pinned revision (target ref resolved to a SHA), with tracking labels
// applied.
func (s *Server) RenderForApp(ctx context.Context, app *domain.Application) (*RenderResult, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return nil, err
	}
	sha, err := s.cache.Resolve(bareDir, app.TargetRevision)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s|%s|%s", repo.URL, sha, app.Path)
	if cached, ok := s.render.Get(key); ok {
		return &RenderResult{Revision: sha, Objects: cloneObjs(cached)}, nil
	}

	rctx, cancel := context.WithTimeout(ctx, s.cfg.RepoRenderTimeout)
	defer cancel()
	objs, err := s.renderAtSHA(rctx, bareDir, sha, app)
	if err != nil {
		return nil, err
	}
	s.render.Add(key, objs)
	return &RenderResult{Revision: sha, Objects: cloneObjs(objs)}, nil
}

// RenderForAppSHA renders manifests at an explicit commit SHA.
func (s *Server) RenderForAppSHA(ctx context.Context, app *domain.Application, sha string) (*RenderResult, error) {
	if sha == "" {
		return nil, fmt.Errorf("empty revision sha")
	}
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s|%s|%s", repo.URL, sha, app.Path)
	if cached, ok := s.render.Get(key); ok {
		return &RenderResult{Revision: sha, Objects: cloneObjs(cached)}, nil
	}
	rctx, cancel := context.WithTimeout(ctx, s.cfg.RepoRenderTimeout)
	defer cancel()
	objs, err := s.renderAtSHA(rctx, bareDir, sha, app)
	if err != nil {
		return nil, err
	}
	s.render.Add(key, objs)
	return &RenderResult{Revision: sha, Objects: cloneObjs(objs)}, nil
}

// ListCommitsForApp returns recent commits optionally scoped to app.Path.
func (s *Server) ListCommitsForApp(ctx context.Context, app *domain.Application, limit int) ([]git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return nil, err
	}
	return s.cache.RecentCommits(bareDir, app.Path, limit)
}

// CommitForRevision resolves app.TargetRevision to a SHA and returns the
// commit metadata. Returns nil commit (no error) when the SHA is not found.
func (s *Server) CommitForRevision(ctx context.Context, app *domain.Application, sha string) (*git.CommitInfo, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return nil, err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return nil, err
	}
	return s.cache.CommitByHash(bareDir, sha)
}

// DiffGitPaths returns a unified diff for app.Path between two commits.
func (s *Server) DiffGitPaths(ctx context.Context, app *domain.Application, fromSHA, toSHA string) (string, error) {
	repo, err := s.loadRepo(ctx, app.RepoID)
	if err != nil {
		return "", err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return "", err
	}
	return s.cache.DiffCommitRange(bareDir, app.Path, fromSHA, toSHA)
}

func (s *Server) renderAtSHA(_ context.Context, bareDir, sha string, app *domain.Application) ([]*unstructured.Unstructured, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	work, err := s.cache.Checkout(bareDir, sha)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	target := work
	if app.Path != "" && app.Path != "." && app.Path != "/" {
		target = fmt.Sprintf("%s/%s", work, app.Path)
	}
	renderer, err := manifest.Detect(target, manifest.RenderContext{
		AppName:        app.Name,
		DestNamespace:  app.DestNamespace,
		HelmValueFiles: app.HelmValueFiles,
		HelmValuesJSON: app.HelmValuesJSON,
	})
	if err != nil {
		return nil, err
	}
	objs, err := renderer.Render(target)
	if err != nil {
		return nil, err
	}
	manifest.ApplyTracking(objs, app.Name, app.DestNamespace)
	return objs, nil
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
	if relPath == "" || relPath == "." {
		return nil, fmt.Errorf("read raw file: empty path")
	}
	repoRec, err := s.store.Repositories.GetByURL(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	repo, err := s.loadRepo(ctx, repoRec.ID)
	if err != nil {
		return nil, err
	}
	bareDir, err := s.cache.CloneOrFetch(repo.URL, repo.Credentials)
	if err != nil {
		return nil, err
	}
	sha, err := s.cache.Resolve(bareDir, revision)
	if err != nil {
		return nil, err
	}
	work, err := s.cache.Checkout(bareDir, sha)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()

	rel := filepath.ToSlash(filepath.Clean(relPath))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("read raw file: path %q escapes repo root", relPath)
	}
	full := filepath.Join(work, filepath.FromSlash(rel))
	relOut, err := filepath.Rel(work, full)
	if err != nil || strings.HasPrefix(relOut, "..") {
		return nil, fmt.Errorf("read raw file: path %q escapes worktree", relPath)
	}
	_ = ctx // reserved for per-call timeout at call sites
	return os.ReadFile(full)
}

func cloneObjs(in []*unstructured.Unstructured) []*unstructured.Unstructured {
	out := make([]*unstructured.Unstructured, 0, len(in))
	for _, o := range in {
		out = append(out, o.DeepCopy())
	}
	return out
}

// CacheAge is for tests/diagnostics.
func (s *Server) CacheAge() time.Duration { return 0 }
