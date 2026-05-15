package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CommitInfo is a single entry for UI / API revision pickers.
type CommitInfo struct {
	SHA        string    `json:"sha"`
	ShortSHA   string    `json:"shortSha"`
	Message    string    `json:"message"`
	Author     string    `json:"author"`
	AuthorDate time.Time `json:"authorDate"`
}

// RecentCommits walks the history from HEAD limited to max entries,
// optionally restricted to commits that touched relPath (repository-relative).
func (c *Cache) RecentCommits(bareDir, relPath string, max int) ([]CommitInfo, error) {
	if max <= 0 || max > 200 {
		max = 50
	}
	repo, err := gogit.PlainOpen(bareDir)
	if err != nil {
		return nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	opts := &gogit.LogOptions{From: head.Hash()}
	if relPath != "" && relPath != "." && relPath != "/" {
		p := relPath
		opts.FileName = &p
	}
	iter, err := repo.Log(opts)
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []CommitInfo
	for len(out) < max {
		cmt, err := iter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		msg := cmt.Message
		if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
			msg = msg[:idx]
		}
		h := cmt.Hash.String()
		short := h
		if len(short) > 7 {
			short = short[:7]
		}
		out = append(out, CommitInfo{
			SHA:        h,
			ShortSHA:   short,
			Message:    msg,
			Author:     cmt.Author.Name,
			AuthorDate: cmt.Author.When.UTC(),
		})
	}
	return out, nil
}

// CommitByHash returns info for a single commit by its full or abbreviated SHA.
// Returns nil (no error) when the commit is not found.
func (c *Cache) CommitByHash(bareDir, sha string) (*CommitInfo, error) {
	repo, err := gogit.PlainOpen(bareDir)
	if err != nil {
		return nil, err
	}
	hash := plumbing.NewHash(sha)
	cmt, err := repo.CommitObject(hash)
	if err != nil {
		return nil, nil //nolint:nilerr // commit not found is not an error for callers
	}
	msg := cmt.Message
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = msg[:idx]
	}
	h := cmt.Hash.String()
	short := h
	if len(short) > 7 {
		short = short[:7]
	}
	return &CommitInfo{
		SHA:        h,
		ShortSHA:   short,
		Message:    msg,
		Author:     cmt.Author.Name,
		AuthorDate: cmt.Author.When.UTC(),
	}, nil
}

// DiffCommitRange returns unified diff for paths under relPath between two SHAs
// using the git CLI (works for bare repos).
func (c *Cache) DiffCommitRange(bareDir, relPath, fromSHA, toSHA string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := []string{"-C", bareDir, "diff", fromSHA, toSHA, "--"}
	if relPath != "" && relPath != "." && relPath != "/" {
		args = append(args, relPath)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
