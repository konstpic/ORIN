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
	// Plugin, when non-nil, overrides auto-detected rendering (Helm/Kustomize/plain)
	// with a CMP-style external command executed in the checkout directory.
	Plugin *domain.Plugin
	// PluginEnv carries per-application env overrides merged on top of Plugin.Env.
	PluginEnv []domain.EnvVar
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
		Plugin:         buildPluginConfig(opts.Plugin, opts.PluginEnv, opts.AppName, opts.DestNamespace),
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

// buildPluginConfig converts domain plugin types to the manifest-package types,
// merging the plugin's base env with per-application overrides (right-wins).
// Returns nil when plugin is nil (no plugin configured → auto-detect renderer).
func buildPluginConfig(plugin *domain.Plugin, appEnv []domain.EnvVar, appName, namespace string) *manifest.PluginConfig {
	if plugin == nil {
		return nil
	}
	// Build merged env: base plugin env first, then per-app overrides.
	merged := make(map[string]string, len(plugin.Env)+len(appEnv))
	for _, e := range plugin.Env {
		merged[e.Name] = e.Value
	}
	for _, e := range appEnv {
		merged[e.Name] = e.Value
	}
	env := make([]manifest.PluginEnvVar, 0, len(merged))
	for k, v := range merged {
		env = append(env, manifest.PluginEnvVar{Name: k, Value: v})
	}
	return &manifest.PluginConfig{
		Command:   plugin.Generate.Command,
		Args:      plugin.Generate.Args,
		Env:       env,
		AppName:   appName,
		Namespace: namespace,
	}
}
