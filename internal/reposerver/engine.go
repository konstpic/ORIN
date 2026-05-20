// Package reposerver renders manifests from Git repositories.
//
// The package provides two ways to access rendering:
//
//   - Server: in-process mode with DB-backed credential loading (all-in-one)
//   - Engine: standalone mode with explicit credentials (gRPC / scalable)
//
// Engine is the core rendering logic. Server wraps Engine + DB credential
// resolution. The gRPC server (internal/grpcserver) also wraps Engine.
package reposerver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/git"
	"github.com/orin/orin/internal/manifest"
)

// RenderResult describes one rendered revision.
type RenderResult struct {
	Revision string
	Objects  []*unstructured.Unstructured
}

// Engine is the core Git rendering engine. It has no DB dependency — all
// credentials are passed explicitly. This allows it to run as a standalone
// gRPC service or embedded in-process.
type Engine struct {
	cache  *git.Cache
	render *lru.Cache[string, []*unstructured.Unstructured]
	mu     sync.Mutex
}

// NewEngine creates a rendering engine backed by a local Git cache.
func NewEngine(cacheDir string) (*Engine, error) {
	c, err := git.NewCache(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("git cache: %w", err)
	}
	renderCache, err := lru.New[string, []*unstructured.Unstructured](512)
	if err != nil {
		return nil, err
	}
	return &Engine{cache: c, render: renderCache}, nil
}

// RenderOpts carries the information needed to render manifests.
type RenderOpts struct {
	RepoURL        string
	TargetRevision string
	Path           string
	AppName        string
	DestNamespace  string
	HelmValueFiles []string
	HelmValuesJSON string
	Credentials    *domain.RepoCreds
}

// Render resolves the target revision and renders manifests.
func (e *Engine) Render(ctx context.Context, opts RenderOpts) (*RenderResult, error) {
	bareDir, err := e.cache.CloneOrFetch(opts.RepoURL, opts.Credentials)
	if err != nil {
		return nil, err
	}
	sha, err := e.cache.Resolve(bareDir, opts.TargetRevision)
	if err != nil {
		return nil, err
	}
	return e.renderSHA(ctx, bareDir, sha, opts)
}

// RenderSHA renders manifests at an explicit commit SHA.
func (e *Engine) RenderSHA(ctx context.Context, opts RenderOpts, sha string) (*RenderResult, error) {
	if sha == "" {
		return nil, fmt.Errorf("empty revision sha")
	}
	bareDir, err := e.cache.CloneOrFetch(opts.RepoURL, opts.Credentials)
	if err != nil {
		return nil, err
	}
	return e.renderSHA(ctx, bareDir, sha, opts)
}

// ResolveResolution resolves a symbolic ref to a commit SHA.
func (e *Engine) ResolveRevision(ctx context.Context, repoURL, targetRevision string, creds *domain.RepoCreds) (string, error) {
	bareDir, err := e.cache.CloneOrFetch(repoURL, creds)
	if err != nil {
		return "", err
	}
	return e.cache.Resolve(bareDir, targetRevision)
}

// ListCommits returns recent commits for a repo path.
func (e *Engine) ListCommits(ctx context.Context, repoURL, path string, limit int, creds *domain.RepoCreds) ([]git.CommitInfo, error) {
	bareDir, err := e.cache.CloneOrFetch(repoURL, creds)
	if err != nil {
		return nil, err
	}
	return e.cache.RecentCommits(bareDir, path, limit)
}

// CommitByHash returns commit metadata for a specific SHA.
func (e *Engine) CommitByHash(ctx context.Context, repoURL, sha string, creds *domain.RepoCreds) (*git.CommitInfo, error) {
	bareDir, err := e.cache.CloneOrFetch(repoURL, creds)
	if err != nil {
		return nil, err
	}
	return e.cache.CommitByHash(bareDir, sha)
}

// DiffCommitRange returns a unified diff between two SHAs for a path.
func (e *Engine) DiffCommitRange(ctx context.Context, repoURL, path, fromSHA, toSHA string, creds *domain.RepoCreds) (string, error) {
	bareDir, err := e.cache.CloneOrFetch(repoURL, creds)
	if err != nil {
		return "", err
	}
	return e.cache.DiffCommitRange(bareDir, path, fromSHA, toSHA)
}

// ReadRawFile reads a file from a repo at a given revision.
func (e *Engine) ReadRawFile(ctx context.Context, repoURL, revision, relPath string, creds *domain.RepoCreds) ([]byte, error) {
	if relPath == "" || relPath == "." {
		return nil, fmt.Errorf("read raw file: empty path")
	}
	bareDir, err := e.cache.CloneOrFetch(repoURL, creds)
	if err != nil {
		return nil, err
	}
	sha, err := e.cache.Resolve(bareDir, revision)
	if err != nil {
		return nil, err
	}
	work, err := e.cache.Checkout(bareDir, sha)
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
	return os.ReadFile(full)
}

func (e *Engine) renderSHA(ctx context.Context, bareDir, sha string, opts RenderOpts) (*RenderResult, error) {
	key := fmt.Sprintf("%s|%s|%s", opts.RepoURL, sha, opts.Path)
	if cached, ok := e.render.Get(key); ok {
		return &RenderResult{Revision: sha, Objects: cloneObjs(cached)}, nil
	}

	e.mu.Lock()
	work, err := e.cache.Checkout(bareDir, sha)
	if err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.mu.Unlock()

	defer os.RemoveAll(work)

	target := work
	if opts.Path != "" && opts.Path != "." && opts.Path != "/" {
		target = fmt.Sprintf("%s/%s", work, opts.Path)
	}

	renderer, err := manifest.Detect(target, manifest.RenderContext{
		AppName:        opts.AppName,
		DestNamespace:  opts.DestNamespace,
		HelmValueFiles: opts.HelmValueFiles,
		HelmValuesJSON: []byte(opts.HelmValuesJSON),
	})
	if err != nil {
		return nil, err
	}
	objs, err := renderer.Render(target)
	if err != nil {
		return nil, err
	}
	manifest.ApplyTracking(objs, opts.AppName, opts.DestNamespace)

	e.render.Add(key, objs)
	return &RenderResult{Revision: sha, Objects: cloneObjs(objs)}, nil
}

func cloneObjs(in []*unstructured.Unstructured) []*unstructured.Unstructured {
	out := make([]*unstructured.Unstructured, 0, len(in))
	for _, o := range in {
		out = append(out, o.DeepCopy())
	}
	return out
}
