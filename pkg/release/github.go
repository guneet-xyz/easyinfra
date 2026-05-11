// Package release provides utilities for checking and managing software releases.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// Client queries the GitHub Releases API.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string // override for testing; defaults to https://api.github.com
	Owner      string
	Repo       string
	Token      string // optional; set for higher rate limits
}

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Assets      []Asset   `json:"assets"`
	PublishedAt time.Time `json:"published_at"`
}

// Asset represents a release asset (downloadable file).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// LatestRelease fetches the latest release from GitHub.
func (c *Client) LatestRelease(ctx context.Context) (*Release, error) {
	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", base, c.Owner, c.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		// OK
	case http.StatusNotFound:
		return nil, fmt.Errorf("no releases found for %s/%s", c.Owner, c.Repo)
	case http.StatusForbidden:
		resetTime := resp.Header.Get("X-RateLimit-Reset")
		return nil, fmt.Errorf("GitHub API rate limit exceeded (resets at unix timestamp %s); set GITHUB_TOKEN for higher limits", resetTime)
	default:
		return nil, fmt.Errorf("GitHub API returned status %d for %s", resp.StatusCode, url)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release response: %w", err)
	}
	return &release, nil
}

// FindAsset finds the release asset matching the given GOOS and GOARCH.
// Uses GoReleaser's default naming convention:
//   - linux/amd64  → easyinfra_Linux_x86_64.tar.gz
//   - linux/arm64  → easyinfra_Linux_arm64.tar.gz
//   - darwin/amd64 → easyinfra_Darwin_x86_64.tar.gz
//   - darwin/arm64 → easyinfra_Darwin_arm64.tar.gz
//   - windows/amd64 → easyinfra_Windows_x86_64.zip
func FindAsset(release *Release, goos, goarch string) (*Asset, error) {
	osTitle := strings.Title(goos) //nolint:staticcheck // Title is fine for OS names
	archStr := goarch
	if goarch == "amd64" {
		archStr = "x86_64"
	}

	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	wantName := fmt.Sprintf("easyinfra_%s_%s%s", osTitle, archStr, ext)

	for i := range release.Assets {
		if release.Assets[i].Name == wantName {
			return &release.Assets[i], nil
		}
	}

	// Build list of available asset names for error message.
	var names []string
	for _, a := range release.Assets {
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf("no asset found for %s/%s (looking for %q); available: %s",
		goos, goarch, wantName, strings.Join(names, ", "))
}
