// Package git is a small wrapper around go-git providing the operations the
// repo server needs: clone-or-fetch, resolve a symbolic ref to a SHA, and
// check out a SHA into a temp worktree.
package git

import (
	"crypto/sha1" //nolint:gosec // used only for stable cache filenames
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/orin/orin/internal/domain"
)

// Cache manages a disk cache of bare clones keyed by repository URL.
type Cache struct {
	root string
	mu   sync.Mutex
	locks map[string]*sync.Mutex
}

// NewCache constructs a Cache rooted at dir, creating it if necessary.
func NewCache(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{root: dir, locks: make(map[string]*sync.Mutex)}, nil
}

func (c *Cache) lockFor(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.locks[key]; ok {
		return l
	}
	l := &sync.Mutex{}
	c.locks[key] = l
	return l
}

// CloneOrFetch ensures a bare clone of repoURL exists in the cache and is
// up to date.
func (c *Cache) CloneOrFetch(repoURL string, creds *domain.RepoCreds) (string, error) {
	key := cacheKey(repoURL)
	l := c.lockFor(key)
	l.Lock()
	defer l.Unlock()

	dir := filepath.Join(c.root, key+".git")
	auth := authMethod(creds)

	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		_, err := gogit.PlainClone(dir, true, &gogit.CloneOptions{
			URL:        repoURL,
			Auth:       auth,
			NoCheckout: true,
		})
		if err != nil {
			return "", fmt.Errorf("clone: %w", err)
		}
		return dir, nil
	}

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("open cached: %w", err)
	}
	err = repo.Fetch(&gogit.FetchOptions{
		Auth:       auth,
		Force:      true,
		RefSpecs:   []config.RefSpec{"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*"},
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("fetch: %w", err)
	}
	return dir, nil
}

// Resolve returns the commit SHA for a symbolic revision (branch, tag, or
// already-a-sha).
func (c *Cache) Resolve(bareDir, revision string) (string, error) {
	repo, err := gogit.PlainOpen(bareDir)
	if err != nil {
		return "", err
	}
	if revision == "" || revision == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return "", err
		}
		return head.Hash().String(), nil
	}
	if h, err := repo.ResolveRevision(plumbing.Revision(revision)); err == nil {
		return h.String(), nil
	}
	// try as branch
	if ref, err := repo.Reference(plumbing.NewBranchReferenceName(revision), true); err == nil {
		return ref.Hash().String(), nil
	}
	if ref, err := repo.Reference(plumbing.NewTagReferenceName(revision), true); err == nil {
		return ref.Hash().String(), nil
	}
	return "", fmt.Errorf("cannot resolve revision %q", revision)
}

// Checkout extracts the working tree at the given SHA into a fresh temp dir.
// The caller must remove the returned dir when done.
func (c *Cache) Checkout(bareDir, sha string) (string, error) {
	tmp, err := os.MkdirTemp("", "k8sui-checkout-")
	if err != nil {
		return "", err
	}
	// Clone from the local bare repo to a new working tree at the requested SHA.
	wt, err := gogit.PlainClone(tmp, false, &gogit.CloneOptions{
		URL:      bareDir,
		NoCheckout: false,
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("clone for checkout: %w", err)
	}
	work, err := wt.Worktree()
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := work.Checkout(&gogit.CheckoutOptions{Hash: plumbing.NewHash(sha), Force: true}); err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("checkout %s: %w", sha, err)
	}
	return tmp, nil
}

func authMethod(creds *domain.RepoCreds) *httpauth.BasicAuth {
	if creds == nil || (creds.Username == "" && creds.Password == "") {
		return nil
	}
	user := creds.Username
	if user == "" {
		// Most providers accept any non-empty username with a PAT.
		user = "x-access-token"
	}
	return &httpauth.BasicAuth{Username: user, Password: creds.Password}
}

func cacheKey(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err == nil && u.Host != "" {
		hash := sha1.Sum([]byte(repoURL)) //nolint:gosec
		return u.Host + "_" + hex.EncodeToString(hash[:8])
	}
	hash := sha1.Sum([]byte(repoURL)) //nolint:gosec
	return hex.EncodeToString(hash[:])
}
