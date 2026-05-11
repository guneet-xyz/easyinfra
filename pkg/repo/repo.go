// Package repo provides utilities for managing git repositories.
package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guneet-xyz/easyinfra/pkg/exec"
)

// Status holds information about the current state of the repo.
type Status struct {
	Commit string
	Origin string
	Dirty  bool
}

// Manager manages the cloned infra repository.
type Manager struct {
	Runner  exec.Runner
	RepoDir string
}

// Exists returns true if the repo directory contains a .git directory.
func (m *Manager) Exists() bool {
	_, err := os.Stat(filepath.Join(m.RepoDir, ".git"))
	return err == nil
}

// Clone clones gitURL to m.RepoDir.
// If m.RepoDir already exists and force is false, returns an error.
// If force is true, removes the existing directory first.
// If branch is non-empty, passes --branch to git clone.
func (m *Manager) Clone(ctx context.Context, gitURL, branch string, force bool) error {
	if m.Exists() {
		if !force {
			return fmt.Errorf("infra repo already exists at %s; use --force to overwrite, or run `easyinfra update` to pull latest", m.RepoDir)
		}
		if err := os.RemoveAll(m.RepoDir); err != nil {
			return fmt.Errorf("removing existing repo at %s: %w", m.RepoDir, err)
		}
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(m.RepoDir), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, gitURL, m.RepoDir)

	_, stderr, err := m.Runner.Run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("cloning %s: %w\n%s", gitURL, err, stderr)
	}
	return nil
}

// Pull runs git pull --ff-only in m.RepoDir.
func (m *Manager) Pull(ctx context.Context) error {
	if !m.Exists() {
		return fmt.Errorf("no infra repo at %s; run `easyinfra init <url>` first", m.RepoDir)
	}
	_, stderr, err := m.Runner.Run(ctx, "git", "-C", m.RepoDir, "pull", "--ff-only")
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if strings.Contains(msg, "not fast-forward") || strings.Contains(msg, "Not possible to fast-forward") {
			return fmt.Errorf("cannot fast-forward: local repo has diverged from remote\n%s", msg)
		}
		return fmt.Errorf("git pull failed: %w\n%s", err, msg)
	}
	return nil
}

// Status returns the current HEAD commit and remote origin URL.
func (m *Manager) Status(ctx context.Context) (*Status, error) {
	if !m.Exists() {
		return nil, fmt.Errorf("no infra repo at %s", m.RepoDir)
	}

	commit, _, err := m.Runner.Run(ctx, "git", "-C", m.RepoDir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	origin, _, _ := m.Runner.Run(ctx, "git", "-C", m.RepoDir, "remote", "get-url", "origin")

	return &Status{
		Commit: strings.TrimSpace(commit),
		Origin: strings.TrimSpace(origin),
	}, nil
}
