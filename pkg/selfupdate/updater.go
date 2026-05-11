// Package selfupdate provides utilities for self-updating the easyinfra binary.
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

	// detector is an optional injection point for testing. If nil, a real
	// go-selfupdate-backed detector is constructed at call time.
	detector releaseDetector
}

// CheckResult holds the result of a version check.
type CheckResult struct {
	LatestVersion string
	HasUpdate     bool
	DownloadURL   string
}

// releaseInfo abstracts a release returned by the upstream library so that
// tests can supply fakes without needing to construct a *selfupdate.Release
// (which has unexported fields).
type releaseInfo struct {
	Version     string
	AssetURL    string
	GreaterThan func(current string) bool
	// raw holds the underlying release for use by UpdateTo. It may be nil in
	// tests where UpdateTo is faked out.
	raw *selfupdate.Release
}

// releaseDetector is the small surface of the upstream library that Updater
// needs. It is satisfied by a real go-selfupdate-backed implementation in
// production, and can be faked in tests.
type releaseDetector interface {
	DetectLatest(ctx context.Context, owner, repo string) (*releaseInfo, bool, error)
	UpdateTo(ctx context.Context, info *releaseInfo, cmdPath string) error
}

// realDetector wraps the upstream go-selfupdate library.
type realDetector struct {
	updater *selfupdate.Updater
}

func newRealDetector() (*realDetector, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating GitHub source: %w", err)
	}
	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}
	return &realDetector{updater: updater}, nil
}

func (d *realDetector) DetectLatest(ctx context.Context, owner, repo string) (*releaseInfo, bool, error) {
	r := &GitHubRepository{Owner: owner, Repo: repo}
	latest, found, err := d.updater.DetectLatest(ctx, r)
	if err != nil {
		return nil, false, err
	}
	if !found || latest == nil {
		return nil, false, nil
	}
	return &releaseInfo{
		Version:     latest.Version(),
		AssetURL:    latest.AssetURL,
		GreaterThan: latest.GreaterThan,
		raw:         latest,
	}, true, nil
}

func (d *realDetector) UpdateTo(ctx context.Context, info *releaseInfo, cmdPath string) error {
	return d.updater.UpdateTo(ctx, info.raw, cmdPath)
}

// getDetector returns the injected detector or constructs a real one.
func (u *Updater) getDetector() (releaseDetector, error) {
	if u.detector != nil {
		return u.detector, nil
	}
	return newRealDetector()
}

// Check queries GitHub Releases for the latest version without applying any update.
func (u *Updater) Check(ctx context.Context) (*CheckResult, error) {
	detector, err := u.getDetector()
	if err != nil {
		return nil, err
	}

	latest, found, err := detector.DetectLatest(ctx, u.Owner, u.Repo)
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	if !found {
		return &CheckResult{LatestVersion: u.CurrentVersion, HasUpdate: false}, nil
	}

	hasUpdate := latest.GreaterThan(u.CurrentVersion)
	return &CheckResult{
		LatestVersion: latest.Version,
		HasUpdate:     hasUpdate,
		DownloadURL:   latest.AssetURL,
	}, nil
}

// Update downloads and applies the latest release, replacing the current binary.
// Returns the new version string.
func (u *Updater) Update(ctx context.Context) (string, error) {
	detector, err := u.getDetector()
	if err != nil {
		return "", err
	}

	latest, found, err := detector.DetectLatest(ctx, u.Owner, u.Repo)
	if err != nil {
		return "", fmt.Errorf("detecting latest release: %w", err)
	}
	if !found || !latest.GreaterThan(u.CurrentVersion) {
		return u.CurrentVersion, nil // already up to date
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding current executable: %w", err)
	}

	if err := detector.UpdateTo(ctx, latest, exe); err != nil {
		return "", fmt.Errorf("applying update: %w", err)
	}

	return latest.Version, nil
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
