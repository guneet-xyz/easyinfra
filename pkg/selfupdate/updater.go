package selfupdate

import (
	"context"
	"fmt"
	"os"

	"github.com/creativeprojects/go-selfupdate"
)

// Updater handles self-upgrade of the easyinfra binary.
type Updater struct {
	Owner          string
	Repo           string
	CurrentVersion string
}

// CheckResult holds the result of a version check.
type CheckResult struct {
	LatestVersion string
	HasUpdate     bool
	DownloadURL   string
}

// Check queries GitHub Releases for the latest version without applying any update.
func (u *Updater) Check(ctx context.Context) (*CheckResult, error) {
	// Create a GitHub source with default config
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating GitHub source: %w", err)
	}

	// Create updater with the GitHub source
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}

	// Create a repository reference
	repo := &GitHubRepository{
		Owner: u.Owner,
		Repo:  u.Repo,
	}

	// Detect latest release
	latest, found, err := updater.DetectLatest(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	if !found {
		return &CheckResult{LatestVersion: u.CurrentVersion, HasUpdate: false}, nil
	}

	hasUpdate := latest.GreaterThan(u.CurrentVersion)
	return &CheckResult{
		LatestVersion: latest.Version(),
		HasUpdate:     hasUpdate,
		DownloadURL:   latest.AssetURL,
	}, nil
}

// Update downloads and applies the latest release, replacing the current binary.
// Returns the new version string.
func (u *Updater) Update(ctx context.Context) (string, error) {
	// Create a GitHub source with default config
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return "", fmt.Errorf("creating GitHub source: %w", err)
	}

	// Create updater with the GitHub source
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
	})
	if err != nil {
		return "", fmt.Errorf("creating updater: %w", err)
	}

	// Create a repository reference
	repo := &GitHubRepository{
		Owner: u.Owner,
		Repo:  u.Repo,
	}

	// Detect latest release
	latest, found, err := updater.DetectLatest(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("detecting latest release: %w", err)
	}
	if !found || !latest.GreaterThan(u.CurrentVersion) {
		return u.CurrentVersion, nil // already up to date
	}

	// Get the current executable path
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding current executable: %w", err)
	}

	// Apply the update
	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return "", fmt.Errorf("applying update: %w", err)
	}

	return latest.Version(), nil
}

// GitHubRepository implements the Repository interface for GitHub.
type GitHubRepository struct {
	Owner string
	Repo  string
}

// GetSlug returns the owner and repo name.
func (r *GitHubRepository) GetSlug() (string, string, error) {
	return r.Owner, r.Repo, nil
}

// Get returns the underlying repository object (not used for GitHub).
func (r *GitHubRepository) Get() (interface{}, error) {
	return nil, nil
}
